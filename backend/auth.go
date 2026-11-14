// auth.go - passwords, sessions and cookies.
package main

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// A logged-in visit. CSRF is a secret token that every form must send back,
// so other websites cannot submit forms on the user's behalf.
type Session struct {
	Email, CSRF string
	Expires     time.Time
}

// Counts failed logins from one address (used to block guessing).
type Attempts struct {
	Count int
	Until time.Time
}

// A 6-digit code for the forgot-password flow. It expires after 15 minutes.
type ResetCode struct {
	Code    string
	Expires time.Time
}

func generateResetCode() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	digits := make([]byte, 6)
	for i := range b {
		digits[i] = '0' + (b[i] % 10)
	}
	return string(digits)
}

// Makes a long random secret (used for sessions, salts and temporary passwords).
func token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Turns a password into a scrambled value that cannot be turned back.
// PBKDF2 with 600,000 rounds makes password guessing very slow.
func passwordHash(password, salt string) string {
	key, err := pbkdf2.Key(sha256.New, password, []byte(salt), 600000, 32)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(key)
}

// Stores a new password: a fresh random salt plus the hash (never the password).
func setPassword(u *User, password string) {
	u.Salt = token()
	u.Hash = passwordHash(password, u.Salt)
}

// Checks a typed password. The constant-time compare avoids leaking hints
// through how long the check takes.
func matches(u User, password string) bool {
	return subtle.ConstantTimeCompare([]byte(passwordHash(password, u.Salt)), []byte(u.Hash)) == 1
}

// Finds who is logged in from the cookie. The last value is false if the
// cookie is missing, unknown or expired.
func (a *App) session(r *http.Request) (string, Session, User, bool) {
	c, err := r.Cookie("school_session")
	if err != nil {
		return "", Session{}, User{}, false
	}
	s, ok := a.sessions[c.Value]
	u, exists := a.users[s.Email]
	return c.Value, s, u, ok && exists && time.Now().Before(s.Expires)
}

// Sets the login cookie. HttpOnly = scripts cannot read it, SameSite Strict =
// not sent from other sites, Secure = HTTPS only (on the live site).
func (a *App) cookie(w http.ResponseWriter, id string, age int) {
	http.SetCookie(w, &http.Cookie{Name: "school_session", Value: id, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode, MaxAge: age})
}

// Both school admins and the super admin may use the school office admin panel.
func isAdminRole(role string) bool {
	return role == "admin" || role == "superadmin"
}

// Teachers, school admins and the super admin sign in at /admin/login.
func isStaffRole(role string) bool {
	return role == "admin" || role == "superadmin" || role == "teacher"
}

// Where to send a user after login: change password first, then their dashboard.
func destination(u User) string {
	if u.MustChange {
		return "/change-password"
	}
	if u.Role == "teacher" {
		return "/admin?tab=attendance"
	}
	if isAdminRole(u.Role) {
		return "/admin"
	}
	return "/student"
}

func (a *App) sessionsFile() string {
	return filepath.Join(filepath.Dir(a.file), "sessions.json")
}

// Loads saved sessions from data/sessions.json so server restarts do not log users out.
func (a *App) loadSessions() error {
	data, err := os.ReadFile(a.sessionsFile())
	if err == nil {
		return json.Unmarshal(data, &a.sessions)
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (a *App) saveSessions() error {
	return writeJSONAtomic(a.sessionsFile(), a.sessions)
}
