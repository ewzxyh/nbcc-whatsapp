package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNotificationUsesVisibleWindow(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("lab", "Lab - Due", ";VALUE=DATE:20261028", ""), testEvent("campus", "Campus", ";VALUE=DATE:20261014", ""))
	h.app.check(ctx)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	before, _ := h.up.snapshot()
	h.up.feed = strings.Replace(h.up.feed, "SUMMARY:Campus", "SUMMARY:Renamed Campus", 1)
	h.app.check(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != before {
		t.Fatal("invisible campus change sent the same agenda")
	}
	h.clock.advance(24 * time.Hour)
	h.app.check(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != before+2 {
		t.Fatal("new activity or today's separate campus message was missed", h.up.sent)
	}
}

func TestChangedActivityIsMarkedAndUnchangedResendIsNot(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("one", "Lab One - Due", ";TZID=America/Halifax:20261015T090000", ""), testEvent("two", "Lab Two - Due", ";VALUE=DATE:20261016", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	h.up.feed = strings.Replace(h.up.feed, "T090000", "T110000", 1)
	h.app.check(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 || !strings.Contains(h.up.sent[1], "🔄 *UPDATED* 🔄 *Lab One*") || strings.Contains(h.up.sent[1], "🔄 *UPDATED* 🔄 *Lab Two*") {
		t.Fatal("change did not mark only the edited activity", h.up.sent)
	}
	h.app.command(groupMessage{ID: "unchanged-resend"}, h.app.settings.current())
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 3 || strings.Contains(h.up.sent[2], "*UPDATED*") {
		t.Fatal("unchanged activity was marked again", h.up.sent)
	}
}

func TestPreviewWithoutNotificationStore(t *testing.T) {
	h := newHarness(t)
	h.app.store = nil
	p, err := h.app.preview(context.Background(), h.app.settings.current())
	if err != nil || p.Count == 0 {
		t.Fatal("read-only CLI preview must work before opening the notification store", err)
	}
}

func TestActivityCourseChangeKeepsChangeIdentity(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("one", "Lab One - Due", ";VALUE=DATE:20261015", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	h.up.feed = strings.Replace(h.up.feed, "LOCATION:Linux I: Linux Fundamentals", "LOCATION:Microsoft I: Windows", 1)
	h.app.check(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 || !strings.Contains(h.up.sent[1], "*Microsoft I* — 🔄 *UPDATED* 🔄 *Lab One*") {
		t.Fatal("course change was not identified as an edit", h.up.sent)
	}
}

func TestLegacyActivityBaselineSeedsWithoutSending(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("one", "Lab One - Due", ";VALUE=DATE:20261015", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	data, err := os.ReadFile(h.app.store.path)
	if err != nil {
		t.Fatal(err)
	}
	var legacy map[string]any
	if err = json.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	delete(legacy, "last_activity_versions")
	data, _ = json.Marshal(legacy)
	if err = os.WriteFile(h.app.store.path, data, 0600); err != nil {
		t.Fatal(err)
	}
	h.app.store, err = openStore(h.app.store.path)
	if err != nil {
		t.Fatal(err)
	}
	h.app.check(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("baseline migration caused a send", posts)
	}
	h.up.feed = strings.Replace(h.up.feed, "Lab One - Due", "Renamed Lab - Due", 1)
	h.app.check(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 || !strings.Contains(h.up.sent[1], "🔄 *UPDATED* 🔄 *Renamed Lab*") {
		t.Fatal("migrated baseline did not detect the first edit", h.up.sent)
	}
}

func TestCampusEventsAtEightAreSeparateAndOnlyForToday(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.clock.advance(-6*time.Hour - time.Second)
	h.up.feed = testICS(
		testEvent("lab", "Course Lab - Due", ";VALUE=DATE:20261016", ""),
		testEvent("lunch", "Campus Lunch", ";TZID=America/Halifax:20261013T123000", "DTEND;TZID=America/Halifax:20261013T131500\r\n"),
		testEvent("all-day", "Campus Fair", ";VALUE=DATE:20261013", ""),
		testEvent("yesterday", "Yesterday Event", ";VALUE=DATE:20261012", ""),
		testEvent("tomorrow", "Tomorrow Event", ";VALUE=DATE:20261014", ""),
	)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("campus events were sent before 08:00", h.up.sent)
	}
	h.clock.advance(time.Second)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("campus message was not sent at 08:00", h.up.sent)
	}
	message := h.up.sent[1]
	for _, want := range []string{"*NBCC — Campus Events Today*", "*(TODAY) Oct 13*", "*Campus Lunch*", "*12:30–13:15*", "*Campus Fair* — *All day*", "https://nbcc.example.test/", "Type *#NBCC* for the latest agenda."} {
		if !strings.Contains(message, want) {
			t.Fatal("campus message omitted", want, message)
		}
	}
	for _, absent := range []string{"Yesterday Event", "Tomorrow Event", "Course Lab"} {
		if strings.Contains(message, absent) {
			t.Fatal("campus message included another day or an activity", message)
		}
	}
	var err error
	h.app.store, err = openStore(h.app.store.path)
	if err != nil {
		t.Fatal(err)
	}
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("campus message repeated after restart", h.up.sent)
	}
	// The campus post must not replace the last accepted course agenda in deduplication.
	h.up.messages = []groupMessage{
		{ID: "campus", ChatID: testGroup, IsGroup: true, FromMe: ptrBool(true), Text: message},
		{ID: "agenda", ChatID: testGroup, IsGroup: true, FromMe: ptrBool(true), Text: h.up.sent[0]},
	}
	h.app.command(groupMessage{ID: "still-visible"}, h.app.settings.current())
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("campus message broke the recent-five agenda rule", h.up.sent)
	}
	h.up.feed = strings.Replace(h.up.feed, "Campus Lunch", "Updated Campus Lunch", 1)
	h.app.check(ctx)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 3 || !strings.Contains(h.up.sent[2], "*Updated Campus Lunch*") || strings.Contains(h.up.sent[2], "Course Lab") {
		t.Fatal("today's campus edit did not send only the campus list", h.up.sent)
	}
	h.up.feed = strings.ReplaceAll(h.up.feed, "20261013", "20261014")
	h.app.check(ctx)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 4 || !strings.Contains(h.up.sent[3], "No campus events scheduled for today.") {
		t.Fatal("canceled campus events were not corrected once", h.up.sent)
	}
}

func TestCampusRetryExpiresAtMidnightAndEmptyDaysStayQuiet(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.clock.advance(-6*time.Hour - time.Second)
	h.up.feed = testICS(testEvent("lab", "Lab - Due", ";VALUE=DATE:20261016", ""), testEvent("campus", "Campus Today", ";VALUE=DATE:20261013", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	h.clock.advance(time.Second)
	h.app.schedule(ctx)
	h.up.hook = func(r *http.Request) (*http.Response, bool) {
		if r.Method == "POST" {
			return testResponse(503, `{}`), true
		}
		return nil, false
	}
	h.app.processPending(ctx)
	h.clock.advance(24 * time.Hour)
	h.up.hook = nil
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("yesterday's failed campus message or an empty day was broadcast", h.up.sent)
	}
}

func TestCampusJobsDoNotConsumeDeadlineReminders(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("lab", "Lab - Due", ";TZID=America/Halifax:20261014T050000", ""), testEvent("campus", "Campus Today", ";VALUE=DATE:20261013", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	h.app.schedule(ctx)
	h.clock.advance(time.Hour)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 3 {
		t.Fatal("a campus job swallowed the deadline reminder", h.up.sent)
	}
	if strings.Contains(h.up.sent[1], "Lab") || !strings.Contains(h.up.sent[1], "*Campus Today*") || !strings.Contains(h.up.sent[2], "*_14 hours left_*") {
		t.Fatal("campus events and reminders were mixed", h.up.sent)
	}
}

func TestCommandCooldownAndRecentConversation(t *testing.T) {
	h := newHarness(t)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	ctx := context.Background()
	command := func(id string) {
		body := strings.Replace(commandBody(testGroup, false), `"messageid":"abc"`, `"messageid":"`+id+`"`, 1)
		if w := h.webhook(body, h.clock.now()); w.Code != 202 {
			t.Fatal(w.Code)
		}
		h.app.processPending(ctx)
	}
	command("first")
	command("storm")
	h.clock.advance(14 * time.Minute)
	command("early")
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("15 minute cooldown did not hold", posts)
	}
	h.clock.advance(time.Minute)
	h.up.messages = []groupMessage{{ID: "one", ChatID: testGroup, IsGroup: true, FromMe: ptrBool(false), Text: "Hi"}, {ID: "two", ChatID: testGroup, IsGroup: true, FromMe: ptrBool(false), Text: "Thanks"}, {ID: "three", ChatID: testGroup, IsGroup: true, FromMe: ptrBool(false), Text: "Lab"}, {ID: "four", ChatID: testGroup, IsGroup: true, FromMe: ptrBool(false), Text: "#NBCC"}, {ID: "agenda", ChatID: testGroup, IsGroup: true, FromMe: ptrBool(true), Text: h.up.sent[0]}}
	command("recent")
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("unchanged agenda still in last five was resent")
	}
	h.up.messages[4] = groupMessage{ID: "fifth-participant", ChatID: testGroup, IsGroup: true, FromMe: ptrBool(false), Text: "Another message"}
	command("out-of-view")
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("agenda outside last five was not resent", posts)
	}
	store, err := openStore(h.app.store.path)
	if err != nil {
		t.Fatal(err)
	}
	h.app.store = store
	command("after-restart")
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("cooldown was lost on restart")
	}
}

func TestAutomaticMessagesDoNotStartCommandCooldown(t *testing.T) {
	for _, kind := range []string{"agenda", "reminder"} {
		t.Run(kind, func(t *testing.T) {
			h := newHarness(t)
			ctx := context.Background()
			h.up.feed = testICS(testEvent("due", "Lab - Due", ";TZID=America/Halifax:20261014T050000", ""))
			h.settingsPatch(map[string]any{"sendEnabled": true})
			h.app.check(ctx)
			h.app.processPending(ctx)
			want := 2
			if kind == "reminder" {
				h.clock.advance(time.Hour)
				h.app.schedule(ctx)
				h.app.processPending(ctx)
				want++
			}
			// An empty conversation history means the automatic agenda has left the latest five.
			h.app.command(groupMessage{ID: "after-automatic"}, h.app.settings.current())
			h.app.processPending(ctx)
			if posts, _ := h.up.snapshot(); posts != want {
				t.Fatal("automatic message blocked an eligible command", posts)
			}
		})
	}
}

func TestAutomaticMessageDoesNotExtendCommandCooldown(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("due", "Lab - Due", ";VALUE=DATE:20261016", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.command(groupMessage{ID: "first-response"}, h.app.settings.current())
	h.app.processPending(ctx)
	h.clock.advance(5 * time.Minute)
	h.up.feed = strings.Replace(h.up.feed, "Lab - Due", "Updated Lab - Due", 1)
	h.app.check(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("automatic update was not sent", posts)
	}
	h.clock.advance(10 * time.Minute)
	h.app.command(groupMessage{ID: "exactly-after-15-minutes"}, h.app.settings.current())
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 3 {
		t.Fatal("automatic update prolonged the command cooldown", posts)
	}
}

func TestCommandCountdownIsUpdatedAfterHistoryLookup(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.clock.advance(-8 * time.Hour)
	h.up.feed = testICS(testEvent("today", "Lab Today - Due", ";TZID=America/Halifax:20261013T235900", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.command(groupMessage{ID: "first"}, h.app.settings.current())
	if err := h.app.processPending(ctx); err != nil {
		t.Fatal(err)
	}
	h.clock.advance(16 * time.Minute)
	h.up.hook = func(r *http.Request) (*http.Response, bool) {
		if strings.HasSuffix(r.URL.Path, "/groups/messages") {
			h.clock.advance(3 * time.Minute)
		}
		return nil, false
	}
	h.app.command(groupMessage{ID: "latest"}, h.app.settings.current())
	if err := h.app.processPending(ctx); err != nil {
		t.Fatal(err)
	}
	if posts, _ := h.up.snapshot(); posts != 2 || !strings.Contains(h.up.sent[0], "*_17h 59m left_*") || !strings.Contains(h.up.sent[1], "*_17h 40m left_*") {
		t.Fatal("command response did not use the current remaining time", h.up.sent)
	}
	for _, text := range h.up.sent {
		if !strings.HasSuffix(text, "\n\n🔗 *Full agenda:* https://nbcc.example.test/\nType *#NBCC* for the latest agenda.") {
			t.Fatal("command response omitted the agenda link", text)
		}
	}
}

func ptrBool(v bool) *bool { return &v }

func TestDisableStopsRemainingMessageParts(t *testing.T) {
	h := newHarness(t)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.store.enqueue("test:long", strings.Repeat("x", 5000), true, h.clock.now())
	posts := 0
	h.up.hook = func(r *http.Request) (*http.Response, bool) {
		if r.Method != "POST" {
			return nil, false
		}
		posts++
		if posts == 1 {
			h.settingsPatch(map[string]any{"sendEnabled": false})
		}
		return testResponse(202, `{}`), true
	}
	h.app.processPending(context.Background())
	if posts != 1 {
		t.Fatal("continued after disable", posts)
	}
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.processPending(context.Background())
	if posts != 1 {
		t.Fatal("cancelled parts replayed after enable")
	}
}

func TestPreviewOmitsBadCampusAndKeepsDistinctActivities(t *testing.T) {
	raw := testICS(testEvent("a", "Same Lab - Due", ";VALUE=DATE:20261014", ""), testEvent("b", "Same Lab - Due", ";VALUE=DATE:20261016", ""), testEvent("campus", "Campus", ";VALUE=DATE:20261015", "RECURRENCE-ID;VALUE=DATE:20261015\r\n"))
	p, err := testPreview([]byte(raw), testNow())
	if err != nil || p.Count != 2 {
		t.Fatal("campus omission or ambiguous identity lost activities", p.Count, err)
	}
}

func TestReminderThresholdsUseDueAndSurviveRestart(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("due", "Deadline - Due", ";TZID=America/Halifax:20261014T050000", "DESCRIPTION:https://elearning.nbcc.ca/d2l/?ou=999999&qi=100\r\n"), testEvent("end", "Deadline - Availability Ends", ";TZID=America/Halifax:20261015T050000", "DESCRIPTION:https://elearning.nbcc.ca/d2l/?ou=999999&qi=100\r\n"))
	h.app.check(ctx)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("activation did not send initial agenda")
	}
	h.clock.advance(time.Hour)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("14 hour reminder missing", posts)
	}
	if !strings.Contains(h.up.sent[1], "⚠️ *_14 hours left_* *Deadline*") {
		t.Fatal(h.up.sent[1])
	}
	store, err := openStore(h.app.store.path)
	if err != nil {
		t.Fatal(err)
	}
	h.app.store = store
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("reminder repeated after restart")
	}
	h.clock.advance(10 * time.Hour)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 3 {
		t.Fatal("4 hour reminder missing", posts)
	}
	if !strings.Contains(h.up.sent[2], "⚠️ *_4 hours left_* *Deadline*") {
		t.Fatal(h.up.sent[2])
	}
	h.clock.advance(4 * time.Hour)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 4 || strings.Contains(h.up.sent[3], "Deadline") {
		t.Fatal("expired activity was not removed from the updated agenda", h.up.sent)
	}
}

func TestRecentAgendaSuppressesAndConsumesReminderStages(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("due", "Deadline - Due", ";TZID=America/Halifax:20261014T050000", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	for i := 0; i < 4; i++ {
		h.up.messages = append(h.up.messages, groupMessage{ID: fmt.Sprint(i), ChatID: testGroup, IsGroup: true, FromMe: ptrBool(false), Text: "Participant message"})
	}
	h.up.messages = append(h.up.messages, groupMessage{ID: "recent-agenda", ChatID: testGroup, IsGroup: true, FromMe: ptrBool(true), Text: h.up.sent[0]})
	for stage, advance := range []time.Duration{time.Hour, 10 * time.Hour} {
		h.clock.advance(advance)
		if err := h.app.schedule(ctx); err != nil {
			t.Fatal(err)
		}
		if err := h.app.processPending(ctx); err != nil {
			t.Fatal(err)
		}
		if posts, _ := h.up.snapshot(); posts != 1 {
			t.Fatal("unchanged agenda in the latest five was resent for a reminder", stage, posts)
		}
		store, err := openStore(h.app.store.path)
		if err != nil {
			t.Fatal(err)
		}
		h.app.store = store
		if len(store.data.Reminders) != stage+1 {
			t.Fatal("suppressed reminder stage was not persisted", stage)
		}
		h.app.schedule(ctx)
		h.app.processPending(ctx)
		if pending, _ := store.counts(); pending != 0 {
			t.Fatal("suppressed reminder was requeued after restart", pending)
		}
	}
	h.up.feed = strings.Replace(h.up.feed, "Deadline - Due", "Updated Deadline - Due", 1)
	h.app.check(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 || !strings.Contains(h.up.sent[1], "Updated Deadline") {
		t.Fatal("meaningful agenda change was suppressed despite the old agenda remaining visible", posts)
	}
}

func TestReminderRetryRechecksRecentConversation(t *testing.T) {
	for _, failure := range []string{"history", "send"} {
		t.Run(failure, func(t *testing.T) {
			h := newHarness(t)
			ctx := context.Background()
			h.up.feed = testICS(testEvent("due", "Deadline - Due", ";TZID=America/Halifax:20261014T050000", ""))
			h.settingsPatch(map[string]any{"sendEnabled": true})
			h.app.check(ctx)
			h.app.processPending(ctx)
			h.clock.advance(time.Hour)
			h.up.hook = func(r *http.Request) (*http.Response, bool) {
				if failure == "history" && strings.HasSuffix(r.URL.Path, "/groups/messages") || failure == "send" && r.Method == "POST" {
					return testResponse(503, `{}`), true
				}
				return nil, false
			}
			h.app.schedule(ctx)
			h.app.processPending(ctx)
			if posts, _ := h.up.snapshot(); posts != 1 {
				t.Fatal("failed recent-history lookup sent a duplicate reminder", posts)
			}
			if pending, _ := h.app.store.counts(); pending != 1 || len(h.app.store.data.Reminders) != 0 {
				t.Fatal("failure consumed or lost the reminder instead of retrying", pending)
			}
			h.up.hook = nil
			h.up.messages = []groupMessage{{ID: "recent-agenda", ChatID: testGroup, IsGroup: true, FromMe: ptrBool(true), Text: h.up.sent[0]}}
			h.clock.advance(2 * time.Minute)
			h.app.processPending(ctx)
			if posts, _ := h.up.snapshot(); posts != 1 {
				t.Fatal("retry ignored the unchanged agenda now visible in the latest five", posts)
			}
			if pending, _ := h.app.store.counts(); pending != 0 || len(h.app.store.data.Reminders) != 1 {
				t.Fatal("suppressed retry did not consume its reminder stage", pending)
			}
		})
	}
}

func TestDuePassageUpdatesAgendaWithoutFeedChange(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("soon", "Expiring - Due", ";TZID=America/Halifax:20261013T140030", ""), testEvent("later", "Upcoming - Due", ";VALUE=DATE:20261015", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	if _, _, err := h.app.check(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.app.processPending(ctx); err != nil {
		t.Fatal(err)
	}
	h.clock.advance(30 * time.Second)
	if err := h.app.schedule(ctx); err != nil {
		t.Fatal(err)
	}
	if pending, _ := h.app.store.counts(); pending != 1 {
		t.Fatal("deadline passage did not queue the remaining agenda", pending)
	}
	if err := h.app.processPending(ctx); err != nil {
		t.Fatal(err)
	}
	if posts, _ := h.up.snapshot(); posts != 2 || strings.Contains(h.up.sent[1], "Expiring") || !strings.Contains(h.up.sent[1], "Upcoming") {
		t.Fatal("sent an expired Due or lost the future one", h.up.sent)
	}
}

func TestExpiredDueCannotSurviveRetry(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("soon", "Expiring - Due", ";TZID=America/Halifax:20261013T140030", ""), testEvent("later", "Upcoming - Due", ";VALUE=DATE:20261015", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	var texts, keys []string
	h.up.hook = func(r *http.Request) (*http.Response, bool) {
		if r.Method != "POST" {
			return nil, false
		}
		var body struct{ Text string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		texts = append(texts, body.Text)
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if len(texts) == 1 {
			return testResponse(503, `{}`), true
		}
		return testResponse(202, `{}`), true
	}
	h.app.processPending(ctx)
	h.clock.advance(2 * time.Minute)
	h.app.processPending(ctx)
	if len(texts) != 2 || !strings.Contains(texts[0], "Expiring") || strings.Contains(texts[1], "Expiring") || !strings.Contains(texts[1], "Upcoming") || keys[0] == keys[1] {
		t.Fatal("expired retry body was sent or its idempotency key was reused", texts, keys)
	}
}

func TestDuePassageStopsRemainingAgendaParts(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	events := []string{testEvent("soon", "Expiring - Due", ";TZID=America/Halifax:20261013T140030", "")}
	for i := 0; i < 80; i++ {
		events = append(events, testEvent(fmt.Sprint(i), fmt.Sprint(i)+strings.Repeat("Long title ", 8)+" - Due", ";VALUE=DATE:20261015", ""))
	}
	h.up.feed = testICS(events...)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.up.hook = func(r *http.Request) (*http.Response, bool) {
		if r.Method == "POST" {
			h.clock.advance(time.Minute)
		}
		return nil, false
	}
	if err := h.app.processPending(ctx); err != nil {
		t.Fatal(err)
	}
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("continued the old batch after its Due passed", posts)
	}
	h.up.hook = nil
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts < 2 {
		t.Fatal("remaining future activities were not sent")
	}
	for _, text := range h.up.sent[1:] {
		if strings.Contains(text, "Expiring") {
			t.Fatal("expired Due appeared in the rebuilt batch", text)
		}
	}
}

func TestAgendaLinkChangeStopsRemainingParts(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	var events []string
	for i := 0; i < 80; i++ {
		events = append(events, testEvent(fmt.Sprint(i), fmt.Sprint(i)+strings.Repeat("Long title ", 8)+" - Due", ";VALUE=DATE:20261015", ""))
	}
	h.up.feed = testICS(events...)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.up.hook = func(r *http.Request) (*http.Response, bool) {
		if r.Method == "POST" {
			h.settingsPatch(map[string]any{"publicBaseUrl": "https://new.example.test"})
		}
		return nil, false
	}
	if err := h.app.processPending(ctx); err != nil {
		t.Fatal(err)
	}
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("continued sending with the old agenda link", posts)
	}
	h.up.hook = nil
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts < 2 {
		t.Fatal("agenda with the new link was not sent")
	}
	for _, text := range h.up.sent[1:] {
		if !strings.HasSuffix(text, "\n\n🔗 *Full agenda:* https://new.example.test/\nType *#NBCC* for the latest agenda.") || strings.Contains(text, "https://nbcc.example.test/") {
			t.Fatal("new batch lost its new agenda link", text)
		}
	}
}

func TestOlderRetryCannotFollowNewAgenda(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("a", "Lab - Due", ";TZID=America/Halifax:20261015T090000", ""))
	h.app.check(ctx)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	fail := true
	h.up.hook = func(r *http.Request) (*http.Response, bool) {
		if r.Method == "POST" && fail {
			fail = false
			return testResponse(503, `{}`), true
		}
		return nil, false
	}
	h.up.feed = strings.Replace(h.up.feed, "T090000", "T100000", 1)
	h.app.check(ctx)
	h.app.processPending(ctx)
	h.clock.advance(time.Minute)
	h.up.feed = strings.Replace(h.up.feed, "T100000", "T110000", 1)
	h.app.check(ctx)
	h.app.processPending(ctx)
	h.clock.advance(3 * time.Minute)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 || strings.Contains(h.up.sent[len(h.up.sent)-1], "*10:00*") {
		t.Fatal("old retry followed the new agenda", h.up.sent)
	}
}

func TestOwnNumberCommandFromHistory(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	h.clock.advance(16 * time.Minute)
	h.up.messages = []groupMessage{{ID: "own-command", ChatID: testGroup, IsGroup: true, FromMe: ptrBool(true), Text: "#NBCC", Timestamp: h.clock.now().UnixMilli()}}
	if err := h.app.pollCommands(ctx); err != nil {
		t.Fatal(err)
	}
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("own-number command not processed", posts)
	}
	h.app.pollCommands(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("polled command duplicated")
	}
}

func TestRecentHistoryTrustBoundary(t *testing.T) {
	h := newHarness(t)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.up.hook = func(r *http.Request) (*http.Response, bool) {
		if strings.HasSuffix(r.URL.Path, "/groups/messages") {
			b, _ := json.Marshal(map[string]any{"messages": []map[string]any{{"messageid": "wrong", "chatid": "123456789@g.us", "isGroup": true, "fromMe": true}}})
			return testResponse(200, string(b)), true
		}
		return nil, false
	}
	if _, err := h.app.groupMessages(context.Background(), h.app.settings.current(), 5, 0); err == nil {
		t.Fatal("accepted wrong chat in history")
	}
}

func TestParallelCommandsQueueOneResponse(t *testing.T) {
	h := newHarness(t)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m := groupMessage{ID: strings.Repeat("x", i+1), Text: "#NBCC"}
			if err := h.app.command(m, h.app.settings.current()); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if count, _ := h.app.store.counts(); count != 1 {
		t.Fatal("parallel requests queued multiple agendas", count)
	}
	h.app.processPending(context.Background())
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("parallel requests sent multiple agendas", posts)
	}
}

func TestReminderCatchUpAndDeadlineChanges(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("a", "Deadline - Due", ";TZID=America/Halifax:20261013T180000", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 1 || !strings.Contains(h.up.sent[0], "*_4 hours left_*") {
		t.Fatal("catch-up did not combine activation and urgent reminder", h.up.sent)
	}
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("late 14 hour stage followed 4 hour stage")
	}
	h.up.feed = strings.Replace(h.up.feed, "T180000", "T200000", 1)
	h.app.check(ctx)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("changed deadline did not reset reminder stage")
	}
	h.clock.advance(2 * time.Hour)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 3 || !strings.Contains(h.up.sent[2], "*_4 hours left_*") {
		t.Fatal("new 4 hour stage missing", h.up.sent)
	}
}

func TestFailed14HourReminderIsReplacedBy4HourReminder(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.up.feed = testICS(testEvent("a", "Deadline - Due", ";TZID=America/Halifax:20261014T050000", ""))
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	h.clock.advance(time.Hour)
	h.up.hook = func(r *http.Request) (*http.Response, bool) {
		if r.Method == "POST" {
			return testResponse(503, `{}`), true
		}
		return nil, false
	}
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	h.clock.advance(10 * time.Hour)
	h.up.hook = nil
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	h.app.schedule(ctx)
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 || !strings.Contains(h.up.sent[1], "*_4 hours left_*") {
		t.Fatal("late reminders were duplicated or not upgraded", h.up.sent)
	}
}

func TestInvisibleChangesDoNotChangeFingerprint(t *testing.T) {
	base := testEvent("lab", "Lab - Due", ";VALUE=DATE:20261014", "")
	for _, pair := range [][2]string{
		{testICS(base, testEvent("later", "Later - Due", ";VALUE=DATE:20261201", "")), testICS(base, testEvent("later", "Later - Due", ";VALUE=DATE:20261202", ""))},
		{testICS(base), testICS(strings.Replace(base, "END:VEVENT", "SEQUENCE:2\r\nEND:VEVENT", 1))},
	} {
		a, err := testPreview([]byte(pair[0]), testNow())
		if err != nil {
			t.Fatal(err)
		}
		b, err := testPreview([]byte(pair[1]), testNow())
		if err != nil {
			t.Fatal(err)
		}
		if a.FeedHash != b.FeedHash {
			t.Fatal("invisible change altered fingerprint")
		}
	}
}

func TestDelayedCommandInsideCooldownIsNeverReplayed(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.webhook(commandBody(testGroup, false), h.clock.now())
	h.app.processPending(ctx)
	inside := h.clock.now().Add(5 * time.Minute)
	h.clock.advance(5 * time.Minute)
	h.app.command(groupMessage{ID: "during-cooldown", Timestamp: inside.UnixMilli()}, h.app.settings.current())
	h.app.processPending(ctx)
	h.clock.advance(11 * time.Minute)
	if err := h.app.command(groupMessage{ID: "delayed", Timestamp: inside.UnixMilli()}, h.app.settings.current()); err != nil {
		t.Fatal(err)
	}
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("ignored command was replayed after cooldown")
	}
	h.app.command(groupMessage{ID: "after-cooldown", Timestamp: h.clock.now().UnixMilli()}, h.app.settings.current())
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 2 {
		t.Fatal("a fresh command after cooldown was not processed", posts)
	}
}

func TestUnavailableRecentHistoryDoesNotResend(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.check(ctx)
	h.app.processPending(ctx)
	h.clock.advance(16 * time.Minute)
	h.up.hook = func(r *http.Request) (*http.Response, bool) {
		if strings.HasSuffix(r.URL.Path, "/groups/messages") {
			return testResponse(503, `{}`), true
		}
		return nil, false
	}
	h.app.command(groupMessage{ID: "retry-history"}, h.app.settings.current())
	h.app.processPending(ctx)
	if posts, _ := h.up.snapshot(); posts != 1 {
		t.Fatal("failed history lookup triggered duplicate agenda")
	}
}

func TestLegacyStateMigrationPreservesHistory(t *testing.T) {
	h := newHarness(t)
	legacy := `{"feed_hash":"old","jobs":{"event:old":{"id":"event:old","status":"pending","text":"stale"},"event:disabled":{"id":"event:disabled","status":"suppressed"}}}`
	if err := os.WriteFile(h.app.store.path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := openStore(h.app.store.path)
	if err != nil {
		t.Fatal(err)
	}
	if s.currentJob("event:old").Status != "superseded" || s.currentJob("event:disabled").Status != "suppressed" || s.data.FeedHash != "old" {
		t.Fatal("unsafe or destructive state migration")
	}
}

func TestDisabledDuringFailedSendCancelsRetry(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.app.store.enqueue("test:failure", "text", true, h.clock.now())
	attempts := 0
	h.up.hook = func(r *http.Request) (*http.Response, bool) {
		if r.Method == "POST" {
			attempts++
			h.settingsPatch(map[string]any{"sendEnabled": false})
			return testResponse(503, `{}`), true
		}
		return nil, false
	}
	h.app.processPending(ctx)
	h.settingsPatch(map[string]any{"sendEnabled": true})
	h.clock.advance(10 * time.Minute)
	h.app.processPending(ctx)
	if attempts != 1 {
		t.Fatal("retry survived disabling during failed POST")
	}
}
