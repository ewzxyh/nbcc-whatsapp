package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// Synthetic fixtures only. The SENTINEL strings let tests prove that secrets never reach responses, logs or plaintext files.
const (
	testFeedURL       = "https://elearning.nbcc.ca/d2l/le/calendar/feed/user/feed.ics?token=SENTINEL-feed-token-4f2e"
	testInstance      = "11111111-2222-4333-8444-555555555555"
	testGroup         = "120363000000000000@g.us"
	testAPIKey        = "SENTINEL-grapnel-key-7d1f"
	testSecret        = "SENTINEL-webhook-secret-9a2c-0123456789abcdef"
	testPassword      = "sample-password-2026"
	testRemote        = "192.0.2.10:50000"
	testHost          = "nbcc.example.test"
	testSetupCodeRe   = `admin setup code: ([A-Z2-7]{4}-[A-Z2-7]{4}-[A-Z2-7]{4})`
	testGroupsDefault = `{"groups":[{"jid":"` + testGroup + `","name":"  NBCC Study Group","owner_can_send_message":true}]}`
)

// fakeClock is safe for use from the worker goroutine and the test goroutine.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// testTransport lets a test replace the HTTP round trip for one client.
type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

// fakeUpstream plays the NBCC feed and the Grapnel API.
type fakeUpstream struct {
	mu                sync.Mutex
	feed              string
	feedFail          bool
	feedHits          int
	groups            string
	webhook           string // JSON returned for GET .../groups/webhook; empty means none configured
	webhookRegistered int
	posts             int
	keys              []string
	messages          []groupMessage
	sent              []string
	hook              func(*http.Request) (*http.Response, bool)
}

// snapshot returns the number of Grapnel sends and NBCC feed fetches so far.
func (u *fakeUpstream) snapshot() (posts, feedHits int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.posts, u.feedHits
}

func writeFileForTest(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

func readFileForTest(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

func (u *fakeUpstream) RoundTrip(r *http.Request) (*http.Response, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.hook != nil {
		if res, ok := u.hook(r); ok {
			return res, nil
		}
	}
	if r.URL.Host == "elearning.nbcc.ca" {
		u.feedHits++
		if u.feedFail {
			return testResponse(500, "upstream failure"), nil
		}
		return testResponse(200, u.feed), nil
	}
	if strings.HasSuffix(r.URL.Path, "/groups/webhook") {
		if r.Method == http.MethodPost {
			u.webhookRegistered++
			u.webhook = `{"webhook":{"url":"https://nbcc.example.test/webhooks/grapnel"}}`
			return testResponse(200, `{}`), nil
		}
		if u.webhook != "" {
			return testResponse(200, u.webhook), nil
		}
		return testResponse(200, `{"webhook":null}`), nil
	}
	if strings.HasSuffix(r.URL.Path, "/groups") {
		return testResponse(200, u.groups), nil
	}
	if strings.HasSuffix(r.URL.Path, "/groups/messages") {
		messages := u.messages
		if messages == nil {
			messages = []groupMessage{}
		}
		b, _ := json.Marshal(map[string]any{"messages": messages, "pagination": map[string]any{"has_more": false}})
		return testResponse(200, string(b)), nil
	}
	if r.Method == http.MethodPost {
		u.posts++
		u.keys = append(u.keys, r.Header.Get("Idempotency-Key"))
		var body struct {
			Text string `json:"text"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		u.sent = append(u.sent, body.Text)
		return testResponse(202, `{"accepted":true}`), nil
	}
	return testResponse(404, ""), nil
}

// syncBuffer collects log output from any goroutine.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func captureLog(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	prev := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return buf
}

// writeWebDir creates a temporary WEB_DIR with a public page, an admin page and one hashed asset.
func writeWebDir(t *testing.T, withIndex, withAdmin bool) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if withIndex {
		write("index.html", "<!doctype html><title>Sample UI</title>")
	}
	if withAdmin {
		write("admin/index.html", "<!doctype html><title>Sample Admin</title>")
	}
	write("assets/app-1a2b.js", "console.log('sample')")
	write("api/nope", "not an API response")
	return dir
}

// testSettingsSeed is the synthetic starting configuration: every secret is configured, sending is off.
func testSettingsSeed() Settings {
	s := defaultSettings()
	s.NBCCFeedURL = testFeedURL
	s.GrapnelAPIKey = testAPIKey
	s.GrapnelInstanceID = testInstance
	s.GrapnelGroupJID = testGroup
	s.WebhookSecret = testSecret
	s.PublicBaseURL = "https://nbcc.example.test"
	return s
}

// testHarness is a complete App over a temporary state directory, a fake clock and a fake upstream.
type testHarness struct {
	t       *testing.T
	app     *App
	handler http.Handler
	clock   *fakeClock
	up      *fakeUpstream
	dir     string
	logs    *syncBuffer
}

func newHarness(t *testing.T) *testHarness {
	t.Helper()
	return newHarnessWeb(t, true, true)
}

// newHarnessWeb is newHarness with control over which static files exist in WEB_DIR.
func newHarnessWeb(t *testing.T, withIndex, withAdmin bool) *testHarness {
	t.Helper()
	logs := captureLog(t)
	dir := t.TempDir()
	clock := &fakeClock{t: testNow()}
	up := &fakeUpstream{feed: testICS(testEvent("due", "Lab07 - Due", ";TZID=America/Halifax:20261013T235959", "")), groups: testGroupsDefault}
	cfg := Config{ListenAddr: ":0", StatePath: filepath.Join(dir, "state.json"), WebDir: writeWebDir(t, withIndex, withAdmin)}
	a := newApp(cfg, &http.Client{Transport: up})
	a.now = clock.now
	var err error
	if a.settings, err = openSettings(dir, testSettingsSeed); err != nil {
		t.Fatal(err)
	}
	if a.admin, err = openAdmin(dir, defaultAdminEmail); err != nil {
		t.Fatal(err)
	}
	if a.audit, err = openAudit(dir); err != nil {
		t.Fatal(err)
	}
	if a.store, err = openStore(cfg.StatePath); err != nil {
		t.Fatal(err)
	}
	return &testHarness{t: t, app: a, handler: a.handler(), clock: clock, up: up, dir: dir, logs: logs}
}

// sess is a browser session: the cookie value and the CSRF token returned with it.
type sess struct {
	cookie string
	csrf   string
}

type call struct {
	method, target string
	body           any
	s              *sess
	header         http.Header
	remote         string
	tls            bool
}

func (h *testHarness) send(c call) *httptest.ResponseRecorder {
	h.t.Helper()
	var rd io.Reader
	if c.body != nil {
		b, err := json.Marshal(c.body)
		if err != nil {
			h.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	r := httptest.NewRequest(c.method, c.target, rd)
	r.Host = testHost
	if c.body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	r.RemoteAddr = c.remote
	if r.RemoteAddr == "" {
		r.RemoteAddr = testRemote
	}
	if c.tls {
		r.TLS = &tls.ConnectionState{}
	}
	if c.s != nil {
		if c.s.cookie != "" {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: c.s.cookie})
		}
		if c.s.csrf != "" {
			r.Header.Set("X-CSRF-Token", c.s.csrf)
		}
	}
	for k, vs := range c.header {
		r.Header[k] = vs
	}
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)
	return w
}

// setupAdmin runs first-time setup with the code from the log and returns the new session.
func (h *testHarness) setupAdmin(password string) (*httptest.ResponseRecorder, *sess) {
	h.t.Helper()
	m := regexp.MustCompile(testSetupCodeRe).FindAllStringSubmatch(h.logs.String(), -1)
	if len(m) == 0 {
		h.t.Fatal("setup code was not logged")
	}
	code := m[len(m)-1][1]
	w := h.send(call{method: "POST", target: "/api/admin/setup", body: map[string]string{"setupCode": code, "email": defaultAdminEmail, "password": password}})
	return w, sessionOf(h.t, w)
}

// login posts the credentials and returns the response and the session it created (nil on failure).
func (h *testHarness) login(email, password string) (*httptest.ResponseRecorder, *sess) {
	h.t.Helper()
	w := h.send(call{method: "POST", target: "/api/admin/login", body: map[string]string{"email": email, "password": password}})
	if w.Code != http.StatusOK {
		return w, nil
	}
	return w, sessionOf(h.t, w)
}

// sessionOf extracts the session cookie and CSRF token from a successful auth response.
func sessionOf(t *testing.T, w *httptest.ResponseRecorder) *sess {
	t.Helper()
	var cookie string
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName && c.MaxAge >= 0 {
			cookie = c.Value
		}
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	csrf, _ := body["csrfToken"].(string)
	if cookie == "" {
		t.Fatalf("no session cookie in response %d: %s", w.Code, w.Body.String())
	}
	return &sess{cookie: cookie, csrf: csrf}
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %d %q: %v", w.Code, w.Body.String(), err)
	}
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return body.Error
}

// settingsPatch applies a partial update through the same path the admin API uses.
func (h *testHarness) settingsPatch(patch map[string]any) {
	h.t.Helper()
	raw := map[string]json.RawMessage{}
	for k, v := range patch {
		b, err := json.Marshal(v)
		if err != nil {
			h.t.Fatal(err)
		}
		raw[k] = b
	}
	if _, err := h.app.settings.update(raw); err != nil {
		h.t.Fatal(err)
	}
}

// adminSession returns a live authenticated session for a fresh harness.
func (h *testHarness) adminSession() *sess {
	h.t.Helper()
	w, s := h.setupAdmin(testPassword)
	if w.Code != http.StatusCreated {
		h.t.Fatal("setup failed", w.Code, w.Body.String())
	}
	return s
}

// dirContents returns the raw bytes of every file under dir, for plaintext scans.
func dirContents(t *testing.T, dir string) string {
	t.Helper()
	var all strings.Builder
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		all.Write(b)
		all.WriteString("\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return all.String()
}
