package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type App struct {
	cfg               Config
	client            *http.Client
	store             *Store
	settings          *SettingsStore
	admin             *AdminStore
	audit             *auditLog
	sessions          *sessionStore
	limiter           *attemptLimiter
	status            *runtimeStatus
	now               func() time.Time
	feed              feedCache
	workMu            sync.Mutex
	monitorRaw        []byte
	monitorTarget     string
	monitorDay        string
	lastHistoryAt     time.Time
	lastHistoryReadAt time.Time
	// onPollTicker is a test hook, called whenever the worker creates its poll ticker.
	onPollTicker func(time.Duration)
}

// runtimeStatus holds the most recent outcomes shown in the admin status. Only generic codes are stored.
type runtimeStatus struct {
	mu            sync.Mutex
	lastCheckAt   time.Time
	feedErr       string
	sendErr       string
	agendaOmitted []omittedEvent // omissions of the most recent default-window build
	agendaErr     string
}

func (r *runtimeStatus) recordCheck(at time.Time, code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastCheckAt = at
	r.feedErr = code
}

func (r *runtimeStatus) recordSend(code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sendErr = code
}

func (r *runtimeStatus) snapshot() (time.Time, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	code := r.feedErr
	if code == "" {
		code = r.sendErr
	}
	return r.lastCheckAt, code
}

// fetchFeed returns the raw upstream feed. Errors are generic and never include the feed URL.
func (a *App) fetchFeed(ctx context.Context, feedURL string) ([]byte, error) {
	if feedURL == "" {
		return nil, errors.New("NBCC feed is not configured")
	}
	r, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, errors.New("invalid NBCC feed URL")
	}
	r.Header.Set("Accept", "text/calendar")
	res, err := a.client.Do(r)
	if err != nil {
		return nil, errors.New("NBCC feed network error")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("NBCC feed is unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, (8<<20)+1))
	if err != nil || len(raw) > 8<<20 {
		return nil, errors.New("NBCC feed cannot be read or is too large")
	}
	return raw, nil
}

func (a *App) preview(ctx context.Context, s Settings) (Preview, error) {
	raw, err := a.fetchFeed(ctx, s.NBCCFeedURL)
	if err != nil {
		return Preview{}, err
	}
	return buildPreview(raw, a.now(), a.settings.key)
}

func jsonResponse(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

var uiSecurityHeaders = map[string]string{
	"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
	"X-Content-Type-Options":  "nosniff",
	"Referrer-Policy":         "no-referrer",
	"X-Frame-Options":         "DENY",
}

func setUISecurityHeaders(w http.ResponseWriter) {
	for name, value := range uiSecurityHeaders {
		w.Header().Set(name, value)
	}
}

// serveStatic serves one regular file from files. Only regular files are served; there are no directory listings.
func serveStatic(w http.ResponseWriter, r *http.Request, files fs.FS, name, cacheControl string) {
	if !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}
	f, err := files.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	content, seekable := f.(io.ReadSeeker)
	if err != nil || info.IsDir() || !seekable {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", cacheControl)
	http.ServeContent(w, r, name, info.ModTime(), content)
}

// ui serves the built public React app from WEB_DIR.
func (a *App) ui() http.Handler {
	files := os.DirFS(a.cfg.WebDir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setUISecurityHeaders(w)
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		cache := "no-cache"
		if strings.HasPrefix(name, "assets/") {
			cache = "public, max-age=31536000, immutable"
		}
		serveStatic(w, r, files, name, cache)
	})
}

// adminUI serves admin/index.html for /admin and every /admin/... path except /admin/assets/.
func (a *App) adminUI() http.Handler {
	files := os.DirFS(a.cfg.WebDir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setUISecurityHeaders(w)
		rest := strings.TrimPrefix(r.URL.Path, "/admin")
		if rest == "/assets" || strings.HasPrefix(rest, "/assets/") {
			http.NotFound(w, r)
			return
		}
		serveStatic(w, r, files, "admin/index.html", "no-cache")
	})
}

func isAPIPath(p string) bool {
	return p == "/api" || strings.HasPrefix(p, "/api/") || p == "/webhooks" || strings.HasPrefix(p, "/webhooks/")
}

func isAdminPath(p string) bool {
	return p == "/admin" || strings.HasPrefix(p, "/admin/")
}

func (a *App) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"status": "ok", "send_enabled": a.settings.current().SendEnabled})
	})
	mux.HandleFunc("GET /api/agenda", a.agenda)
	mux.HandleFunc("POST /webhooks/grapnel", a.webhook)

	mux.HandleFunc("GET /api/admin/session", a.adminSessionState)
	mux.HandleFunc("POST /api/admin/setup", publicPost(a.setup))
	mux.HandleFunc("POST /api/admin/login", publicPost(a.login))
	mux.HandleFunc("POST /api/admin/logout", a.requireSession(a.logout))
	mux.HandleFunc("POST /api/admin/session/extend", a.requireSession(a.extendSession))
	mux.HandleFunc("GET /api/admin/status", a.requireSession(a.adminStatus))
	mux.HandleFunc("GET /api/admin/settings", a.requireSession(a.getSettings))
	mux.HandleFunc("PUT /api/admin/settings", a.requireSession(a.putSettings))
	mux.HandleFunc("POST /api/admin/settings/test-feed", a.requireSession(a.testFeed))
	mux.HandleFunc("POST /api/admin/settings/test-grapnel", a.requireSession(a.testGrapnel))
	mux.HandleFunc("POST /api/admin/webhook/register", a.requireSession(a.registerWebhookAdmin))
	mux.HandleFunc("GET /api/admin/courses", a.requireSession(a.courses))
	mux.HandleFunc("POST /api/admin/check", a.requireSession(a.adminCheck))
	mux.HandleFunc("GET /api/admin/preview", a.requireSession(a.adminPreview))
	mux.HandleFunc("GET /api/admin/activity", a.requireSession(a.activity))
	mux.HandleFunc("POST /api/admin/account/email", a.requireSession(a.changeEmail))
	mux.HandleFunc("POST /api/admin/password", a.requireSession(a.changePassword))
	mux.HandleFunc("POST /api/admin/sessions/revoke-others", a.requireSession(a.revokeOthers))

	ui := a.ui()
	adminUI := a.adminUI()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/admin") {
			w.Header().Set("Cache-Control", "no-store")
		}
		// Unmatched GET/HEAD requests outside /api and /webhooks go to the static UI; all others keep the mux's 404/405.
		if _, pattern := mux.Handler(r); pattern == "" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && !isAPIPath(r.URL.Path) {
			if isAdminPath(r.URL.Path) {
				adminUI.ServeHTTP(w, r)
				return
			}
			ui.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (a *App) webhook(w http.ResponseWriter, r *http.Request) {
	s := a.settings.current()
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
	// With no secret configured, every signature is refused; an empty HMAC key must never verify.
	if s.WebhookSecret == "" {
		jsonResponse(w, 401, map[string]string{"error": "invalid signature"})
		return
	}
	mac := hmac.New(sha256.New, []byte(s.WebhookSecret))
	mac.Write([]byte(ts + "." + string(body)))
	signature, err := hex.DecodeString(strings.TrimPrefix(r.Header.Get("X-Grapnel-Signature"), "sha256="))
	if !strings.HasPrefix(r.Header.Get("X-Grapnel-Signature"), "sha256=") || err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		jsonResponse(w, 401, map[string]string{"error": "invalid signature"})
		return
	}
	var event struct {
		ID         string       `json:"id"`
		Event      string       `json:"event"`
		InstanceID string       `json:"instance_id"`
		Message    groupMessage `json:"message"`
	}
	if json.Unmarshal(body, &event) != nil || !uuidPattern.MatchString(event.ID) || event.ID != r.Header.Get("X-Grapnel-Event-Id") {
		jsonResponse(w, 400, map[string]string{"error": "invalid webhook event"})
		return
	}
	match := event.Event == "whatsapp.group.message.received" && s.GrapnelInstanceID != "" && event.InstanceID == s.GrapnelInstanceID && s.GrapnelGroupJID != "" && event.Message.ChatID == s.GrapnelGroupJID && event.Message.IsGroup && event.Message.FromMe != nil && event.Message.ID != "" && len(event.Message.ID) <= 255 && event.Message.Timestamp >= 0 && event.Message.Timestamp <= a.now().Add(time.Minute).UnixMilli() && strings.EqualFold(strings.TrimSpace(event.Message.Text), "#NBCC")
	if match {
		// Persist before acknowledgement. Disabled commands are suppressed, never replayed later.
		if err = a.command(event.Message, s); err != nil {
			jsonResponse(w, 503, map[string]string{"error": "cannot persist webhook"})
			return
		}
	}
	jsonResponse(w, 202, map[string]any{"received": true, "command_matched": match, "send_enabled": s.SendEnabled})
}

// runWorker polls the feed and drains the job queue. The poll ticker is recreated when pollIntervalMinutes changes.
func (a *App) runWorker(ctx context.Context) {
	jobs := time.NewTicker(time.Second)
	defer jobs.Stop()
	interval := a.settings.current().pollInterval()
	poll := time.NewTicker(interval)
	defer func() { poll.Stop() }()
	a.pollTickerCreated(interval)
	check := func() {
		if !a.settings.current().feedConfigured() {
			a.status.recordCheck(a.now(), "not_configured")
			return
		}
		changed, queued, err := a.check(ctx)
		if err != nil {
			log.Printf("feed check: %v", err)
		} else if changed {
			log.Printf("feed changed; queued=%t send_enabled=%t", queued, a.settings.current().SendEnabled)
		}
	}
	check()
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.settings.changedCh():
			check()
			if next := a.settings.current().pollInterval(); next != interval {
				poll.Stop()
				interval = next
				poll = time.NewTicker(interval)
				a.pollTickerCreated(interval)
			}
		case <-poll.C:
			check()
		case <-jobs.C:
			if err := a.schedule(ctx); err != nil {
				log.Printf("schedule: %v", err)
			}
			if err := a.pollCommands(ctx); err != nil {
				log.Printf("group history: %v", err)
			}
			if err := a.processPending(ctx); err != nil {
				log.Printf("queue: %v", err)
			}
		}
	}
}

func (a *App) pollTickerCreated(d time.Duration) {
	if a.onPollTicker != nil {
		a.onPollTicker(d)
	}
}
