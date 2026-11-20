// handlers_api.go - public JSON API endpoints used by the frontend pages:
//   - GET  /healthz               -> server health check
//   - GET  /api/teachers          -> live faculty list for teachers.html
//   - GET  /api/notices           -> live notice board list for notice.html
//   - GET  /api/gallery           -> live photo list for gallery.html
//   - GET  /api/achievements      -> live achievements list for achievements.html
//   - GET  /api/admissions/seats  -> open seat counts for admission.html & admission-form.html
//   - GET  /api/stats             -> "School at a glance" numbers for home.html
//   - POST /api/admissions        -> online admission form submission (with photo & PDF uploads)
package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// servePublicAPI answers public JSON endpoints that do not require a login session.
// It returns true if the request path matched a public API route.
func (a *App) servePublicAPI(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return true
	}
	// Public list of teachers (JSON). teachers.js reads it so the page updates live.
	if r.URL.Path == "/api/teachers" {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return true
		}
		a.mu.Lock()
		teachers := append([]Teacher(nil), a.teachers...)
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(teachers)
		return true
	}
	// Public list of notices (JSON). notice.html reads it so new notices appear live.
	if r.URL.Path == "/api/notices" {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return true
		}
		a.mu.Lock()
		notices := append([]NoticeRecord(nil), a.notices...)
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(notices)
		return true
	}
	// Public list of gallery photos (JSON). gallery.html reads it live.
	if r.URL.Path == "/api/gallery" {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return true
		}
		a.mu.Lock()
		items := append([]GalleryItem(nil), a.gallery...)
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(items)
		return true
	}
	// Public list of achievements (JSON). achievements.html reads it live.
	if r.URL.Path == "/api/achievements" {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return true
		}
		a.mu.Lock()
		items := append([]AchievementRecord(nil), a.achievements...)
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(items)
		return true
	}
	// Public seat availability for admission.html and admission-form.html.
	if r.URL.Path == "/api/admissions/seats" {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return true
		}
		a.mu.Lock()
		seats := a.seats
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(seats)
		return true
	}
	// Public "School at a glance" statistics for home.html.
	if r.URL.Path == "/api/stats" {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return true
		}
		a.mu.Lock()
		st := a.stats
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(st)
		return true
	}
	// Public endpoint where admission-form.html submits an online application to the school office.
	if r.URL.Path == "/api/admissions" {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return true
		}
		var app AdmissionApplication
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
			if err := r.ParseMultipartForm(16 << 20); err != nil {
				http.Error(w, "Invalid form or uploaded files exceed limit", 400)
				return true
			}
			app.StudentName = strings.TrimSpace(r.FormValue("student"))
			app.StudentBangla = strings.TrimSpace(r.FormValue("studentBangla"))
			app.Class = strings.TrimSpace(r.FormValue("class"))
			app.Group = strings.TrimSpace(r.FormValue("group"))
			app.DOB = strings.TrimSpace(r.FormValue("dob"))
			app.BirthRegistration = strings.TrimSpace(r.FormValue("birthRegistration"))
			app.Gender = strings.TrimSpace(r.FormValue("gender"))
			app.FatherName = strings.TrimSpace(r.FormValue("father"))
			app.MotherName = strings.TrimSpace(r.FormValue("mother"))
			app.GuardianName = strings.TrimSpace(r.FormValue("guardian"))
			app.Relationship = strings.TrimSpace(r.FormValue("relationship"))
			app.GuardianPhone = strings.TrimSpace(r.FormValue("phone"))
			app.GuardianEmail = strings.TrimSpace(r.FormValue("email"))
			app.GuardianNID = strings.TrimSpace(r.FormValue("guardianNid"))
			app.Address = strings.TrimSpace(r.FormValue("address"))
			app.PermanentAddress = strings.TrimSpace(r.FormValue("permanentAddress"))
			app.District = strings.TrimSpace(r.FormValue("district"))
			app.Upazila = strings.TrimSpace(r.FormValue("upazila"))
			app.PreviousSchool = strings.TrimSpace(r.FormValue("previousSchool"))
			app.PreviousResult = strings.TrimSpace(r.FormValue("previousResult"))
			app.PreviousRoll = strings.TrimSpace(r.FormValue("previousRoll"))
			app.Category = strings.TrimSpace(r.FormValue("category"))
			app.CategoryEvidence = strings.TrimSpace(r.FormValue("categoryEvidence"))
			if p, err := a.saveUploadedAdmissionFile(r, "studentPhoto"); err == nil {
				app.StudentPhoto = p
			}
			if p, err := a.saveUploadedAdmissionFile(r, "transferCertificate"); err == nil {
				app.TransferCertificate = p
			}
			if p, err := a.saveUploadedAdmissionFile(r, "academicCertificate"); err == nil {
				app.AcademicCertificate = p
			}
		} else {
			r.Body = http.MaxBytesReader(w, r.Body, 32768)
			if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
				http.Error(w, "Invalid application data", 400)
				return true
			}
			app.StudentName = strings.TrimSpace(app.StudentName)
			app.Class = strings.TrimSpace(app.Class)
			app.GuardianName = strings.TrimSpace(app.GuardianName)
			app.GuardianPhone = strings.TrimSpace(app.GuardianPhone)
			if !strings.HasPrefix(app.StudentPhoto, "downloads/") && !strings.HasPrefix(app.StudentPhoto, "images/") {
				app.StudentPhoto = ""
			}
			if !strings.HasPrefix(app.TransferCertificate, "downloads/") {
				app.TransferCertificate = ""
			}
			if !strings.HasPrefix(app.AcademicCertificate, "downloads/") {
				app.AcademicCertificate = ""
			}
		}
		if app.StudentName == "" || len(app.StudentName) > 120 || app.Class == "" || strings.TrimSpace(app.GuardianPhone) == "" {
			http.Error(w, "Missing required applicant fields", 400)
			return true
		}
		a.mu.Lock()
		app.ID = "APP-" + strings.ToUpper(token()[:8])
		app.Status = "Pending"
		app.SubmittedAt = time.Now().Format("2006-01-02 15:04")
		a.admissions = append([]AdmissionApplication{app}, a.admissions...)
		_ = a.saveAdmissions()
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": app.ID, "status": app.Status})
		return true
	}
	return false
}
