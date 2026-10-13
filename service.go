package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type App struct {
	cfg    Config
	client *http.Client
	store  *Store
	now    func() time.Time
}

func (a *App) preview(ctx context.Context) (Preview, error) {
	r, err := http.NewRequestWithContext(ctx, "GET", a.cfg.FeedURL, nil)
	if err != nil {
		return Preview{}, errors.New("invalid NBCC feed URL")
	}
	r.Header.Set("Accept", "text/calendar")
	res, err := a.client.Do(r)
	if err != nil {
		return Preview{}, errors.New("NBCC feed network error")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Preview{}, errors.New("NBCC feed is unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, (8<<20)+1))
	if err != nil || len(raw) > 8<<20 {
		return Preview{}, errors.New("NBCC feed cannot be read or is too large")
	}
	return buildPreview(raw, a.now())
}

func jsonResponse(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func (a *App) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+a.cfg.AdminToken)) != 1 {
			jsonResponse(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (a *App) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"status": "ok", "send_enabled": a.cfg.SendEnabled})
	})
	mux.HandleFunc("GET /api/preview", a.admin(func(w http.ResponseWriter, r *http.Request) {
		p, err := a.preview(r.Context())
		if err != nil {
			jsonResponse(w, 502, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, 200, p)
	}))
	mux.HandleFunc("GET /api/group", a.admin(func(w http.ResponseWriter, r *http.Request) {
		g, err := a.discover(r.Context())
		if err != nil {
			jsonResponse(w, 502, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, 200, g)
	}))
	mux.HandleFunc("POST /api/check", a.admin(func(w http.ResponseWriter, r *http.Request) {
		changed, queued, err := a.check(r.Context())
		if err != nil {
			jsonResponse(w, 502, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, 200, map[string]any{"feed_changed": changed, "queued": queued, "send_enabled": a.cfg.SendEnabled})
	}))
	mux.HandleFunc("POST /webhooks/grapnel", a.webhook)
	return mux
}

func (a *App) webhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonResponse(w, 413, map[string]string{"error": "payload too large"})
		return
	}
	ts := r.Header.Get("X-Grapnel-Timestamp")
	stamp, err := strconv.ParseInt(ts, 10, 64)
	diff := a.now().Sub(time.Unix(stamp, 0))
	if err != nil || diff > 5*time.Minute || diff < -5*time.Minute {
		jsonResponse(w, 401, map[string]string{"error": "expired signature"})
		return
	}
	mac := hmac.New(sha256.New, []byte(a.cfg.WebhookSecret))
	mac.Write([]byte(ts + "." + string(body)))
	signature, err := hex.DecodeString(strings.TrimPrefix(r.Header.Get("X-Grapnel-Signature"), "sha256="))
	if !strings.HasPrefix(r.Header.Get("X-Grapnel-Signature"), "sha256=") || err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		jsonResponse(w, 401, map[string]string{"error": "invalid signature"})
		return
	}
	var event struct {
		ID         string `json:"id"`
		Event      string `json:"event"`
		InstanceID string `json:"instance_id"`
		Message    struct {
			ID      string `json:"messageid"`
			ChatID  string `json:"chatid"`
			Text    string `json:"text"`
			IsGroup bool   `json:"isGroup"`
			FromMe  *bool  `json:"fromMe"`
		} `json:"message"`
	}
	if json.Unmarshal(body, &event) != nil || !uuidPattern.MatchString(event.ID) || event.ID != r.Header.Get("X-Grapnel-Event-Id") {
		jsonResponse(w, 400, map[string]string{"error": "invalid webhook event"})
		return
	}
	match := event.Event == "whatsapp.group.message.received" && event.InstanceID == a.cfg.InstanceID && event.Message.ChatID == a.cfg.GroupJID && event.Message.IsGroup && event.Message.FromMe != nil && !*event.Message.FromMe && event.Message.ID != "" && strings.EqualFold(strings.TrimSpace(event.Message.Text), "#NBCC")
	if match {
		// Persist before acknowledgement. Disabled commands are suppressed, never replayed later.
		if err = a.store.enqueue("event:"+event.ID, "", a.cfg.SendEnabled, a.now()); err != nil {
			jsonResponse(w, 503, map[string]string{"error": "cannot persist webhook"})
			return
		}
	}
	jsonResponse(w, 202, map[string]any{"received": true, "command_matched": match, "send_enabled": a.cfg.SendEnabled})
}

func (a *App) check(ctx context.Context) (bool, bool, error) {
	p, err := a.preview(ctx)
	if err != nil {
		return false, false, err
	}
	return a.store.observe(p, a.cfg.SendEnabled, a.now())
}

func (a *App) processPending(ctx context.Context) error {
	if !a.cfg.SendEnabled {
		return nil
	}
	for _, job := range a.store.pending(a.now()) {
		if a.now().Sub(job.CreatedAt) > 23*time.Hour {
			job.Status = "failed"
			job.Text = ""
			if err := a.store.update(job, a.now()); err != nil {
				return err
			}
			continue
		}
		var err error
		if job.Text == "" {
			var p Preview
			p, err = a.preview(ctx)
			if err == nil {
				job.Text = p.Text
				if err = a.store.update(job, a.now()); err != nil {
					return err
				}
			}
		}
		if err == nil {
			parts := messageParts(job.Text)
			for job.NextPart < len(parts) {
				err = a.sendText(ctx, parts[job.NextPart], job.ID+":p"+strconv.Itoa(job.NextPart))
				if err != nil {
					break
				}
				job.NextPart++
				if err = a.store.update(job, a.now()); err != nil {
					return err
				}
			}
		}
		job.Attempts++
		if err == nil {
			job.Status = "accepted"
			job.Text = ""
		} else {
			var apiErr *apiError
			permanent := errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 && apiErr.Status != 408 && apiErr.Status != 409 && apiErr.Status != 429
			if job.Attempts >= 8 || permanent {
				job.Status = "failed"
				job.Text = ""
			} else {
				job.NextAttempt = a.now().Add(time.Minute * time.Duration(1<<min(job.Attempts, 5)))
				if apiErr != nil && apiErr.RetryAfter > 0 {
					job.NextAttempt = a.now().Add(time.Duration(min(apiErr.RetryAfter, 3600)) * time.Second)
				}
			}
			log.Printf("job %s: %v", job.ID, err)
		}
		if err = a.store.update(job, a.now()); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) runWorker(ctx context.Context) {
	poll := time.NewTicker(a.cfg.PollInterval)
	defer poll.Stop()
	jobs := time.NewTicker(time.Second)
	defer jobs.Stop()
	check := func() {
		changed, queued, err := a.check(ctx)
		if err != nil {
			log.Printf("feed check: %v", err)
		} else if changed {
			log.Printf("feed changed; queued=%t send_enabled=%t", queued, a.cfg.SendEnabled)
		}
	}
	check()
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			check()
		case <-jobs.C:
			if err := a.processPending(ctx); err != nil {
				log.Printf("queue: %v", err)
			}
		}
	}
}
