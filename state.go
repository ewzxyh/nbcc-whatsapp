package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Job struct {
	ID           string    `json:"id"`
	Text         string    `json:"text,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	NextAttempt  time.Time `json:"next_attempt"`
	Attempts     int       `json:"attempts"`
	NextPart     int       `json:"next_part"`
	Kind         string    `json:"kind,omitempty"`
	Target       string    `json:"target,omitempty"`
	FeedHash     string    `json:"feed_hash,omitempty"`
	MessageHash  string    `json:"message_hash,omitempty"`
	ReminderKeys []string  `json:"reminder_keys,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

type diskState struct {
	FeedHash              string               `json:"feed_hash"`
	Jobs                  map[string]Job       `json:"jobs"`
	Enabled               bool                 `json:"enabled,omitempty"`
	Target                string               `json:"target,omitempty"`
	LastSentHash          string               `json:"last_sent_hash,omitempty"`
	LastMessageHash       string               `json:"last_message_hash,omitempty"`
	LastActivityVersions  map[string]string    `json:"last_activity_versions"`
	CampusHash            string               `json:"campus_hash,omitempty"`
	LastCampusHash        string               `json:"last_campus_hash,omitempty"`
	LastCampusMessageHash string               `json:"last_campus_message_hash,omitempty"`
	LastCampusDate        string               `json:"last_campus_date,omitempty"`
	CommandAfter          time.Time            `json:"command_after,omitempty"`
	CommandsSince         time.Time            `json:"commands_since,omitempty"`
	Reminders             map[string]time.Time `json:"reminders,omitempty"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data diskState
}

func openStore(path string) (*Store, error) {
	s := &Store{path: path, data: diskState{Jobs: map[string]Job{}}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if json.Unmarshal(b, &s.data) != nil || s.data.Jobs == nil {
		return nil, errors.New("invalid saved state; refusing to reset deduplication")
	}
	// Legacy pending snapshots have no destination/version; never replay them after migration.
	for id, job := range s.data.Jobs {
		if job.Status == "pending" && job.Kind == "" && (strings.HasPrefix(id, "feed:") || strings.HasPrefix(id, "event:")) {
			job.Status, job.Text = "superseded", ""
			s.data.Jobs[id] = job
		}
	}
	return s, nil
}

func (s *Store) copyState(now time.Time) diskState {
	next := s.data
	next.Jobs = make(map[string]Job, len(s.data.Jobs))
	next.Reminders = map[string]time.Time{}
	for key, at := range s.data.Reminders {
		if now.Sub(at) < 7*24*time.Hour {
			next.Reminders[key] = at
		}
	}
	for id, job := range s.data.Jobs {
		if job.Status == "pending" || now.Sub(job.CreatedAt) < 7*24*time.Hour {
			next.Jobs[id] = job
		}
	}
	return next
}

func (s *Store) save(next diskState) error {
	b, err := json.Marshal(next)
	if err == nil {
		err = writeFileAtomic(s.path, b)
	}
	if err == nil {
		s.data = next
	}
	return err
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *Store) enqueue(id, text string, enabled bool, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.Jobs[id]; ok {
		return nil
	}
	next := s.copyState(now)
	if len(next.Jobs) >= 10000 {
		return errors.New("queue capacity exceeded")
	}
	status := "pending"
	if !enabled {
		status = "suppressed"
		text = ""
	}
	next.Jobs[id] = Job{ID: id, Text: text, Status: status, CreatedAt: now, NextAttempt: now}
	return s.save(next)
}

func (s *Store) observe(p Preview, enabled bool, target string, now time.Time) (changed, queued bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Upgrade an already delivered version without generating a migration broadcast.
	seed := s.data.LastActivityVersions == nil && s.data.Target == target && s.data.LastSentHash != "" && s.data.LastSentHash == p.FeedHash
	if p.FeedHash == s.data.FeedHash && enabled == s.data.Enabled && target == s.data.Target {
		if seed {
			next := s.copyState(now)
			next.LastActivityVersions = p.activityVersions
			return false, false, s.save(next)
		}
		return false, false, nil
	}
	changed = s.data.FeedHash != ""
	changed = changed && p.FeedHash != s.data.FeedHash
	activated := enabled && (!s.data.Enabled || target != s.data.Target)
	next := s.copyState(now)
	if seed {
		next.LastActivityVersions = p.activityVersions
	}
	if target != next.Target {
		next.LastSentHash, next.LastMessageHash = "", ""
		next.LastActivityVersions = nil
		next.CampusHash, next.LastCampusHash, next.LastCampusMessageHash, next.LastCampusDate = "", "", "", ""
		next.CommandAfter = time.Time{}
		next.Reminders = map[string]time.Time{}
	}
	if activated {
		next.CommandsSince = now
	}
	for id, job := range next.Jobs {
		if job.Status == "pending" && (!enabled || job.Kind != "" && (job.Target != target || job.Kind != "campus" && job.FeedHash != "" && job.FeedHash != p.FeedHash)) {
			job.Status, job.Text = "superseded", ""
			next.Jobs[id] = job
		}
	}
	next.FeedHash = p.FeedHash
	next.Target, next.Enabled = target, enabled
	if enabled && (activated || changed) && !hasPendingAgenda(next) {
		if len(next.Jobs) >= 10000 {
			return false, false, errors.New("queue capacity exceeded")
		}
		id := "feed:" + randomID()
		next.Jobs[id] = Job{ID: id, Kind: "agenda", Target: target, FeedHash: p.FeedHash, Status: "pending", CreatedAt: now, NextAttempt: now}
		queued = true
	}
	err = s.save(next)
	return
}

func hasPendingAgenda(d diskState) bool {
	for _, j := range d.Jobs {
		if j.Status == "pending" && j.Kind != "" && j.Kind != "campus" {
			return true
		}
	}
	return false
}

func (s *Store) command(id string, enabled bool, target string, at, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.Jobs[id]; ok {
		return nil
	}
	next := s.copyState(now)
	if len(next.Jobs) >= 10000 {
		return errors.New("queue capacity exceeded")
	}
	status := "pending"
	blocked := !enabled || target == next.Target && (at.Before(next.CommandAfter) || at.Before(next.CommandsSince))
	for _, j := range next.Jobs {
		if j.Status == "pending" && j.Kind != "" && j.Kind != "campus" && j.Target == target {
			blocked = true
		}
	}
	if blocked {
		status = "suppressed"
	}
	next.Jobs[id] = Job{ID: id, Kind: "command", Target: target, Status: status, CreatedAt: now, NextAttempt: now}
	if next.CommandsSince.IsZero() && enabled {
		next.CommandsSince = now
	}
	return s.save(next)
}

func (s *Store) pause(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.copyState(now)
	changed := next.Enabled
	next.Enabled = false
	for id, j := range next.Jobs {
		if j.Status == "pending" {
			j.Status, j.Text = "suppressed", ""
			next.Jobs[id] = j
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.save(next)
}

func (s *Store) currentJob(id string) Job { s.mu.Lock(); defer s.mu.Unlock(); return s.data.Jobs[id] }

func (s *Store) activityVersions(target string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	versions := map[string]string{}
	if s.data.Target == target {
		for id, version := range s.data.LastActivityVersions {
			versions[id] = version
		}
	}
	return versions
}

func (s *Store) recentHashes(hash, target string, campus bool) map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	hashes := map[string]bool{}
	if s.data.Target != target {
		return hashes
	}
	if campus {
		if s.data.LastCampusHash == hash && s.data.LastCampusMessageHash != "" {
			hashes[s.data.LastCampusMessageHash] = true
		}
	} else if s.data.LastSentHash == hash && s.data.LastMessageHash != "" {
		hashes[s.data.LastMessageHash] = true
	}
	var latestAccepted time.Time
	for _, job := range s.data.Jobs {
		if job.Target == target && job.Kind != "" && (job.Kind == "campus") == campus && job.Status == "accepted" && job.CreatedAt.After(latestAccepted) {
			latestAccepted = job.CreatedAt
		}
	}
	// An upstream timeout can occur after WhatsApp accepted the message.
	for _, job := range s.data.Jobs {
		if job.Target == target && job.FeedHash == hash && job.Kind != "" && (job.Kind == "campus") == campus && job.Status != "accepted" && !job.CreatedAt.Before(latestAccepted) && job.MessageHash != "" {
			hashes[job.MessageHash] = true
		}
	}
	return hashes
}

func (s *Store) observeCampus(p Preview, target string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.data.Enabled || s.data.Target != target || now.In(halifax).Hour() < 8 || s.data.CampusHash == p.FeedHash {
		return nil
	}
	next := s.copyState(now)
	for id, job := range next.Jobs {
		if job.Status == "pending" && job.Kind == "campus" {
			job.Status, job.Text = "superseded", ""
			next.Jobs[id] = job
		}
	}
	next.CampusHash = p.FeedHash
	// Empty days stay quiet; cancellation of today's previously sent list is an update.
	if p.Count > 0 || next.LastCampusDate == p.From {
		if len(next.Jobs) >= 10000 {
			return errors.New("queue capacity exceeded")
		}
		id := "campus:" + randomID()
		next.Jobs[id] = Job{ID: id, Kind: "campus", Target: target, FeedHash: p.FeedHash, ExpiresAt: p.validUntil, Status: "pending", CreatedAt: now, NextAttempt: now}
	}
	return s.save(next)
}

func (s *Store) commandSince() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.CommandsSince
}

func reminderPlan(p Preview, now time.Time) map[string]time.Time {
	keys := map[string]time.Time{}
	for _, d := range p.deadlines {
		left := d.At.Sub(now)
		if left <= 0 || left > 14*time.Hour {
			continue
		}
		base := d.ID + "\x00" + d.At.Format(time.RFC3339)
		keys[textHash(base+"\x0014")] = d.At
		if left <= 4*time.Hour {
			keys[textHash(base+"\x004")] = d.At
		}
	}
	return keys
}

func (s *Store) queueReminders(p Preview, target string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.copyState(now)
	if !next.Enabled || target != next.Target {
		return nil
	}
	plan := reminderPlan(p, now)
	keys := []string{}
	var expires time.Time
	unscheduled := false
	for key, at := range plan {
		if _, ok := next.Reminders[key]; ok {
			continue
		}
		pending := false
		for _, j := range next.Jobs {
			if j.Status == "pending" || j.Status == "failed" {
				for _, k := range j.ReminderKeys {
					if k == key {
						pending = true
					}
				}
			}
		}
		if !pending {
			unscheduled = true
		}
		keys = append(keys, key)
		if expires.IsZero() || at.Before(expires) {
			expires = at
		}
	}
	if !unscheduled {
		return nil
	}
	sort.Strings(keys)
	// A closer threshold replaces an outstanding reminder rather than sending both late.
	for id, j := range next.Jobs {
		if j.Status == "pending" && j.Kind == "reminder" {
			j.Status, j.Text = "superseded", ""
			next.Jobs[id] = j
		}
	}
	for id, j := range next.Jobs {
		if j.Status == "pending" && j.Kind != "" && j.Kind != "campus" && j.Attempts == 0 && j.NextPart == 0 {
			j.Kind, j.ReminderKeys, j.ExpiresAt = "reminder", keys, expires
			next.Jobs[id] = j
			return s.save(next)
		}
	}
	if len(next.Jobs) >= 10000 {
		return errors.New("queue capacity exceeded")
	}
	id := "reminder:" + randomID()
	next.Jobs[id] = Job{ID: id, Kind: "reminder", Target: target, FeedHash: p.FeedHash, ReminderKeys: keys, ExpiresAt: expires, Status: "pending", CreatedAt: now, NextAttempt: now}
	return s.save(next)
}

func (s *Store) complete(job Job, p Preview, target, lastPart string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.copyState(now)
	job.Status, job.Text = "accepted", ""
	next.Jobs[job.ID] = job
	if job.Kind == "campus" {
		next.LastCampusHash, next.LastCampusMessageHash, next.LastCampusDate = job.FeedHash, textHash(lastPart), p.From
	} else if job.Kind != "" {
		next.Target = target
		next.LastSentHash, next.LastMessageHash = job.FeedHash, textHash(lastPart)
		next.LastActivityVersions = p.activityVersions
		// A command combined with a reminder keeps its original command ID.
		if job.Kind == "command" || strings.HasPrefix(job.ID, "command:") {
			next.CommandAfter = now.Add(15 * time.Minute)
		}
		for _, key := range job.ReminderKeys {
			next.Reminders[key] = job.ExpiresAt
		}
	}
	return s.save(next)
}

func (s *Store) pending(now time.Time) []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	var jobs []Job
	for _, j := range s.data.Jobs {
		if j.Status == "pending" && !j.NextAttempt.After(now) {
			jobs = append(jobs, j)
		}
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.Before(jobs[j].CreatedAt) })
	if len(jobs) > 20 {
		jobs = jobs[:20]
	}
	return jobs
}

// counts returns the number of pending and failed jobs for the admin status.
func (s *Store) counts() (pending, failed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.data.Jobs {
		switch j.Status {
		case "pending":
			pending++
		case "failed":
			failed++
		}
	}
	return pending, failed
}

func (s *Store) update(job Job, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.copyState(now)
	next.Jobs[job.ID] = job
	// A reminder suppressed by recent history must not return to the queue on the next tick.
	if job.Status == "suppressed" {
		for _, key := range job.ReminderKeys {
			next.Reminders[key] = job.ExpiresAt
		}
	}
	return s.save(next)
}
