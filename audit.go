package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const auditCapacity = 200

// auditEntry records an admin action. Fields lists setting or account field names only, never values or addresses.
type auditEntry struct {
	At     time.Time `json:"at"`
	Action string    `json:"action"`
	Fields []string  `json:"fields"`
}

type auditLog struct {
	mu      sync.Mutex
	path    string
	entries []auditEntry // oldest first
}

func openAudit(dir string) (*auditLog, error) {
	l := &auditLog{path: filepath.Join(dir, "audit.json")}
	b, err := os.ReadFile(l.path)
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	if json.Unmarshal(b, &l.entries) != nil {
		return nil, errors.New("invalid audit log file")
	}
	if len(l.entries) > auditCapacity {
		l.entries = l.entries[len(l.entries)-auditCapacity:]
	}
	return l, nil
}

// record appends an entry and persists the capped list. A write failure is logged generically and does not undo the action.
func (l *auditLog) record(at time.Time, action string, fields ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := auditEntry{At: at.In(moncton).Truncate(time.Second), Action: action}
	if len(fields) > 0 {
		entry.Fields = append([]string(nil), fields...)
	}
	l.entries = append(l.entries, entry)
	if len(l.entries) > auditCapacity {
		l.entries = append([]auditEntry(nil), l.entries[len(l.entries)-auditCapacity:]...)
	}
	b, err := json.Marshal(l.entries)
	if err == nil {
		err = writeFileAtomic(l.path, b)
	}
	if err != nil {
		log.Printf("audit log write failed")
	}
}

// newestFirst returns a copy of the entries, most recent first.
func (l *auditLog) newestFirst() []auditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]auditEntry, 0, len(l.entries))
	for i := len(l.entries) - 1; i >= 0; i-- {
		entry := l.entries[i]
		entry.Fields = append([]string{}, entry.Fields...)
		out = append(out, entry)
	}
	return out
}
