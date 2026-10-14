package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"
)

func textHash(text string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))))
	return hex.EncodeToString(h[:])
}

func deliveryTarget(s Settings, key []byte) string {
	return agendaHash(key, "whatsapp-target", s.NBCCFeedURL, s.GrapnelInstanceID, s.GrapnelGroupJID)
}

func (a *App) command(m groupMessage, s Settings) error {
	at := a.now()
	if m.Timestamp > 0 {
		at = time.UnixMilli(m.Timestamp)
	}
	target := deliveryTarget(s, a.settings.key)
	current := a.settings.current()
	if deliveryTarget(current, a.settings.key) != target {
		return nil
	}
	id := "command:" + agendaHash(a.settings.key, target, m.ID)
	return a.store.command(id, current.SendEnabled, target, at, a.now())
}

// Serialize monitor checks and queue processing so an older fetch cannot replace newer state.
func (a *App) check(ctx context.Context) (bool, bool, error) {
	a.workMu.Lock()
	defer a.workMu.Unlock()
	return a.checkLocked(ctx)
}

func (a *App) checkLocked(ctx context.Context) (bool, bool, error) {
	s, revision := a.settings.deliverySnapshot()
	raw, err := a.fetchFeed(ctx, s.NBCCFeedURL)
	var p Preview
	if err == nil {
		p, err = buildPreview(raw, a.now(), a.settings.key, s.PublicBaseURL, a.store.activityVersions(deliveryTarget(s, a.settings.key)))
	}
	code := ""
	if err != nil {
		code = "feed_unavailable"
	}
	a.status.recordCheck(a.now(), code)
	if err != nil {
		return false, false, err
	}
	current, currentRevision := a.settings.deliverySnapshot()
	if revision != currentRevision {
		return false, false, errors.New("settings changed during feed check")
	}
	target := deliveryTarget(current, a.settings.key)
	a.monitorRaw, a.monitorTarget, a.monitorDay = raw, target, p.From
	return a.store.observe(p, current.SendEnabled, target, a.now())
}

// Deadlines and the moving calendar window are checked independently of the feed poll interval.
func (a *App) schedule(ctx context.Context) error {
	a.workMu.Lock()
	defer a.workMu.Unlock()
	s := a.settings.current()
	if !s.SendEnabled {
		return a.store.pause(a.now())
	}
	target := deliveryTarget(s, a.settings.key)
	if a.monitorTarget != target || a.monitorDay != midnight(a.now()).Format(dateLayout) {
		if last, _ := a.status.snapshot(); a.now().Sub(last) < time.Minute {
			return nil
		}
		if _, _, err := a.checkLocked(ctx); err != nil {
			return err
		}
	}
	p, err := buildPreview(a.monitorRaw, a.now(), a.settings.key, s.PublicBaseURL, a.store.activityVersions(target))
	if err != nil {
		return err
	}
	if _, _, err = a.store.observe(p, true, target, a.now()); err != nil {
		return err
	}
	if err = a.store.queueReminders(p, target, a.now()); err != nil {
		return err
	}
	return a.store.observeCampus(campusPreview(p, s.PublicBaseURL), target, a.now())
}

// The provider webhook excludes fromMe messages; history polling also covers commands from our number.
func (a *App) pollCommands(ctx context.Context) error {
	a.workMu.Lock()
	defer a.workMu.Unlock()
	s := a.settings.current()
	now := a.now()
	if !s.SendEnabled || !s.grapnelSendReady() || now.Sub(a.lastHistoryAt) < time.Minute {
		return nil
	}
	since := a.store.commandSince()
	if since.IsZero() {
		return nil
	}
	cutoff := a.lastHistoryReadAt.Add(-5 * time.Minute)
	if cutoff.Before(since) {
		cutoff = since
	}
	a.lastHistoryAt = now
	for offset := 0; ; {
		page, err := a.groupMessages(ctx, s, 100, offset)
		if err != nil {
			return err
		}
		older := false
		for _, m := range page.Messages {
			at := time.UnixMilli(m.Timestamp)
			if m.Timestamp == 0 {
				continue
			}
			if at.Before(cutoff) {
				older = true
				continue
			}
			if at.After(now.Add(time.Minute)) {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(m.Text), "#NBCC") {
				if err = a.command(m, s); err != nil {
					return err
				}
			}
		}
		if older || !page.Pagination.HasMore {
			a.lastHistoryReadAt = now
			return nil
		}
		offset = page.Pagination.NextOffset
	}
}

func (a *App) recentAgenda(ctx context.Context, s Settings, p Preview) (bool, error) {
	same, hash := a.store.recent(p.FeedHash, deliveryTarget(s, a.settings.key), p.isCampus)
	if !same {
		return false, nil
	}
	page, err := a.groupMessages(ctx, s, 5, 0)
	if err != nil {
		return false, err
	}
	for _, m := range page.Messages {
		if *m.FromMe && textHash(m.Text) == hash {
			return true, nil
		}
	}
	return false, nil
}

func (a *App) processPending(ctx context.Context) error {
	a.workMu.Lock()
	defer a.workMu.Unlock()
	s, revision := a.settings.deliverySnapshot()
	if !s.SendEnabled {
		return a.store.pause(a.now())
	}
	jobs := a.store.pending(a.now())
	if len(jobs) == 0 {
		return nil
	}
	var agendaPreview Preview
	var previewErr error
	for _, j := range jobs {
		if j.Kind != "" {
			_, _, previewErr = a.checkLocked(ctx)
			if previewErr == nil {
				agendaPreview, previewErr = buildPreview(a.monitorRaw, a.now(), a.settings.key, s.PublicBaseURL, a.store.activityVersions(deliveryTarget(s, a.settings.key)))
				if previewErr == nil {
					previewErr = a.store.queueReminders(agendaPreview, deliveryTarget(s, a.settings.key), a.now())
				}
				if previewErr == nil {
					previewErr = a.store.observeCampus(campusPreview(agendaPreview, s.PublicBaseURL), deliveryTarget(s, a.settings.key), a.now())
				}
			}
			break
		}
	}
	if previewErr == nil {
		jobs = a.store.pending(a.now())
	}
	target := deliveryTarget(s, a.settings.key)
	for _, job := range jobs {
		p := agendaPreview
		if job.Kind == "campus" {
			p = campusPreview(agendaPreview, s.PublicBaseURL)
		}
		current, currentRevision := a.settings.deliverySnapshot()
		if !current.SendEnabled || revision != currentRevision {
			return a.store.pause(a.now())
		}
		if a.store.currentJob(job.ID).Status != "pending" {
			continue
		}
		if a.now().Sub(job.CreatedAt) > 23*time.Hour || !job.ExpiresAt.IsZero() && !a.now().Before(job.ExpiresAt) || job.Kind != "" && job.Target != target {
			job.Status, job.Text = "superseded", ""
			if err := a.store.update(job, a.now()); err != nil {
				return err
			}
			continue
		}
		err := previewErr
		failureCode := "feed_unavailable"
		if job.Kind == "" {
			err = nil
		}
		if err == nil && job.Kind == "reminder" {
			plan := reminderPlan(p, a.now())
			for _, key := range job.ReminderKeys {
				if _, ok := plan[key]; !ok {
					job.Status, job.Text = "superseded", ""
					break
				}
			}
			if job.Status != "pending" {
				if err = a.store.update(job, a.now()); err != nil {
					return err
				}
				continue
			}
		}
		if err == nil && job.Kind != "" && job.NextPart == 0 {
			var recent bool
			recent, err = a.recentAgenda(ctx, s, p)
			failureCode = "grapnel_unavailable"
			if recent {
				job.Status, job.Text = "suppressed", ""
				if err = a.store.update(job, a.now()); err != nil {
					return err
				}
				continue
			}
		}
		if err == nil && job.Kind != "" && job.Text == "" {
			p, err = buildPreview(a.monitorRaw, a.now(), a.settings.key, s.PublicBaseURL, a.store.activityVersions(target))
			if err == nil && job.Kind == "campus" {
				p = campusPreview(p, s.PublicBaseURL)
			}
			if err == nil {
				job.Text, job.FeedHash = p.Text, p.FeedHash
				err = a.store.update(job, a.now())
				if err != nil {
					return err
				}
			}
		}
		parts := messageParts(job.Text)
		if err == nil {
			failureCode = "grapnel_unavailable"
			for job.NextPart < len(parts) {
				current, currentRevision = a.settings.deliverySnapshot()
				if !current.SendEnabled || revision != currentRevision || a.store.currentJob(job.ID).Status != "pending" || job.Kind != "" && !a.now().Before(p.validUntil) {
					job.Status, job.Text = "superseded", ""
					break
				}
				err = a.sendText(ctx, s, parts[job.NextPart], job.ID+":p"+strconv.Itoa(job.NextPart))
				if err != nil {
					break
				}
				job.NextPart++
				if err = a.store.update(job, a.now()); err != nil {
					return err
				}
			}
		}
		if job.Status != "pending" {
			if err = a.store.update(job, a.now()); err != nil {
				return err
			}
			continue
		}
		current, currentRevision = a.settings.deliverySnapshot()
		if !current.SendEnabled || revision != currentRevision {
			job.Status, job.Text = "superseded", ""
			if err = a.store.update(job, a.now()); err != nil {
				return err
			}
			continue
		}
		job.Attempts++
		if err == nil && len(parts) > 0 {
			a.status.recordSend("")
			if err = a.store.complete(job, p, target, parts[len(parts)-1], a.now()); err != nil {
				return err
			}
			continue
		}
		if err == nil {
			err = errors.New("empty WhatsApp message")
		}
		var apiErr *apiError
		permanent := errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 && apiErr.Status != 408 && apiErr.Status != 409 && apiErr.Status != 429
		a.status.recordSend(failureCode)
		if job.Attempts >= 8 || permanent {
			job.Status, job.Text = "failed", ""
		} else {
			job.NextAttempt = a.now().Add(time.Minute * time.Duration(1<<min(job.Attempts, 5)))
			if apiErr != nil && apiErr.RetryAfter > 0 {
				job.NextAttempt = a.now().Add(time.Duration(min(apiErr.RetryAfter, 3600)) * time.Second)
			}
		}
		log.Printf("job %s: %v", job.ID, err)
		if err = a.store.update(job, a.now()); err != nil {
			return err
		}
	}
	return nil
}
