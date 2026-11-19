// main.go - starts the school website.
// It creates the App (all data kept in memory + saved to files in data/),
// loads the page template, creates the Super Admin from environment
// variables, and then starts the web server.
package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// FlashData is a one-time message shown after a form is submitted.
// We keep it on the server for ONE page load and then delete it, so a
// temporary password can never be shown again after a refresh.
type FlashData struct {
	Success     string
	Error       string
	TempPass    string
	TempEmail   string
	AccountType string
}

// App holds everything the website needs while it is running.
// mu is a lock: only one request may change the data at a time.
type App struct {
	mu          sync.Mutex
	users        map[string]User
	teachers     []Teacher
	notices      []NoticeRecord
	gallery      []GalleryItem
	seats        SeatConfig
	admissions   []AdmissionApplication
	achievements []AchievementRecord
	stats        SchoolStats
	resultPDFs   []ResultPDFRecord
	sessions     map[string]Session
	flashes     map[string]FlashData
	attempts    map[string]Attempts
	resets      map[string]ResetCode
	file        string
	frontendDir string
	pages       *template.Template
	secure      bool
}

// Finds the folder that holds the public pages (home.html, notice.html ...).
func detectFrontendDir() string {
	for _, dir := range []string{"frontend", "../frontend"} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return "."
}

// Finds templates/portal.html - the page used for login, student and admin screens.
func detectTemplatePath() string {
	for _, p := range []string{"templates/portal.html", "backend/templates/portal.html", "../templates/portal.html"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "templates/portal.html"
}

// newApp builds the App and loads saved accounts, teachers and notices from disk.
func newApp(file string) (*App, error) {
	a := &App{
		users:       map[string]User{},
		sessions:    map[string]Session{},
		flashes:     map[string]FlashData{},
		attempts:    map[string]Attempts{},
		resets:      map[string]ResetCode{},
		file:        file,
		frontendDir: detectFrontendDir(),
		secure:      os.Getenv("COOKIE_SECURE") == "true",
	}
	if err := a.load(); err != nil {
		return nil, err
	}
	if err := a.loadTeachers(); err != nil {
		return nil, err
	}
	if err := a.loadNotices(); err != nil {
		return nil, err
	}
	if err := a.loadGallery(); err != nil {
		return nil, err
	}
	if err := a.loadAdmissions(); err != nil {
		return nil, err
	}
	if err := a.loadSessions(); err != nil {
		return nil, err
	}
	var err error
	a.pages, err = template.ParseFiles(detectTemplatePath())
	return a, err
}

// ensureSuperAdmin creates the Super Admin account from environment variables.
// The email and password are NOT written in the code, so nobody can read them
// from GitHub. On the hosting site (Render) we set them in the dashboard.
func (a *App) ensureSuperAdmin() {
	email := strings.ToLower(strings.TrimSpace(os.Getenv("SUPERADMIN_EMAIL")))
	password := os.Getenv("SUPERADMIN_PASSWORD")
	if email == "" || password == "" {
		log.Printf("No Super Admin created: set SUPERADMIN_EMAIL and SUPERADMIN_PASSWORD to create one.")
		return
	}
	if !validEmail(email) || len(password) < 8 || len(password) > 128 {
		log.Printf("Super Admin not created: use a valid email and a password of 8-128 characters.")
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	// If the account already exists we leave it alone, so a password the
	// owner changed later is never overwritten when the server restarts.
	if _, exists := a.users[email]; exists {
		return
	}
	name := strings.TrimSpace(os.Getenv("SUPERADMIN_NAME"))
	if name == "" {
		name = "Super Admin"
	}
	superAdmin := User{Email: email, Name: name, Role: "superadmin"}
	setPassword(&superAdmin, password)
	if err := a.put(superAdmin); err != nil {
		log.Printf("Unable to save the Super Admin account: %v", err)
		return
	}
	log.Printf("Super Admin account created for %s", email)
}

// ensureDemoAccounts adds a sample school admin and student for local testing.
// It only runs when SEED_DEMO=true, so the live website never has these
// well-known accounts. Passwords are deliberately not printed to the log.
func (a *App) ensureDemoAccounts() {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Do nothing if the school already has real accounts besides a Super Admin.
	for _, existing := range a.users {
		if existing.Role != "superadmin" {
			return
		}
	}
	admin := User{
		Email:      "admin@smpilot.edu.bd",
		Name:       "School Office Admin",
		Role:       "admin",
		MustChange: false,
	}
	setPassword(&admin, "AdminPass123")
	_ = a.put(admin)

	teacher := User{
		Email:          "teacher@smpilot.edu.bd",
		Name:           "Md. Rahim Uddin (Mathematics Teacher)",
		Role:           "teacher",
		TeacherSubject: "Mathematics",
		TeacherClasses: []string{"9", "10"},
		MustChange:     false,
	}
	setPassword(&teacher, "TeacherPass123")
	_ = a.put(teacher)

	student := User{
		Email:             "student@smpilot.edu.bd",
		Name:              "Rahim Uddin",
		NameBangla:        "রহিম উদ্দিন",
		StudentID:         "2026-901",
		Class:             "9",
		Group:             "Science",
		Section:           "B",
		RollNo:            "17",
		Session:           "2026",
		Role:              "student",
		Gender:            "Male",
		DateOfBirth:       "2011-04-15",
		BloodGroup:        "O+",
		Religion:          "Islam",
		Nationality:       "Bangladeshi",
		BirthRegistration: "20111234567894821",
		EducationBoard:    "Cumilla",
		AdmissionDate:     "02 January 2021",
		FatherName:        "Abdul Karim Uddin",
		FatherOccupation:  "Businessman",
		MotherName:        "Amina Begum",
		MotherOccupation:  "Homemaker",
		GuardianName:      "Abdul Karim Uddin (Father)",
		ParentPhone:       "01711-223344",
		EmergencyPhone:    "01819-556677",
		Address:           "House 12, College Road, Bancharampur, Brahmanbaria",
		PermanentAddress:  "Village: Bancharampur, Post: Bancharampur (3420), Upazila: Bancharampur, District: Brahmanbaria",
		MustChange:        false,
	}
	setPassword(&student, "StudentPass123")
	_ = a.put(student)
	log.Printf("Demo accounts created (SEED_DEMO=true): admin@smpilot.edu.bd, teacher@smpilot.edu.bd and student@smpilot.edu.bd")
}

// main runs first. It reads settings from environment variables:
//
//	DATA_FILE  where accounts are saved (default data/accounts.json)
//	PORT       port number given by the hosting site (default 8080)
//	SEED_DEMO  set to "true" to also create demo accounts (local testing only)
func main() {
	dataFile := os.Getenv("DATA_FILE")
	if dataFile == "" {
		dataFile = "data/accounts.json"
	}
	a, err := newApp(dataFile)
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 {
		if len(os.Args) != 3 || (os.Args[1] != "init-admin" && os.Args[1] != "reset-admin") {
			log.Fatal("Usage: go run . [init-admin|reset-admin] office@example.com")
		}
		var err error
		if os.Args[1] == "reset-admin" {
			err = a.resetAdmin(os.Args[2])
		} else {
			err = a.initializeAdmin(os.Args[2])
		}
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	a.ensureSuperAdmin()
	if os.Getenv("SEED_DEMO") == "true" {
		a.ensureDemoAccounts()
	} else if _, hasDemoAdmin := a.users["admin@smpilot.edu.bd"]; hasDemoAdmin {
		if _, hasDemoTeacher := a.users["teacher@smpilot.edu.bd"]; !hasDemoTeacher {
			teacher := User{
				Email:          "teacher@smpilot.edu.bd",
				Name:           "Md. Rahim Uddin (Mathematics Teacher)",
				Role:           "teacher",
				TeacherSubject: "Mathematics",
				TeacherClasses: []string{"9", "10"},
				MustChange:     false,
			}
			setPassword(&teacher, "TeacherPass123")
			_ = a.put(teacher)
		}
	}
	// An older demo teacher saved before classes existed gets Classes 9 and 10.
	if t, ok := a.users["teacher@smpilot.edu.bd"]; ok && t.Role == "teacher" && len(t.TeacherClasses) == 0 {
		if _, hasDemoAdmin := a.users["admin@smpilot.edu.bd"]; hasDemoAdmin {
			t.TeacherClasses = []string{"9", "10"}
			_ = a.put(t)
		}
	}
	// Backfill demo student's personal profile details if not yet set.
	if st, ok := a.users["student@smpilot.edu.bd"]; ok && st.Role == "student" && st.NameBangla == "" {
		st.NameBangla = "রহিম উদ্দিন"
		st.Section = "B"
		st.RollNo = "17"
		st.Session = "2026"
		st.Religion = "Islam"
		st.Nationality = "Bangladeshi"
		st.BirthRegistration = "20111234567894821"
		st.EducationBoard = "Cumilla"
		st.AdmissionDate = "02 January 2021"
		st.Gender = "Male"
		st.DateOfBirth = "2011-04-15"
		st.BloodGroup = "O+"
		st.FatherName = "Abdul Karim Uddin"
		st.FatherOccupation = "Businessman"
		st.MotherName = "Amina Begum"
		st.MotherOccupation = "Homemaker"
		st.GuardianName = "Abdul Karim Uddin (Father)"
		st.ParentPhone = "01711-223344"
		st.EmergencyPhone = "01819-556677"
		st.Address = "House 12, College Road, Bancharampur, Brahmanbaria"
		st.PermanentAddress = "Village: Bancharampur, Post: Bancharampur (3420), Upazila: Bancharampur, District: Brahmanbaria"
		_ = a.put(st)
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		if port := os.Getenv("PORT"); port != "" {
			addr = "0.0.0.0:" + port
		} else {
			addr = "0.0.0.0:8080"
		}
	}
	server := &http.Server{Addr: addr, Handler: a, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("School website running at http://%s (local: http://localhost:8080)", addr)
	log.Fatal(server.ListenAndServe())
}
