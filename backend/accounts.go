// accounts.go - the user accounts (students, school admins, super admin)
// and how they are saved to data/accounts.json.
package main

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
)

// One attendance entry: which day, which subject, present or absent.
type AttendanceRecord struct {
	Date    string
	Subject string
	Status  string
}

// One exam result entry for a student (exam name, subject, marks out of 100, and letter grade).
type ResultRecord struct {
	Exam    string
	Subject string
	Marks   int
	Grade   string
}

// Converts marks (0-100) into the standard Bangladesh board letter grade.
func gradeForMarks(marks int) string {
	switch {
	case marks >= 80:
		return "A+"
	case marks >= 70:
		return "A"
	case marks >= 60:
		return "A-"
	case marks >= 50:
		return "B"
	case marks >= 40:
		return "C"
	case marks >= 33:
		return "D"
	default:
		return "F"
	}
}

// User is one account. Role is "student", "admin" or "superadmin".
// Salt and Hash store the password safely - the real password is never saved.
// MustChange is true until the user replaces a temporary password.
type User struct {
	Attendance []AttendanceRecord
	Results    []ResultRecord
	Group      string
	Email      string
	Name       string
	StudentID  string
	Class      string
	Role       string
	Salt       string
	Hash       string
	MustChange bool

	// Student profile details (optional, maintained by school administrators).
	Gender         string
	DateOfBirth    string
	BloodGroup     string
	Address        string
	FatherName     string
	MotherName     string
	ParentPhone    string
	EmergencyPhone string
	ReportCardPDF  string
	ReportCardExam string
	TeacherSubject string
	// TeacherClasses lists the classes (e.g. "9", "10") a teacher is assigned to.
	// It limits which students the teacher may see in Student Progress.
	TeacherClasses []string `json:",omitempty"`
	CurrentStatus  string `json:"-"`
	CurrentMarks   string `json:"-"`
	CurrentGrade   string `json:"-"`
}

// save writes all accounts to a temporary file first and then renames it.
// This way a crash in the middle of saving cannot leave a broken accounts file.
func (a *App) save() error {
	if err := os.MkdirAll(filepath.Dir(a.file), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a.users, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(a.file), "accounts-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, a.file); err != nil {
		return err
	}
	a.writeSQLSnapshot()
	return nil
}

// put adds or updates one account and saves. If saving fails, it undoes the
// change so memory and the file always match.
func (a *App) put(u User) error {
	old, exists := a.users[u.Email]
	a.users[u.Email] = u
	if err := a.save(); err != nil {
		if exists {
			a.users[u.Email] = old
		} else {
			delete(a.users, u.Email)
		}
		return err
	}
	return nil
}

// validEmail checks that the text looks like a real email address.
func validEmail(s string) bool {
	m, e := mail.ParseAddress(s)
	return e == nil && m.Address == s && len(s) <= 254
}

// load reads the accounts file when the server starts (no file yet is fine).
func (a *App) load() error {
	data, err := os.ReadFile(a.file)
	if err == nil {
		if err = json.Unmarshal(data, &a.users); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Command-line helper: "go run . init-admin email" creates the first school admin
// and prints a temporary password.
func (a *App) initializeAdmin(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) {
		return fmt.Errorf("enter a valid email")
	}
	for _, u := range a.users {
		if u.Role == "admin" {
			return fmt.Errorf("an administrator already exists")
		}
	}
	password := token()[:20]
	u := User{Email: email, Name: "School office", Role: "admin", MustChange: true}
	setPassword(&u, password)
	if err := a.put(u); err != nil {
		return err
	}
	fmt.Printf("Email: %s\nTemporary password: %s\n", email, password)
	return nil
}

// Command-line helper: gives a school admin a new temporary password and logs
// them out everywhere.
func (a *App) resetAdmin(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	u, exists := a.users[email]
	if !exists || u.Role != "admin" {
		return fmt.Errorf("administrator account not found")
	}
	password := token()[:20]
	setPassword(&u, password)
	u.MustChange = true
	if err := a.put(u); err != nil {
		return err
	}
	for id, session := range a.sessions {
		if session.Email == email {
			delete(a.sessions, id)
		}
	}
	fmt.Printf("Email: %s\nTemporary password: %s\n", email, password)
	return nil
}
