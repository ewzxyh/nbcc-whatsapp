package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookieName  = "nbcc_admin"
	sessionIdleTimeout = 60 * time.Minute
	sessionMaxLifetime = 12 * time.Hour
	failureWindow      = 15 * time.Minute
	failureLimit       = 5
	lockDuration       = 15 * time.Minute
	maxLimiterEntries  = 10000
)

var (
	errNoSession      = errors.New("no session")
	errSessionExpired = errors.New("session expired")
)

// adminSession is a server-side session. Only the SHA-256 of the cookie token is ever stored.
type adminSession struct {
	key      [32]byte
	csrf     string
	created  time.Time
	lastSeen time.Time
}

// expiresAt is the earlier of the idle deadline (reset by extend) and the absolute 12-hour cap.
func (s adminSession) expiresAt() time.Time {
	idle := s.lastSeen.Add(sessionIdleTimeout)
	absolute := s.created.Add(sessionMaxLifetime)
	if idle.Before(absolute) {
		return idle
	}
	return absolute
}

func (s adminSession) expired(now time.Time) bool {
	return !now.Before(s.expiresAt())
}

func tokenKey(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}

type sessionStore struct {
	mu       sync.Mutex
	sessions map[[32]byte]adminSession
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: map[[32]byte]adminSession{}}
}

// create issues a new session with a fresh token and CSRF token. Expired sessions are swept first.
func (st *sessionStore) create(now time.Time) (string, adminSession) {
	token := randomToken(32)
	s := adminSession{key: tokenKey(token), csrf: randomToken(32), created: now, lastSeen: now}
	st.mu.Lock()
	defer st.mu.Unlock()
	for k, old := range st.sessions {
		if old.expired(now) {
			delete(st.sessions, k)
		}
	}
	st.sessions[s.key] = s
	return token, s
}

// lookup resolves a cookie token. An expired session is removed and reported as errSessionExpired.
func (st *sessionStore) lookup(token string, now time.Time) (adminSession, error) {
	if token == "" {
		return adminSession{}, errNoSession
	}
	key := tokenKey(token)
	st.mu.Lock()
	defer st.mu.Unlock()
	s, ok := st.sessions[key]
	if !ok {
		return adminSession{}, errNoSession
	}
	if s.expired(now) {
		delete(st.sessions, key)
		return adminSession{}, errSessionExpired
	}
	return s, nil
}

// extend resets the idle timer of a live session.
func (st *sessionStore) extend(key [32]byte, now time.Time) (adminSession, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	s, ok := st.sessions[key]
	if !ok || s.expired(now) {
		delete(st.sessions, key)
		return adminSession{}, false
	}
	s.lastSeen = now
	st.sessions[key] = s
	return s, true
}

// rotateAndRevoke gives the current session a new token and CSRF token and removes every other session.
func (st *sessionStore) rotateAndRevoke(oldKey [32]byte, now time.Time) (string, adminSession, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	old, ok := st.sessions[oldKey]
	if !ok || old.expired(now) {
		return "", adminSession{}, false
	}
	for k := range st.sessions {
		if k != oldKey {
			delete(st.sessions, k)
		}
	}
	delete(st.sessions, oldKey)
	token := randomToken(32)
	s := adminSession{key: tokenKey(token), csrf: randomToken(32), created: old.created, lastSeen: now}
	st.sessions[s.key] = s
	return token, s, true
}

// revokeOthers removes every session except keep.
func (st *sessionStore) revokeOthers(keep [32]byte) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for k := range st.sessions {
		if k != keep {
			delete(st.sessions, k)
		}
	}
}

func (st *sessionStore) remove(key [32]byte) {
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.sessions, key)
}

// attemptLimiter counts failed credential attempts per client IP.
type attemptLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientAttempts
}

type clientAttempts struct {
	failures    []time.Time
	lockedUntil time.Time
}

func newAttemptLimiter() *attemptLimiter {
	return &attemptLimiter{clients: map[string]*clientAttempts{}}
}

// blocked reports whether ip is currently locked and for how long.
func (l *attemptLimiter) blocked(ip string, now time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.clients[ip]
	if c == nil || !now.Before(c.lockedUntil) {
		return 0, false
	}
	return c.lockedUntil.Sub(now), true
}

// failed records one failure. When it is the failureLimit-th failure inside failureWindow, the IP is locked
// for lockDuration and that duration is returned with true.
func (l *attemptLimiter) failed(ip string, now time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.clients) >= maxLimiterEntries {
		for k, c := range l.clients {
			if !now.Before(c.lockedUntil) && len(c.failures) == 0 {
				delete(l.clients, k)
			}
		}
	}
	c := l.clients[ip]
	if c == nil {
		c = &clientAttempts{}
		l.clients[ip] = c
	}
	kept := c.failures[:0]
	for _, t := range c.failures {
		if now.Sub(t) < failureWindow {
			kept = append(kept, t)
		}
	}
	c.failures = append(kept, now)
	if len(c.failures) >= failureLimit {
		c.failures = nil
		c.lockedUntil = now.Add(lockDuration)
		return lockDuration, true
	}
	return 0, false
}

// succeeded clears the record for ip after a successful credential check.
func (l *attemptLimiter) succeeded(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.clients, ip)
}

// clientIP trusts only loopback and the explicitly configured proxy, including a Docker bridge gateway.
func clientIP(r *http.Request, trustedProxy net.IP) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsLoopback() || ip.Equal(trustedProxy)) {
		if real := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); real != nil {
			return real.String()
		}
	}
	if ip != nil {
		return ip.String()
	}
	return host
}

// isSecure reports whether the browser connection is HTTPS, directly or through the proxy.
func isSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.EqualFold(strings.TrimSpace(first), "https")
}

// originAllowed accepts a request without Origin, or one whose Origin equals scheme and Host.
func originAllowed(r *http.Request) bool {
	values, present := r.Header["Origin"]
	if !present {
		return true
	}
	if len(values) != 1 {
		return false
	}
	scheme := "http"
	if isSecure(r) {
		scheme = "https"
	}
	return strings.EqualFold(values[0], scheme+"://"+r.Host)
}

// csrfValid compares the X-CSRF-Token header with the session token using SHA-256 on both sides, then a constant-time compare.
func csrfValid(r *http.Request, s adminSession) bool {
	got := r.Header.Get("X-CSRF-Token")
	if got == "" {
		return false
	}
	a := sha256.Sum256([]byte(got))
	b := sha256.Sum256([]byte(s.csrf))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

func isMutating(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", HttpOnly: true, Secure: isSecure(r), SameSite: http.SameSiteStrictMode})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: isSecure(r), SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(0, 0)})
}

// retryAfterSeconds rounds a wait up to whole seconds, with a minimum of one.
func retryAfterSeconds(d time.Duration) string {
	secs := int(math.Ceil(d.Seconds()))
	if secs < 1 {
		secs = 1
	}
	return strconv.Itoa(secs)
}

// tooManyAttempts writes the 429 response used by both the login and setup limiter.
func tooManyAttempts(w http.ResponseWriter, wait time.Duration) {
	w.Header().Set("Retry-After", retryAfterSeconds(wait))
	jsonResponse(w, 429, map[string]string{"error": "too_many_attempts"})
}

// sessionFor resolves the session cookie of r.
func (a *App) sessionFor(r *http.Request) (adminSession, error) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return adminSession{}, errNoSession
	}
	return a.sessions.lookup(c.Value, a.now())
}

type sessionHandler func(w http.ResponseWriter, r *http.Request, s adminSession)

// requireSession answers 401 without a valid session and 403 for a mutating request without a matching
// CSRF token or with a foreign Origin.
func (a *App) requireSession(next sessionHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := a.sessionFor(r)
		if err != nil {
			if errors.Is(err, errSessionExpired) {
				clearSessionCookie(w, r)
				jsonResponse(w, 401, map[string]string{"error": "session_expired"})
				return
			}
			jsonResponse(w, 401, map[string]string{"error": "unauthenticated"})
			return
		}
		if isMutating(r.Method) && (!originAllowed(r) || !csrfValid(r, s)) {
			jsonResponse(w, 403, map[string]string{"error": "csrf"})
			return
		}
		next(w, r, s)
	}
}

// publicPost guards unauthenticated mutating endpoints (setup, login) against foreign Origins.
func publicPost(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !originAllowed(r) {
			jsonResponse(w, 403, map[string]string{"error": "csrf"})
			return
		}
		next(w, r)
	}
}
