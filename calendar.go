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

type Preview struct {
	Text          string    `json:"text"`
	From          string    `json:"from"`
	Through       string    `json:"through"`
	Count         int       `json:"activities"`
	FeedHash      string    `json:"feed_hash"`
	FetchedAt     time.Time `json:"fetched_at"`
	OmittedEvents int       `json:"omitted_events"`
	deadlines     []deadline
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

func buildPreview(raw []byte, now time.Time, idKey []byte) (Preview, error) {
	start := midnight(now)
	end := start.AddDate(0, 0, 15)
	p := Preview{From: start.Format(dateLayout), Through: end.AddDate(0, 0, -1).Format(dateLayout), FetchedAt: now}
	items, omitted, err := buildAgenda(raw, start, end, nil, idKey)
	if err != nil {
		return p, err
	}
	p.OmittedEvents = len(omitted)
	groups := map[string]map[string]agendaItem{}
	for _, it := range items {
		if it.Type != "activity" {
			continue
		}
		id := *it.ActivityID
		if groups[id] == nil {
			groups[id] = map[string]agendaItem{}
		}
		groups[id][*it.Phase] = it
	}
	type row struct {
		at             time.Time
		text, identity string
	}
	rows := []row{}
	for id, phases := range groups {
		it, ok := phases["due"]
		label := "due"
		if !ok {
			it, ok = phases["available"]
			label = "available"
		}
		if !ok {
			it = phases["closes"]
			label = "access closes"
		}
		when := " on this date"
		if !it.AllDay {
			when = " at *" + it.at.In(halifax).Format("15:04") + "*"
		}
		course := ""
		if it.Course != nil {
			course = "*" + *it.Course + "* — "
		}
		title := "*" + it.Title + "*"
		text := "• " + course + title + ": " + label + when + "."
		if closes, ok := phases["closes"]; ok && label != "access closes" && !closes.at.Equal(it.at) {
			close := closes.at.In(halifax).Format("15:04")
			if closes.AllDay {
				close = closes.at.Format("Jan 2")
			} else if !midnight(closes.at).Equal(midnight(it.at)) {
				close = closes.at.Format("Jan 2 at 15:04")
			}
			text += " Access closes "
			if !closes.AllDay {
				text += "at "
			} else {
				text += "on "
			}
			text += close + "."
		}
		// Compare visible content, excluding the moving heading and countdown labels.
		identity := it.at.Format(time.RFC3339) + "\x00" + text
		if due, ok := phases["due"]; ok {
			at := due.at
			// A date-only delivery deadline lasts through the end of that local date.
			if due.AllDay {
				at = midnight(at).AddDate(0, 0, 1).Add(-time.Second)
			}
			p.deadlines = append(p.deadlines, deadline{ID: id, At: at})
			if left := at.Sub(now); left > 0 && left <= 14*time.Hour {
				minutes := int((left + time.Minute - 1) / time.Minute)
				warning := fmt.Sprintf("%d hours left", (minutes+59)/60)
				if minutes%60 != 0 {
					warning = fmt.Sprintf("%dh %02dm left", minutes/60, minutes%60)
				}
				text = strings.Replace(text, title, "⚠️ *_"+warning+"_* "+title, 1)
			}
		}
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
	lastDate := ""
	for _, r := range rows {
		fmt.Fprintf(&canonical, "%d:%s\n", len(r.identity), r.identity)
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
		b.WriteString("\n• No scheduled labs, quizzes or assignments in this period.\n")
	}
	hash := sha256.Sum256([]byte(canonical.String()))
	p.FeedHash = hex.EncodeToString(hash[:])
	p.Text = strings.TrimSpace(b.String())
	p.Count = len(rows)
	return p, nil
}
