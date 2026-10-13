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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func testResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

// Synthetic fixtures; never use production instance or group identifiers here.
const testInstance = "11111111-2222-4333-8444-555555555555"
const testGroup = "120363000000000000@g.us"
const testAdmin = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const testSecret = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func testApp(t *testing.T, enabled bool) (*App, *int, *int) {
	t.Helper()
	posts, fetches := new(int), new(int)
	client := &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "elearning.nbcc.ca" {
			*fetches++
			return testResponse(200, testICS(testEvent("due", "Lab07 - Due", ";TZID=America/Halifax:20261013T235959", ""))), nil
		}
		if r.Method == "POST" {
			*posts++
			if r.Header.Get("Idempotency-Key") == "" {
				t.Error("missing idempotency key")
			}
			return testResponse(202, `{"accepted":true}`), nil
		}
		return testResponse(200, `{"groups":[{"jid":"`+testGroup+`","name":"📚  NBCC Study Group","owner_can_send_message":true}]}`), nil
	})}
	store, err := openStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	app := &App{cfg: Config{FeedURL: "https://elearning.nbcc.ca/d2l/le/calendar/feed/user/feed.ics?token=test", APIKey: "test-key", InstanceID: testInstance, GroupJID: testGroup, GroupName: "NBCC Study Group", AdminToken: testAdmin, WebhookSecret: testSecret, SendEnabled: enabled}, client: client, store: store, now: func() time.Time { return testNow() }}
	return app, posts, fetches
}

func signedRequest(body string, stamp time.Time) *http.Request {
	r := httptest.NewRequest("POST", "/webhooks/grapnel", strings.NewReader(body))
	ts := strconv.FormatInt(stamp.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write([]byte(ts + "." + body))
	r.Header.Set("X-Grapnel-Timestamp", ts)
	r.Header.Set("X-Grapnel-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	r.Header.Set("X-Grapnel-Event-Id", "11111111-1111-4111-8111-111111111111")
	return r
}

func commandBody(group string, fromMe bool) string {
	b, _ := json.Marshal(map[string]any{"id": "11111111-1111-4111-8111-111111111111", "event": "whatsapp.group.message.received", "instance_id": testInstance, "message": map[string]any{"messageid": "abc", "chatid": group, "text": " #NBCC ", "isGroup": true, "fromMe": fromMe}})
	return string(b)
}

func TestWebhookDisabledNeverSendsOrReplaysAfterEnable(t *testing.T) {
	a, posts, _ := testApp(t, false)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, signedRequest(commandBody(testGroup, false), testNow()))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	a.cfg.SendEnabled = true
	if err := a.processPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *posts != 0 {
		t.Fatal("disabled command was replayed after enabling")
	}
}

func TestWebhookFreshFeedDuplicateAndRestart(t *testing.T) {
	a, posts, fetches := testApp(t, true)
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, signedRequest(commandBody(testGroup, false), testNow()))
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if *posts != 0 || *fetches != 0 {
		t.Fatal("webhook waited for external processing before acknowledgement")
	}
	if err := a.processPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *posts != 1 || *fetches != 1 {
		t.Fatal("command did not fetch/send exactly once")
	}
	s, err := openStore(a.store.path)
	if err != nil {
		t.Fatal(err)
	}
	a.store = s
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, signedRequest(commandBody(testGroup, false), testNow()))
	a.processPending(context.Background())
	if *posts != 1 {
		t.Fatal("duplicate after restart")
	}
}

func TestWebhookAuthenticationAndScope(t *testing.T) {
	a, posts, _ := testApp(t, true)
	for _, tc := range []struct {
		name, body   string
		stamp        time.Time
		badSignature bool
		want         int
	}{
		{"signature", commandBody(testGroup, false), testNow(), true, 401},
		{"replay", commandBody(testGroup, false), testNow().Add(-6 * time.Minute), false, 401},
		{"other group", commandBody("123456789@g.us", false), testNow(), false, 202},
		{"own message", commandBody(testGroup, true), testNow(), false, 202},
		{"missing direction", strings.Replace(commandBody(testGroup, false), `"fromMe":false,`, "", 1), testNow(), false, 202},
		{"other text", strings.Replace(commandBody(testGroup, false), "#NBCC", "hello", 1), testNow(), false, 202},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := signedRequest(tc.body, tc.stamp)
			if tc.badSignature {
				r.Header.Set("X-Grapnel-Signature", "sha256=bad")
			}
			w := httptest.NewRecorder()
			a.handler().ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
	a.processPending(context.Background())
	if *posts != 0 {
		t.Fatal("out-of-scope webhook sent a message")
	}
}

func TestPreviewRequiresTokenAndDoesNotSend(t *testing.T) {
	a, posts, _ := testApp(t, true)
	r := httptest.NewRequest("GET", "/api/preview", nil)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("GET", "/api/preview", nil)
	r.Header.Set("Authorization", "Bearer "+testAdmin)
	w = httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "(TODAY)") || *posts != 0 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestMonitorBaselineChangeAndDisabledGuard(t *testing.T) {
	a, posts, _ := testApp(t, true)
	p, err := buildPreview([]byte(testICS(testEvent("due", "Lab07 - Due", ";VALUE=DATE:20261014", ""))), testNow())
	if err != nil {
		t.Fatal(err)
	}
	changed, queued, err := a.store.observe(p, true, testNow())
	if err != nil || changed || queued {
		t.Fatal("first fetch must only establish baseline")
	}
	changed, queued, err = a.store.observe(p, true, testNow())
	if err != nil || changed || queued {
		t.Fatal("unchanged feed queued a send")
	}
	p.FeedHash = "changed"
	changed, queued, err = a.store.observe(p, true, testNow())
	if err != nil || !changed || !queued {
		t.Fatal("change was not queued")
	}
	a.cfg.SendEnabled = false
	if err = a.processPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *posts != 0 {
		t.Fatal("send guard failed")
	}
}

func TestRetryUsesExactlyTheSameKeyAndBody(t *testing.T) {
	a, _, _ := testApp(t, true)
	var keys, bodies []string
	a.client.Transport = testTransport(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		bodies = append(bodies, string(body))
		if len(keys) == 1 {
			return testResponse(503, `{"code":"busy"}`), nil
		}
		return testResponse(202, `{}`), nil
	})
	if err := a.store.enqueue("event:test", "fixed message", true, testNow()); err != nil {
		t.Fatal(err)
	}
	a.processPending(context.Background())
	a.now = func() time.Time { return testNow().Add(10 * time.Minute) }
	a.processPending(context.Background())
	if len(keys) != 2 || keys[0] != keys[1] || bodies[0] != bodies[1] {
		t.Fatal("retry changed idempotency key/body")
	}
}

func TestCorruptStateFailsClosed(t *testing.T) {
	a, _, _ := testApp(t, false)
	if err := os.WriteFile(a.store.path, []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := openStore(a.store.path); err == nil {
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
	a, _, _ := testApp(t, true)
	var keys []string
	a.client.Transport = testTransport(func(r *http.Request) (*http.Response, error) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if len(keys) == 2 {
			return testResponse(503, `{}`), nil
		}
		return testResponse(202, `{}`), nil
	})
	a.store.enqueue("event:long", strings.Repeat("a", 5000), true, testNow())
	a.processPending(context.Background())
	a.now = func() time.Time { return testNow().Add(10 * time.Minute) }
	a.processPending(context.Background())
	if len(keys) != 3 || keys[0] == keys[1] || keys[1] != keys[2] {
		t.Fatal("accepted message part was resent or retry key changed", keys)
	}
}
