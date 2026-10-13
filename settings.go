package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	defaultGroupName    = "NBCC Study Group"
	defaultPollMinutes  = 5
	defaultCacheMinutes = 60
)

// Settings is the complete runtime configuration. Secret fields are held decrypted in memory only;
// they are never serialized to the browser.
type Settings struct {
	NBCCFeedURL         string
	GrapnelAPIKey       string
	GrapnelInstanceID   string
	GrapnelGroupJID     string
	GrapnelGroupName    string
	WebhookSecret       string
	PublicBaseURL       string
	SendEnabled         bool
	PollIntervalMinutes int
	AgendaCacheMinutes  int
	CourseInstructors   map[string]string
}

func defaultSettings() Settings {
	return Settings{GrapnelGroupName: defaultGroupName, PollIntervalMinutes: defaultPollMinutes, AgendaCacheMinutes: defaultCacheMinutes, CourseInstructors: map[string]string{}}
}

func (s Settings) clone() Settings {
	c := s
	c.CourseInstructors = make(map[string]string, len(s.CourseInstructors))
	for k, v := range s.CourseInstructors {
		c.CourseInstructors[k] = v
	}
	return c
}

func (s Settings) feedConfigured() bool { return s.NBCCFeedURL != "" }

// grapnelDiscoveryReady reports whether group discovery and webhook registration can run.
func (s Settings) grapnelDiscoveryReady() bool {
	return s.GrapnelAPIKey != "" && s.GrapnelInstanceID != ""
}

// grapnelSendReady reports whether sendEnabled may be true.
func (s Settings) grapnelSendReady() bool {
	return s.grapnelDiscoveryReady() && s.GrapnelGroupJID != ""
}

func (s Settings) pollInterval() time.Duration {
	return time.Duration(s.PollIntervalMinutes) * time.Minute
}

func (s Settings) agendaTTL() time.Duration {
	return time.Duration(s.AgendaCacheMinutes) * time.Minute
}

// validationError carries field-level codes for a rejected settings change.
type validationError struct {
	Fields map[string]string
}

func (e *validationError) Error() string { return "validation failed" }

// SettingsStore keeps the current settings in memory and persists them encrypted under the state directory.
type SettingsStore struct {
	mu      sync.RWMutex
	path    string
	key     []byte
	cur     Settings
	changed chan struct{}
}

// openSettings loads <dir>/settings.json. When the file does not exist, seed supplies the initial values
// (nil means defaults) and the file is written once; later startups never re-import the legacy values.
func openSettings(dir string, seed func() Settings) (*SettingsStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "settings.json")
	raw, err := os.ReadFile(path)
	fresh := errors.Is(err, fs.ErrNotExist)
	if err != nil && !fresh {
		return nil, err
	}
	key, err := loadSecretKey(dir, fresh)
	if err != nil {
		return nil, err
	}
	st := &SettingsStore{path: path, key: key, changed: make(chan struct{}, 1)}
	if fresh {
		s := defaultSettings()
		if seed != nil {
			s = seed()
		}
		st.cur = s
		if err = st.persist(s); err != nil {
			return nil, err
		}
		return st, nil
	}
	s, err := decodeSettings(raw, key)
	if err != nil {
		return nil, err
	}
	st.cur = s
	return st, nil
}

// loadSecretKey reads <dir>/secret.key. It is created (32 random bytes, mode 0600) only for a fresh state;
// an existing settings file without its key is refused rather than silently orphaned.
func loadSecretKey(dir string, create bool) ([]byte, error) {
	path := filepath.Join(dir, "secret.key")
	b, err := os.ReadFile(path)
	if err == nil {
		if len(b) != 32 {
			return nil, errors.New("secret.key is invalid")
		}
		return b, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if !create {
		return nil, errors.New("secret.key is missing; settings cannot be decrypted")
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return nil, err
	}
	if err = writeFileAtomic(path, key); err != nil {
		return nil, err
	}
	return key, nil
}

// settingsFile is the on-disk shape. Secret fields are sealed; everything else is plaintext JSON.
type settingsFile struct {
	NBCCFeedURL         *sealedValue      `json:"nbccFeedUrl,omitempty"`
	GrapnelAPIKey       *sealedValue      `json:"grapnelApiKey,omitempty"`
	WebhookSecret       *sealedValue      `json:"webhookSecret,omitempty"`
	GrapnelInstanceID   string            `json:"grapnelInstanceId"`
	GrapnelGroupJID     string            `json:"grapnelGroupJid"`
	GrapnelGroupName    string            `json:"grapnelGroupName"`
	PublicBaseURL       string            `json:"publicBaseUrl"`
	SendEnabled         bool              `json:"sendEnabled"`
	PollIntervalMinutes int               `json:"pollIntervalMinutes"`
	AgendaCacheMinutes  int               `json:"agendaCacheMinutes"`
	CourseInstructors   map[string]string `json:"courseInstructors"`
}

type sealedValue struct {
	V     int    `json:"v"`
	Nonce string `json:"nonce"`
	CT    string `json:"ct"`
}

func sealField(key []byte, field, plain string) (*sealedValue, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, []byte(plain), []byte(field))
	return &sealedValue{V: 1, Nonce: base64.StdEncoding.EncodeToString(nonce), CT: base64.StdEncoding.EncodeToString(ct)}, nil
}

// openField decrypts one sealed value. The field name is bound as associated data so ciphertexts cannot be swapped between fields.
func openField(key []byte, field string, sv *sealedValue) (string, error) {
	if sv == nil || sv.V != 1 {
		return "", errors.New("unsupported sealed value")
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce, err := base64.StdEncoding.DecodeString(sv.Nonce)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return "", errors.New("invalid sealed nonce")
	}
	ct, err := base64.StdEncoding.DecodeString(sv.CT)
	if err != nil {
		return "", errors.New("invalid sealed value")
	}
	plain, err := gcm.Open(nil, nonce, ct, []byte(field))
	if err != nil {
		return "", errors.New("cannot decrypt setting")
	}
	return string(plain), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func decodeSettings(raw []byte, key []byte) (Settings, error) {
	var f settingsFile
	if json.Unmarshal(raw, &f) != nil {
		return Settings{}, errors.New("invalid settings file")
	}
	s := defaultSettings()
	var err error
	if f.NBCCFeedURL != nil {
		if s.NBCCFeedURL, err = openField(key, "nbccFeedUrl", f.NBCCFeedURL); err != nil {
			return Settings{}, err
		}
	}
	if f.GrapnelAPIKey != nil {
		if s.GrapnelAPIKey, err = openField(key, "grapnelApiKey", f.GrapnelAPIKey); err != nil {
			return Settings{}, err
		}
	}
	if f.WebhookSecret != nil {
		if s.WebhookSecret, err = openField(key, "webhookSecret", f.WebhookSecret); err != nil {
			return Settings{}, err
		}
	}
	s.GrapnelInstanceID = f.GrapnelInstanceID
	s.GrapnelGroupJID = f.GrapnelGroupJID
	if f.GrapnelGroupName != "" {
		s.GrapnelGroupName = f.GrapnelGroupName
	}
	s.PublicBaseURL = f.PublicBaseURL
	s.SendEnabled = f.SendEnabled
	// Out-of-range numbers from a hand-edited file fall back to defaults; a zero poll interval would stop the worker.
	if f.PollIntervalMinutes >= 1 && f.PollIntervalMinutes <= 1440 {
		s.PollIntervalMinutes = f.PollIntervalMinutes
	}
	if f.AgendaCacheMinutes >= 1 && f.AgendaCacheMinutes <= 1440 {
		s.AgendaCacheMinutes = f.AgendaCacheMinutes
	}
	if f.CourseInstructors != nil {
		s.CourseInstructors = f.CourseInstructors
	}
	return s, nil
}

// persist writes s encrypted with the store key. Callers serialize access.
func (st *SettingsStore) persist(s Settings) error {
	f := settingsFile{
		GrapnelInstanceID:   s.GrapnelInstanceID,
		GrapnelGroupJID:     s.GrapnelGroupJID,
		GrapnelGroupName:    s.GrapnelGroupName,
		PublicBaseURL:       s.PublicBaseURL,
		SendEnabled:         s.SendEnabled,
		PollIntervalMinutes: s.PollIntervalMinutes,
		AgendaCacheMinutes:  s.AgendaCacheMinutes,
		CourseInstructors:   s.CourseInstructors,
	}
	var err error
	if s.NBCCFeedURL != "" {
		if f.NBCCFeedURL, err = sealField(st.key, "nbccFeedUrl", s.NBCCFeedURL); err != nil {
			return err
		}
	}
	if s.GrapnelAPIKey != "" {
		if f.GrapnelAPIKey, err = sealField(st.key, "grapnelApiKey", s.GrapnelAPIKey); err != nil {
			return err
		}
	}
	if s.WebhookSecret != "" {
		if f.WebhookSecret, err = sealField(st.key, "webhookSecret", s.WebhookSecret); err != nil {
			return err
		}
	}
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return writeFileAtomic(st.path, b)
}

// current returns an immutable copy of the settings. Safe to call from any goroutine.
func (st *SettingsStore) current() Settings {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.cur.clone()
}

// changedCh is signalled after every successful update so the worker can re-read pollIntervalMinutes.
func (st *SettingsStore) changedCh() <-chan struct{} { return st.changed }

// update validates a partial change, persists it and swaps the snapshot. It returns the names of fields whose value changed.
func (st *SettingsStore) update(raw map[string]json.RawMessage) ([]string, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	next, errs := applyPatch(st.cur, raw)
	for field, code := range checkCrossFields(next) {
		errs[field] = code
	}
	if len(errs) > 0 {
		return nil, &validationError{Fields: errs}
	}
	changed := changedFields(st.cur, next)
	if len(changed) == 0 {
		return nil, nil
	}
	if err := st.persist(next); err != nil {
		return nil, err
	}
	st.cur = next
	select {
	case st.changed <- struct{}{}:
	default:
	}
	return changed, nil
}

// checkCrossFields enforces rules that involve more than one field.
func checkCrossFields(s Settings) map[string]string {
	errs := map[string]string{}
	if s.SendEnabled && !s.grapnelSendReady() {
		errs["sendEnabled"] = "missing_grapnel_config"
	}
	return errs
}

func changedFields(a, b Settings) []string {
	var out []string
	if a.NBCCFeedURL != b.NBCCFeedURL {
		out = append(out, "nbccFeedUrl")
	}
	if a.GrapnelAPIKey != b.GrapnelAPIKey {
		out = append(out, "grapnelApiKey")
	}
	if a.GrapnelInstanceID != b.GrapnelInstanceID {
		out = append(out, "grapnelInstanceId")
	}
	if a.GrapnelGroupJID != b.GrapnelGroupJID {
		out = append(out, "grapnelGroupJid")
	}
	if a.GrapnelGroupName != b.GrapnelGroupName {
		out = append(out, "grapnelGroupName")
	}
	if a.WebhookSecret != b.WebhookSecret {
		out = append(out, "webhookSecret")
	}
	if a.PublicBaseURL != b.PublicBaseURL {
		out = append(out, "publicBaseUrl")
	}
	if a.SendEnabled != b.SendEnabled {
		out = append(out, "sendEnabled")
	}
	if a.PollIntervalMinutes != b.PollIntervalMinutes {
		out = append(out, "pollIntervalMinutes")
	}
	if a.AgendaCacheMinutes != b.AgendaCacheMinutes {
		out = append(out, "agendaCacheMinutes")
	}
	if !sameInstructors(a.CourseInstructors, b.CourseInstructors) {
		out = append(out, "courseInstructors")
	}
	sort.Strings(out)
	return out
}

func sameInstructors(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// applyPatch applies the present keys of raw to cur. Unknown keys are ignored. Cross-field rules are checked separately.
func applyPatch(cur Settings, raw map[string]json.RawMessage) (Settings, map[string]string) {
	next := cur.clone()
	errs := map[string]string{}
	for key, msg := range raw {
		switch key {
		case "nbccFeedUrl":
			applySecret(key, msg, &next.NBCCFeedURL, checkFeedURL, errs)
		case "grapnelApiKey":
			applySecret(key, msg, &next.GrapnelAPIKey, checkAPIKey, errs)
		case "webhookSecret":
			applySecret(key, msg, &next.WebhookSecret, checkWebhookSecret, errs)
		case "grapnelInstanceId":
			applyText(key, msg, &next.GrapnelInstanceID, true, checkUUID, errs)
		case "grapnelGroupJid":
			applyText(key, msg, &next.GrapnelGroupJID, true, checkGroupJID, errs)
		case "grapnelGroupName":
			applyText(key, msg, &next.GrapnelGroupName, false, checkGroupName, errs)
		case "publicBaseUrl":
			applyText(key, msg, &next.PublicBaseURL, true, checkPublicBaseURL, errs)
		case "sendEnabled":
			var b bool
			if isJSONNull(msg) || json.Unmarshal(msg, &b) != nil {
				errs[key] = "invalid_value"
			} else {
				next.SendEnabled = b
			}
		case "pollIntervalMinutes":
			applyMinutes(key, msg, &next.PollIntervalMinutes, errs)
		case "agendaCacheMinutes":
			applyMinutes(key, msg, &next.AgendaCacheMinutes, errs)
		case "courseInstructors":
			applyInstructors(key, msg, &next.CourseInstructors, errs)
		}
	}
	return next, errs
}

func isJSONNull(msg json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(msg), []byte("null"))
}

// nullableString decodes a JSON string or null. ok is false for any other JSON type.
func nullableString(msg json.RawMessage) (value string, isNull bool, ok bool) {
	if isJSONNull(msg) {
		return "", true, true
	}
	if err := json.Unmarshal(msg, &value); err != nil {
		return "", false, false
	}
	return value, false, true
}

// applySecret: a string sets the value (empty is rejected), null clears it, and an omitted key keeps it.
func applySecret(field string, msg json.RawMessage, target *string, check func(string) (string, string), errs map[string]string) {
	value, isNull, ok := nullableString(msg)
	switch {
	case !ok:
		errs[field] = "invalid_value"
	case isNull:
		*target = ""
	case strings.TrimSpace(value) == "":
		errs[field] = "required"
	default:
		normalized, code := check(strings.TrimSpace(value))
		if code != "" {
			errs[field] = code
		} else {
			*target = normalized
		}
	}
}

// applyText replaces a plain string. Optional fields accept null or "" as a clear; required fields reject them.
func applyText(field string, msg json.RawMessage, target *string, optional bool, check func(string) (string, string), errs map[string]string) {
	value, isNull, ok := nullableString(msg)
	value = strings.TrimSpace(value)
	switch {
	case !ok:
		errs[field] = "invalid_value"
	case (isNull || value == "") && optional:
		*target = ""
	case value == "":
		errs[field] = "required"
	default:
		normalized, code := check(value)
		if code != "" {
			errs[field] = code
		} else {
			*target = normalized
		}
	}
}

func applyMinutes(field string, msg json.RawMessage, target *int, errs map[string]string) {
	var f float64
	if isJSONNull(msg) || json.Unmarshal(msg, &f) != nil || f != math.Trunc(f) {
		errs[field] = "invalid_value"
		return
	}
	if f < 1 || f > 1440 {
		errs[field] = "out_of_range"
		return
	}
	*target = int(f)
}

func applyInstructors(field string, msg json.RawMessage, target *map[string]string, errs map[string]string) {
	if isJSONNull(msg) {
		*target = map[string]string{}
		return
	}
	var m map[string]string
	if json.Unmarshal(msg, &m) != nil {
		errs[field] = "invalid_instructor_map"
		return
	}
	clean, ok := cleanInstructors(m)
	if !ok {
		errs[field] = "invalid_instructor_map"
		return
	}
	*target = clean
}

func cleanInstructors(m map[string]string) (map[string]string, bool) {
	if len(m) > 100 {
		return nil, false
	}
	out := make(map[string]string, len(m))
	for code, name := range m {
		code, name = strings.TrimSpace(code), strings.TrimSpace(name)
		if n := utf8.RuneCountInString(code); n < 1 || n > 40 || hasControl(code) {
			return nil, false
		}
		if n := utf8.RuneCountInString(name); n < 1 || n > 100 || hasControl(name) {
			return nil, false
		}
		if _, dup := out[code]; dup {
			return nil, false
		}
		out[code] = name
	}
	return out, true
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func checkFeedURL(s string) (string, string) {
	if len(s) > 2048 {
		return "", "too_long"
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host != "elearning.nbcc.ca" || u.Path != "/d2l/le/calendar/feed/user/feed.ics" || u.Query().Get("token") == "" || u.User != nil || u.Fragment != "" {
		return "", "invalid_feed_url"
	}
	return s, ""
}

func checkAPIKey(s string) (string, string) {
	if utf8.RuneCountInString(s) > 512 {
		return "", "too_long"
	}
	if strings.IndexFunc(s, unicode.IsSpace) >= 0 || hasControl(s) {
		return "", "invalid_value"
	}
	return s, ""
}

func checkWebhookSecret(s string) (string, string) {
	n := utf8.RuneCountInString(s)
	switch {
	case n < 32:
		return "", "too_short"
	case n > 256:
		return "", "too_long"
	case hasControl(s):
		return "", "invalid_value"
	}
	return s, ""
}

func checkUUID(s string) (string, string) {
	if !uuidPattern.MatchString(s) {
		return "", "invalid_uuid"
	}
	return s, ""
}

func checkGroupJID(s string) (string, string) {
	if !groupJIDPattern.MatchString(s) {
		return "", "invalid_group_jid"
	}
	return s, ""
}

func checkGroupName(s string) (string, string) {
	if utf8.RuneCountInString(s) > 100 {
		return "", "too_long"
	}
	if hasControl(s) {
		return "", "invalid_value"
	}
	return s, ""
}

// checkPublicBaseURL accepts an HTTPS origin on the default port. A lone trailing slash is removed.
func checkPublicBaseURL(s string) (string, string) {
	if len(s) > 2048 {
		return "", "too_long"
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Port() != "" && u.Port() != "443") {
		return "", "invalid_https_url"
	}
	host := u.Hostname()
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return "https://" + host, ""
}
