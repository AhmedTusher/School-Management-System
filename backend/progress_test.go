package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func progressTestApp(t *testing.T) (*App, *http.Cookie, *http.Cookie, *http.Cookie, *http.Cookie) {
	t.Helper()
	a, err := newApp(filepath.Join(t.TempDir(), "accounts.json"))
	if err != nil {
		t.Fatal(err)
	}
	a.users["t9@example.com"] = User{Email: "t9@example.com", Name: "Class Nine Teacher", Role: "teacher", TeacherSubject: "Mathematics", TeacherClasses: []string{"9"}}
	a.users["admin@example.com"] = User{Email: "admin@example.com", Name: "Admin", Role: "admin"}
	a.users["rahim@example.com"] = User{Email: "rahim@example.com", Name: "Rahim Nine", StudentID: "901", Class: "9", Group: "Science", Role: "student",
		Attendance: []AttendanceRecord{{"2026-01-01", "Physics", "present"}, {"2026-01-02", "Physics", "present"}, {"2026-01-03", "Physics", "absent"}, {"2026-01-04", "Physics", "present"}},
		Results: []ResultRecord{
			{"Annual Examination", "General Mathematics", 80, "A+"}, {"Class Test", "General Mathematics", 60, "A-"}, {"Half-Yearly Examination", "General Mathematics", 70, "A"},
			{"Class Test", "Physics", 50, "B"}, {"Half-Yearly Examination", "Physics", 55, "B"}, {"Annual Examination", "Physics", 58, "B"},
		}}
	a.users["karim@example.com"] = User{Email: "karim@example.com", Name: "Karim Nine", StudentID: "902", Class: "9", Group: "Science", Role: "student",
		Results: []ResultRecord{{"Class Test", "General Mathematics", 40, "C"}, {"Half-Yearly Examination", "General Mathematics", 50, "B"}, {"Annual Examination", "General Mathematics", 60, "A-"}}}
	a.users["tenth@example.com"] = User{Email: "tenth@example.com", Name: "Secret Tenth", StudentID: "1001", Class: "10", Group: "Science", Role: "student",
		Results: []ResultRecord{{"Class Test", "General Mathematics", 99, "A+"}}}
	exp := time.Now().Add(time.Hour)
	a.sessions["t"] = Session{"t9@example.com", "tcsrf", exp}
	a.sessions["a"] = Session{"admin@example.com", "acsrf", exp}
	a.sessions["s"] = Session{"rahim@example.com", "scsrf", exp}
	ck := func(v string) *http.Cookie { return &http.Cookie{Name: "school_session", Value: v} }
	return a, ck("t"), ck("a"), ck("s"), nil
}

func TestTeacherCannotPublishMarks(t *testing.T) {
	a, tc, _, _, _ := progressTestApp(t)
	before := len(a.users["rahim@example.com"].Results)
	for _, form := range []url.Values{
		{"csrf": {"tcsrf"}, "action": {"teacher-marks-save"}, "roll_class": {"9"}, "roll_subject": {"General Mathematics"}, "roll_exam": {"Class Test"}, "marks_rahim@example.com": {"99"}},
		{"csrf": {"tcsrf"}, "action": {"result-save"}, "email": {"rahim@example.com"}, "exam": {"Class Test"}, "subject": {"Physics"}, "marks": {"99"}},
		{"csrf": {"tcsrf"}, "action": {"result-save-bulk"}, "email": {"rahim@example.com"}, "exam": {"Class Test"}, "marks_Physics": {"99"}},
		{"csrf": {"tcsrf"}, "action": {"result-delete"}, "email": {"rahim@example.com"}, "exam": {"Class Test"}, "subject": {"Physics"}},
	} {
		if w := request(a, "POST", "/admin?tab=progress", form, tc); w.Code != 403 {
			t.Fatalf("teacher action %q expected 403, got %d", form.Get("action"), w.Code)
		}
	}
	rahim := a.users["rahim@example.com"]
	if len(rahim.Results) != before {
		t.Fatal("teacher changed marks")
	}
	for _, r := range rahim.Results {
		if r.Marks == 99 {
			t.Fatal("teacher marks were saved")
		}
	}
}

func TestTeacherTabsHaveNoMarksTab(t *testing.T) {
	a, tc, _, _, _ := progressTestApp(t)
	page := request(a, "GET", "/admin?tab=marks", nil, tc)
	body := page.Body.String()
	if strings.Contains(body, "Publish Subject Marks") || strings.Contains(body, "teacher-marks-save") {
		t.Fatal("marks UI still present for teacher")
	}
	if !strings.Contains(body, "Student Progress") || !strings.Contains(body, "Mark Class Attendance") {
		t.Fatal("teacher should land on attendance with a Student Progress tab")
	}
	body = request(a, "GET", "/admin?tab=progress", nil, tc).Body.String()
	if !strings.Contains(body, "Rahim Nine") || !strings.Contains(body, "Karim Nine") {
		t.Fatal("assigned class students missing from progress page")
	}
	if strings.Contains(body, "Secret Tenth") {
		t.Fatal("student of an unassigned class leaked into the progress page")
	}
}

func TestTeacherProgressAccessControl(t *testing.T) {
	a, tc, ac, sc, _ := progressTestApp(t)
	get := func(path string, c *http.Cookie) int { return request(a, "GET", path, nil, c).Code }

	if got := get("/api/teacher/students/901/progress", nil); got != 401 {
		t.Fatalf("anonymous expected 401, got %d", got)
	}
	if got := get("/api/teacher/students/901/progress", sc); got != 403 {
		t.Fatalf("student expected 403, got %d", got)
	}
	if got := get("/api/teacher/students/902/progress", sc); got != 403 {
		t.Fatalf("student reading a classmate expected 403, got %d", got)
	}
	if got := get("/api/teacher/students/901/progress", ac); got != 403 {
		t.Fatalf("non-teacher staff expected 403, got %d", got)
	}
	if got := get("/api/teacher/students/901/progress", tc); got != 200 {
		t.Fatalf("assigned teacher expected 200, got %d", got)
	}
	if w := request(a, "GET", "/api/teacher/students/1001/progress", nil, tc); w.Code != 403 || strings.Contains(w.Body.String(), "Secret") {
		t.Fatalf("teacher outside class expected 403 without data, got %d", w.Code)
	}
	if got := get("/api/teacher/students/nope/progress", tc); got != 404 {
		t.Fatalf("unknown student expected 404, got %d", got)
	}
	if got := get("/api/teacher/students/901/progress?subject=Nonsense", tc); got != 400 {
		t.Fatalf("bad subject expected 400, got %d", got)
	}
	if w := request(a, "POST", "/api/teacher/students/901/progress", url.Values{}, tc); w.Code != 405 {
		t.Fatalf("POST expected 405, got %d", w.Code)
	}
	// A teacher with no assigned classes sees nobody.
	t9 := a.users["t9@example.com"]
	t9.TeacherClasses = nil
	a.users["t9@example.com"] = t9
	if got := get("/api/teacher/students/901/progress", tc); got != 403 {
		t.Fatalf("teacher without classes expected 403, got %d", got)
	}
}

func TestTeacherProgressResponse(t *testing.T) {
	a, tc, _, _, _ := progressTestApp(t)
	var all ProgressResponse
	w := request(a, "GET", "/api/teacher/students/901/progress", nil, tc)
	if err := json.Unmarshal(w.Body.Bytes(), &all); err != nil {
		t.Fatal(err)
	}
	if strings.Join(all.Exams, "|") != "Class Test|Half-Yearly Examination|Annual Examination" {
		t.Fatalf("exams not chronological: %v", all.Exams)
	}
	if len(all.Series) != 2 || all.Overall == nil {
		t.Fatalf("expected 2 subjects and an overall line, got %d / %v", len(all.Series), all.Overall)
	}
	// Overall = (60+50)/2=55, (70+55)/2=62.5, (80+58)/2=69
	o := all.Overall.Points
	if *o[0].Marks != 55 || *o[1].Marks != 62.5 || *o[2].Marks != 69 {
		t.Fatalf("overall wrong: %v %v %v", *o[0].Marks, *o[1].Marks, *o[2].Marks)
	}
	if o[0].Change != nil || *o[1].Change != 7.5 || *o[2].Change != 6.5 {
		t.Fatal("overall change wrong")
	}
	if all.Summary.Trend != "Improving" || all.Summary.BestSubject != "General Mathematics" || all.Summary.WeakestSubject != "Physics" {
		t.Fatalf("summary wrong: %+v", all.Summary)
	}
	if all.Summary.Attendance == nil || *all.Summary.Attendance != 75 {
		t.Fatalf("attendance wrong: %v", all.Summary.Attendance)
	}

	var one ProgressResponse
	w = request(a, "GET", "/api/teacher/students/901/progress?subject=General+Mathematics", nil, tc)
	if err := json.Unmarshal(w.Body.Bytes(), &one); err != nil {
		t.Fatal(err)
	}
	if len(one.Series) != 1 || one.Overall != nil {
		t.Fatal("single subject should return one line")
	}
	p := one.Series[0].Points
	if *p[0].Marks != 60 || p[0].Grade != "A-" || *p[1].Change != 10 || p[2].Grade != "A+" {
		t.Fatalf("subject points wrong: %+v", p)
	}
	// Class average of Class 9 Mathematics: (60+40)/2=50, (70+50)/2=60, (80+60)/2=70. Class 10 must not count.
	if *one.ClassAverage[0].Average != 50 || *one.ClassAverage[1].Average != 60 || *one.ClassAverage[2].Average != 70 {
		t.Fatalf("class average wrong: %+v", one.ClassAverage)
	}
	// A student with no results gets an empty (not failing) response.
	a.users["karim@example.com"] = User{Email: "karim@example.com", Name: "Karim", StudentID: "902", Class: "9", Role: "student"}
	var empty ProgressResponse
	w = request(a, "GET", "/api/teacher/students/902/progress", nil, tc)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &empty) != nil || len(empty.Exams) != 0 || len(empty.Series) != 0 {
		t.Fatalf("empty state wrong: %d %s", w.Code, w.Body.String())
	}
}

func TestTrendFor(t *testing.T) {
	cases := []struct {
		name string
		in   []float64
		want string
	}{
		{"no data", nil, "Not enough data"},
		{"one exam", []float64{70}, "Not enough data"},
		{"improving", []float64{50, 60, 70}, "Improving"},
		{"declining", []float64{80, 70, 60}, "Declining"},
		{"steady small move", []float64{70, 71, 71.5}, "Steady"},
		{"exactly +2", []float64{70, 72}, "Improving"},
		{"exactly -2", []float64{72, 70}, "Declining"},
		{"only last 3 count", []float64{10, 90, 80, 70}, "Declining"},
		{"recovery counts", []float64{90, 40, 50, 60}, "Improving"},
	}
	for _, c := range cases {
		if got := trendFor(c.in); got != c.want {
			t.Errorf("%s: trendFor(%v) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

