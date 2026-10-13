package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

const testRemoteLoopbackProxy = "127.0.0.1:40000"
const defaultAdminEmail = "admin@example.test"

func TestClientIPTrustsOnlyTheConfiguredDockerProxy(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/admin/session", nil)
	r.RemoteAddr = "172.18.0.1:40000"
	r.Header.Set("X-Real-IP", "198.51.100.10")
	if got := clientIP(r, nil); got != "172.18.0.1" {
		t.Fatal("unconfigured proxy was trusted", got)
	}
	proxy := net.ParseIP("172.18.0.1")
	if got := clientIP(r, proxy); got != "198.51.100.10" {
		t.Fatal("configured proxy lost the real client", got)
	}
	r.RemoteAddr = "172.18.0.3:40000"
	if got := clientIP(r, proxy); got != "172.18.0.3" {
		t.Fatal("another container can spoof the client address", got)
	}
}

func loginFrom(h *testHarness, remote, email, password string) *http.Response {
	h.t.Helper()
	return h.send(call{method: "POST", target: "/api/admin/login", remote: remote, body: map[string]string{"email": email, "password": password}}).Result()
}

func TestSetupFlowCreatesAccountWithArgon2AndRejectsSecondSetup(t *testing.T) {
	h := newHarness(t)
	w := h.send(call{method: "GET", target: "/api/admin/session"})
	var st map[string]string
	decodeJSON(t, w, &st)
	if st["state"] != "setup" || st["email"] != "" {
		t.Fatal("initial state", st)
	}
	if w := h.send(call{method: "POST", target: "/api/admin/setup", body: map[string]string{"setupCode": "AAAA-AAAA-AAAA", "password": testPassword}}); w.Code != 401 || errorCode(t, w) != "invalid_setup_code" {
		t.Fatal("wrong setup code", w.Code, w.Body.String())
	}
	code := regexp.MustCompile(testSetupCodeRe).FindStringSubmatch(h.logs.String())[1]
	if w := h.send(call{method: "POST", target: "/api/admin/setup", body: map[string]string{"setupCode": code, "email": defaultAdminEmail, "password": "short"}}); w.Code != 422 || !strings.Contains(w.Body.String(), `"password":"too_short"`) {
		t.Fatal("short setup password", w.Code, w.Body.String())
	}
	w, s := h.setupAdmin(testPassword)
	if w.Code != 201 || s.csrf == "" || len(s.cookie) < 40 {
		t.Fatal("setup", w.Code, w.Body.String())
	}
	if w := h.send(call{method: "POST", target: "/api/admin/setup", body: map[string]string{"setupCode": code, "password": "another-sample-password"}}); w.Code != 409 || errorCode(t, w) != "already_configured" {
		t.Fatal("second setup", w.Code, w.Body.String())
	}
	raw, err := readFileForTest(h.dir + "/admin.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "$argon2id$v=19$m=19456,t=2,p=1$") || strings.Contains(raw, testPassword) {
		t.Fatal("admin.json must hold an argon2id PHC hash only")
	}
	if all := dirContents(t, h.dir); strings.Contains(all, testPassword) || strings.Contains(all, code) || strings.Contains(all, strings.ReplaceAll(code, "-", "")) {
		t.Fatal("plaintext password or setup code found on disk")
	}
}

func TestSetupCodeIsLoggedExactlyOnce(t *testing.T) {
	h := newHarness(t)
	lines := strings.Split(strings.TrimSpace(h.logs.String()), "\n")
	count := 0
	pattern := regexp.MustCompile(`^.*admin setup code: [A-Z2-7]{4}-[A-Z2-7]{4}-[A-Z2-7]{4} \(use it at /admin within this process lifetime\)$`)
	for _, line := range lines {
		if strings.Contains(line, "admin setup code:") {
			count++
			if !pattern.MatchString(line) {
				t.Fatal("setup code line format:", line)
			}
		}
	}
	if count != 1 {
		t.Fatalf("setup code logged %d times", count)
	}
}

func TestSetupFailuresShareTheLimiter(t *testing.T) {
	h := newHarness(t)
	for i := 1; i <= 5; i++ {
		w := h.send(call{method: "POST", target: "/api/admin/setup", remote: "198.51.100.50:4000", body: map[string]string{"setupCode": "AAAA-AAAA-AAAA", "password": testPassword}})
		if i < 5 && w.Code != 401 {
			t.Fatal("attempt", i, w.Code)
		}
		if i == 5 && (w.Code != 429 || w.Header().Get("Retry-After") != "900") {
			t.Fatal("fifth failure must lock", w.Code, w.Header())
		}
	}
	code := regexp.MustCompile(testSetupCodeRe).FindStringSubmatch(h.logs.String())[1]
	if w := h.send(call{method: "POST", target: "/api/admin/setup", remote: "198.51.100.50:4000", body: map[string]string{"setupCode": code, "password": testPassword}}); w.Code != 429 {
		t.Fatal("correct code must still be blocked during the lock", w.Code)
	}
	h.clock.advance(15 * time.Minute)
	if w, _ := h.setupAdmin(testPassword); w.Code != 201 {
		t.Fatal("setup after lock expired", w.Code, w.Body.String())
	}
}

func TestLoginFailuresAreGenericAndIdentical(t *testing.T) {
	h := newHarness(t)
	h.adminSession()
	wrongPassword := h.send(call{method: "POST", target: "/api/admin/login", remote: "198.51.100.60:4000", body: map[string]string{"email": defaultAdminEmail, "password": "wrong-password-value"}})
	unknownEmail := h.send(call{method: "POST", target: "/api/admin/login", remote: "198.51.100.61:4000", body: map[string]string{"email": "nobody@example.test", "password": "wrong-password-value"}})
	if wrongPassword.Code != 401 || unknownEmail.Code != 401 {
		t.Fatal(wrongPassword.Code, unknownEmail.Code)
	}
	if wrongPassword.Body.String() != unknownEmail.Body.String() || errorCode(t, wrongPassword) != "invalid_credentials" {
		t.Fatal("login failures must be identical", wrongPassword.Body.String(), unknownEmail.Body.String())
	}
}

func TestLoginLockoutAfterFiveFailuresForFifteenMinutes(t *testing.T) {
	h := newHarness(t)
	h.adminSession()
	const remote = "198.51.100.70:4000"
	for i := 1; i <= 5; i++ {
		w := h.send(call{method: "POST", target: "/api/admin/login", remote: remote, body: map[string]string{"email": defaultAdminEmail, "password": "not-the-password-1"}})
		if i < 5 && (w.Code != 401 || errorCode(t, w) != "invalid_credentials") {
			t.Fatal("failure", i, w.Code)
		}
		if i == 5 && (w.Code != 429 || errorCode(t, w) != "too_many_attempts" || w.Header().Get("Retry-After") != "900") {
			t.Fatal("fifth failure must lock with Retry-After", w.Code, w.Header(), w.Body.String())
		}
	}
	if resp := loginFrom(h, remote, defaultAdminEmail, testPassword); resp.StatusCode != 429 {
		t.Fatal("correct password must be blocked during the lock", resp.StatusCode)
	}
	h.clock.advance(15*time.Minute - time.Second)
	if resp := loginFrom(h, remote, defaultAdminEmail, testPassword); resp.StatusCode != 429 {
		t.Fatal("still locked 1s before expiry", resp.StatusCode)
	}
	h.clock.advance(time.Second)
	if resp := loginFrom(h, remote, defaultAdminEmail, testPassword); resp.StatusCode != 200 {
		t.Fatal("lock must end after 15 minutes", resp.StatusCode)
	}
}

func TestClientIPUsesXRealIPOnlyBehindLoopbackProxy(t *testing.T) {
	h := newHarness(t)
	h.adminSession()
	failFrom := func(remote, realIP string) *http.Response {
		hdr := http.Header{}
		if realIP != "" {
			hdr.Set("X-Real-IP", realIP)
		}
		return h.send(call{method: "POST", target: "/api/admin/login", remote: remote, header: hdr, body: map[string]string{"email": defaultAdminEmail, "password": "not-the-password-2"}}).Result()
	}
	// Direct client forging X-Real-IP: counted against its real RemoteAddr.
	for i := 0; i < 5; i++ {
		failFrom("203.0.113.77:4000", "198.51.100.90")
	}
	if resp := failFrom("203.0.113.77:4000", ""); resp.StatusCode != 429 {
		t.Fatal("spoofed X-Real-IP must not bypass the lock", resp.StatusCode)
	}
	if resp := failFrom("198.51.100.90:4000", ""); resp.StatusCode != 401 {
		t.Fatal("header value must not create a lock for a foreign address", resp.StatusCode)
	}
	// Loopback proxy: X-Real-IP identifies the browser.
	for i := 0; i < 5; i++ {
		failFrom(testRemoteLoopbackProxy, "198.51.100.91")
	}
	if resp := failFrom(testRemoteLoopbackProxy, "198.51.100.92"); resp.StatusCode != 401 {
		t.Fatal("different browser behind the proxy must not be locked", resp.StatusCode)
	}
	if resp := failFrom(testRemoteLoopbackProxy, "198.51.100.91"); resp.StatusCode != 429 {
		t.Fatal("browser behind the proxy must be locked", resp.StatusCode)
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	h := newHarness(t)
	h.adminSession()
	cookieHeader := func(w *http.Response) string {
		for _, v := range w.Header.Values("Set-Cookie") {
			if strings.HasPrefix(v, sessionCookieName+"=") {
				return v
			}
		}
		t.Fatal("no session cookie")
		return ""
	}
	plain := loginFrom(h, "198.51.100.100:4000", defaultAdminEmail, testPassword)
	c := cookieHeader(plain)
	for _, want := range []string{"HttpOnly", "SameSite=Strict", "Path=/"} {
		if !strings.Contains(c, want) {
			t.Fatal("cookie missing", want, c)
		}
	}
	if strings.Contains(c, "Secure") || strings.Contains(c, "Max-Age") || strings.Contains(c, "Expires") {
		t.Fatal("plain HTTP cookie must not be Secure or persistent", c)
	}
	forwarded := h.send(call{method: "POST", target: "/api/admin/login", remote: "198.51.100.101:4000", header: http.Header{"X-Forwarded-Proto": {"https"}}, body: map[string]string{"email": defaultAdminEmail, "password": testPassword}}).Result()
	if c := cookieHeader(forwarded); !strings.Contains(c, "Secure") {
		t.Fatal("X-Forwarded-Proto https must set Secure", c)
	}
	tlsLogin := h.send(call{method: "POST", target: "/api/admin/login", remote: "198.51.100.102:4000", tls: true, body: map[string]string{"email": defaultAdminEmail, "password": testPassword}}).Result()
	if c := cookieHeader(tlsLogin); !strings.Contains(c, "Secure") {
		t.Fatal("TLS login must set Secure", c)
	}
}

func TestEachLoginRotatesSessionToken(t *testing.T) {
	h := newHarness(t)
	h.adminSession()
	_, first := h.login(defaultAdminEmail, testPassword)
	_, second := h.login(defaultAdminEmail, testPassword)
	if first == nil || second == nil || first.cookie == second.cookie || first.csrf == second.csrf {
		t.Fatal("login must issue a new token and CSRF token")
	}
}

func TestSessionIdleTimeoutAndExtend(t *testing.T) {
	h := newHarness(t)
	_, s := h.setupAdmin(testPassword)
	h.clock.advance(59 * time.Minute)
	if w := h.send(call{method: "GET", target: "/api/admin/settings", s: s}); w.Code != 200 {
		t.Fatal("alive at 59 minutes", w.Code)
	}
	h.clock.advance(time.Minute)
	if w := h.send(call{method: "GET", target: "/api/admin/settings", s: s}); w.Code != 401 || errorCode(t, w) != "session_expired" {
		t.Fatal("idle expiry at 60 minutes", w.Code, w.Body.String())
	}

	_, s = h.login(defaultAdminEmail, testPassword)
	h.clock.advance(50 * time.Minute)
	if w := h.send(call{method: "POST", target: "/api/admin/session/extend", s: s}); w.Code != 200 {
		t.Fatal("extend", w.Code, w.Body.String())
	}
	h.clock.advance(50 * time.Minute)
	if w := h.send(call{method: "GET", target: "/api/admin/status", s: s}); w.Code != 200 {
		t.Fatal("extend must keep the session alive", w.Code)
	}
	h.clock.advance(11 * time.Minute)
	if w := h.send(call{method: "GET", target: "/api/admin/status", s: s}); w.Code != 401 {
		t.Fatal("idle expiry after extend", w.Code)
	}
}

func TestSessionAbsoluteLifetimeTwelveHours(t *testing.T) {
	h := newHarness(t)
	_, s := h.setupAdmin(testPassword)
	for i := 0; i < 23; i++ {
		h.clock.advance(30 * time.Minute)
		if w := h.send(call{method: "POST", target: "/api/admin/session/extend", s: s}); w.Code != 200 {
			t.Fatal("active session expired early", i, w.Code)
		}
	}
	h.clock.advance(30 * time.Minute)
	if w := h.send(call{method: "POST", target: "/api/admin/session/extend", s: s}); w.Code != 401 || errorCode(t, w) != "session_expired" {
		t.Fatal("absolute 12-hour cap must apply even when active", w.Code, w.Body.String())
	}
}

func TestLogoutInvalidatesCookie(t *testing.T) {
	h := newHarness(t)
	_, s := h.setupAdmin(testPassword)
	if w := h.send(call{method: "POST", target: "/api/admin/logout", s: s}); w.Code != 204 {
		t.Fatal("logout", w.Code)
	}
	if w := h.send(call{method: "GET", target: "/api/admin/settings", s: s}); w.Code != 401 || errorCode(t, w) != "unauthenticated" {
		t.Fatal("logged-out cookie still works", w.Code)
	}
}

func TestCSRFAndOriginRules(t *testing.T) {
	h := newHarness(t)
	_, s := h.setupAdmin(testPassword)
	body := map[string]any{"grapnelGroupName": "Sample Group"}
	if w := h.send(call{method: "GET", target: "/api/admin/settings", s: s}); w.Code != 200 {
		t.Fatal("GET needs no CSRF token", w.Code)
	}
	if w := h.send(call{method: "PUT", target: "/api/admin/settings", body: body, s: &sess{cookie: s.cookie}}); w.Code != 403 || errorCode(t, w) != "csrf" {
		t.Fatal("missing token", w.Code)
	}
	if w := h.send(call{method: "PUT", target: "/api/admin/settings", body: body, s: &sess{cookie: s.cookie, csrf: "wrong-token"}}); w.Code != 403 {
		t.Fatal("wrong token", w.Code)
	}
	foreign := http.Header{"Origin": {"https://evil.example"}}
	if w := h.send(call{method: "PUT", target: "/api/admin/settings", body: body, s: s, header: foreign}); w.Code != 403 || errorCode(t, w) != "csrf" {
		t.Fatal("foreign origin", w.Code)
	}
	if w := h.send(call{method: "PUT", target: "/api/admin/settings", body: body, s: s, header: http.Header{"Origin": {"http://" + testHost}}}); w.Code != 200 {
		t.Fatal("matching origin with token", w.Code, w.Body.String())
	}
	if w := h.send(call{method: "PUT", target: "/api/admin/settings", body: body, s: s}); w.Code != 200 {
		t.Fatal("correct token", w.Code, w.Body.String())
	}
}

func TestPublicPostRejectsForeignOriginBeforeCounting(t *testing.T) {
	h := newHarness(t)
	h.adminSession()
	w := h.send(call{method: "POST", target: "/api/admin/login", header: http.Header{"Origin": {"https://evil.example"}}, body: map[string]string{"email": defaultAdminEmail, "password": "x"}})
	if w.Code != 403 || errorCode(t, w) != "csrf" {
		t.Fatal("cross-origin login", w.Code)
	}
	if strings.Contains(dirContents(t, h.dir), "login.failed") {
		t.Fatal("rejected origin must not be audited as a failed login")
	}
}

func TestPasswordChangePolicyAndSessionRevocation(t *testing.T) {
	h := newHarness(t)
	_, sA := h.setupAdmin(testPassword)
	_, sB := h.login(defaultAdminEmail, testPassword)
	change := func(s *sess, current, next, remote string) *httptest.ResponseRecorder {
		return h.send(call{method: "POST", target: "/api/admin/password", s: s, remote: remote, body: map[string]string{"currentPassword": current, "newPassword": next}})
	}
	if r := change(sA, "wrong-current-password", "brand-new-sample-password", "198.51.100.110:4000"); r.Code != 401 {
		t.Fatal("wrong current password", r.Code)
	}
	if r := change(sA, testPassword, strings.Repeat("s", 11), "198.51.100.111:4000"); r.Code != 422 || !strings.Contains(r.Body.String(), `"newPassword":"too_short"`) {
		t.Fatal("too short", r.Body.String())
	}
	if r := change(sA, testPassword, strings.Repeat("x", 257), "198.51.100.111:4000"); r.Code != 422 || !strings.Contains(r.Body.String(), `"too_long"`) {
		t.Fatal("too long", r.Body.String())
	}
	if r := change(sA, testPassword, strings.ToUpper(defaultAdminEmail), "198.51.100.111:4000"); r.Code != 422 || !strings.Contains(r.Body.String(), `"same_as_email"`) {
		t.Fatal("same as email", r.Body.String())
	}
	// Boundary: exactly 256 characters is the longest accepted password.
	newPassword := strings.Repeat("p", 256)
	r := change(sA, testPassword, newPassword, "198.51.100.112:4000")
	if r.Code != 200 {
		t.Fatal("password change at the 256-character boundary", r.Code, r.Body.String())
	}
	var body map[string]string
	decodeJSON(t, r, &body)
	if body["csrfToken"] == "" || body["csrfToken"] == sA.csrf {
		t.Fatal("password change must rotate the CSRF token")
	}
	if w := h.send(call{method: "GET", target: "/api/admin/settings", s: sB}); w.Code != 401 {
		t.Fatal("other session must be revoked", w.Code)
	}
	if w := h.send(call{method: "GET", target: "/api/admin/settings", s: sA}); w.Code != 401 {
		t.Fatal("old token of the current session must be rotated away", w.Code)
	}
	if w, _ := h.login(defaultAdminEmail, testPassword); w.Code != 401 {
		t.Fatal("old password must stop working")
	}
	if w, _ := h.login(defaultAdminEmail, newPassword); w.Code != 200 {
		t.Fatal("new password must work", w.Code)
	}
}

func TestEmailChange(t *testing.T) {
	h := newHarness(t)
	_, s := h.setupAdmin(testPassword)
	change := func(current, email string) *httptest.ResponseRecorder {
		return h.send(call{method: "POST", target: "/api/admin/account/email", s: s, body: map[string]string{"currentPassword": current, "email": email}})
	}
	if w := change("wrong-current-password", "new.owner@example.test"); w.Code != 401 {
		t.Fatal("current password required", w.Code)
	}
	if w := change(testPassword, "not-an-email"); w.Code != 422 || !strings.Contains(w.Body.String(), `"email":"invalid_email"`) {
		t.Fatal("invalid email", w.Body.String())
	}
	if w := change(testPassword, testPassword); w.Code != 422 {
		t.Fatal("email equal to the password must be refused", w.Code)
	}
	if w := change(testPassword, "New.Owner@Example.test"); w.Code != 204 {
		t.Fatal("email change", w.Code, w.Body.String())
	}
	if w, _ := h.login("NEW.owner@example.test", testPassword); w.Code != 200 {
		t.Fatal("new email must log in case-insensitively", w.Code)
	}
	if w, _ := h.login(defaultAdminEmail, testPassword); w.Code != 401 {
		t.Fatal("old email must stop working", w.Code)
	}
}

func TestRevokeOthersKeepsCurrentSession(t *testing.T) {
	h := newHarness(t)
	_, sA := h.setupAdmin(testPassword)
	_, sB := h.login(defaultAdminEmail, testPassword)
	_, sC := h.login(defaultAdminEmail, testPassword)
	if w := h.send(call{method: "POST", target: "/api/admin/sessions/revoke-others", s: sA}); w.Code != 204 {
		t.Fatal("revoke others", w.Code)
	}
	for _, s := range []*sess{sB, sC} {
		if w := h.send(call{method: "GET", target: "/api/admin/settings", s: s}); w.Code != 401 {
			t.Fatal("other session survived", w.Code)
		}
	}
	if w := h.send(call{method: "GET", target: "/api/admin/settings", s: sA}); w.Code != 200 {
		t.Fatal("current session must survive", w.Code)
	}
}

func TestAuditActivityNewestFirstWithoutValuesOrAddresses(t *testing.T) {
	h := newHarness(t)
	_, s := h.setupAdmin(testPassword)
	h.login("nobody@example.test", "not-the-password-3")
	const newSecret = "SENTINEL-new-webhook-secret-0000000000000000"
	if w := h.send(call{method: "PUT", target: "/api/admin/settings", s: s, remote: "198.51.100.120:4000", body: map[string]any{"pollIntervalMinutes": 3, "webhookSecret": newSecret}}); w.Code != 200 {
		t.Fatal("settings update", w.Code, w.Body.String())
	}
	w := h.send(call{method: "GET", target: "/api/admin/activity", s: s})
	var entries []struct {
		At     string   `json:"at"`
		Action string   `json:"action"`
		Fields []string `json:"fields"`
	}
	decodeJSON(t, w, &entries)
	want := []string{"settings.updated", "login.failed", "setup.completed"}
	if len(entries) != len(want) {
		t.Fatal("activity entries", entries)
	}
	for i, action := range want {
		if entries[i].Action != action {
			t.Fatalf("entry %d is %s, want %s (newest first)", i, entries[i].Action, action)
		}
	}
	if f := entries[0].Fields; len(f) != 2 || f[0] != "pollIntervalMinutes" || f[1] != "webhookSecret" {
		t.Fatal("settings.updated lists field names only", f)
	}
	body := w.Body.String()
	for _, leak := range []string{"SENTINEL", "198.51.100", "192.0.2", "127.0.0.1", "not-the-password", testPassword, "nobody@example.test"} {
		if strings.Contains(body, leak) {
			t.Fatalf("audit leaks %q", leak)
		}
	}
	if strings.Contains(dirContents(t, h.dir), newSecret) {
		t.Fatal("state files hold a secret in plaintext")
	}
}

func TestSessionStateEndpointReportsStates(t *testing.T) {
	h := newHarness(t)
	var st map[string]string
	decodeJSON(t, h.send(call{method: "GET", target: "/api/admin/session"}), &st)
	if st["state"] != "setup" {
		t.Fatal(st)
	}
	h.setupAdmin(testPassword)
	decodeJSON(t, h.send(call{method: "GET", target: "/api/admin/session"}), &st)
	if st["state"] != "login" || st["email"] != "" || st["csrfToken"] != "" {
		t.Fatal("login state without session", st)
	}
	_, s := h.login(defaultAdminEmail, testPassword)
	decodeJSON(t, h.send(call{method: "GET", target: "/api/admin/session", s: s}), &st)
	if st["state"] != "authenticated" || st["csrfToken"] != s.csrf || st["expiresAt"] == "" {
		t.Fatal("authenticated state", st)
	}
}

func TestSetupUsesChosenEmailAndOnlyRevealsItAfterAuthentication(t *testing.T) {
	h := newHarness(t)
	code := regexp.MustCompile(testSetupCodeRe).FindStringSubmatch(h.logs.String())[1]
	for _, email := range []string{"", "not-an-email"} {
		w := h.send(call{method: "POST", target: "/api/admin/setup", body: map[string]string{"setupCode": code, "email": email, "password": testPassword}})
		if w.Code != 422 || !strings.Contains(w.Body.String(), `"email":"invalid_email"`) {
			t.Fatal("invalid setup email accepted", w.Code, w.Body.String())
		}
	}
	w := h.send(call{method: "POST", target: "/api/admin/setup", body: map[string]string{"setupCode": code, "email": " Owner@Example.test ", "password": testPassword}})
	if w.Code != 201 {
		t.Fatal("chosen email setup failed", w.Code, w.Body.String())
	}
	var auth map[string]string
	decodeJSON(t, w, &auth)
	if auth["email"] != "owner@example.test" {
		t.Fatal("authenticated response must contain normalized email", auth)
	}
	if w, _ := h.login("owner@example.test", testPassword); w.Code != 200 {
		t.Fatal("chosen email cannot sign in", w.Code)
	}
	public := h.send(call{method: "GET", target: "/api/admin/session"})
	if strings.Contains(public.Body.String(), "owner@example.test") {
		t.Fatal("unauthenticated endpoint exposes account email")
	}
}
