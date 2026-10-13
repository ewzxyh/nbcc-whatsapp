package main

import (
	"strings"
	"testing"
	"time"
)

func TestTestFeedUsesSavedURLAndReportsCounts(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	h.up.feed = sampleAgendaFeed()
	w := h.send(call{method: "POST", target: "/api/admin/settings/test-feed", s: s})
	var body struct {
		OK            bool `json:"ok"`
		Events        int  `json:"events"`
		Courses       int  `json:"courses"`
		OmittedEvents int  `json:"omittedEvents"`
	}
	decodeJSON(t, w, &body)
	if w.Code != 200 || !body.OK || body.Events == 0 || body.Courses != 1 || body.OmittedEvents != 0 {
		t.Fatal("test-feed", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "SENTINEL") {
		t.Fatal("test-feed leaked the feed URL")
	}
	h.up.feedFail = true
	if w := h.send(call{method: "POST", target: "/api/admin/settings/test-feed", s: s}); w.Code != 502 || errorCode(t, w) != "feed_unavailable" {
		t.Fatal("upstream failure", w.Code, w.Body.String())
	}
	if r := putSettings(h, s, map[string]any{"nbccFeedUrl": nil}); r.StatusCode != 200 {
		t.Fatal("clear feed", r.StatusCode)
	}
	if w := h.send(call{method: "POST", target: "/api/admin/settings/test-feed", s: s}); w.Code != 409 || errorCode(t, w) != "not_configured" {
		t.Fatal("unconfigured feed", w.Code)
	}
}

func TestTestGrapnelIsDiscoveryOnly(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	w := h.send(call{method: "POST", target: "/api/admin/settings/test-grapnel", s: s})
	if w.Code != 200 || !strings.Contains(w.Body.String(), "NBCC Study Group") || strings.Contains(w.Body.String(), "SENTINEL") {
		t.Fatal("test-grapnel", w.Code, w.Body.String())
	}
	if posts, _ := h.up.snapshot(); posts != 0 {
		t.Fatal("discovery must never send a message")
	}
	h.up.groups = `{"groups":[]}`
	if w := h.send(call{method: "POST", target: "/api/admin/settings/test-grapnel", s: s}); w.Code != 502 || errorCode(t, w) != "grapnel_unavailable" {
		t.Fatal("no matching group", w.Code, w.Body.String())
	}
	if r := putSettings(h, s, map[string]any{"grapnelInstanceId": nil}); r.StatusCode != 200 {
		t.Fatal("clear instance", r.StatusCode)
	}
	if w := h.send(call{method: "POST", target: "/api/admin/settings/test-grapnel", s: s}); w.Code != 409 {
		t.Fatal("unconfigured Grapnel", w.Code)
	}
}

func TestRegisterWebhookUsesSavedBaseURLAndRefusesReplacement(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	w := h.send(call{method: "POST", target: "/api/admin/webhook/register", s: s})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"url":"https://nbcc.example.test/webhooks/grapnel"`) {
		t.Fatal("register", w.Code, w.Body.String())
	}
	if w := h.send(call{method: "POST", target: "/api/admin/webhook/register", s: s}); w.Code != 409 || errorCode(t, w) != "webhook_already_configured" {
		t.Fatal("existing webhook must not be replaced", w.Code, w.Body.String())
	}
	if !strings.Contains(dirContents(t, h.dir), "webhook.registered") {
		t.Fatal("registration must be audited")
	}
	if r := putSettings(h, s, map[string]any{"publicBaseUrl": nil}); r.StatusCode != 200 {
		t.Fatal("clear base URL", r.StatusCode)
	}
	if w := h.send(call{method: "POST", target: "/api/admin/webhook/register", s: s}); w.Code != 409 || errorCode(t, w) != "not_configured" {
		t.Fatal("base URL required", w.Code)
	}
}

func TestCoursesListsActivitiesWithInstructors(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	h.up.feed = sampleAgendaFeed()
	h.settingsPatch(map[string]any{"courseInstructors": map[string]string{"Sample I": "Sample Instructor"}})
	w := h.send(call{method: "GET", target: "/api/admin/courses", s: s})
	var rows []courseRow
	decodeJSON(t, w, &rows)
	if w.Code != 200 || len(rows) != 1 {
		t.Fatal("courses", w.Code, w.Body.String())
	}
	row := rows[0]
	if row.Code != "Sample I" || row.Title == nil || *row.Title != "Sample Fundamentals" || row.Instructor == nil || *row.Instructor != "Sample Instructor" || row.Activities != 3 {
		t.Fatalf("course row: %+v", row)
	}
}

func TestCoursesUnavailableWithoutFeedCopy(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	h.up.feedFail = true
	if w := h.send(call{method: "GET", target: "/api/admin/courses", s: s}); w.Code != 502 || errorCode(t, w) != "feed_unavailable" {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestAdminCheckAndPreview(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	w := h.send(call{method: "POST", target: "/api/admin/check", s: s})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"sendEnabled":false`) || !strings.Contains(w.Body.String(), `"feedChanged":false`) {
		t.Fatal("check", w.Code, w.Body.String())
	}
	if posts, _ := h.up.snapshot(); posts != 0 {
		t.Fatal("check must not send while sending is disabled")
	}
	w = h.send(call{method: "GET", target: "/api/admin/preview", s: s})
	if w.Code != 200 || !strings.Contains(w.Body.String(), "(TODAY)") || !strings.Contains(w.Body.String(), `"activities":1`) {
		t.Fatal("preview", w.Code, w.Body.String())
	}
	if posts, _ := h.up.snapshot(); posts != 0 {
		t.Fatal("preview must never send")
	}
}

func TestStatusReportsOmittedDetailsAndStaleness(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	var before struct {
		Agenda struct {
			UpdatedAt     *string        `json:"updatedAt"`
			OmittedEvents []omittedEvent `json:"omittedEvents"`
			Stale         bool           `json:"stale"`
			CacheMinutes  int            `json:"cacheMinutes"`
			LastError     *string        `json:"lastError"`
		} `json:"agenda"`
		WhatsApp struct {
			SendEnabled bool    `json:"sendEnabled"`
			LastCheckAt *string `json:"lastCheckAt"`
			LastError   *string `json:"lastError"`
		} `json:"whatsapp"`
		Configured map[string]bool `json:"configured"`
	}
	decodeJSON(t, h.send(call{method: "GET", target: "/api/admin/status", s: s}), &before)
	if before.Agenda.UpdatedAt != nil || before.Agenda.OmittedEvents == nil || len(before.Agenda.OmittedEvents) != 0 || before.Agenda.CacheMinutes != 60 {
		t.Fatalf("status before any copy: %+v", before.Agenda)
	}
	if !before.Configured["nbccFeed"] || !before.Configured["grapnel"] || !before.Configured["webhookSecret"] || !before.Configured["publicBaseUrl"] {
		t.Fatal("configured flags", before.Configured)
	}

	override := agendaEvent("uid-c2@test.invalid", "Sample Campus Ping", "Sample Campus", ";TZID=America/Halifax:20261020T100000", "RRULE:FREQ=HOURLY;COUNT=3\r\n")
	h.up.feed = testICS(append(sampleAgendaEvents(), override)...)
	if w := agendaGet(h, "/api/agenda"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	var after struct {
		Agenda struct {
			UpdatedAt     *string        `json:"updatedAt"`
			OmittedEvents []omittedEvent `json:"omittedEvents"`
			Stale         bool           `json:"stale"`
			LastError     *string        `json:"lastError"`
		} `json:"agenda"`
		WhatsApp struct {
			PendingJobs int     `json:"pendingJobs"`
			FailedJobs  int     `json:"failedJobs"`
			LastError   *string `json:"lastError"`
		} `json:"whatsapp"`
	}
	raw := h.send(call{method: "GET", target: "/api/admin/status", s: s})
	decodeJSON(t, raw, &after)
	if after.Agenda.UpdatedAt == nil || after.Agenda.Stale || after.Agenda.LastError != nil {
		t.Fatalf("status after a copy: %+v", after.Agenda)
	}
	if len(after.Agenda.OmittedEvents) != 1 || after.Agenda.OmittedEvents[0].Title != "Sample Campus Ping" || after.Agenda.OmittedEvents[0].Date != "2026-10-20" || after.Agenda.OmittedEvents[0].Reason != "high_frequency_recurrence" {
		t.Fatalf("omitted details: %+v", after.Agenda.OmittedEvents)
	}
	if after.WhatsApp.PendingJobs != 0 || after.WhatsApp.FailedJobs != 0 || after.WhatsApp.LastError != nil {
		t.Fatal("whatsapp counters", after.WhatsApp)
	}
	if strings.Contains(raw.Body.String(), "SENTINEL") {
		t.Fatal("status leaked a secret")
	}

	h.up.feedFail = true
	h.clock.advance(30 * time.Minute)
	h.send(call{method: "POST", target: "/api/admin/session/extend", s: s})
	h.clock.advance(31 * time.Minute)
	agendaGet(h, "/api/agenda")
	var stale struct {
		Agenda struct {
			Stale     bool    `json:"stale"`
			LastError *string `json:"lastError"`
		} `json:"agenda"`
	}
	decodeJSON(t, h.send(call{method: "GET", target: "/api/admin/status", s: s}), &stale)
	if !stale.Agenda.Stale || stale.Agenda.LastError == nil || *stale.Agenda.LastError != "feed_unavailable" {
		t.Fatalf("stale copy must be flagged with a generic error: %+v", stale.Agenda)
	}
}

func TestStatusLastCheckAfterCheck(t *testing.T) {
	h := newHarness(t)
	s := h.adminSession()
	h.send(call{method: "POST", target: "/api/admin/check", s: s})
	w := h.send(call{method: "GET", target: "/api/admin/status", s: s})
	if !strings.Contains(w.Body.String(), `"lastCheckAt":"2026-10-13T`) {
		t.Fatal("lastCheckAt must be set after a check", w.Body.String())
	}
}
