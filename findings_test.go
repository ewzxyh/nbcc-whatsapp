package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestIPv6CredentialAttemptsSharePrefix(t *testing.T) {
	h := newHarness(t)
	h.setupAdmin(testPassword)
	for i, ip := range []string{"2001:db8:1:2::1", "2001:db8:1:2::2", "2001:db8:1:2:abcd::3", "2001:0db8:0001:0002::4", "2001:db8:1:2::5"} {
		response := loginFrom(h, "["+ip+"]:4000", defaultAdminEmail, "wrong-password-value")
		want := 401
		if i == 4 {
			want = 429
		}
		if response.StatusCode != want {
			t.Fatalf("IPv6 attempt %d: got %d, want %d", i+1, response.StatusCode, want)
		}
	}
	if r := loginFrom(h, "[2001:db8:1:2::99]:4000", defaultAdminEmail, testPassword); r.StatusCode != 429 {
		t.Fatal("same prefix bypasses lockout", r.StatusCode)
	}
	if r := loginFrom(h, "[2001:db8:1:3::99]:4000", defaultAdminEmail, testPassword); r.StatusCode != 200 {
		t.Fatal("unrelated IPv6 prefix is blocked", r.StatusCode)
	}
}

func TestAgendaIDsDependOnPersistedPrivateKey(t *testing.T) {
	h := newHarness(t)
	h.up.feed = sampleAgendaFeed()
	first := decodeAgenda(t, agendaGet(h, "/api/agenda"))
	other := newHarness(t)
	other.up.feed = sampleAgendaFeed()
	second := decodeAgenda(t, agendaGet(other, "/api/agenda"))
	for i, item := range first.Items {
		if item.ID == second.Items[i].ID {
			t.Fatal("public ID is identical with a different private key")
		}
		if item.ActivityID != nil && *item.ActivityID == *second.Items[i].ActivityID {
			t.Fatal("public assessment ID is identical with a different private key")
		}
	}
	var err error
	h.app.settings, err = openSettings(h.dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	restarted := decodeAgenda(t, agendaGet(h, "/api/agenda"))
	for i, item := range first.Items {
		if item.ID != restarted.Items[i].ID {
			t.Fatal("public IDs changed after reopening persistent settings")
		}
	}
	legacy := sha256.Sum256([]byte("activity\x00Sample I\x00qi:111111:222222\x00"))
	guess := hex.EncodeToString(legacy[:8])
	for _, item := range first.Items {
		if item.ActivityID != nil && *item.ActivityID == guess {
			t.Fatal("an offline guess recovers the original assessment identifier")
		}
	}
}

func TestAgendaRemovesURLsBeforeSplittingActivityLocation(t *testing.T) {
	for _, location := range []string{
		"https://example.invalid/private?token=SENTINEL",
		"ftp://example.invalid/private?token=SENTINEL",
		"//example.invalid/private?token=SENTINEL",
		"https&#58;//example.invalid/private?token=SENTINEL",
		"Sample I: Fundamentals https://example.invalid/private?token=SENTINEL",
		"mailto:private@example.invalid?token=SENTINEL",
		"https://example.invalid/<b>private?token=SENTINEL</b>",
		"https://example.invalid/<b class=\"sample\">private?token=SENTINEL</b>",
		"Sample I:https://example.invalid/private?token=SENTINEL",
	} {
		t.Run(location, func(t *testing.T) {
			h := newHarness(t)
			h.up.feed = testICS(agendaEvent("private-location@test.invalid", "Sample Lab - Due", location, ";VALUE=DATE:20261014", ""))
			w := agendaGet(h, "/api/agenda")
			reply := decodeAgenda(t, w)
			if len(reply.Items) != 1 {
				t.Fatal("activity was lost")
			}
			if strings.HasPrefix(location, "Sample I:") && (reply.Items[0].Course == nil || *reply.Items[0].Course != "Sample I") {
				t.Fatal("URL redaction changed the legitimate course code")
			}
			for _, private := range []string{"example.invalid", "SENTINEL", "private?token"} {
				if strings.Contains(w.Body.String(), private) {
					t.Fatal("activity location URL leaked", private)
				}
			}
		})
	}
}

func TestLocationSanitizationDoesNotMergeDistinctCourseIdentities(t *testing.T) {
	h := newHarness(t)
	h.up.feed = testICS(
		agendaEvent("first@test.invalid", "Sample Lab - Due", "COURSE_1: First course", ";VALUE=DATE:20261014", ""),
		agendaEvent("second@test.invalid", "Sample Lab - Due", "COURSE1: Second course", ";VALUE=DATE:20261015", ""),
	)
	reply := decodeAgenda(t, agendaGet(h, "/api/agenda"))
	if len(reply.Items) != 2 || *reply.Items[0].ActivityID == *reply.Items[1].ActivityID {
		t.Fatal("sanitization merged two separate assessments", reply.Items)
	}
}

func TestOmittedRecurrenceIsCountedWithinLaterWindow(t *testing.T) {
	for _, tt := range []struct {
		name, extra string
		want        int
	}{
		{"ongoing hourly", "RRULE:FREQ=HOURLY\r\n", 1},
		{"expired count", "RRULE:FREQ=HOURLY;COUNT=3\r\n", 0},
		{"expired until", "RRULE:FREQ=HOURLY;UNTIL=20260902T120000Z\r\n", 0},
		{"unreadable exceptions", "RRULE:FREQ=DAILY;UNTIL=20261020T120000Z\r\nEXDATE:20261015T120000Z,20261016T120000Z\r\n", 1},
		{"later explicit date", "RDATE:20261015T120000Z\r\n", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.up.feed = testICS(agendaEvent("omitted@test.invalid", "Sample Campus Series", "Sample Campus", ":20260901T120000Z", tt.extra))
			reply := decodeAgenda(t, agendaGet(h, "/api/agenda?from=2026-10-13&to=2026-10-20"))
			if reply.OmittedEvents != tt.want {
				t.Fatalf("omittedEvents = %d, want %d", reply.OmittedEvents, tt.want)
			}
		})
	}
}

func TestRecurrenceOverrideDoesNotClaimMasterUID(t *testing.T) {
	master := agendaEvent("shared@test.invalid", "Sample Campus Series", "Sample Campus", ";VALUE=DATE:20261013", "RRULE:FREQ=DAILY;COUNT=3\r\n")
	override := agendaEvent("shared@test.invalid", "Sample Override", "Sample Campus", ";VALUE=DATE:20261015", "RECURRENCE-ID;VALUE=DATE:20261014\r\n")
	h := newHarness(t)
	h.up.feed = testICS(master, override)
	forward := decodeAgenda(t, agendaGet(h, "/api/agenda?from=2026-10-13&to=2026-10-20"))
	other := newHarness(t)
	other.up.feed = testICS(override, master)
	reversed := decodeAgenda(t, agendaGet(other, "/api/agenda?from=2026-10-13&to=2026-10-20"))
	if len(forward.Items) != 3 || len(reversed.Items) != 3 || forward.OmittedEvents != 1 || reversed.OmittedEvents != 1 {
		t.Fatalf("feed order changes the result: forward items=%d omissions=%d; reversed items=%d omissions=%d", len(forward.Items), forward.OmittedEvents, len(reversed.Items), reversed.OmittedEvents)
	}
}

func TestConcurrentCredentialVerificationCannotUndoLockout(t *testing.T) {
	l := newAttemptLimiter()
	now := testNow()
	clock := func() time.Time { return now }
	for i := 0; i < 4; i++ {
		l.verify("192.0.2.10", clock, func() bool { return false })
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	failed := make(chan struct{})
	correctStarted := make(chan struct{})
	correctResult := make(chan bool, 1)
	go func() {
		l.verify("192.0.2.10", clock, func() bool { close(entered); <-release; return false })
		close(failed)
	}()
	<-entered
	go func() {
		ok, _ := l.verify("192.0.2.10", clock, func() bool { close(correctStarted); <-failed; return true })
		correctResult <- ok
	}()
	select {
	case <-correctStarted:
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if <-correctResult {
		t.Fatal("parallel correct guess was accepted after the fifth failure locked the client")
	}
	<-failed
	now = now.Add(lockDuration)
	if ok, _ := l.verify("192.0.2.10", clock, func() bool { return true }); !ok {
		t.Fatal("legitimate sign-in remains blocked after lock expiry")
	}
}

func TestParallelLoginRequestsStopAtFailureLimit(t *testing.T) {
	h := newHarness(t)
	h.adminSession()
	start := make(chan struct{})
	results := make(chan int, 12)
	const remote = "192.0.2.123:50000"
	for i := 0; i < cap(results); i++ {
		go func() {
			<-start
			w := h.send(call{method: "POST", target: "/api/admin/login", remote: remote, body: map[string]string{"email": defaultAdminEmail, "password": "wrong-password"}})
			results <- w.Code
		}()
	}
	close(start)
	counts := map[int]int{}
	for i := 0; i < cap(results); i++ {
		counts[<-results]++
	}
	if counts[401] != failureLimit-1 || counts[429] != cap(results)-failureLimit+1 {
		t.Fatal("parallel requests bypassed the shared credential gate", counts)
	}
	failedAudits := 0
	for _, entry := range h.app.audit.newestFirst() {
		if entry.Action == "login.failed" {
			failedAudits++
		}
	}
	if failedAudits != failureLimit {
		t.Fatal("the failure that triggered lockout was not audited", failedAudits)
	}
	if w := loginFrom(h, remote, defaultAdminEmail, testPassword); w.StatusCode != 429 {
		t.Fatal("correct guess bypassed the active lock", w.StatusCode)
	}
	h.clock.advance(lockDuration)
	if w := loginFrom(h, remote, defaultAdminEmail, testPassword); w.StatusCode != 200 {
		t.Fatal("legitimate login failed after expiry", w.StatusCode)
	}
}

func TestCredentialLimiterBoundsMemoryWithoutDroppingActiveBuckets(t *testing.T) {
	l := newAttemptLimiter()
	c := &fakeClock{t: testNow()}
	for i := 0; i < maxLimiterEntries; i++ {
		l.clients[string(rune(i))] = &clientAttempts{lastUsed: c.now()}
	}
	l.clients["active"] = l.clients["\x00"]
	delete(l.clients, "\x00")
	l.clients["active"].users = 1
	if _, wait := l.verify("192.0.2.10", c.now, func() bool { t.Fatal("full limiter allowed verification"); return true }); wait == 0 {
		t.Fatal("full limiter did not apply backoff")
	}
	c.advance(lockDuration)
	if ok, _ := l.verify("192.0.2.10", c.now, func() bool { return true }); !ok || len(l.clients) != 2 {
		t.Fatal("expired inactive buckets were not reclaimed", len(l.clients))
	}
	if l.clients["active"] == nil {
		t.Fatal("an in-flight bucket was discarded")
	}
}

func TestCredentialKeysCanonicalizeEquivalentAddresses(t *testing.T) {
	for _, ip := range []string{"::ffff:192.0.2.10", "0:0:0:0:0:ffff:c000:020a", "192.0.2.10"} {
		if attemptKey(ip) != "192.0.2.10" {
			t.Fatal("mapped IPv4 has a separate bucket", ip)
		}
	}
	if attemptKey("2001:db8:1:2::1") != attemptKey("2001:0db8:0001:0002:ffff:ffff:ffff:ffff") {
		t.Fatal("equivalent IPv6 prefix has separate buckets")
	}
}

func TestExistingAdminIdentitySurvivesBootstrapChanges(t *testing.T) {
	h := newHarness(t)
	h.adminSession()
	before, _ := readFileForTest(h.dir + "/admin.json")
	st, err := openAdmin(h.dir, "different@example.test")
	if err != nil || !st.verify(defaultAdminEmail, testPassword) {
		t.Fatal("bootstrap changed the existing account", err)
	}
	if err := st.update(func(x *adminAccount) { x.Email = "different@example.test" }); err == nil {
		t.Fatal("account store allowed an email mutation")
	}
	after, _ := readFileForTest(h.dir + "/admin.json")
	if before != after {
		t.Fatal("existing account file was changed")
	}
	if _, err := openAdmin(t.TempDir(), "not-an-email"); err == nil {
		t.Fatal("fresh setup accepted an invalid fixed identity")
	}
}

func TestAgendaRequiresPrivateIDKey(t *testing.T) {
	raw := []byte(testICS(testEvent("due", "Sample Lab - Due", ";VALUE=DATE:20261014", "")))
	for _, key := range [][]byte{nil, []byte("too-short")} {
		if _, _, err := buildAgenda(raw, testNow(), testNow().AddDate(0, 0, 14), nil, key); err == nil {
			t.Fatal("agenda fell back to an unkeyed ID")
		}
	}
}
