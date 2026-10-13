package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	ical "github.com/emersion/go-ical"
)

var moncton, _ = time.LoadLocation("America/Moncton")

const (
	feedRetry = 5 * time.Minute  // a failed refresh is not retried sooner than this
	feedStale = 24 * time.Hour   // a copy this old may still be served when a refresh fails
	dateLayout = "2006-01-02"
)

var (
	agendaTagPattern = regexp.MustCompile(`<[^>]*>`)
	agendaURLPattern = regexp.MustCompile(`(?i)\S*(?:http|www\.|://)\S*`)
)

var phaseLabels = map[string]string{"Available": "available", "Due": "due", "Availability Ends": "closes"}

// feedCache holds the last good upstream copy. source is the SHA-256 of the feed URL it was fetched from.
type feedCache struct {
	mu        sync.Mutex
	source    [32]byte
	raw       []byte
	fetchedAt time.Time
	attemptAt time.Time
}

type agendaItem struct {
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
	at          time.Time
}

type agendaResponse struct {
	Timezone       string       `json:"timezone"`
	Today          string       `json:"today"`
	From           string       `json:"from"`
	To             string       `json:"to"`
	UpdatedAt      string       `json:"updatedAt"`
	OmittedEvents  int          `json:"omittedEvents"`
	Items          []agendaItem `json:"items"`
}

// omittedEvent is a campus event left out of the agenda. Title is plain text; Date is empty when the feed date is unreadable.
type omittedEvent struct {
	Title  string `json:"title"`
	Date   string `json:"date"`
	Reason string `json:"reason"`
	at     time.Time
}

// inWindow reports whether the event starts inside [from, until). An event whose start cannot be read counts in every window.
func (o omittedEvent) inWindow(from, until time.Time) bool {
	return o.at.IsZero() || (!o.at.Before(from) && o.at.Before(until))
}

// agendaProblem is an event-level problem with a reason code. Course-activity problems are fatal; campus-event problems omit the event.
type agendaProblem struct {
	reason  string
	message string
}

func (p *agendaProblem) Error() string { return p.message }

func newProblem(reason, message string) error {
	return &agendaProblem{reason: reason, message: message}
}

func problemReason(err error) string {
	var p *agendaProblem
	if errors.As(err, &p) {
		return p.reason
	}
	return "other"
}

// agendaSlot is one occurrence of a feed event inside the requested window.
type agendaSlot struct {
	start   time.Time // Moncton instant; all-day events use midnight of their first day
	end     time.Time // zero when a timed event has no end in the feed
	endDate string    // inclusive last day of a multi-day all-day event
	allDay  bool
}

type activityPhase struct {
	agendaSlot
	title, course, courseTitle string
}

func (s agendaSlot) fill(it *agendaItem) {
	it.AllDay = s.allDay
	it.Date = s.start.Format(dateLayout)
	it.at = s.start
	if s.allDay {
		it.EndDate = nullable(s.endDate)
		return
	}
	it.StartAt = ptr(s.start.Format(time.RFC3339))
	if !s.end.IsZero() {
		it.EndAt = ptr(s.end.Format(time.RFC3339))
	}
}

func ptr(s string) *string { return &s }

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// dayOf returns the Moncton calendar date of t as a UTC midnight, so date arithmetic avoids DST.
func dayOf(t time.Time) time.Time {
	y, m, d := t.In(moncton).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// dayStart returns midnight in Moncton on the calendar date of d.
func dayStart(d time.Time) time.Time {
	y, m, day := d.Date()
	return time.Date(y, m, day, 0, 0, 0, 0, moncton)
}

// defaultWindow returns the inclusive dates of the default agenda window: today minus 14 days through today plus 90.
func defaultWindow(today time.Time) (time.Time, time.Time) {
	return today.AddDate(0, 0, -14), today.AddDate(0, 0, 90)
}

func plainText(s string) string {
	s = html.UnescapeString(s)
	return cleanText(agendaURLPattern.ReplaceAllString(agendaTagPattern.ReplaceAllString(s, " "), " "))
}

func agendaLocation(s string) *string {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), " (New Brunswick Community College)"))
	lower := strings.ToLower(s)
	if strings.Contains(lower, "http") || strings.Contains(lower, "://") || strings.Contains(lower, "www.") {
		return nil
	}
	return nullable(plainText(s))
}

func agendaHash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// agendaSlots expands one event into its occurrences in [from, until). Errors are agendaProblems.
func agendaSlots(e ical.Event, start, from, until time.Time) ([]agendaSlot, error) {
	allDay := e.Props.Get(ical.PropDateTimeStart).ValueType() == ical.ValueDate
	hasEnd := e.Props.Get(ical.PropDateTimeEnd) != nil || e.Props.Get(ical.PropDuration) != nil
	var span time.Duration
	days := 0
	if hasEnd {
		end, err := e.DateTimeEnd(moncton)
		if err != nil || end.Before(start) {
			return nil, newProblem("invalid_end", "invalid event end")
		}
		span = end.Sub(start)
		days = int((span + 12*time.Hour) / (24 * time.Hour))
	}
	var starts []time.Time
	if rule := e.Props.Get(ical.PropRecurrenceRule); rule != nil {
		if strings.Contains(rule.Value, "FREQ=HOURLY") || strings.Contains(rule.Value, "FREQ=MINUTELY") || strings.Contains(rule.Value, "FREQ=SECONDLY") {
			return nil, newProblem("high_frequency_recurrence", "unexpected high-frequency recurrence")
		}
		set, err := e.RecurrenceSet(moncton)
		if err != nil {
			return nil, newProblem("unsupported_recurrence", "invalid event recurrence")
		}
		for _, d := range set.Between(from, until, true) {
			if d.Before(until) {
				starts = append(starts, d)
			}
		}
	} else if e.Props.Get(ical.PropRecurrenceDates) != nil || e.Props.Get(ical.PropExceptionDates) != nil {
		return nil, newProblem("unsupported_recurrence", "standalone recurrence exceptions require review")
	} else if !start.Before(from) && start.Before(until) {
		starts = []time.Time{start}
	}
	slots := make([]agendaSlot, 0, len(starts))
	for _, d := range starts {
		s := agendaSlot{start: d.In(moncton), allDay: allDay}
		switch {
		case allDay:
			s.start = dayStart(s.start)
			if days > 1 {
				s.endDate = s.start.AddDate(0, 0, days-1).Format(dateLayout)
			}
		case hasEnd:
			s.end = s.start.Add(span)
		}
		slots = append(slots, s)
	}
	return slots, nil
}

// rawEvent is one feed event with its summary and start read once, before any checks.
type rawEvent struct {
	ev        ical.Event
	summary   string
	summaryOK bool
	name      string
	phase     string
	start     time.Time
	startOK   bool
}

type agendaBuilder struct {
	from, until time.Time
	seen        map[string]bool
	activities  map[string]map[string]activityPhase
	items       []agendaItem
	omitted     []omittedEvent
}

// add classifies one event as course activity or campus event, runs its checks and routes any problem:
// a course-activity problem is returned (fatal); a campus-event problem is recorded as an omission.
func (b *agendaBuilder) add(e ical.Event) error {
	r := rawEvent{ev: e}
	if summary, err := e.Props.Text(ical.PropSummary); err == nil {
		r.summary, r.summaryOK = summary, true
		r.name, r.phase = eventTitle(summary)
	}
	if start, err := e.DateTimeStart(moncton); err == nil && !start.IsZero() {
		r.start, r.startOK = start, true
	}
	err := b.process(r)
	if err == nil {
		return nil
	}
	if r.phase != "" {
		return err
	}
	o := omittedEvent{Reason: problemReason(err)}
	if r.summaryOK {
		o.Title = plainText(r.summary)
	}
	if r.startOK {
		o.at = r.start.In(moncton)
		o.Date = o.at.Format(dateLayout)
	}
	if o.inWindow(b.from, b.until) {
		b.omitted = append(b.omitted, o)
	}
	return nil
}

func (b *agendaBuilder) process(r rawEvent) error {
	e := r.ev
	uid, err := e.Props.Text(ical.PropUID)
	if err != nil || uid == "" {
		return newProblem("other", "missing or duplicate calendar UID")
	}
	if b.seen[uid] {
		return newProblem("duplicate_uid", "missing or duplicate calendar UID")
	}
	b.seen[uid] = true
	if e.Props.Get(ical.PropRecurrenceID) != nil {
		return newProblem("recurrence_override", "recurrence overrides require review before sending")
	}
	if !r.summaryOK {
		return newProblem("invalid_title", "invalid event title")
	}
	if !r.startOK {
		return newProblem("invalid_date", "invalid event date")
	}
	if status := e.Props.Get(ical.PropStatus); status != nil && strings.EqualFold(status.Value, "CANCELLED") {
		return nil
	}
	location, err := e.Props.Text(ical.PropLocation)
	if err != nil {
		return newProblem("invalid_location", "invalid event location")
	}
	slots, err := agendaSlots(e, r.start, b.from, b.until)
	if err != nil {
		return err
	}
	if r.phase == "" {
		for _, s := range slots {
			it := agendaItem{ID: agendaHash("event", uid, s.start.Format(time.RFC3339)), Type: "event", Title: plainText(r.summary), Location: agendaLocation(location)}
			s.fill(&it)
			b.items = append(b.items, it)
		}
		return nil
	}
	description, _ := e.Props.Text(ical.PropDescription)
	identity := activityIdentity(r.name, description)
	course, courseTitle, _ := strings.Cut(location, ":")
	recurring := e.Props.Get(ical.PropRecurrenceRule) != nil
	label := phaseLabels[r.phase]
	for _, s := range slots {
		key := course + "\x00" + identity
		if recurring {
			key += "\x00" + s.start.Format(dateLayout)
		}
		if b.activities[key] == nil {
			b.activities[key] = map[string]activityPhase{}
		}
		if prev, ok := b.activities[key][label]; !ok || s.start.Before(prev.start) {
			b.activities[key][label] = activityPhase{agendaSlot: s, title: plainText(r.name), course: plainText(course), courseTitle: plainText(courseTitle)}
		}
	}
	return nil
}

// buildAgenda returns course activities (one item per phase) and all other feed events whose start falls in [from, until).
// Problems with course activities or an empty calendar are errors. Problems with other events omit that event and are
// returned as omitted entries that fall inside the window.
func buildAgenda(raw []byte, from, until time.Time, instructors map[string]string) ([]agendaItem, []omittedEvent, error) {
	cal, err := parseCalendar(raw)
	if err != nil {
		return nil, nil, err
	}
	events := cal.Events()
	if len(events) == 0 {
		return nil, nil, errors.New("empty calendar")
	}
	b := &agendaBuilder{from: from, until: until, seen: map[string]bool{}, activities: map[string]map[string]activityPhase{}, items: []agendaItem{}}
	for _, e := range events {
		if err := b.add(e); err != nil {
			return nil, nil, err
		}
	}
	for key, phases := range b.activities {
		for label, p := range phases {
			it := agendaItem{ID: agendaHash("activity", key, label), Type: "activity", Phase: ptr(label), Title: p.title, Course: nullable(p.course), CourseTitle: nullable(p.courseTitle), Instructor: nullable(plainText(instructors[p.course])), ActivityID: ptr(agendaHash("activity", key))}
			p.fill(&it)
			b.items = append(b.items, it)
		}
	}
	items := b.items
	sort.Slice(items, func(i, j int) bool {
		x, y := items[i], items[j]
		switch {
		case x.Date != y.Date:
			return x.Date < y.Date
		case x.AllDay != y.AllDay:
			return !x.AllDay
		case !x.at.Equal(y.at):
			return x.at.Before(y.at)
		case x.Title != y.Title:
			return x.Title < y.Title
		}
		return x.ID < y.ID
	})
	omitted := b.omitted
	sort.SliceStable(omitted, func(i, j int) bool {
		x, y := omitted[i], omitted[j]
		if x.Date != y.Date {
			return x.Date < y.Date
		}
		if x.Title != y.Title {
			return x.Title < y.Title
		}
		return x.Reason < y.Reason
	})
	return items, omitted, nil
}

// agendaFeed serves the upstream copy. A successful copy lives for the configured agendaCacheMinutes. A failed refresh
// is retried no sooner than feedRetry, and the previous copy is served for up to feedStale. Refresh validation fails
// only for course-activity problems.
func (a *App) agendaFeed(ctx context.Context) ([]byte, time.Time, error) {
	s := a.settings.current()
	ttl := s.agendaTTL()
	c := &a.feed
	c.mu.Lock()
	defer c.mu.Unlock()
	now := a.now()
	if source := sha256.Sum256([]byte(s.NBCCFeedURL)); source != c.source {
		c.source, c.raw, c.fetchedAt, c.attemptAt = source, nil, time.Time{}, time.Time{}
	}
	if c.raw != nil && now.Sub(c.fetchedAt) < ttl {
		return c.raw, c.fetchedAt, nil
	}
	if c.attemptAt.IsZero() || now.Sub(c.attemptAt) >= min(ttl, feedRetry) {
		c.attemptAt = now
		// Detached from the request so that one client disconnecting cannot fail a shared refresh.
		raw, err := a.fetchFeed(context.WithoutCancel(ctx), s.NBCCFeedURL)
		code := "feed_unavailable"
		if err == nil {
			from, to := defaultWindow(dayOf(now))
			var omitted []omittedEvent
			_, omitted, err = buildAgenda(raw, dayStart(from), dayStart(to.AddDate(0, 0, 1)), nil)
			code = "feed_invalid"
			if err == nil {
				c.raw, c.fetchedAt = raw, now
				a.status.setAgendaOmitted(omitted)
				return raw, now, nil
			}
		}
		log.Printf("agenda feed refresh: %v", err)
		a.status.setAgendaError(code)
	}
	if c.raw != nil && now.Sub(c.fetchedAt) <= feedStale {
		return c.raw, c.fetchedAt, nil
	}
	return nil, time.Time{}, errors.New("NBCC feed is unavailable")
}

func (r *runtimeStatus) setAgendaOmitted(omitted []omittedEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agendaOmitted = append([]omittedEvent(nil), omitted...)
	r.agendaErr = ""
}

func (r *runtimeStatus) setAgendaError(code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agendaErr = code
}

func (r *runtimeStatus) agendaSnapshot() ([]omittedEvent, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]omittedEvent{}, r.agendaOmitted...), r.agendaErr
}

func (a *App) agenda(w http.ResponseWriter, r *http.Request) {
	today := dayOf(a.now())
	defFrom, defTo := defaultWindow(today)
	from, to := defFrom, defTo
	q := r.URL.Query()
	var err error
	if q.Has("from") {
		if from, err = time.Parse(dateLayout, q.Get("from")); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid date range"})
			return
		}
	}
	if q.Has("to") {
		if to, err = time.Parse(dateLayout, q.Get("to")); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid date range"})
			return
		}
	}
	if from.After(to) || to.Sub(from) > 185*24*time.Hour {
		jsonResponse(w, 400, map[string]string{"error": "invalid date range"})
		return
	}
	raw, fetchedAt, err := a.agendaFeed(r.Context())
	if err != nil {
		jsonResponse(w, 502, map[string]string{"error": "NBCC feed is unavailable"})
		return
	}
	items, omitted, err := buildAgenda(raw, dayStart(from), dayStart(to.AddDate(0, 0, 1)), a.settings.current().CourseInstructors)
	if err != nil {
		log.Printf("agenda: %v", err)
		a.status.setAgendaError("feed_invalid")
		jsonResponse(w, 502, map[string]string{"error": "NBCC feed is unavailable"})
		return
	}
	if from.Equal(defFrom) && to.Equal(defTo) {
		a.status.setAgendaOmitted(omitted)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	json.NewEncoder(w).Encode(agendaResponse{
		Timezone:      "America/Moncton",
		Today:         today.Format(dateLayout),
		From:          from.Format(dateLayout),
		To:            to.Format(dateLayout),
		UpdatedAt:     fetchedAt.In(moncton).Format(time.RFC3339),
		OmittedEvents: len(omitted),
		Items:         items,
	})
}
