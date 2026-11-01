package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestErrorLogReportsOncePerChange(t *testing.T) {
	got := make(chan string, 10)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/7/store/" || !strings.Contains(r.Header.Get("X-Sentry-Auth"), "sentry_key=pub") {
			t.Errorf("bad request %s %q", r.URL.Path, r.Header.Get("X-Sentry-Auth"))
		}
		got <- r.URL.Path
	}))
	defer srv.Close()
	e := newErrorLog("https://pub@"+strings.TrimPrefix(srv.URL, "https://")+"/7", srv.Client())
	down := errors.New("Grapnel HTTP 502 (whatsappunavailable)")
	for _, err := range []error{down, down, down, nil, nil, down} {
		e.note("group history", err)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-got:
		case <-time.After(5 * time.Second):
			t.Fatalf("got %d reports, want 2", i)
		}
	}
	select {
	case <-got:
		t.Fatal("repeated error was reported again")
	case <-time.After(200 * time.Millisecond):
	}
	if newErrorLog("http://pub@example.com/7", nil).endpoint != "" || newErrorLog("not a dsn", nil).endpoint != "" || newErrorLog("https://pub@bad host/7", nil).endpoint != "" {
		t.Fatal("accepted an invalid DSN")
	}
}
