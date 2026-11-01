package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// errorLog logs a worker error once per change and reports it to GlitchTip (SENTRY_DSN).
// Repeats of the same error stay silent until it clears, so a long outage is one log line, not one per tick.
type errorLog struct {
	mu       sync.Mutex
	last     map[string]string
	endpoint string // GlitchTip store URL; empty when SENTRY_DSN is unset or invalid
	auth     string
	client   *http.Client
}

func newErrorLog(dsn string, client *http.Client) *errorLog {
	e := &errorLog{last: map[string]string{}, client: client}
	if dsn == "" {
		return e
	}
	u, err := url.Parse(dsn)
	if err != nil {
		u = &url.URL{}
	}
	project := strings.Trim(u.Path, "/")
	if u.Scheme != "https" || u.User == nil || u.User.Username() == "" || project == "" || strings.Contains(project, "/") {
		log.Printf("SENTRY_DSN ignored: invalid value")
		return e
	}
	e.endpoint = fmt.Sprintf("https://%s/api/%s/store/", u.Host, project)
	e.auth = "Sentry sentry_version=7, sentry_client=nbcc-whatsapp/1, sentry_key=" + u.User.Username()
	return e
}

// note records the latest outcome of source; err == nil marks it healthy again.
func (e *errorLog) note(source string, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	e.mu.Lock()
	prev := e.last[source]
	e.last[source] = msg
	e.mu.Unlock()
	switch {
	case msg == prev:
	case msg == "":
		log.Printf("%s: recovered", source)
	default:
		log.Printf("%s: %s", source, msg)
		e.report(source, msg)
	}
}

// report sends one error event in the background. Messages never carry secrets (see api in grapnel.go).
func (e *errorLog) report(source, msg string) {
	if e.endpoint == "" {
		return
	}
	id := make([]byte, 16)
	rand.Read(id)
	body, _ := json.Marshal(map[string]any{
		"event_id":    hex.EncodeToString(id),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"platform":    "go",
		"level":       "error",
		"logger":      source,
		"environment": "production",
		"message":     source + ": " + msg,
	})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Sentry-Auth", e.auth)
		resp, err := e.client.Do(req)
		if err != nil {
			log.Printf("GlitchTip report failed: network error")
			return
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			log.Printf("GlitchTip report failed: HTTP %d", resp.StatusCode)
		}
	}()
}
