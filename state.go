package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Job struct {
	ID          string    `json:"id"`
	Text        string    `json:"text,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	NextAttempt time.Time `json:"next_attempt"`
	Attempts    int       `json:"attempts"`
	NextPart    int       `json:"next_part"`
}

type diskState struct {
	FeedHash string         `json:"feed_hash"`
	Jobs     map[string]Job `json:"jobs"`
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
	return s, nil
}

func (s *Store) copyState(now time.Time) diskState {
	next := diskState{FeedHash: s.data.FeedHash, Jobs: make(map[string]Job, len(s.data.Jobs))}
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

func (s *Store) observe(p Preview, enabled bool, now time.Time) (changed, queued bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.FeedHash == s.data.FeedHash {
		return false, false, nil
	}
	changed = s.data.FeedHash != ""
	next := s.copyState(now)
	next.FeedHash = p.FeedHash
	if changed && enabled {
		if len(next.Jobs) >= 10000 {
			return false, false, errors.New("queue capacity exceeded")
		}
		id := "feed:" + randomID()
		next.Jobs[id] = Job{ID: id, Text: p.Text, Status: "pending", CreatedAt: now, NextAttempt: now}
		queued = true
	}
	err = s.save(next)
	return
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
	return s.save(next)
}
