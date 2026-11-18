// handlers.go - the main HTTP router (ServeHTTP), login & password flows,
// student portal routes (/student, /student/attendance), and static file server.
// Public JSON endpoints live in handlers_api.go and /admin handlers in handlers_admin.go.
//
// ServeHTTP is the "front door". Each request passes through it in this order:
//  1. public JSON APIs (/healthz, /api/teachers, /api/notices, /api/gallery, /api/admissions...)
//  2. static files     (pages, css, js, images, downloads)
//  3. safety checks    (a POST must come from our own website)
//  4. login, forgot password, reset password
//  5. pages that need login: change password, /admin, /student
package main

import (
	"crypto/subtle"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// View is the data given to templates/portal.html for one page.
// Tab says which admin section is open; Profile is the student being viewed.
type View struct {
	Attendance                        []AttendanceRecord
	DemoAttendance                    bool
	Subjects                          []Subject
	LoginPath                         string
	AdminLogin                        bool
	IsSuperAdmin                      bool
	IsTeacher                         bool
	TeacherSubjectLabel               string
	ResetEmail                        string
	Title, Mode, Error, Success, CSRF string
	TempPassword                      string
	TempUserEmail                     string
	TempAccountType                   string
	User                              User
	Students                          []User
	Admins                            []User
	Teachers                          []Teacher
	Notices                           []NoticeRecord
	Gallery                           []GalleryItem
	Seats                             SeatConfig
	Admissions                        []AdmissionApplication
	Achievements                      []AchievementRecord
	Stats                             SchoolStats
	ResultPDFs                        []ResultPDFRecord
	ClassResultPDFs                   []ResultPDFRecord
	ClassRoutinePDF                   string
	RollClass                         string
	RollSubject                       string
	RollDate                          string
	RollExam                          string
	RollSubjects                      []Subject
	RollStudents                      []User
	ProgressClasses                   []string
	ProgressStudents                  []ProgressStudent
	TeacherClassesByEmail             map[string]string
	Tab                               string
	Profile                           User
	ProfileInitial                    string
	ProfileSubjects                   []Subject
	BloodGroups                       []string
	MaskedParentPhone                 string
	MaskedEmergencyPhone              string
	MaskedBirthReg                    string
}

// A phone number may have digits, +, -, spaces and brackets (6-20 characters).
var phonePattern = regexp.MustCompile(`^[0-9+\-\s()]{6,20}$`)

var bloodGroups = []string{"A+", "A-", "B+", "B-", "AB+", "AB-", "O+", "O-"}

// Blood group is optional, but if given it must be one of the 8 normal types.
func validBloodGroup(g string) bool {
	if g == "" {
		return true
	}
	for _, b := range bloodGroups {
		if g == b {
			return true
		}
	}
	return false
}

// Fills the HTML template with the View data and sends it to the browser.
func (a *App) render(w http.ResponseWriter, v View) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.pages.Execute(w, v); err != nil {
		log.Print(err)
	}
}

// Runs for every request to the website.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	if a.servePublicAPI(w, r) {
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/teacher/students/") {
		a.serveTeacherProgress(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api") && (strings.HasSuffix(r.URL.Path, ".css") || strings.HasSuffix(r.URL.Path, ".js") || strings.HasPrefix(r.URL.Path, "/images/") || strings.HasPrefix(r.URL.Path, "/downloads/")) {
		a.static(w, r)
		return
	}
	switch r.URL.Path {
	case "/login", "/admin/login", "/forgot-password", "/reset-password", "/change-password", "/student", "/student/attendance", "/admin", "/logout", "/academics.html":
	default:
		a.static(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" && r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	// Safety check: a form submission must come from this same website
	// (Origin header). This blocks other websites from sending forms for a user.
	if r.Method == "POST" {
		scheme := "http"
		if r.TLS != nil || a.secure || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		if r.Header.Get("Origin") != scheme+"://"+r.Host {
			http.Error(w, "Request origin rejected", 403)
			return
		}
		if r.URL.Path == "/admin" && strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
			if r.ParseMultipartForm(6<<20) != nil {
				http.Error(w, "Invalid form or image exceeds 5 MB", 400)
				return
			}
		} else {
			r.Body = http.MaxBytesReader(w, r.Body, 16384)
			if r.ParseForm() != nil {
				http.Error(w, "Invalid form", 400)
				return
			}
		}
	}
	// From here on we read/change shared data, so lock it: one request at a time.
	// We also clean out expired sessions, login attempts and reset codes.
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for id, s := range a.sessions {
		if now.After(s.Expires) {
			delete(a.sessions, id)
		}
	}
	for ip, v := range a.attempts {
		if now.After(v.Until) {
			delete(a.attempts, ip)
		}
	}
	for emailKey, rc := range a.resets {
		if now.After(rc.Expires) {
			delete(a.resets, emailKey)
		}
	}
	// Who is making this request? (logged = false if nobody is logged in)
	id, s, u, logged := a.session(r)
	if r.URL.Path == "/academics.html" {
		http.Redirect(w, r, "/login", 303)
		return
	}
	// Forgot password, step 1: enter email, we make a 6-digit code (valid 15 minutes).
	if r.URL.Path == "/forgot-password" {
		v := View{Mode: "forgot", Title: "Forgot password"}
		if r.Method == "POST" {
			email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
			candidate, exists := a.users[email]
			if !validEmail(email) {
				v.Error = "Enter a valid school email address."
			} else if !exists {
				v.Error = "No account found with that email address."
			} else {
				code := generateResetCode()
				a.resets[email] = ResetCode{Code: code, Expires: now.Add(15 * time.Minute)}
				log.Printf("Password reset verification code for %s (%s): %s", candidate.Name, email, code)
				go func(to, name, c string) {
					_ = a.sendSchoolEmail(to, "S. M. Pilot Model School — Password Reset Code: "+c,
						"Dear "+name+",\n\nYour 6-digit password reset verification code is: "+c+"\n\nThis code is valid for 15 minutes.\n\nS. M. Pilot Model School")
				}(email, candidate.Name, code)
				http.Redirect(w, r, "/reset-password?email="+url.QueryEscape(email)+"&demo_code="+url.QueryEscape(code), 303)
				return
			}
		}
		a.render(w, v)
		return
	}
	// Forgot password, step 2: enter the code and a new password.
	if r.URL.Path == "/reset-password" {
		email := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
		if r.Method == "POST" {
			email = strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		}
		v := View{Mode: "reset", Title: "Reset password", ResetEmail: email}
		if r.Method == "GET" {
			if demoCode := r.URL.Query().Get("demo_code"); demoCode != "" {
				v.Success = "Verification code sent to " + email + " · Verification code: " + demoCode
			}
		} else if r.Method == "POST" {
			code := strings.TrimSpace(r.FormValue("code"))
			p := r.FormValue("password")
			confirm := r.FormValue("confirm")
			rc, hasReset := a.resets[email]
			candidate, exists := a.users[email]
			if !exists || !hasReset || now.After(rc.Expires) || subtle.ConstantTimeCompare([]byte(code), []byte(rc.Code)) != 1 {
				v.Error = "Invalid or expired 6-digit verification code."
			} else if len(p) < 8 || len(p) > 128 {
				v.Error = "Use a password between 8 and 128 characters."
			} else if p != confirm {
				v.Error = "The passwords do not match."
			} else {
				setPassword(&candidate, p)
				candidate.MustChange = false
				if err := a.put(candidate); err != nil {
					v.Error = "Unable to save the new password. Please try again."
				} else {
					delete(a.resets, email)
					for key, session := range a.sessions {
						if session.Email == email {
							delete(a.sessions, key)
						}
					}
					targetLogin := "/login?reset=1"
					if isStaffRole(candidate.Role) {
						targetLogin = "/admin/login?reset=1"
					}
					http.Redirect(w, r, targetLogin, 303)
					return
				}
			}
		}
		a.render(w, v)
		return
	}
	// Login for students (/login) and for teachers/admins (/admin/login).
	// A student account cannot log in on the admin page, and the other way round.
	if r.URL.Path == "/login" || r.URL.Path == "/admin/login" {
		role := "student"
		title := "Student login"
		if r.URL.Path == "/admin/login" {
			role = "admin"
			title = "Teacher & Admin login"
		}
		if logged && ((role == "student" && u.Role == "student") || (role == "admin" && isStaffRole(u.Role))) {
			http.Redirect(w, r, destination(u), 303)
			return
		}
		v := View{Mode: "login", Title: title, LoginPath: r.URL.Path, AdminLogin: role == "admin"}
		if r.Method == "GET" && r.URL.Query().Get("reset") == "1" {
			v.Success = "Your password has been reset successfully. Please log in with your new password."
		}
		if r.Method == "POST" {
			// Block password guessing: after 10 wrong tries from one address, wait 15 minutes.
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			isLoopback := ip == "127.0.0.1" || ip == "::1"
			limit := a.attempts[ip]
			if !isLoopback && limit.Count >= 10 {
				w.WriteHeader(429)
				v.Error = "Too many attempts. Try again in 15 minutes."
				a.render(w, v)
				return
			}
			if !isLoopback {
				limit.Count++
				if limit.Until.IsZero() {
					limit.Until = now.Add(15 * time.Minute)
				}
				a.attempts[ip] = limit
			}
			email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
			candidate, exists := a.users[email]
			if !exists {
				candidate.Salt = "unknown-account"
				candidate.Hash = strings.Repeat("0", 64)
			}
			roleAllowed := (role == "student" && candidate.Role == "student") || (role == "admin" && isStaffRole(candidate.Role))
			if matches(candidate, r.FormValue("password")) && exists && roleAllowed {
				delete(a.attempts, ip)
				delete(a.sessions, id)
				sid := token()
				a.sessions[sid] = Session{email, token(), now.Add(8 * time.Hour)}
				_ = a.saveSessions()
				a.cookie(w, sid, 28800)
				http.Redirect(w, r, destination(candidate), 303)
				return
			}
			v.Error = "Email or password is incorrect."
		}
		a.render(w, v)
		return
	}
	// Everything below needs a login. Not logged in -> send to the login page.
	if !logged {
		loginPath := "/login"
		if r.URL.Path == "/admin" {
			loginPath = "/admin/login"
		}
		http.Redirect(w, r, loginPath, 303)
		return
	}
	// CSRF protection: every form carries the secret token of this session.
	// If it is missing or wrong, the request is refused.
	if r.Method == "POST" && r.FormValue("csrf") != s.CSRF {
		http.Error(w, "Session verification failed. Reload the page.", 403)
		return
	}
	if r.URL.Path == "/logout" {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		delete(a.sessions, id)
		_ = a.saveSessions()
		a.cookie(w, "", -1)
		loginPath := "/login"
		if isStaffRole(u.Role) {
			loginPath = "/admin/login"
		}
		http.Redirect(w, r, loginPath, 303)
		return
	}
	if u.MustChange && r.URL.Path != "/change-password" {
		http.Redirect(w, r, "/change-password", 303)
		return
	}
	v := View{User: u, CSRF: s.CSRF, Subjects: subjectsFor(u), Attendance: u.Attendance}
	if r.URL.Path == "/student/attendance" && u.Role == "student" && len(u.Attendance) == 0 {
		v.Attendance = sampleAttendance(v.Subjects)
		v.DemoAttendance = true
	}
	switch r.URL.Path {
	case "/change-password":
		v.Mode = "password"
		v.Title = "Choose your own password"
		if r.Method == "POST" {
			p := r.FormValue("password")
			if len(p) < 8 || len(p) > 128 {
				v.Error = "Use a password between 8 and 128 characters."
			} else if p != r.FormValue("confirm") {
				v.Error = "The passwords do not match."
			} else if !matches(u, r.FormValue("current")) {
				v.Error = "Your current password is incorrect."
			} else if matches(u, p) {
				v.Error = "Choose a different password from your current password."
			} else {
				setPassword(&u, p)
				u.MustChange = false
				if err := a.put(u); err != nil {
					v.Error = "Unable to save the password. Please try again."
				} else {
					for key, session := range a.sessions {
						if session.Email == u.Email {
							delete(a.sessions, key)
						}
					}
					sid := token()
					a.sessions[sid] = Session{u.Email, token(), now.Add(8 * time.Hour)}
					_ = a.saveSessions()
					a.cookie(w, sid, 28800)
					http.Redirect(w, r, destination(u), 303)
					return
				}
			}
		}
	case "/admin":
		a.serveAdmin(w, r, id, u, &v, now)
		return
	case "/student", "/student/attendance":
		// Student dashboard. A student only ever sees their own account (taken from
		// the login session, never from the URL).
		if u.Role != "student" {
			http.Redirect(w, r, "/admin", 303)
			return
		}
		v.Mode = "student"
		v.Title = "Your student dashboard"
		if r.Method == "GET" {
			if flash, ok := a.flashes[id]; ok {
				v.Success = flash.Success
				v.Error = flash.Error
				delete(a.flashes, id)
			}
		}
		if r.Method == "POST" && r.URL.Path == "/student" {
			action := strings.TrimSpace(r.FormValue("action"))
			if action == "request-correction" {
				field := strings.TrimSpace(r.FormValue("correction_field"))
				details := strings.TrimSpace(r.FormValue("correction_details"))
				if field == "" || len(field) > 80 || details == "" || len(details) > 400 {
					v.Error = "Select a profile field and describe the correction needed (up to 400 characters)."
				} else {
					req := CorrectionRequest{
						ID:          "COR-" + strings.ToUpper(token()[:6]),
						Field:       field,
						Details:     details,
						Status:      "Pending",
						SubmittedAt: now.Format("2006-01-02 15:04"),
					}
					u.CorrectionRequests = append([]CorrectionRequest{req}, u.CorrectionRequests...)
					if err := a.put(u); err != nil {
						v.Error = "Unable to submit correction request."
					} else {
						v.Success = "Correction request (" + req.ID + ") sent to the school office for review."
					}
				}
			}
			a.flashes[id] = FlashData{Success: v.Success, Error: v.Error}
			http.Redirect(w, r, "/student", 303)
			return
		}
		v.MaskedParentPhone = maskPhone(u.ParentPhone)
		v.MaskedEmergencyPhone = maskPhone(u.EmergencyPhone)
		v.MaskedBirthReg = maskBirthReg(u.BirthRegistration)
		v.ProfileInitial = studentInitials(u.Name)
		v.Stats = a.stats
		v.Notices = append([]NoticeRecord(nil), a.notices...)
		if a.stats.RoutinePDFs != nil {
			if p := a.stats.RoutinePDFs[u.Class]; p != "" {
				v.ClassRoutinePDF = p
			} else if p := a.stats.RoutinePDFs["All"]; p != "" {
				v.ClassRoutinePDF = p
			}
		}
		for _, rp := range a.resultPDFs {
			if rp.Class == u.Class || rp.Class == "All" {
				v.ClassResultPDFs = append(v.ClassResultPDFs, rp)
			}
		}
		if r.URL.Path == "/student/attendance" {
			v.Mode = "attendance"
			v.Title = "Your attendance"
		}
	}
	a.render(w, v)
}

// Serves the public website files. Only listed file types and folders are
// allowed, so private files (data/, .go files, templates) always return 404.
func (a *App) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		p = "home.html"
	}
	allowed := false
	if !strings.Contains(p, "/") {
		ext := filepath.Ext(p)
		allowed = ext == ".html" || ext == ".css" || ext == ".js"
		if p == "academics.js" {
			allowed = false
		}
	} else if strings.HasPrefix(p, "css/") && strings.Count(p, "/") == 1 {
		allowed = filepath.Ext(p) == ".css"
	} else if strings.HasPrefix(p, "js/") && strings.Count(p, "/") == 1 {
		allowed = filepath.Ext(p) == ".js" && filepath.Base(p) != "academics.js"
	} else if strings.HasPrefix(p, "images/") || strings.HasPrefix(p, "downloads/") {
		ext := strings.ToLower(filepath.Ext(p))
		allowed = ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" || ext == ".svg" || ext == ".pdf" || ext == ".html"
	}
	clean := filepath.Clean(p)
	if !allowed || clean != p || strings.HasPrefix(clean, ".") {
		http.NotFound(w, r)
		return
	}
	candidates := []string{filepath.Join(a.frontendDir, p), p}
	if !strings.Contains(p, "/") {
		switch filepath.Ext(p) {
		case ".css":
			candidates = append(candidates, filepath.Join(a.frontendDir, "css", p), filepath.Join("css", p))
		case ".js":
			candidates = append(candidates, filepath.Join(a.frontendDir, "js", p), filepath.Join("js", p))
		}
	}
	for _, target := range candidates {
		resolved, err := filepath.EvalSymlinks(target)
		if err != nil || resolved != filepath.Clean(target) {
			continue
		}
		info, err := os.Stat(target)
		if err != nil || info.IsDir() {
			continue
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, target)
		return
	}
	http.NotFound(w, r)
}
