package main

import (
	"strings"
	"testing"
	"time"
)

func testPreview(raw []byte, now time.Time) (Preview, error) {
	return buildPreview(raw, now, make([]byte, 32), "https://nbcc.example.test")
}

func testNow() time.Time {
	loc, _ := time.LoadLocation("America/Halifax")
	return time.Date(2026, 10, 13, 14, 0, 0, 0, loc)
}

func testICS(events ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//NBCC Test//EN\r\n" + strings.Join(events, "") + "END:VCALENDAR\r\n"
}

func testEvent(uid, title, date, extra string) string {
	return "BEGIN:VEVENT\r\nUID:" + uid + "\r\nSUMMARY:" + title + "\r\nLOCATION:Linux I: Linux Fundamentals\r\nDTSTART" + date + "\r\n" + extra + "END:VEVENT\r\n"
}

func TestCalendarWindowTodayAndDuplicatePhases(t *testing.T) {
	raw := testICS(
		testEvent("past", "Old - Due", ":20261012T230000Z", ""),
		testEvent("due", "Lab01 - Due", ";TZID=America/Halifax:20261013T170000", "DESCRIPTION:https://elearning.nbcc.ca/d2l/?ou=999999&qi=100\r\n"),
		testEvent("end", "Lab01 - Availability Ends", ";TZID=America/Halifax:20261013T235959", "DESCRIPTION:https://elearning.nbcc.ca/d2l/?ou=999999&qi=100\r\n"),
		testEvent("last", "Assignment27 - Due", ";VALUE=DATE:20261027", ""),
		testEvent("future", "Outside - Due", ";VALUE=DATE:20261028", ""),
		testEvent("campus", "Therapy Dog on Campus", ";VALUE=DATE:20261013", ""),
	)
	preview, err := testPreview([]byte(raw), testNow())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"*(TODAY) Oct 13*", "Due at *17:00*", "*Oct 27*", "Assignment27"} {
		if !strings.Contains(preview.Text, want) {
			t.Errorf("missing %q in %s", want, preview.Text)
		}
	}
	for _, bad := range []string{"Old", "Outside", "Therapy Dog", "Halifax", "Access closes"} {
		if strings.Contains(preview.Text, bad) {
			t.Errorf("unexpected %q", bad)
		}
	}
	if strings.Count(preview.Text, "Lab01") != 1 || preview.Count != 2 {
		t.Fatal("duplicate activity or wrong window")
	}
}

func TestFeedFingerprintIgnoresFetchTimestampAndOrder(t *testing.T) {
	a := testEvent("a", "Lab01 - Due", ";VALUE=DATE:20261014", "DTSTAMP:20261013T100000Z\r\n")
	b := testEvent("b", "Lab02 - Due", ";VALUE=DATE:20261015", "DTSTAMP:20261013T100000Z\r\n")
	p1, err := testPreview([]byte(testICS(a, b)), testNow())
	if err != nil {
		t.Fatal(err)
	}
	p2, err := testPreview([]byte(strings.ReplaceAll(testICS(b, a), "20261013T100000Z", "20261013T120000Z")), testNow())
	if err != nil {
		t.Fatal(err)
	}
	if p1.FeedHash != p2.FeedHash {
		t.Fatal("timestamp/order created false feed change")
	}
	p3, err := testPreview([]byte(testICS(strings.Replace(a, "Lab01", "Lab03", 1), b)), testNow())
	if err != nil {
		t.Fatal(err)
	}
	if p1.FeedHash == p3.FeedHash {
		t.Fatal("real change was missed")
	}
}

func TestCalendarRecurrenceAndTimeZone(t *testing.T) {
	raw := testICS(testEvent("weekly", "Practice Quiz - Due", ";TZID=America/Halifax:20261013T090000", "RRULE:FREQ=WEEKLY;COUNT=3\r\nEXDATE;TZID=America/Halifax:20261020T090000\r\n"))
	p, err := testPreview([]byte(raw), testNow())
	if err != nil {
		t.Fatal(err)
	}
	if p.Count != 1 || strings.Contains(p.Text, "*Oct 20*") || strings.Contains(p.Text, "(TODAY)") || !strings.Contains(p.Text, "*09:00*") {
		t.Fatal(p.Text)
	}
	dst := time.Date(2026, 10, 30, 0, 0, 0, 0, testNow().Location())
	p, err = testPreview([]byte(testICS(testEvent("dst", "Quiz - Due", ":20261103T131000Z", ""))), dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Text, "*09:10*") {
		t.Fatal("DST conversion: " + p.Text)
	}
}

func TestCalendarRejectsInvalidFeed(t *testing.T) {
	for _, raw := range []string{"<html>Login</html>", testICS(testEvent("a", "Lab - Due", ":not-a-date", "")), testICS(testEvent("a", "Lab - Due", ";VALUE=DATE:20261014", ""), testEvent("a", "Lab - Due", ";VALUE=DATE:20261014", ""))} {
		if _, err := testPreview([]byte(raw), testNow()); err == nil {
			t.Fatal("accepted invalid feed")
		}
	}
}

func TestSameTitleDifferentAssessmentIDsRemainSeparate(t *testing.T) {
	description := func(id string) string {
		return "DESCRIPTION:Quizzes: Lab - https://elearning.nbcc.ca/d2l/lms/quizzing/quizzing.d2l?ou=999999&qi=" + id + "\r\n"
	}
	raw := testICS(testEvent("a", "Lab - Due", ";VALUE=DATE:20261014", description("100")), testEvent("b", "Lab - Due", ";VALUE=DATE:20261015", description("200")), testEvent("c", "Lab - Availability Ends", ";VALUE=DATE:20261014", description("100")))
	p, err := testPreview([]byte(raw), testNow())
	if err != nil {
		t.Fatal(err)
	}
	if p.Count != 2 {
		t.Fatal("different assessments with the same title were merged", p.Text)
	}
}

func TestPreviewIncludesOnlyUnexpiredDue(t *testing.T) {
	description := func(id string) string {
		return "DESCRIPTION:https://elearning.nbcc.ca/d2l/?ou=999999&qi=" + id + "\r\n"
	}
	raw := testICS(
		testEvent("past", "Expired - Due", ";TZID=America/Halifax:20261013T130000", description("100")),
		testEvent("past-end", "Expired - Availability Ends", ";VALUE=DATE:20261015", description("100")),
		testEvent("exact", "Exact deadline - Due", ";TZID=America/Halifax:20261013T140000", description("200")),
		testEvent("access", "Access only - Available", ";VALUE=DATE:20261014", description("300")),
		testEvent("access-end", "Access only - Availability Ends", ";VALUE=DATE:20261015", description("300")),
		testEvent("future", "Upcoming - Due", ";TZID=America/Halifax:20261013T170000", description("400")),
		testEvent("future-end", "Upcoming - Availability Ends", ";TZID=America/Halifax:20261015T230000", description("400")),
		testEvent("date", "Date only - Due", ";VALUE=DATE:20261013", ""),
	)
	p, err := testPreview([]byte(raw), testNow())
	if err != nil || p.Count != 2 || len(p.deadlines) != 2 {
		t.Fatal("wrong future Due activities", p.Count, err, p.Text)
	}
	for _, bad := range []string{"Expired", "Exact deadline", "Access only", "Access closes", "23:00"} {
		if strings.Contains(p.Text, bad) {
			t.Fatalf("included %q: %s", bad, p.Text)
		}
	}
	for _, want := range []string{"Due at *17:00*", "Due by *23:59*"} {
		if !strings.Contains(p.Text, want) {
			t.Fatalf("missing %q: %s", want, p.Text)
		}
	}
	changed, err := testPreview([]byte(strings.Replace(raw, "20261015T230000", "20261015T220000", 1)), testNow())
	if err != nil || p.FeedHash != changed.FeedHash {
		t.Fatal("access closing time changed the Due agenda", err)
	}
}

func TestDateOnlyDueExpiresAtEndOfLocalDate(t *testing.T) {
	raw := []byte(testICS(testEvent("date", "Date only - Due", ";VALUE=DATE:20261013", "")))
	at := time.Date(2026, 10, 13, 23, 59, 59, 0, halifax)
	for _, tc := range []struct {
		now  time.Time
		want int
	}{{at.Add(-time.Second), 1}, {at, 0}, {at.Add(time.Second), 0}} {
		p, err := testPreview(raw, tc.now)
		if err != nil || p.Count != tc.want {
			t.Fatal(tc.now, p.Count, err)
		}
	}
}

func TestTodayCountdownRefreshesBeyond14Hours(t *testing.T) {
	raw := []byte(testICS(testEvent("today", "Lab Today - Due", ";TZID=America/Halifax:20261013T235900", ""), testEvent("later", "Quiz Tomorrow - Due", ";VALUE=DATE:20261014", "")))
	now := time.Date(2026, 10, 13, 6, 0, 0, 0, halifax)
	first, err := testPreview(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	later, err := testPreview(raw, now.Add(25*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.Text, "⚠️ *_17h 59m left_* *Lab Today*") || !strings.Contains(later.Text, "⚠️ *_17h 34m left_* *Lab Today*") {
		t.Fatal("today's countdown was missing or stale", first.Text, later.Text)
	}
	if first.FeedHash != later.FeedHash || strings.Count(later.Text, "left_*") != 1 {
		t.Fatal("countdown created a feed change or warned about a distant deadline")
	}
	sameName := []byte(testICS(testEvent("same", "Linux I - Due", ";TZID=America/Halifax:20261013T070000", "")))
	p, err := testPreview(sameName, now)
	if err != nil || !strings.Contains(p.Text, "• *Linux I* — ⚠️ *_1 hour left_* *Linux I*: Due") {
		t.Fatal("warning must precede the activity title", err, p.Text)
	}
}
