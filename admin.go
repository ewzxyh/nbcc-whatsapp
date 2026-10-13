package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	minPasswordLength = 12
	maxPasswordLength = 256

	// OWASP argon2id parameters.
	argonMemoryKiB = 19456
	argonTime      = 2
	argonThreads   = 1
	argonKeyLen    = 32
	argonSaltLen   = 16
)

var errAlreadyConfigured = errors.New("admin account already exists")

type adminAccount struct {
	Email        string    `json:"email"`
	PasswordHash string    `json:"passwordHash"`
	CreatedAt    time.Time `json:"createdAt"`
}

// AdminStore holds the single administrator account. While no account exists, a one-time setup code is kept
// in memory as a SHA-256 digest only.
type AdminStore struct {
	mu        sync.Mutex
	path      string
	acct      *adminAccount
	setupHash [32]byte
}

// openAdmin loads <dir>/admin.json. When it is absent, a setup code is generated and logged exactly once.
func openAdmin(dir string) (*AdminStore, error) {
	st := &AdminStore{path: filepath.Join(dir, "admin.json")}
	b, err := os.ReadFile(st.path)
	if errors.Is(err, fs.ErrNotExist) {
		code, digest := newSetupCode()
		st.setupHash = digest
		log.Printf("admin setup code: %s (use it at /admin within this process lifetime)", code)
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	var acct adminAccount
	if json.Unmarshal(b, &acct) != nil || acct.Email == "" || acct.PasswordHash == "" {
		return nil, errors.New("invalid admin account file")
	}
	st.acct = &acct
	return st, nil
}

// newSetupCode returns a 60-bit code in XXXX-XXXX-XXXX form and the digest of its canonical form.
func newSetupCode() (string, [32]byte) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	canonical := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])[:12]
	code := canonical[0:4] + "-" + canonical[4:8] + "-" + canonical[8:12]
	return code, sha256.Sum256([]byte(canonical))
}

// normalizeSetupCode accepts the code with or without hyphens and in any letter case.
func normalizeSetupCode(s string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(s)))
}

func (st *AdminStore) configured() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.acct != nil
}

// account returns a copy of the account, or false during setup.
func (st *AdminStore) account() (adminAccount, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.acct == nil {
		return adminAccount{}, false
	}
	return *st.acct, true
}

// checkSetupCode compares digests in constant time. It reports false once the account exists.
func (st *AdminStore) checkSetupCode(input string) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.acct != nil {
		return false
	}
	digest := sha256.Sum256([]byte(normalizeSetupCode(input)))
	return subtle.ConstantTimeCompare(digest[:], st.setupHash[:]) == 1
}

// create stores the first account and retires the setup code.
func (st *AdminStore) create(email, passwordHash string, now time.Time) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.acct != nil {
		return errAlreadyConfigured
	}
	acct := adminAccount{Email: email, PasswordHash: passwordHash, CreatedAt: now.In(moncton)}
	if err := saveAdmin(st.path, acct); err != nil {
		return err
	}
	st.acct = &acct
	st.setupHash = [32]byte{}
	return nil
}

// update applies a change to a copy of the account and commits it only after it is persisted.
func (st *AdminStore) update(change func(*adminAccount)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.acct == nil {
		return errors.New("admin account is not configured")
	}
	next := *st.acct
	change(&next)
	if err := saveAdmin(st.path, next); err != nil {
		return err
	}
	st.acct = &next
	return nil
}

func saveAdmin(path string, acct adminAccount) error {
	b, err := json.Marshal(acct)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b)
}

// verify checks the login e-mail and password. An unknown e-mail is verified against a dummy hash so that
// both outcomes spend the same argon2 work.
func (st *AdminStore) verify(email, password string) bool {
	acct, ok := st.account()
	if !ok {
		verifyPassword(password, dummyHash())
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == acct.Email {
		return verifyPassword(password, acct.PasswordHash)
	}
	verifyPassword(password, dummyHash())
	return false
}

var dummyHash = sync.OnceValue(func() string {
	h, err := hashPassword(randomToken(32))
	if err != nil {
		panic(err)
	}
	return h
})

// hashPassword returns an argon2id PHC string.
func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// verifyPassword compares in constant time against an argon2id PHC string. Malformed hashes never verify.
func verifyPassword(password, phc string) bool {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return false
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 || len(want) > 64 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// passwordCode returns "" when the password is acceptable for the given e-mail, or a validation code.
func passwordCode(password, email string) string {
	n := utf8.RuneCountInString(password)
	switch {
	case n < minPasswordLength:
		return "too_short"
	case n > maxPasswordLength:
		return "too_long"
	case strings.EqualFold(password, email):
		return "same_as_email"
	}
	return ""
}

// normalizeEmail lower-cases and validates a plain address (no display name).
func normalizeEmail(s string) (string, bool) {
	e := strings.ToLower(strings.TrimSpace(s))
	if len(e) < 3 || len(e) > 254 || strings.Count(e, "@") != 1 {
		return "", false
	}
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e {
		return "", false
	}
	if !strings.Contains(e[strings.IndexByte(e, '@')+1:], ".") {
		return "", false
	}
	return e, true
}

// randomToken returns n random bytes encoded as unpadded base64url.
func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
