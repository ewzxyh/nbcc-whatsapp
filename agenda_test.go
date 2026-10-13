package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Synthetic upstream feed for the agenda endpoint; every identifier and name here is fictional.
type agendaUpstream struct {
	body    string
	fail    bool
	fetches int
}

func agendaTestApp(t *testing.T, clock *time.Time, up *agendaUpstream, instructors map[string]string) *App {
	t.Helper()
	client := &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		up.fetches++
		if up.fail {
			return testResponse(500, "upstream failure"), nil
		}
		return testResponse(200, up.body), nil
	})}
	cfg := Config{FeedURL: "https://elearning.nbcc.ca/d2l/le/calendar/feed/user/feed.ics?token=test", AdminToken: testAdmin, WebhookSecret: testSecret, WebDir: t.TempDir(), CourseInstructors: instructors}
	return &App{cfg: cfg, client: client, now: func() time.Time { return *clock }}
}

func agendaEvent(uid, summary, location, dtstart, extra string) string {
	return "BEGIN:VEVENT\r\nUID:" + uid + "\r\nSUMMARY:" + summary + "\r\nLOCATION:" + location + "\r\nDTSTART" + dtstart + "\r\n" + extra + "END:VEVENT\r\n"
}

func sampleAgendaFeed() string {
	const lab = "Sample I: Sample Fundamentals"
	return testICS(
		agendaEvent("uid-0001@test.invalid", "Sample Lab 01 - Available", lab, ":20261013T130000Z", ""),
		agendaEvent("uid-0002@test.invalid", "Sample Lab 01 - Due", lab, ";TZID=America/Halifax:20261020T235900", ""),
		agendaEvent("uid-0003@test.invalid", "Sample Lab 01 - Availability Ends", lab, ";VALUE=DATE:20261021", ""),
		agendaEvent("uid-0004@test.invalid", "Quiz Special Access: Sample Quiz 02 - Due", lab, ";TZID=America/Halifax:20261027T170000", "DESCRIPTION:Sample quiz notes: https://elearning.nbcc.ca/d2l/lms/quizzing/quizzing.d2l?ou=111111&amp;qi=222222\r\n"),
		agendaEvent("uid-0005@test.invalid", "Sample Career Fair", "Sample Campus - Library (New Brunswick Community College)", ";TZID=America/Halifax:20261015T123000", "DTEND;TZID=America/Halifax:20261015T133000\r\nDESCRIPTION:Sample description text https://example.invalid/private\r\n"),
		agendaEvent("uid-0006@test.invalid", "Sample Virtual Session", "https://example.invalid/meet/sample", ";TZID=America/Halifax:20261016T090000", "DESCRIPTION:Sample join notes\r\n"),
		agendaEvent("uid-0007@test.invalid", "Sample Cancelled Event", "Sample Campus - Room 1", ";VALUE=DATE:20261014", "STATUS:CANCELLED\r\n"),
		agendaEvent("uid-0008@test.invalid", "Sample Weekly Practice", "Sample Campus - Room 2", ";TZID=America/Halifax:20261014T180000", "DTEND;TZID=America/Halifax:20261014T190000\r\nRRULE:FREQ=WEEKLY;COUNT=4\r\nEXDATE;TZID=America/Halifax:20261021T180000\r\n"),
		agendaEvent("uid-0009@test.invalid", "Sample Open House", "Sample Campus - Gym", ";VALUE=DATE:20261110", "DTEND;VALUE=DATE:20261113\r\n"),
		agendaEvent("uid-0010@test.invalid", "Sample Walk-in Help", "Sample Campus - Help Desk", ":20261016T130000Z", ""),
		agendaEvent("uid-0011@test.invalid", "Sample Old Event", "Sample Campus", ";VALUE=DATE:20260928", ""),
		agendaEvent("uid-0012@test.invalid", "Sample Window Start", "Sample Campus", ";VALUE=DATE:20260929", ""),
		agendaEvent("uid-0013@test.invalid", "Sample Window End", "Sample Campus", ";VALUE=DATE:20270111", ""),
		agendaEvent("uid-0014@test.invalid", "Sample After Window", "Sample Campus", ";VALUE=DATE:20270112", ""),
		agendaEvent("uid-0015@test.invalid", "Sample Lab 09 - Due", lab, ";TZID=America/Halifax:20260901T120000", ""),
	)
}

type replyItem struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Phase       *string `json:"phase"`
	Title       string  `json:"title"`
	Course      *string `json:"course"`
	CourseTitle *string `json:"courseTitle"`
	Instructor  *string `json:"instructor"`
	ActivityID  *string `json:"activityId"`
	Location    *string `json:"location"`
	AllDay      bool    `json:"allDay"`
	Date        string  `json:"date"`
	StartAt     *string `json:"startAt"`
	EndAt       *string `json:"endAt"`
	EndDate     *string `json:"endDate"`
}

type agendaReply struct {
	Timezone  string      `json:"timezone"`
	Today     string      `json:"today"`
	From      string      `json:"from"`
	To        string      `json:"to"`
	UpdatedAt string      `json:"updatedAt"`
	Items     []replyItem `json:"items"`
}

func agendaGet(t *testing.T, a *App, target string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, httptest.NewRequest("GET", target, nil))
	return w
}

func decodeAgenda(t *testing.T, w *httptest.ResponseRecorder) agendaReply {
	t.Helper()
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var reply agendaReply
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	return reply
}

func nilOr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func titled(items []replyItem, title string) []replyItem {
	var out []replyItem
	for _, it := range items {
		if it.Title == title {
			out = append(out, it)
		}
	}
	return out
}

func TestAgendaGroupsActivitiesAndIncludesEvents(t *testing.T) {
	clock := testNow()
	up := &agendaUpstream{body: sampleAgendaFeed()}
	a := agendaTestApp(t, &clock, up, map[string]string{"Sample I": "Sample Instructor"})
	w := agendaGet(t, a, "/api/agenda")
	reply := decodeAgenda(t, w)
	if w.Header().Get("Cache-Control") != "public, max-age=300" || w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatal(w.Header())
	}
	if reply.Timezone != "America/Moncton" || reply.Today != "2026-10-13" || reply.From != "2026-09-29" || reply.To != "2027-01-11" || reply.UpdatedAt != "2026-10-13T14:00:00-03:00" {
		t.Fatalf("header: %+v", reply)
	}
	if up.fetches != 1 {
		t.Fatal("upstream fetches", up.fetches)
	}

	lab := titled(reply.Items, "Sample Lab 01")
	if len(lab) != 3 {
		t.Fatalf("expected one item per phase for the assessment, got %d", len(lab))
	}
	byPhase := map[string]replyItem{}
	for _, it := range lab {
		byPhase[nilOr(it.Phase)] = it
		if it.Type != "activity" || nilOr(it.ActivityID) != nilOr(lab[0].ActivityID) || nilOr(it.ActivityID) == "<nil>" || nilOr(it.Course) != "Sample I" || nilOr(it.CourseTitle) != "Sample Fundamentals" || nilOr(it.Instructor) != "Sample Instructor" || it.Location != nil {
			t.Fatalf("activity fields: %+v", it)
		}
	}
	if got := byPhase["available"]; nilOr(got.StartAt) != "2026-10-13T10:00:00-03:00" || got.Date != "2026-10-13" || got.AllDay || got.EndAt != nil {
		t.Fatalf("available phase: %+v", got)
	}
	if got := byPhase["due"]; nilOr(got.StartAt) != "2026-10-20T23:59:00-03:00" || got.Date != "2026-10-20" || got.AllDay || got.EndAt != nil {
		t.Fatalf("due phase: %+v", got)
	}
	if got := byPhase["closes"]; !got.AllDay || got.Date != "2026-10-21" || got.StartAt != nil || got.EndDate != nil {
		t.Fatalf("closes phase: %+v", got)
	}

	quiz := titled(reply.Items, "Sample Quiz 02")
	if len(quiz) != 1 || quiz[0].Type != "activity" || nilOr(quiz[0].Phase) != "due" || nilOr(quiz[0].StartAt) != "2026-10-27T17:00:00-03:00" || nilOr(quiz[0].ActivityID) == nilOr(lab[0].ActivityID) || nilOr(quiz[0].Instructor) != "Sample Instructor" {
		t.Fatalf("quiz: %+v", quiz)
	}

	fair := titled(reply.Items, "Sample Career Fair")
	if len(fair) != 1 || fair[0].Type != "event" || fair[0].Phase != nil || fair[0].Course != nil || fair[0].Instructor != nil || fair[0].ActivityID != nil || nilOr(fair[0].Location) != "Sample Campus - Library" || nilOr(fair[0].StartAt) != "2026-10-15T12:30:00-03:00" || nilOr(fair[0].EndAt) != "2026-10-15T13:30:00-03:00" {
		t.Fatalf("career fair: %+v", fair)
	}
	if got := titled(reply.Items, "Sample Virtual Session"); len(got) != 1 || got[0].Location != nil || nilOr(got[0].StartAt) != "2026-10-16T09:00:00-03:00" || got[0].EndAt != nil {
		t.Fatalf("virtual session: %+v", got)
	}
	if got := titled(reply.Items, "Sample Walk-in Help"); len(got) != 1 || nilOr(got[0].StartAt) != "2026-10-16T10:00:00-03:00" || got[0].EndAt != nil || nilOr(got[0].Location) != "Sample Campus - Help Desk" {
		t.Fatalf("UTC event converted to Moncton: %+v", got)
	}

	weekly := titled(reply.Items, "Sample Weekly Practice")
	wantStarts := []string{"2026-10-14T18:00:00-03:00", "2026-10-28T18:00:00-03:00", "2026-11-04T18:00:00-04:00"}
	if len(weekly) != len(wantStarts) {
		t.Fatalf("recurring event occurrences: %+v", weekly)
	}
	for i, it := range weekly {
		if nilOr(it.StartAt) != wantStarts[i] {
			t.Fatalf("occurrence %d starts at %s", i, nilOr(it.StartAt))
		}
	}
	if nilOr(weekly[2].EndAt) != "2026-11-04T19:00:00-04:00" || weekly[0].ID == weekly[1].ID {
		t.Fatalf("recurring occurrence end or identity: %+v", weekly)
	}

	open := titled(reply.Items, "Sample Open House")
	if len(open) != 1 || !open[0].AllDay || open[0].Date != "2026-11-10" || nilOr(open[0].EndDate) != "2026-11-12" || open[0].StartAt != nil || open[0].EndAt != nil {
		t.Fatalf("multi-day all-day event: %+v", open)
	}

	for _, title := range []string{"Sample Cancelled Event", "Sample Old Event", "Sample After Window", "Sample Lab 09"} {
		if got := titled(reply.Items, title); len(got) != 0 {
			t.Fatalf("%q should be outside the agenda: %+v", title, got)
		}
	}
	if got := titled(reply.Items, "Sample Window Start"); len(got) != 1 || got[0].Date != "2026-09-29" {
		t.Fatalf("window start is inclusive: %+v", got)
	}
	if got := titled(reply.Items, "Sample Window End"); len(got) != 1 || got[0].Date != "2027-01-11" {
		t.Fatalf("window end is inclusive: %+v", got)
	}

	for i := 1; i < len(reply.Items); i++ {
		prev, cur := reply.Items[i-1], reply.Items[i]
		if prev.Date > cur.Date || prev.Date == cur.Date && prev.AllDay && !cur.AllDay {
			t.Fatalf("items out of order at %d", i)
		}
	}

	body := w.Body.String()
	for _, leak := range []string{"http", "elearning", "uid-", "UID", "@test.invalid", "ou=", "qi=", "111111", "222222", "Sample description", "Sample quiz notes", "DESCRIPTION", "example.invalid", "STATUS", "Sample Cancelled Event"} {
		if strings.Contains(body, leak) {
			t.Fatalf("response leaks %q", leak)
		}
	}
	if !strings.Contains(body, `"items":[`) {
		t.Fatal("items must be an array")
	}
}

func TestAgendaInstructorIsNullWhenNotConfigured(t *testing.T) {
	clock := testNow()
	a := agendaTestApp(t, &clock, &agendaUpstream{body: sampleAgendaFeed()}, nil)
	for _, it := range titled(decodeAgenda(t, agendaGet(t, a, "/api/agenda")).Items, "Sample Lab 01") {
		if it.Instructor != nil {
			t.Fatalf("instructor should be null: %+v", it)
		}
	}
}

func TestAgendaRejectsInvalidDateRanges(t *testing.T) {
	clock := testNow()
	a := agendaTestApp(t, &clock, &agendaUpstream{body: sampleAgendaFeed()}, nil)
	base := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	maxTo := base.AddDate(0, 0, 185).Format(dateLayout)
	tooLong := base.AddDate(0, 0, 186).Format(dateLayout)
	for _, target := range []string{
		"/api/agenda?from=2026-02-30",
		"/api/agenda?from=2026-1-5",
		"/api/agenda?to=10/13/2026",
		"/api/agenda?from=",
		"/api/agenda?from=2026-10-20&to=2026-10-19",
		"/api/agenda?from=2026-10-13&to=" + tooLong,
	} {
		w := agendaGet(t, a, target)
		if w.Code != 400 || strings.TrimSpace(w.Body.String()) != `{"error":"invalid date range"}` {
			t.Fatal(target, w.Code, w.Body.String())
		}
	}
	if w := agendaGet(t, a, "/api/agenda?from=2026-10-13&to="+maxTo); w.Code != 200 {
		t.Fatal("186-day span must be accepted", w.Code)
	}
}

func TestAgendaEmptyRangeReturnsEmptyArray(t *testing.T) {
	clock := testNow()
	a := agendaTestApp(t, &clock, &agendaUpstream{body: sampleAgendaFeed()}, nil)
	w := agendaGet(t, a, "/api/agenda?from=2026-01-01&to=2026-01-02")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestAgendaFeedIsFetchedOncePerTTL(t *testing.T) {
	clock := testNow()
	up := &agendaUpstream{body: sampleAgendaFeed()}
	a := agendaTestApp(t, &clock, up, nil)
	h := a.handler()
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/agenda", nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if up.fetches != 1 {
		t.Fatal("two requests inside the TTL fetched upstream", up.fetches)
	}
	clock = clock.Add(feedTTL - time.Second)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/agenda", nil))
	if up.fetches != 1 {
		t.Fatal("refetched before the TTL elapsed", up.fetches)
	}
	clock = clock.Add(time.Second)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/agenda", nil))
	if up.fetches != 2 {
		t.Fatal("expected refetch after the TTL", up.fetches)
	}
}

func TestAgendaConcurrentRequestsShareOneFetch(t *testing.T) {
	clock := testNow()
	up := &agendaUpstream{body: sampleAgendaFeed()}
	a := agendaTestApp(t, &clock, up, nil)
	h := a.handler()
	var wg sync.WaitGroup
	codes := make(chan int, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/api/agenda", nil))
			codes <- w.Code
		}()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 {
			t.Fatal("concurrent request failed", code)
		}
	}
	if up.fetches != 1 {
		t.Fatal("thundering herd: upstream fetches", up.fetches)
	}
}

func TestAgendaServesStaleCopyWhenRefreshFails(t *testing.T) {
	clock := testNow()
	up := &agendaUpstream{body: sampleAgendaFeed()}
	a := agendaTestApp(t, &clock, up, nil)
	first := decodeAgenda(t, agendaGet(t, a, "/api/agenda")).UpdatedAt
	up.fail = true

	clock = clock.Add(feedTTL + time.Minute)
	stale := agendaGet(t, a, "/api/agenda")
	if stale.Code != 200 || decodeAgenda(t, stale).UpdatedAt != first || up.fetches != 2 {
		t.Fatal("expected stale copy with original updatedAt", stale.Code, up.fetches)
	}
	clock = clock.Add(time.Minute)
	if w := agendaGet(t, a, "/api/agenda"); w.Code != 200 || up.fetches != 2 {
		t.Fatal("failed refresh must not be retried within the TTL", w.Code, up.fetches)
	}
	clock = clock.Add(feedTTL)
	if w := agendaGet(t, a, "/api/agenda"); w.Code != 200 || up.fetches != 3 {
		t.Fatal("expected a retry after the TTL, serving the stale copy", w.Code, up.fetches)
	}
	clock = clock.Add(time.Hour)
	w := agendaGet(t, a, "/api/agenda")
	if w.Code != 502 || strings.TrimSpace(w.Body.String()) != `{"error":"NBCC feed is unavailable"}` {
		t.Fatal("stale copy older than an hour must not be served", w.Code, w.Body.String())
	}
}

func TestAgendaUnavailableWithoutCopyDoesNotLeakFeed(t *testing.T) {
	clock := testNow()
	up := &agendaUpstream{fail: true}
	a := agendaTestApp(t, &clock, up, nil)
	for i := 0; i < 2; i++ {
		w := agendaGet(t, a, "/api/agenda")
		if w.Code != 502 || strings.TrimSpace(w.Body.String()) != `{"error":"NBCC feed is unavailable"}` || strings.Contains(w.Body.String(), "token") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if up.fetches != 1 {
		t.Fatal("failed refresh retried within the TTL", up.fetches)
	}
}

func TestAgendaRejectsFeedsItCannotRepresent(t *testing.T) {
	recurrenceOverride := agendaEvent("uid-r1@test.invalid", "Sample Lab 03 - Due", "Sample I: Sample Fundamentals", ";VALUE=DATE:20261020", "RECURRENCE-ID;VALUE=DATE:20261013\r\n")
	highFrequency := agendaEvent("uid-h1@test.invalid", "Sample Campus Ping", "Sample Campus", ";VALUE=DATE:20261020", "RRULE:FREQ=HOURLY;COUNT=3\r\n")
	for _, body := range []string{testICS(recurrenceOverride), testICS(highFrequency), testICS()} {
		clock := testNow()
		a := agendaTestApp(t, &clock, &agendaUpstream{body: body}, nil)
		w := agendaGet(t, a, "/api/agenda")
		if w.Code != 502 || strings.TrimSpace(w.Body.String()) != `{"error":"NBCC feed is unavailable"}` {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
