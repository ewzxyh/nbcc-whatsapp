package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"

	ical "github.com/emersion/go-ical"
)

var halifax, _ = time.LoadLocation("America/Halifax")
var nbccActivityLinks = regexp.MustCompile(`https://elearning\.nbcc\.ca/[^\s<>"']+`)

type Preview struct {
	Text      string    `json:"text"`
	From      string    `json:"from"`
	Through   string    `json:"through"`
	Count     int       `json:"activities"`
	FeedHash  string    `json:"feed_hash"`
	FetchedAt time.Time `json:"fetched_at"`
}

type activity struct {
	title, course string
	dates         map[string][]time.Time
	allDay        map[string]bool
}

func midnight(t time.Time) time.Time {
	y, m, d := t.In(halifax).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, halifax)
}

func cleanText(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.NewReplacer("*", "", "_", "", "~", "", "`", "").Replace(s)
}

func eventTitle(title string) (string, string) {
	for _, phase := range []string{"Due", "Availability Ends", "Available"} {
		if strings.HasSuffix(title, " - "+phase) {
			return strings.TrimPrefix(strings.TrimSuffix(title, " - "+phase), "Quiz Special Access: "), phase
		}
	}
	return title, ""
}

func buildPreview(raw []byte, now time.Time) (Preview, error) {
	start := midnight(now)
	end := start.AddDate(0, 0, 15) // Today through today+14, including the last day.
	p := Preview{From: start.Format("2006-01-02"), Through: end.AddDate(0, 0, -1).Format("2006-01-02"), FetchedAt: now}
	cal, err := ical.NewDecoder(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}))).Decode()
	if err != nil || cal == nil {
		return p, errors.New("invalid NBCC calendar")
	}
	seen := map[string]bool{}
	var canonical []*ical.Component
	activities := map[string]*activity{}
	for _, e := range cal.Events() {
		uid, err := e.Props.Text(ical.PropUID)
		if err != nil || uid == "" || seen[uid] {
			return p, errors.New("missing or duplicate calendar UID")
		}
		seen[uid] = true
		if e.Props.Get(ical.PropRecurrenceID) != nil {
			return p, errors.New("recurrence overrides require review before sending")
		}
		delete(e.Props, ical.PropDateTimeStamp) // Brightspace regenerates this on every fetch.
		canonical = append(canonical, e.Component)
		title, err := e.Props.Text(ical.PropSummary)
		if err != nil {
			return p, errors.New("invalid event title")
		}
		date, err := e.DateTimeStart(halifax)
		if err != nil || date.IsZero() {
			return p, errors.New("invalid event date")
		}
		name, phase := eventTitle(title)
		if phase == "" || e.Props.Get(ical.PropStatus) != nil && e.Props.Get(ical.PropStatus).Value == "CANCELLED" {
			continue
		}
		course, err := e.Props.Text(ical.PropLocation)
		if err != nil {
			return p, errors.New("invalid event location")
		}
		course, _, _ = strings.Cut(course, ":")
		identity := name
		description, _ := e.Props.Text(ical.PropDescription)
		for _, link := range nbccActivityLinks.FindAllString(description, -1) {
			u, err := url.Parse(html.UnescapeString(link))
			if err != nil {
				continue
			}
			for _, field := range []string{"qi", "db"} {
				if id := u.Query().Get(field); id != "" {
					identity = field + ":" + u.Query().Get("ou") + ":" + id
					break
				}
			}
			if identity != name {
				break
			}
		}
		allDay := e.Props.Get(ical.PropDateTimeStart).ValueType() == ical.ValueDate
		dates := []time.Time{date}
		recurring := e.Props.Get(ical.PropRecurrenceRule) != nil
		if recurring {
			rule := e.Props.Get(ical.PropRecurrenceRule).Value
			if strings.Contains(rule, "FREQ=HOURLY") || strings.Contains(rule, "FREQ=MINUTELY") || strings.Contains(rule, "FREQ=SECONDLY") {
				return p, errors.New("unexpected high-frequency recurrence")
			}
			set, err := e.RecurrenceSet(halifax)
			if err != nil {
				return p, errors.New("invalid event recurrence")
			}
			dates = set.Between(start, end, false)
			if set.Before(start, true).Equal(start) {
				dates = append(dates, start)
			}
		} else if e.Props.Get(ical.PropRecurrenceDates) != nil || e.Props.Get(ical.PropExceptionDates) != nil {
			return p, errors.New("standalone recurrence exceptions require review")
		}
		for _, d := range dates {
			d = d.In(halifax)
			if d.Before(start) || !d.Before(end) {
				continue
			}
			key := course + "\x00" + identity
			if recurring {
				key += "\x00" + d.Format("2006-01-02")
			}
			a := activities[key]
			if a == nil {
				a = &activity{title: cleanText(name), course: cleanText(course), dates: map[string][]time.Time{}, allDay: map[string]bool{}}
				activities[key] = a
			}
			a.dates[phase] = append(a.dates[phase], d)
			a.allDay[phase] = allDay
		}
	}
	if len(seen) == 0 {
		return p, errors.New("empty calendar; refusing to replace the last baseline")
	}
	sort.Slice(canonical, func(i, j int) bool {
		return canonical[i].Props.Get(ical.PropUID).Value < canonical[j].Props.Get(ical.PropUID).Value
	})
	canonicalJSON, err := json.Marshal(canonical)
	if err != nil {
		return p, err
	}
	hash := sha256.Sum256(canonicalJSON)
	p.FeedHash = hex.EncodeToString(hash[:])
	type row struct {
		date time.Time
		text string
	}
	var rows []row
	for _, a := range activities {
		for phase := range a.dates {
			sort.Slice(a.dates[phase], func(i, j int) bool { return a.dates[phase][i].Before(a.dates[phase][j]) })
		}
		phase := "Due"
		if len(a.dates[phase]) == 0 {
			phase = "Available"
		}
		if len(a.dates[phase]) == 0 {
			phase = "Availability Ends"
		}
		date := a.dates[phase][0]
		label := map[string]string{"Due": "due", "Available": "available", "Availability Ends": "access closes"}[phase]
		when := " on this date"
		if !a.allDay[phase] {
			when = " at *" + date.Format("15:04") + "*"
		}
		text := "• " + a.course + " — " + a.title + ": " + label + when + "."
		if ends := a.dates["Availability Ends"]; phase != "Availability Ends" && len(ends) > 0 && !ends[0].Equal(date) {
			close := ends[0].Format("15:04")
			if midnight(ends[0]) != midnight(date) {
				close = ends[0].Format("Jan 2 at 15:04")
			}
			text += " Access closes at " + close + "."
		}
		rows = append(rows, row{date, text})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].date.Equal(rows[j].date) {
			return rows[i].text < rows[j].text
		}
		return rows[i].date.Before(rows[j].date)
	})
	var b strings.Builder
	fmt.Fprintf(&b, "📚 *NBCC — Next 2 Weeks*\n📅 *%s–%s*\n", start.Format("Jan 2"), end.AddDate(0, 0, -1).Format("Jan 2, 2006"))
	lastDate := ""
	for _, r := range rows {
		day := r.date.Format("Jan 2")
		if day != lastDate {
			lastDate = day
			if midnight(r.date).Equal(start) {
				day = "(TODAY) " + day
			}
			fmt.Fprintf(&b, "\n*%s*\n", day)
		}
		fmt.Fprintln(&b, r.text)
	}
	if len(rows) == 0 {
		b.WriteString("\n• No scheduled labs, quizzes or assignments in this period.\n")
	}
	p.Text = strings.TrimSpace(b.String())
	p.Count = len(rows)
	return p, nil
}
