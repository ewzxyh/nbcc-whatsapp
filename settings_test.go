package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// isolateLegacyEnv unsets every legacy variable for the test; t.Setenv restores the original values afterwards.
func isolateLegacyEnv(t *testing.T) {
	t.Helper()
	for _, key := range legacyEnvKeys {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func TestSendingRequiresPublicAgendaLink(t *testing.T) {
	s := testSettingsSeed()
	s.SendEnabled, s.PublicBaseURL = true, ""
	if checkCrossFields(s)["publicBaseUrl"] != "required" {
		t.Fatal("sending without the required agenda link was permitted")
	}
}

func manyInstructors(n int) map[string]string {
	m := map[string]string{}
	for i := 0; i < n; i++ {
		m[fmt.Sprintf("Course %03d", i)] = "Sample Instructor"
	}
	return m
}

type settingsResponse struct {
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

func putSettings(h *testHarness, s *sess, body any) *http.Response {
	return h.send(call{method: "PUT", target: "/api/admin/settings", s: s, body: body}).Result()
}

func TestSettingsGetNeverExposesSecrets(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	w := h.send(call{method: "GET", target: "/api/admin/settings", s: s})
	if w.Code != 200 || strings.Contains(w.Body.String(), "SENTINEL") {
		t.Fatal("settings response leaked a secret", w.Code, w.Body.String())
	}
	var v settingsResponse
	decodeJSON(t, w, &v)
	if !v.NBCCFeedURL.Configured || !v.GrapnelAPIKey.Configured || !v.WebhookSecret.Configured || !v.NBCCFeedURL.Secret {
		t.Fatalf("secret flags: %+v", v)
	}
	if v.GrapnelInstanceID != testInstance || v.GrapnelGroupJID != testGroup || v.PollIntervalMinutes != 5 || v.AgendaCacheMinutes != 60 || v.SendEnabled {
		t.Fatalf("plain fields: %+v", v)
	}
	if all := dirContents(t, h.dir); strings.Contains(all, "SENTINEL") {
		t.Fatal("a secret reached a file in plaintext")
	}
	raw, err := readFileForTest(filepath.Join(h.dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(raw, `"v":1`) != 3 {
		t.Fatal("each secret must be stored sealed with version 1", raw)
	}
}

func TestSecretsRoundTripAndAreBoundToTheirField(t *testing.T) {
	h := newHarness(t)
	const next = "SENTINEL-rotated-webhook-secret-aaaaaaaaaaaaaaaa"
	h.settingsPatch(map[string]any{"webhookSecret": next})
	reopened, err := openSettings(h.dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.current(); got.WebhookSecret != next || got.NBCCFeedURL != testFeedURL || got.GrapnelAPIKey != testAPIKey {
		t.Fatal("secrets did not survive encryption at rest")
	}
	// Moving a sealed value under another field name must fail authentication (field name is bound as associated data).
	raw, err := readFileForTest(filepath.Join(h.dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]json.RawMessage
	if err = json.Unmarshal([]byte(raw), &file); err != nil {
		t.Fatal(err)
	}
	file["nbccFeedUrl"] = file["webhookSecret"]
	moved, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = writeFileForTest(filepath.Join(h.dir, "settings.json"), string(moved)); err != nil {
		t.Fatal(err)
	}
	if _, err = openSettings(h.dir, nil); err == nil {
		t.Fatal("ciphertext moved between fields was accepted")
	}
}

func TestSettingsMissingKeyFailsClosed(t *testing.T) {
	h := newHarness(t)
	if err := os.Remove(filepath.Join(h.dir, "secret.key")); err != nil {
		t.Fatal(err)
	}
	if _, err := openSettings(h.dir, nil); err == nil {
		t.Fatal("existing settings opened without their key")
	}
}

func TestSettingsPartialUpdateAndNullClears(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	r := putSettings(h, s, map[string]any{"grapnelGroupName": "Sample Study Group"})
	if r.StatusCode != 200 {
		t.Fatal("partial update", r.StatusCode)
	}
	var v settingsResponse
	decodeJSON(t, h.send(call{method: "GET", target: "/api/admin/settings", s: s}), &v)
	if v.GrapnelGroupName != "Sample Study Group" || !v.WebhookSecret.Configured || v.PollIntervalMinutes != 5 {
		t.Fatalf("partial update touched other fields: %+v", v)
	}
	if r := putSettings(h, s, map[string]any{"webhookSecret": nil}); r.StatusCode != 200 {
		t.Fatal("null clears", r.StatusCode)
	}
	decodeJSON(t, h.send(call{method: "GET", target: "/api/admin/settings", s: s}), &v)
	if v.WebhookSecret.Configured {
		t.Fatal("null must clear the secret")
	}
	if r := putSettings(h, s, map[string]any{"publicBaseUrl": "https://nbcc.example.test/"}); r.StatusCode != 200 {
		t.Fatal("trailing slash origin", r.StatusCode)
	}
	decodeJSON(t, h.send(call{method: "GET", target: "/api/admin/settings", s: s}), &v)
	if v.PublicBaseURL != "https://nbcc.example.test" {
		t.Fatal("public base URL normalization", v.PublicBaseURL)
	}
	if r := putSettings(h, s, map[string]any{"publicBaseUrl": ""}); r.StatusCode != 200 {
		t.Fatal("empty string clears the public base URL", r.StatusCode)
	}
}

func TestSettingsValidationCodes(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	for _, tc := range []struct {
		name  string
		body  map[string]any
		field string
		code  string
	}{
		{"feed scheme", map[string]any{"nbccFeedUrl": "http://elearning.nbcc.ca/d2l/le/calendar/feed/user/feed.ics?token=SENTINEL-x"}, "nbccFeedUrl", "invalid_feed_url"},
		{"feed host", map[string]any{"nbccFeedUrl": "https://example.test/d2l/le/calendar/feed/user/feed.ics?token=SENTINEL-x"}, "nbccFeedUrl", "invalid_feed_url"},
		{"feed without token", map[string]any{"nbccFeedUrl": "https://elearning.nbcc.ca/d2l/le/calendar/feed/user/feed.ics"}, "nbccFeedUrl", "invalid_feed_url"},
		{"feed empty", map[string]any{"nbccFeedUrl": ""}, "nbccFeedUrl", "required"},
		{"instance uuid", map[string]any{"grapnelInstanceId": "not-a-uuid"}, "grapnelInstanceId", "invalid_uuid"},
		{"group jid", map[string]any{"grapnelGroupJid": "sample-group"}, "grapnelGroupJid", "invalid_group_jid"},
		{"group name empty", map[string]any{"grapnelGroupName": "   "}, "grapnelGroupName", "required"},
		{"group name long", map[string]any{"grapnelGroupName": strings.Repeat("g", 101)}, "grapnelGroupName", "too_long"},
		{"webhook short", map[string]any{"webhookSecret": "too-short-secret"}, "webhookSecret", "too_short"},
		{"webhook empty", map[string]any{"webhookSecret": ""}, "webhookSecret", "required"},
		{"base http", map[string]any{"publicBaseUrl": "http://nbcc.example.test"}, "publicBaseUrl", "invalid_https_url"},
		{"base path", map[string]any{"publicBaseUrl": "https://nbcc.example.test/path"}, "publicBaseUrl", "invalid_https_url"},
		{"base query", map[string]any{"publicBaseUrl": "https://nbcc.example.test/?x=1"}, "publicBaseUrl", "invalid_https_url"},
		{"base port", map[string]any{"publicBaseUrl": "https://nbcc.example.test:8443"}, "publicBaseUrl", "invalid_https_url"},
		{"poll zero", map[string]any{"pollIntervalMinutes": 0}, "pollIntervalMinutes", "out_of_range"},
		{"poll too large", map[string]any{"pollIntervalMinutes": 1441}, "pollIntervalMinutes", "out_of_range"},
		{"poll fraction", map[string]any{"pollIntervalMinutes": 1.5}, "pollIntervalMinutes", "invalid_value"},
		{"cache too large", map[string]any{"agendaCacheMinutes": 2000}, "agendaCacheMinutes", "out_of_range"},
		{"instructor empty name", map[string]any{"courseInstructors": map[string]string{"Sample I": ""}}, "courseInstructors", "invalid_instructor_map"},
		{"instructor control", map[string]any{"courseInstructors": map[string]string{"Sample\u0007I": "Sample Instructor"}}, "courseInstructors", "invalid_instructor_map"},
		{"instructor too many", map[string]any{"courseInstructors": manyInstructors(101)}, "courseInstructors", "invalid_instructor_map"},
		{"instructor code too long", map[string]any{"courseInstructors": map[string]string{strings.Repeat("c", 41): "Sample Instructor"}}, "courseInstructors", "invalid_instructor_map"},
		{"send without grapnel key", map[string]any{"grapnelApiKey": nil, "sendEnabled": true}, "sendEnabled", "missing_grapnel_config"},
		{"send type", map[string]any{"sendEnabled": "yes"}, "sendEnabled", "invalid_value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := h.send(call{method: "PUT", target: "/api/admin/settings", s: s, body: tc.body})
			var body struct {
				Error  string            `json:"error"`
				Fields map[string]string `json:"fields"`
			}
			decodeJSON(t, w, &body)
			if w.Code != 422 || body.Error != "validation" || body.Fields[tc.field] != tc.code {
				t.Fatalf("status %d body %s; want %s=%s", w.Code, w.Body.String(), tc.field, tc.code)
			}
			if strings.Contains(w.Body.String(), "SENTINEL") {
				t.Fatal("validation response echoed a value")
			}
		})
	}
}

func TestSettingsRejectionIsAtomic(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	if r := putSettings(h, s, map[string]any{"grapnelGroupName": "Changed Group", "pollIntervalMinutes": 0}); r.StatusCode != 422 {
		t.Fatal("expected validation failure", r.StatusCode)
	}
	var v settingsResponse
	decodeJSON(t, h.send(call{method: "GET", target: "/api/admin/settings", s: s}), &v)
	if v.GrapnelGroupName != defaultGroupName || v.PollIntervalMinutes != 5 {
		t.Fatal("a rejected update changed some fields", v)
	}
}

func TestSettingsAuditRecordsFieldNamesOnly(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	if r := putSettings(h, s, map[string]any{"sendEnabled": true, "pollIntervalMinutes": 2}); r.StatusCode != 200 {
		t.Fatal("update", r.StatusCode)
	}
	if r := putSettings(h, s, map[string]any{"sendEnabled": true}); r.StatusCode != 200 {
		t.Fatal("no-op update", r.StatusCode)
	}
	w := h.send(call{method: "GET", target: "/api/admin/activity", s: s})
	var entries []auditEntry
	decodeJSON(t, w, &entries)
	if entries[0].Action != "settings.updated" || strings.Join(entries[0].Fields, ",") != "pollIntervalMinutes,sendEnabled" {
		t.Fatal("audit fields", entries[0])
	}
	count := 0
	for _, e := range entries {
		if e.Action == "settings.updated" {
			count++
		}
	}
	if count != 1 {
		t.Fatal("an update that changes nothing must not be audited", count)
	}
}

func TestSettingsSendEnabledNeedsCompleteGrapnelConfig(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	if r := putSettings(h, s, map[string]any{"grapnelGroupJid": nil, "sendEnabled": true}); r.StatusCode != 422 {
		t.Fatal("group JID is required to send", r.StatusCode)
	}
	if r := putSettings(h, s, map[string]any{"grapnelGroupJid": nil}); r.StatusCode != 200 {
		t.Fatal("clearing the group while sending is off must be allowed", r.StatusCode)
	}
}

func TestWebhookSecretRotationTakesEffectImmediately(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	const next = "SENTINEL-second-webhook-secret-bbbbbbbbbbbbbbbb"
	signed := func(secret string) int {
		w := httptest.NewRecorder()
		h.handler.ServeHTTP(w, signedWebhook(secret, commandBody(testGroup, false), h.clock.now()))
		return w.Code
	}
	if code := signed(testSecret); code != 202 {
		t.Fatal("old secret before rotation", code)
	}
	if r := putSettings(h, s, map[string]any{"webhookSecret": next}); r.StatusCode != 200 {
		t.Fatal("rotation", r.StatusCode)
	}
	if code := signed(testSecret); code != 401 {
		t.Fatal("old secret must be rejected at once", code)
	}
	if code := signed(next); code != 202 {
		t.Fatal("new secret must be accepted at once", code)
	}
}

func TestAgendaCacheMinutesChangeTakesEffectOnNextRequest(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	h.up.feed = sampleAgendaFeed()
	agendaGet(h, "/api/agenda")
	if _, hits := h.up.snapshot(); hits != 1 {
		t.Fatal("first fetch", hits)
	}
	if r := putSettings(h, s, map[string]any{"agendaCacheMinutes": 5}); r.StatusCode != 200 {
		t.Fatal("cache change", r.StatusCode)
	}
	h.clock.advance(6 * time.Minute)
	if w := agendaGet(h, "/api/agenda"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if _, hits := h.up.snapshot(); hits != 2 {
		t.Fatal("a shorter TTL must apply to the next request", hits)
	}
}

func TestSeedImportsLegacyEnvironmentOnce(t *testing.T) {
	logs := captureLog(t)
	isolateLegacyEnv(t)
	t.Setenv("NBCC_FEED_URL", testFeedURL)
	t.Setenv("GRAPNEL_API_KEY", testAPIKey)
	t.Setenv("GRAPNEL_INSTANCE_ID", testInstance)
	t.Setenv("GRAPNEL_GROUP_JID", testGroup)
	t.Setenv("GRAPNEL_GROUP_NAME", "Sample Study Group")
	t.Setenv("PUBLIC_BASE_URL", "https://nbcc.example.com")
	t.Setenv("WEBHOOK_SECRET", testSecret)
	t.Setenv("SEND_ENABLED", "true")
	t.Setenv("POLL_INTERVAL", "2m30s")
	t.Setenv("COURSE_INSTRUCTORS", "Sample I=Sample Instructor")
	t.Setenv("ADMIN_TOKEN", "SENTINEL-legacy-admin-token-0000000000000000")
	dir := t.TempDir()
	missingEnv := filepath.Join(t.TempDir(), "missing.env")
	st, err := openSettings(dir, func() Settings { return seedSettings(missingEnv) })
	if err != nil {
		t.Fatal(err)
	}
	s := st.current()
	if s.NBCCFeedURL != testFeedURL || s.GrapnelAPIKey != testAPIKey || s.GrapnelInstanceID != testInstance || s.GrapnelGroupJID != testGroup || s.GrapnelGroupName != "Sample Study Group" || s.WebhookSecret != testSecret {
		t.Fatalf("seeded secrets and identifiers: %+v", s)
	}
	if !s.SendEnabled || s.PollIntervalMinutes != 2 || s.CourseInstructors["Sample I"] != "Sample Instructor" || s.PublicBaseURL != "https://nbcc.example.com" {
		t.Fatalf("seeded behaviour: %+v", s)
	}
	// A later startup must not re-import, even with different legacy values.
	t.Setenv("POLL_INTERVAL", "9m")
	t.Setenv("SEND_ENABLED", "false")
	again, err := openSettings(dir, func() Settings { return seedSettings(missingEnv) })
	if err != nil {
		t.Fatal(err)
	}
	if s2 := again.current(); s2.PollIntervalMinutes != 2 || !s2.SendEnabled {
		t.Fatal("legacy values were re-imported", s2.PollIntervalMinutes, s2.SendEnabled)
	}
	out := logs.String()
	if !strings.Contains(out, "ADMIN_TOKEN is no longer used") {
		t.Fatal("ADMIN_TOKEN must be reported once")
	}
	for _, leak := range []string{"SENTINEL", testAPIKey, testSecret, testFeedURL, testGroup, testInstance} {
		if strings.Contains(out, leak) {
			t.Fatalf("seed logged a value: %q", leak)
		}
	}
}

func TestSeedReadsDotEnvWhenEnvironmentIsSilent(t *testing.T) {
	logs := captureLog(t)
	isolateLegacyEnv(t)
	envFile := filepath.Join(t.TempDir(), ".env")
	content := "# legacy notes\nNBCC_FEED_URL=\"" + testFeedURL + "\"\nWEBHOOK_SECRET='" + testSecret + "'\n"
	if err := writeFileForTest(envFile, content); err != nil {
		t.Fatal(err)
	}
	s := seedSettings(envFile)
	if s.NBCCFeedURL != testFeedURL || s.WebhookSecret != testSecret {
		t.Fatal(".env values were not imported")
	}
	if strings.Contains(logs.String(), "SENTINEL") {
		t.Fatal("seed logged a value from .env")
	}
}

func TestSeedNeverFailsForMissingEnvAndSkipsInvalidValues(t *testing.T) {
	logs := captureLog(t)
	isolateLegacyEnv(t)
	t.Setenv("NBCC_FEED_URL", "http://elearning.nbcc.ca/d2l/le/calendar/feed/user/feed.ics?token=SENTINEL-bad")
	t.Setenv("SEND_ENABLED", "true")
	t.Setenv("COURSE_INSTRUCTORS", "Sample I")
	t.Setenv("POLL_INTERVAL", "1440m")
	s := seedSettings(filepath.Join(t.TempDir(), "absent.env"))
	if s.NBCCFeedURL != "" {
		t.Fatal("invalid feed URL must be skipped")
	}
	if s.SendEnabled {
		t.Fatal("sending cannot be seeded on without Grapnel configuration")
	}
	if s.PollIntervalMinutes != 1440 {
		t.Fatal("poll interval in range must be seeded", s.PollIntervalMinutes)
	}
	out := logs.String()
	if !strings.Contains(out, "legacy setting nbccFeedUrl skipped") || !strings.Contains(out, "legacy setting courseInstructors skipped") {
		t.Fatal("skipped fields must be named", out)
	}
	if strings.Contains(out, "SENTINEL") || strings.Contains(out, "Sample I") {
		t.Fatal("skipped values must not be logged", out)
	}
	if defaults := seedSettings(filepath.Join(t.TempDir(), "absent.env")); defaults.PollIntervalMinutes != 1440 {
		t.Fatal("env value must still be read on every fresh seed")
	}
}
