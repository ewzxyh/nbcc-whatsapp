package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

const testEventID = "11111111-1111-4111-8111-111111111111"

func signedWebhook(secret, body string, stamp time.Time) *http.Request {
	r := httptest.NewRequest("POST", "/webhooks/grapnel", strings.NewReader(body))
	ts := strconv.FormatInt(stamp.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + body))
	r.Header.Set("X-Grapnel-Timestamp", ts)
	r.Header.Set("X-Grapnel-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	r.Header.Set("X-Grapnel-Event-Id", testEventID)
	return r
}

func commandBody(group string, fromMe bool) string {
	b, _ := json.Marshal(map[string]any{"id": testEventID, "event": "whatsapp.group.message.received", "instance_id": testInstance, "message": map[string]any{"messageid": "abc", "chatid": group, "text": " #NBCC ", "isGroup": true, "fromMe": fromMe}})
	return string(b)
}

func (h *testHarness) webhook(body string, stamp time.Time) *httptest.ResponseRecorder {
	h.t.Helper()
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, signedWebhook(testSecret, body, stamp))
	return w
}

func TestWebhookDisabledNeverSendsOrReplaysAfterEnable(t *testing.T) {
	h := newHarness(t)
	if w := h.webhook(commandBody(testGroup, false), h.clock.now()); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	h.settingsPatch(map[string]any{"sendEnabled": true})
	if err := h.app.processPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if posts, _ := h.up.snapshot(); posts != 0 {
		t.Fatal("disabled command was replayed after enabling")
	}
}

func TestWebhookFreshFeedDuplicateAndRestart(t *testing.T) {
	h := newHarness(t)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	for i := 0; i < 2; i++ {
		if w := h.webhook(commandBody(testGroup, false), h.clock.now()); w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if posts, fetches := h.up.snapshot(); posts != 0 || fetches != 0 {
		t.Fatal("webhook waited for external processing before acknowledgement")
	}
	if err := h.app.processPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if posts, fetches := h.up.snapshot(); posts != 1 || fetches != 1 {
		t.Fatal("command did not fetch/send exactly once", posts, fetches)
	}
	s, err := openStore(h.app.store.path)
	if err != nil {
		t.Fatal(err)
	}
	h.app.store = s
	h.webhook(commandBody(testGroup, false), h.clock.now())
	h.app.processPending(context.Background())
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("duplicate after restart")
	}
}

func TestWebhookAuthenticationAndScope(t *testing.T) {
	h := newHarness(t)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	for _, tc := range []struct {
		name, body   string
		stamp        time.Time
		badSignature bool
		want         int
	}{
		{"signature", commandBody(testGroup, false), h.clock.now(), true, 401},
		{"replay", commandBody(testGroup, false), h.clock.now().Add(-6 * time.Minute), false, 401},
		{"other group", commandBody("123456789@g.us", false), h.clock.now(), false, 202},
		{"missing direction", strings.Replace(commandBody(testGroup, false), `"fromMe":false,`, "", 1), h.clock.now(), false, 202},
		{"other text", strings.Replace(commandBody(testGroup, false), "#NBCC", "hello", 1), h.clock.now(), false, 202},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := signedWebhook(testSecret, tc.body, tc.stamp)
			if tc.badSignature {
				r.Header.Set("X-Grapnel-Signature", "sha256=bad")
			}
			w := httptest.NewRecorder()
			h.handler.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
	h.app.processPending(context.Background())
	if posts, _ := h.up.snapshot(); posts != 0 {
		t.Fatal("out-of-scope webhook sent a message")
	}
}

func TestWebhookWithoutConfiguredSecretFailsClosed(t *testing.T) {
	h := newHarness(t)
	h.settingsPatch(map[string]any{"sendEnabled": true, "webhookSecret": nil})
	// Signed with the empty key an attacker could compute once the secret is cleared.
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/webhooks/grapnel", strings.NewReader(commandBody(testGroup, false)))
	ts := strconv.FormatInt(h.clock.now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(""))
	mac.Write([]byte(ts + "." + commandBody(testGroup, false)))
	r.Header.Set("X-Grapnel-Timestamp", ts)
	r.Header.Set("X-Grapnel-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	r.Header.Set("X-Grapnel-Event-Id", testEventID)
	h.handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("webhook accepted without a configured secret", w.Code)
	}
}

func TestMonitorBaselineChangeAndDisabledGuard(t *testing.T) {
	h := newHarness(t)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	p, err := testPreview([]byte(testICS(testEvent("due", "Lab07 - Due", ";VALUE=DATE:20261014", ""))), h.clock.now())
	if err != nil {
		t.Fatal(err)
	}
	changed, queued, err := h.app.store.observe(p, true, deliveryTarget(h.app.settings.current(), h.app.settings.key), h.clock.now())
	if err != nil || changed || !queued {
		t.Fatal("activation must queue the initial agenda")
	}
	changed, queued, err = h.app.store.observe(p, true, deliveryTarget(h.app.settings.current(), h.app.settings.key), h.clock.now())
	if err != nil || changed || queued {
		t.Fatal("unchanged feed queued a send")
	}
	p.FeedHash = "changed"
	changed, queued, err = h.app.store.observe(p, true, deliveryTarget(h.app.settings.current(), h.app.settings.key), h.clock.now())
	if err != nil || !changed || !queued {
		t.Fatal("change was not queued")
	}
	h.settingsPatch(map[string]any{"sendEnabled": false})
	if err = h.app.processPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if posts, _ := h.up.snapshot(); posts != 0 {
		t.Fatal("send guard failed")
	}
}

func TestRetryUsesExactlyTheSameKeyAndBody(t *testing.T) {
	h := newHarness(t)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	var keys, bodies []string
	h.app.client.Transport = testTransport(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		bodies = append(bodies, string(body))
		if len(keys) == 1 {
			return testResponse(503, `{"code":"busy"}`), nil
		}
		return testResponse(202, `{}`), nil
	})
	if err := h.app.store.enqueue("event:test", "fixed message", true, h.clock.now()); err != nil {
		t.Fatal(err)
	}
	h.app.processPending(context.Background())
	h.clock.advance(10 * time.Minute)
	h.app.processPending(context.Background())
	if len(keys) != 2 || keys[0] != keys[1] || bodies[0] != bodies[1] {
		t.Fatal("retry changed idempotency key/body")
	}
}

func TestCorruptStateFailsClosed(t *testing.T) {
	h := newHarness(t)
	if err := writeFileForTest(h.app.store.path, "not JSON"); err != nil {
		t.Fatal(err)
	}
	if _, err := openStore(h.app.store.path); err == nil {
		t.Fatal("corrupt state was accepted")
	}
}

func TestLongAgendaSplitsWithoutLosingContent(t *testing.T) {
	text := strings.Repeat("• Linux I — Lab: due at 23:59.\n", 200) + strings.Repeat("📚", 3000)
	parts := messageParts(text)
	if len(parts) < 2 {
		t.Fatal("long agenda was not split")
	}
	for _, p := range parts {
		if len(utf16.Encode([]rune(p))) > 4096 {
			t.Fatal("WhatsApp text limit exceeded")
		}
	}
	if strings.ReplaceAll(strings.Join(parts, ""), "\n", "") != strings.ReplaceAll(text, "\n", "") {
		t.Fatal("content lost while splitting")
	}
}

func TestRetryDoesNotResendAcceptedParts(t *testing.T) {
	h := newHarness(t)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	var keys []string
	h.app.client.Transport = testTransport(func(r *http.Request) (*http.Response, error) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if len(keys) == 2 {
			return testResponse(503, `{}`), nil
		}
		return testResponse(202, `{}`), nil
	})
	h.app.store.enqueue("event:long", strings.Repeat("a", 5000), true, h.clock.now())
	h.app.processPending(context.Background())
	h.clock.advance(10 * time.Minute)
	h.app.processPending(context.Background())
	if len(keys) != 3 || keys[0] == keys[1] || keys[1] != keys[2] {
		t.Fatal("accepted message part was resent or retry key changed", keys)
	}
}

// pollTickerEvents records every ticker the worker creates.
func runWorkerWithTickerLog(t *testing.T, h *testHarness) <-chan time.Duration {
	t.Helper()
	events := make(chan time.Duration, 16)
	h.app.onPollTicker = func(d time.Duration) { events <- d }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.app.runWorker(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return events
}

func expectTicker(t *testing.T, events <-chan time.Duration, want time.Duration) {
	t.Helper()
	select {
	case got := <-events:
		if got != want {
			t.Fatalf("ticker interval %s, want %s", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("no ticker created; wanted %s", want)
	}
}

func TestWorkerRecreatesPollTickerWhenIntervalChanges(t *testing.T) {
	h := newHarness(t)
	events := runWorkerWithTickerLog(t, h)
	expectTicker(t, events, 5*time.Minute)
	// A change that does not touch the interval must not recreate the ticker.
	h.settingsPatch(map[string]any{"agendaCacheMinutes": 30})
	h.settingsPatch(map[string]any{"pollIntervalMinutes": 2})
	expectTicker(t, events, 2*time.Minute)
	h.settingsPatch(map[string]any{"pollIntervalMinutes": 7})
	expectTicker(t, events, 7*time.Minute)
}

const uiContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

func checkUIHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	for name, want := range map[string]string{"Content-Security-Policy": uiContentSecurityPolicy, "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer", "X-Frame-Options": "DENY"} {
		if got := w.Header().Get(name); got != want {
			t.Fatalf("%s = %q on %d response", name, got, w.Code)
		}
	}
}

func uiGet(h *testHarness, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, httptest.NewRequest("GET", target, nil))
	return w
}

func TestStaticUIServesBuildWithSecurityHeaders(t *testing.T) {
	h := newHarness(t)
	for _, target := range []string{"/", "/index.html"} {
		w := uiGet(h, target)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Sample UI") || w.Header().Get("Cache-Control") != "no-cache" {
			t.Fatal(target, w.Code, w.Header())
		}
		checkUIHeaders(t, w)
	}
	w := uiGet(h, "/assets/app-1a2b.js")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatal(w.Code, w.Header())
	}
	checkUIHeaders(t, w)
	w = uiGet(h, "/favicon.ico")
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	checkUIHeaders(t, w)
}

func TestStaticUIRejectsTraversalDirectoriesAndAPIPaths(t *testing.T) {
	h := newHarness(t)
	for _, target := range []string{"/../main.go", "/%2e%2e/main.go", "/..%2fmain.go", "/assets/../index.html", "/assets/", "/assets", "/index.html/", "/.env", "/api/nope", "/api", "/webhooks/nope"} {
		if w := uiGet(h, target); w.Code != 404 {
			t.Fatal(target, w.Code, w.Body.String())
		}
	}
}

func TestAdminStaticServesIndexAndKeepsAssetsAndAPIOut(t *testing.T) {
	h := newHarness(t)
	for _, target := range []string{"/admin", "/admin/", "/admin/entrar", "/admin/settings/x"} {
		w := uiGet(h, target)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Sample Admin") || w.Header().Get("Cache-Control") != "no-cache" {
			t.Fatal(target, w.Code, w.Header())
		}
		checkUIHeaders(t, w)
	}
	for _, target := range []string{"/admin/assets/app-1a2b.js", "/admin/assets", "/admin/assets/"} {
		if w := uiGet(h, target); w.Code != 404 || strings.Contains(w.Body.String(), "Sample Admin") {
			t.Fatal(target, w.Code)
		}
	}
	for _, target := range []string{"/api/admin/nope", "/api/admin"} {
		w := uiGet(h, target)
		if w.Code != 404 || strings.Contains(w.Body.String(), "Sample Admin") {
			t.Fatal(target, w.Code, w.Body.String())
		}
	}
	if w := uiGet(h, "/api/admin/session"); !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("admin API must answer as JSON, never static", w.Header())
	}
}

func TestAdminStaticMissingIndexIsNotFound(t *testing.T) {
	h := newHarnessWeb(t, true, false)
	if w := uiGet(h, "/admin"); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w := uiGet(h, "/"); w.Code != 200 {
		t.Fatal("public UI must still serve", w.Code)
	}
}

func TestRemovedAndRoutedEndpoints(t *testing.T) {
	h := newHarness(t)
	for target, want := range map[string]int{"/webhooks/grapnel": 405, "/healthz": 200, "/api/preview": 404, "/api/group": 404, "/api/admin/preview": 401} {
		if w := uiGet(h, target); w.Code != want {
			t.Fatal("GET", target, w.Code, w.Body.String())
		}
	}
	for _, target := range []string{"/api/check", "/api/preview", "/api/group"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", target, nil)
		r.Header.Set("Authorization", "Bearer "+testSecret)
		h.handler.ServeHTTP(w, r)
		if w.Code != 404 {
			t.Fatal("POST", target, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, httptest.NewRequest("POST", "/api/admin/check", nil))
	if w.Code != 401 || errorCode(t, w) != "unauthenticated" {
		t.Fatal("POST /api/admin/check without session", w.Code, w.Body.String())
	}
	if w := uiGet(h, "/healthz"); !strings.Contains(w.Body.String(), `"send_enabled":false`) {
		t.Fatal(w.Body.String())
	}
}

func TestMissingIndexReturnsPlain404AndAPIStillWorks(t *testing.T) {
	h := newHarnessWeb(t, false, false)
	w := uiGet(h, "/")
	if w.Code != 404 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") || w.Body.String() != "404 page not found\n" {
		t.Fatal(w.Code, w.Header(), w.Body.String())
	}
	if w := uiGet(h, "/healthz"); w.Code != 200 {
		t.Fatal("healthz", w.Code)
	}
}

func TestCourseInstructorsParsing(t *testing.T) {
	got, err := parseCourseInstructors(" Sample I = Sample Instructor ; Sample II=Second Sample Instructor;")
	if err != nil || len(got) != 2 || got["Sample I"] != "Sample Instructor" || got["Sample II"] != "Second Sample Instructor" {
		t.Fatal(got, err)
	}
	if got, err := parseCourseInstructors(""); err != nil || len(got) != 0 {
		t.Fatal("empty setting must be accepted", got, err)
	}
	for _, bad := range []string{"Sample I", "=Sample Instructor", "Sample I=", "Sample I=Sample Instructor;Sample I=Other Instructor", "Sample I=Sample Instructor;;Sample II"} {
		_, err := parseCourseInstructors(bad)
		if err == nil {
			t.Fatal("malformed setting accepted:", bad)
		}
		if strings.Contains(err.Error(), "Sample") {
			t.Fatal("error message echoes configuration:", err)
		}
	}
}

func TestLoadConfigDefaultsAndInfrastructureOverrides(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("STATE_PATH", "")
	t.Setenv("WEB_DIR", "")
	c := loadConfig()
	if c.ListenAddr != ":8080" || c.StatePath != "data/state.json" || c.WebDir != "web/dist" {
		t.Fatal("defaults", c)
	}
	t.Setenv("WEB_DIR", "/srv/sample-ui")
	t.Setenv("STATE_PATH", "/srv/sample-data/state.json")
	t.Setenv("LISTEN_ADDR", "127.0.0.1:18080")
	c = loadConfig()
	if c.WebDir != "/srv/sample-ui" || c.StatePath != "/srv/sample-data/state.json" || c.ListenAddr != "127.0.0.1:18080" {
		t.Fatal("overrides", c)
	}
}
