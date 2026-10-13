package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Admin API handlers. Every response is JSON with Cache-Control: no-store (set by the router).
// Responses never contain secret values, feed URLs, raw upstream error text or IP addresses.

type secretView struct {
	Secret     bool `json:"secret"`
	Configured bool `json:"configured"`
}

// settingsView is the browser shape of Settings. Secret fields expose only whether they are configured.
type settingsView struct {
	NBCCFeedURL         secretView        `json:"nbccFeedUrl"`
	GrapnelAPIKey       secretView        `json:"grapnelApiKey"`
	WebhookSecret       secretView        `json:"webhookSecret"`
	GrapnelInstanceID   string            `json:"grapnelInstanceId"`
	GrapnelGroupJID     string            `json:"grapnelGroupJid"`
	GrapnelGroupName    string            `json:"grapnelGroupName"`
	PublicBaseURL       string            `json:"publicBaseUrl"`
	SendEnabled         bool              `json:"sendEnabled"`
	PollIntervalMinutes int               `json:"pollIntervalMinutes"`
	AgendaCacheMinutes  int               `json:"agendaCacheMinutes"`
	CourseInstructors   map[string]string `json:"courseInstructors"`
}

func viewOf(s Settings) settingsView {
	return settingsView{
		NBCCFeedURL:         secretView{Secret: true, Configured: s.NBCCFeedURL != ""},
		GrapnelAPIKey:       secretView{Secret: true, Configured: s.GrapnelAPIKey != ""},
		WebhookSecret:       secretView{Secret: true, Configured: s.WebhookSecret != ""},
		GrapnelInstanceID:   s.GrapnelInstanceID,
		GrapnelGroupJID:     s.GrapnelGroupJID,
		GrapnelGroupName:    s.GrapnelGroupName,
		PublicBaseURL:       s.PublicBaseURL,
		SendEnabled:         s.SendEnabled,
		PollIntervalMinutes: s.PollIntervalMinutes,
		AgendaCacheMinutes:  s.AgendaCacheMinutes,
		CourseInstructors:   s.CourseInstructors,
	}
}

func rfc3339(t time.Time) string { return t.In(moncton).Format(time.RFC3339) }

func serverError(w http.ResponseWriter) {
	jsonResponse(w, 500, map[string]string{"error": "internal"})
}

func notConfigured(w http.ResponseWriter) {
	jsonResponse(w, 409, map[string]string{"error": "not_configured"})
}

func validationFailed(w http.ResponseWriter, fields map[string]string) {
	jsonResponse(w, 422, map[string]any{"error": "validation", "fields": fields})
}

// readJSON decodes a bounded JSON body. A malformed body is answered with 400 and false is returned.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil || json.Unmarshal(body, v) != nil {
		jsonResponse(w, 400, map[string]string{"error": "bad_request"})
		return false
	}
	return true
}

// issueSession creates a session, sets its cookie and writes the authenticated body shared by setup and login.
func (a *App) issueSession(w http.ResponseWriter, r *http.Request, status int) {
	now := a.now()
	token, s := a.sessions.create(now)
	setSessionCookie(w, r, token)
	acct, _ := a.admin.account()
	jsonResponse(w, status, map[string]string{"state": "authenticated", "email": acct.Email, "csrfToken": s.csrf, "expiresAt": rfc3339(s.expiresAt())})
}

// adminSessionState reports setup, login or authenticated. It is the only admin endpoint that needs no session.
func (a *App) adminSessionState(w http.ResponseWriter, r *http.Request) {
	acct, configured := a.admin.account()
	if !configured {
		jsonResponse(w, 200, map[string]string{"state": "setup", "email": ""})
		return
	}
	s, err := a.sessionFor(r)
	if err != nil {
		jsonResponse(w, 200, map[string]string{"state": "login", "email": ""})
		return
	}
	jsonResponse(w, 200, map[string]string{"state": "authenticated", "email": acct.Email, "csrfToken": s.csrf, "expiresAt": rfc3339(s.expiresAt())})
}

func (a *App) setup(w http.ResponseWriter, r *http.Request) {
	if a.admin.configured() {
		jsonResponse(w, 409, map[string]string{"error": "already_configured"})
		return
	}
	ip := clientIP(r, a.cfg.TrustedProxyIP)
	var body struct {
		SetupCode string `json:"setupCode"`
		Email     string `json:"email"`
		Password  string `json:"password"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if ok, wait := a.limiter.verify(ip, a.now, func() bool { return a.admin.checkSetupCode(body.SetupCode) }); !ok {
		if wait > 0 {
			tooManyAttempts(w, wait)
			return
		}
		jsonResponse(w, 401, map[string]string{"error": "invalid_setup_code"})
		return
	}
	email := a.admin.fixedEmail
	if body.Email != "" && strings.ToLower(strings.TrimSpace(body.Email)) != email {
		validationFailed(w, map[string]string{"email": "fixed_admin_email"})
		return
	}
	if code := passwordCode(body.Password, email); code != "" {
		validationFailed(w, map[string]string{"password": code})
		return
	}
	hash, err := hashPassword(body.Password)
	if err != nil {
		serverError(w)
		return
	}
	if err = a.admin.create(hash, a.now()); err != nil {
		if errors.Is(err, errAlreadyConfigured) {
			jsonResponse(w, 409, map[string]string{"error": "already_configured"})
			return
		}
		serverError(w)
		return
	}
	a.audit.record(a.now(), "setup.completed")
	a.issueSession(w, r, 201)
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r, a.cfg.TrustedProxyIP)
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if ok, wait := a.limiter.verify(ip, a.now, func() bool {
		valid := a.admin.verify(body.Email, body.Password)
		if !valid {
			a.audit.record(a.now(), "login.failed")
		}
		return valid
	}); !ok {
		if wait > 0 {
			tooManyAttempts(w, wait)
			return
		}
		jsonResponse(w, 401, map[string]string{"error": "invalid_credentials"})
		return
	}
	a.audit.record(a.now(), "login.succeeded")
	a.issueSession(w, r, 200)
}

func (a *App) logout(w http.ResponseWriter, r *http.Request, s adminSession) {
	a.sessions.remove(s.key)
	clearSessionCookie(w, r)
	a.audit.record(a.now(), "logout")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) extendSession(w http.ResponseWriter, r *http.Request, s adminSession) {
	now := a.now()
	updated, ok := a.sessions.extend(s.key, now)
	if !ok {
		clearSessionCookie(w, r)
		jsonResponse(w, 401, map[string]string{"error": "session_expired"})
		return
	}
	jsonResponse(w, 200, map[string]string{"expiresAt": rfc3339(updated.expiresAt())})
}

type statusAgenda struct {
	UpdatedAt     *string        `json:"updatedAt"`
	Stale         bool           `json:"stale"`
	CacheMinutes  int            `json:"cacheMinutes"`
	OmittedEvents []omittedEvent `json:"omittedEvents"`
	LastError     *string        `json:"lastError"`
}

type statusWhatsApp struct {
	SendEnabled bool    `json:"sendEnabled"`
	PendingJobs int     `json:"pendingJobs"`
	FailedJobs  int     `json:"failedJobs"`
	LastCheckAt *string `json:"lastCheckAt"`
	LastError   *string `json:"lastError"`
}

type statusConfigured struct {
	NBCCFeed      bool `json:"nbccFeed"`
	Grapnel       bool `json:"grapnel"`
	WebhookSecret bool `json:"webhookSecret"`
	PublicBaseURL bool `json:"publicBaseUrl"`
}

// agendaCopy reports the cached upstream copy's fetch time, if it belongs to the configured feed URL.
func (a *App) agendaCopy(s Settings) (time.Time, bool) {
	a.feed.mu.Lock()
	defer a.feed.mu.Unlock()
	if a.feed.raw == nil || a.feed.source != sha256.Sum256([]byte(s.NBCCFeedURL)) {
		return time.Time{}, false
	}
	return a.feed.fetchedAt, true
}

func (a *App) adminStatus(w http.ResponseWriter, r *http.Request, _ adminSession) {
	now := a.now()
	s := a.settings.current()
	omitted, agendaErr := a.status.agendaSnapshot()
	ag := statusAgenda{CacheMinutes: s.AgendaCacheMinutes, OmittedEvents: omitted, LastError: nullable(agendaErr)}
	if fetchedAt, ok := a.agendaCopy(s); ok {
		ag.UpdatedAt = ptr(rfc3339(fetchedAt))
		ag.Stale = !now.Before(fetchedAt.Add(s.agendaTTL()))
	}
	checkAt, waErr := a.status.snapshot()
	pending, failed := a.store.counts()
	wa := statusWhatsApp{SendEnabled: s.SendEnabled, PendingJobs: pending, FailedJobs: failed, LastError: nullable(waErr)}
	if !checkAt.IsZero() {
		wa.LastCheckAt = ptr(rfc3339(checkAt))
	}
	jsonResponse(w, 200, map[string]any{
		"agenda":   ag,
		"whatsapp": wa,
		"configured": statusConfigured{
			NBCCFeed:      s.feedConfigured(),
			Grapnel:       s.grapnelSendReady(),
			WebhookSecret: s.WebhookSecret != "",
			PublicBaseURL: s.PublicBaseURL != "",
		},
	})
}

func (a *App) getSettings(w http.ResponseWriter, r *http.Request, _ adminSession) {
	jsonResponse(w, 200, viewOf(a.settings.current()))
}

func (a *App) putSettings(w http.ResponseWriter, r *http.Request, _ adminSession) {
	var raw map[string]json.RawMessage
	if !readJSON(w, r, &raw) {
		return
	}
	changed, err := a.settings.update(raw)
	var verr *validationError
	if errors.As(err, &verr) {
		validationFailed(w, verr.Fields)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	if len(changed) > 0 {
		a.audit.record(a.now(), "settings.updated", changed...)
	}
	jsonResponse(w, 200, viewOf(a.settings.current()))
}

// testFeed fetches the saved feed and reports counts for the default window. It does not touch the agenda cache.
func (a *App) testFeed(w http.ResponseWriter, r *http.Request, _ adminSession) {
	s := a.settings.current()
	if !s.feedConfigured() {
		notConfigured(w)
		return
	}
	raw, err := a.fetchFeed(r.Context(), s.NBCCFeedURL)
	if err != nil {
		jsonResponse(w, 502, map[string]string{"error": "feed_unavailable"})
		return
	}
	from, to := defaultWindow(dayOf(a.now()))
	items, omitted, err := buildAgenda(raw, dayStart(from), dayStart(to.AddDate(0, 0, 1)), s.CourseInstructors, a.settings.key)
	if err != nil {
		jsonResponse(w, 502, map[string]string{"error": "feed_unavailable"})
		return
	}
	courses := map[string]bool{}
	for _, it := range items {
		if it.Type == "activity" && it.Course != nil {
			courses[*it.Course] = true
		}
	}
	jsonResponse(w, 200, map[string]any{"ok": true, "events": len(items), "courses": len(courses), "omittedEvents": len(omitted)})
}

// testGrapnel runs group discovery only. It never sends a message.
func (a *App) testGrapnel(w http.ResponseWriter, r *http.Request, _ adminSession) {
	s := a.settings.current()
	if !s.grapnelDiscoveryReady() {
		notConfigured(w)
		return
	}
	g, err := a.discover(r.Context(), s)
	if err != nil {
		jsonResponse(w, 502, map[string]string{"error": "grapnel_unavailable"})
		return
	}
	jsonResponse(w, 200, map[string]any{"ok": true, "groupName": g.Name})
}

func (a *App) registerWebhookAdmin(w http.ResponseWriter, r *http.Request, _ adminSession) {
	s := a.settings.current()
	if s.PublicBaseURL == "" || s.WebhookSecret == "" || !s.grapnelDiscoveryReady() {
		notConfigured(w)
		return
	}
	destination := s.PublicBaseURL + "/webhooks/grapnel"
	err := a.registerWebhook(r.Context(), s, destination)
	if errors.Is(err, errWebhookExists) {
		jsonResponse(w, 409, map[string]string{"error": "webhook_already_configured"})
		return
	}
	if err != nil {
		jsonResponse(w, 502, map[string]string{"error": "grapnel_unavailable"})
		return
	}
	a.audit.record(a.now(), "webhook.registered")
	jsonResponse(w, 200, map[string]any{"ok": true, "url": destination})
}

type courseRow struct {
	Code       string  `json:"code"`
	Title      *string `json:"title"`
	Instructor *string `json:"instructor"`
	Activities int     `json:"activities"`
}

// courses lists the courses in the cached feed, looking one year either side of today.
func (a *App) courses(w http.ResponseWriter, r *http.Request, _ adminSession) {
	raw, _, err := a.agendaFeed(r.Context())
	if err != nil {
		jsonResponse(w, 502, map[string]string{"error": "feed_unavailable"})
		return
	}
	today := dayOf(a.now())
	items, _, err := buildAgenda(raw, dayStart(today.AddDate(0, 0, -365)), dayStart(today.AddDate(0, 0, 366)), a.settings.current().CourseInstructors, a.settings.key)
	if err != nil {
		jsonResponse(w, 502, map[string]string{"error": "feed_unavailable"})
		return
	}
	rows := map[string]*courseRow{}
	activityIDs := map[string]map[string]bool{}
	for _, it := range items {
		if it.Type != "activity" || it.Course == nil {
			continue
		}
		code := *it.Course
		row := rows[code]
		if row == nil {
			row = &courseRow{Code: code}
			rows[code] = row
			activityIDs[code] = map[string]bool{}
		}
		if row.Title == nil {
			row.Title = it.CourseTitle
		}
		if row.Instructor == nil {
			row.Instructor = it.Instructor
		}
		if it.ActivityID != nil && !activityIDs[code][*it.ActivityID] {
			activityIDs[code][*it.ActivityID] = true
			row.Activities++
		}
	}
	out := make([]courseRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	jsonResponse(w, 200, out)
}

func (a *App) adminCheck(w http.ResponseWriter, r *http.Request, _ adminSession) {
	s := a.settings.current()
	if !s.feedConfigured() {
		notConfigured(w)
		return
	}
	changed, queued, err := a.check(r.Context())
	if err != nil {
		jsonResponse(w, 502, map[string]string{"error": "feed_unavailable"})
		return
	}
	jsonResponse(w, 200, map[string]any{"feedChanged": changed, "queued": queued, "sendEnabled": s.SendEnabled})
}

func (a *App) adminPreview(w http.ResponseWriter, r *http.Request, _ adminSession) {
	s := a.settings.current()
	if !s.feedConfigured() {
		notConfigured(w)
		return
	}
	p, err := a.preview(r.Context(), s)
	if err != nil {
		jsonResponse(w, 502, map[string]string{"error": "feed_unavailable"})
		return
	}
	jsonResponse(w, 200, map[string]any{"text": p.Text, "activities": p.Count, "from": p.From, "through": p.Through})
}

func (a *App) activity(w http.ResponseWriter, r *http.Request, _ adminSession) {
	jsonResponse(w, 200, a.audit.newestFirst())
}

func (a *App) changeEmail(w http.ResponseWriter, r *http.Request, _ adminSession) {
	jsonResponse(w, 403, map[string]string{"error": "fixed_admin_email"})
}

// changePassword verifies the current password, applies the policy, rotates this session's token and revokes every other session.
func (a *App) changePassword(w http.ResponseWriter, r *http.Request, s adminSession) {
	ip := clientIP(r, a.cfg.TrustedProxyIP)
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	acct, configured := a.admin.account()
	if !configured {
		jsonResponse(w, 401, map[string]string{"error": "unauthenticated"})
		return
	}
	if ok, wait := a.limiter.verify(ip, a.now, func() bool { return verifyPassword(body.CurrentPassword, acct.PasswordHash) }); !ok {
		if wait > 0 {
			tooManyAttempts(w, wait)
			return
		}
		jsonResponse(w, 401, map[string]string{"error": "invalid_credentials"})
		return
	}
	if code := passwordCode(body.NewPassword, acct.Email); code != "" {
		validationFailed(w, map[string]string{"newPassword": code})
		return
	}
	hash, err := hashPassword(body.NewPassword)
	if err != nil {
		serverError(w)
		return
	}
	if err = a.admin.update(func(x *adminAccount) { x.PasswordHash = hash }); err != nil {
		serverError(w)
		return
	}
	token, next, ok := a.sessions.rotateAndRevoke(s.key, a.now())
	if !ok {
		jsonResponse(w, 401, map[string]string{"error": "unauthenticated"})
		return
	}
	setSessionCookie(w, r, token)
	a.audit.record(a.now(), "password.changed")
	jsonResponse(w, 200, map[string]string{"csrfToken": next.csrf, "expiresAt": rfc3339(next.expiresAt())})
}

func (a *App) revokeOthers(w http.ResponseWriter, r *http.Request, s adminSession) {
	a.sessions.revokeOthers(s.key)
	a.audit.record(a.now(), "sessions.revoked")
	w.WriteHeader(http.StatusNoContent)
}
