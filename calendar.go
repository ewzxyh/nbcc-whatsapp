package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

const agendaLinkLabel = "\n\n🔗 *Full agenda:* "

type Preview struct {
	Text          string    `json:"text"`
	From          string    `json:"from"`
	Through       string    `json:"through"`
	Count         int       `json:"activities"`
	FeedHash      string    `json:"feed_hash"`
	FetchedAt     time.Time `json:"fetched_at"`
	OmittedEvents int       `json:"omitted_events"`

	deadlines        []deadline
	validUntil       time.Time
	activityVersions map[string]string
	events           []agendaItem
	isCampus         bool
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

func parseCalendar(raw []byte) (*ical.Calendar, error) {
	cal, err := ical.NewDecoder(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}))).Decode()
	if err != nil || cal == nil {
		return nil, errors.New("invalid NBCC calendar")
	}
	return cal, nil
}

func activityIdentity(uid, description string) string {
	identity := "uid:" + uid
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
		if identity != "uid:"+uid {
			break
		}
	}
	return identity
}

type deadline struct {
	ID string
	At time.Time
}

func buildPreview(raw []byte, now time.Time, idKey []byte, publicBaseURL string, previous map[string]string) (Preview, error) {
	start := midnight(now)
	end := start.AddDate(0, 0, 15)
	p := Preview{From: start.Format(dateLayout), Through: end.AddDate(0, 0, -1).Format(dateLayout), FetchedAt: now, validUntil: start.AddDate(0, 0, 1)}
	items, omitted, err := buildAgenda(raw, start, end, nil, idKey)
	if err != nil {
		return p, err
	}
	p.OmittedEvents = len(omitted)
	p.activityVersions = map[string]string{}
	type row struct {
		at             time.Time
		text, identity string
	}
	rows := []row{}
	for _, it := range items {
		if it.Type == "event" && it.Date == p.From {
			p.events = append(p.events, it)
		}
		if it.Type != "activity" || *it.Phase != "due" {
			continue
		}
		at := it.at
		// A date-only delivery deadline lasts through the end of that local date.
		if it.AllDay {
			at = midnight(at).AddDate(0, 0, 1).Add(-time.Second)
		}
		if !now.Before(at) {
			continue
		}
		p.deadlines = append(p.deadlines, deadline{ID: *it.ActivityID, At: at})
		if at.Before(p.validUntil) {
			p.validUntil = at
		}
		when := " by *23:59*"
		if !it.AllDay {
			when = " at *" + it.at.In(halifax).Format("15:04") + "*"
		}
		course := ""
		if it.Course != nil {
			course = "*" + *it.Course + "* — "
		}
		title := "*" + it.Title + "*"
		text := "• " + course + title + ": Due" + when + "."
		// Compare visible content, excluding the moving heading and countdown labels.
		identity := it.at.Format(time.RFC3339) + "\x00" + text
		p.activityVersions[it.notificationID] = it.notificationVersion
		prefix := course
		if left := at.Sub(now); left <= 14*time.Hour || midnight(it.at).Equal(start) {
			minutes := int((left + time.Minute - 1) / time.Minute)
			warning := fmt.Sprintf("%d hours left", (minutes+59)/60)
			if minutes%60 != 0 {
				warning = fmt.Sprintf("%dh %02dm left", minutes/60, minutes%60)
			} else if minutes == 60 {
				warning = "1 hour left"
			}
			prefix += "⚠️ *_" + warning + "_* "
		}
		if old, ok := previous[it.notificationID]; ok && old != it.notificationVersion {
			prefix += "🔄 *UPDATED* 🔄 "
		}
		text = "• " + prefix + title + ": Due" + when + "."
		rows = append(rows, row{it.at.In(halifax), text, identity})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].at.Equal(rows[j].at) {
			return rows[i].identity < rows[j].identity
		}
		return rows[i].at.Before(rows[j].at)
	})
	var canonical, b strings.Builder
	fmt.Fprintf(&b, "📚 *NBCC — Next 2 Weeks*\n📅 *%s–%s*\n", start.Format("Jan 2"), end.AddDate(0, 0, -1).Format("Jan 2, 2006"))
	nextWeek := start.AddDate(0, 0, 7-(int(start.Weekday())+6)%7)
	laterWeek := nextWeek.AddDate(0, 0, 7)
	lastWeek := -1
	lastDate := ""
	for _, r := range rows {
		week := 0
		if !r.at.Before(nextWeek) {
			week = 1
		}
		if !r.at.Before(laterWeek) {
			week = 2
		}
		if week != lastWeek {
			label := "THIS WEEK"
			if week == 1 {
				label = "NEXT WEEK"
			} else if week == 2 {
				label = laterWeek.Format("Jan 2")
				if last := end.AddDate(0, 0, -1); !last.Equal(laterWeek) {
					label += "–" + last.Format("Jan 2")
				}
				label = strings.ToUpper(label)
			}
			fmt.Fprintf(&b, "\n*%s* ----\n", label)
			lastWeek = week
		}
		// Refresh the labels when an unchanged activity moves from NEXT WEEK to THIS WEEK.
		fmt.Fprintf(&canonical, "%d:%d:%s\n", week, len(r.identity), r.identity)
		day := r.at.Format("Jan 2")
		if day != lastDate {
			lastDate = day
			if midnight(r.at).Equal(start) {
				day = "(TODAY) " + day
			}
			fmt.Fprintf(&b, "\n*%s*\n", day)
		}
		fmt.Fprintln(&b, r.text)
	}
	if len(rows) == 0 {
		b.WriteString("\n• No upcoming labs, quizzes or assignments with a Due date in this period.\n")
	}
	p.Text = strings.TrimSpace(b.String())
	if publicBaseURL != "" {
		footer := agendaLinkLabel + strings.TrimRight(publicBaseURL, "/") + "/\nType *#NBCC* for the latest agenda."
		p.Text += footer
		canonical.WriteString(footer)
	}
	hash := sha256.Sum256([]byte(canonical.String()))
	p.FeedHash = hex.EncodeToString(hash[:])
	p.Count = len(rows)
	return p, nil
}

func campusPreview(agenda Preview, publicBaseURL string) Preview {
	start := midnight(agenda.FetchedAt)
	p := Preview{From: agenda.From, Through: agenda.From, FetchedAt: agenda.FetchedAt, Count: len(agenda.events), validUntil: start.AddDate(0, 0, 1), isCampus: true}
	var b strings.Builder
	fmt.Fprintf(&b, "🏫 *NBCC — Campus Events Today*\n\n*(TODAY) %s*\n", start.Format("Jan 2"))
	for _, event := range agenda.events {
		when := "All day"
		if !event.AllDay {
			when = event.at.In(halifax).Format("15:04")
			if event.EndAt != nil {
				end, _ := time.Parse(time.RFC3339, *event.EndAt)
				layout := "15:04"
				if !midnight(event.at).Equal(midnight(end)) {
					layout = "Jan 2, 15:04"
				}
				when += "–" + end.In(halifax).Format(layout)
			}
		}
		fmt.Fprintf(&b, "• *%s* — *%s*", event.Title, when)
		if event.Location != nil {
			fmt.Fprintf(&b, " — %s", *event.Location)
		}
		b.WriteByte('\n')
	}
	if p.Count == 0 {
		b.WriteString("• No campus events scheduled for today.\n")
	}
	p.Text = strings.TrimSpace(b.String())
	if publicBaseURL != "" {
		p.Text += agendaLinkLabel + strings.TrimRight(publicBaseURL, "/") + "/\nType *#NBCC* for the latest agenda."
	}
	p.FeedHash = textHash(p.From + "\n" + p.Text)
	return p
}
