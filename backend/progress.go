// progress.go - read-only "Student Progress" for teachers.
// It turns the results already published by the school admin into a
// per-exam timeline (marks, grade, change, class average, trend).
// Nothing in this file creates or changes marks.
package main

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// ProgressStudent is the minimum a teacher needs to pick a student.
type ProgressStudent struct {
	StudentID string `json:"studentId"`
	Name      string `json:"name"`
	Class     string `json:"class"`
}

// ProgressPoint is one exam on one line of the chart. Marks is nil when the
// student has no published result for that exam (the chart shows a gap).
type ProgressPoint struct {
	Exam   string   `json:"exam"`
	Marks  *float64 `json:"marks"`
	Grade  string   `json:"grade"`
	Change *float64 `json:"change"` // difference from the previous exam that has a mark
}

// ProgressSeries is one line: a subject, or the overall average.
type ProgressSeries struct {
	Name   string          `json:"name"`
	Points []ProgressPoint `json:"points"`
}

// ClassAveragePoint is the class average for one exam.
type ClassAveragePoint struct {
	Exam    string   `json:"exam"`
	Average *float64 `json:"average"`
}

// ProgressSummary feeds the cards above the chart. It always describes the
// whole picture (all subjects), even if one subject is selected for the chart.
type ProgressSummary struct {
	LatestOverall  *float64 `json:"latestOverall"`
	LatestExam     string   `json:"latestExam"`
	BestSubject    string   `json:"bestSubject"`
	WeakestSubject string   `json:"weakestSubject"`
	Trend          string   `json:"trend"`
	Attendance     *float64 `json:"attendance"`
}

// ProgressResponse is the JSON returned by the progress endpoint.
type ProgressResponse struct {
	Student      ProgressStudent     `json:"student"`
	Subject      string              `json:"subject"` // "" means All Subjects
	Exams        []string            `json:"exams"`   // chronological
	Series       []ProgressSeries    `json:"series"`
	Overall      *ProgressSeries     `json:"overall,omitempty"` // only for All Subjects
	ClassAverage []ClassAveragePoint `json:"classAverage"`
	Summary      ProgressSummary     `json:"summary"`
}

// The school runs three kinds of exam; this is their order in the school year.
var examOrder = []string{"Class Test", "Half-Yearly Examination", "Annual Examination"}

func examRank(name string) int {
	for i, e := range examOrder {
		if e == name {
			return i
		}
	}
	return len(examOrder)
}

func sortExams(exams []string) {
	sort.SliceStable(exams, func(i, j int) bool {
		ri, rj := examRank(exams[i]), examRank(exams[j])
		if ri != rj {
			return ri < rj
		}
		return exams[i] < exams[j]
	})
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

func fptr(f float64) *float64 { v := round1(f); return &v }

// trendFor looks at the last three overall percentages (oldest first).
// A rise or fall of 2 points or more counts as a real change.
func trendFor(values []float64) string {
	if len(values) < 2 {
		return "Not enough data"
	}
	if len(values) > 3 {
		values = values[len(values)-3:]
	}
	delta := values[len(values)-1] - values[0]
	switch {
	case delta >= 2:
		return "Improving"
	case delta <= -2:
		return "Declining"
	default:
		return "Steady"
	}
}

func attendancePercent(u User) *float64 {
	if len(u.Attendance) == 0 {
		return nil
	}
	present := 0
	for _, r := range u.Attendance {
		if r.Status == "present" {
			present++
		}
	}
	return fptr(float64(present) * 100 / float64(len(u.Attendance)))
}

// teacherHasClass says whether a teacher is assigned to a class.
func teacherHasClass(u User, class string) bool {
	if u.Role != "teacher" || class == "" {
		return false
	}
	for _, c := range u.TeacherClasses {
		if strings.TrimSpace(c) == class {
			return true
		}
	}
	return false
}

// progressRoster returns the teacher's classes and ONLY the students in them.
func progressRoster(teacher User, students []User) ([]string, []ProgressStudent) {
	var classes []string
	seen := map[string]bool{}
	for _, c := range teacher.TeacherClasses {
		c = strings.TrimSpace(c)
		if n, err := strconv.Atoi(c); err == nil && n >= 1 && n <= 10 && !seen[c] {
			seen[c] = true
			classes = append(classes, c)
		}
	}
	sort.Slice(classes, func(i, j int) bool {
		a, _ := strconv.Atoi(classes[i])
		b, _ := strconv.Atoi(classes[j])
		return a < b
	})
	var out []ProgressStudent
	for _, s := range students {
		if s.Role == "student" && seen[s.Class] {
			out = append(out, ProgressStudent{StudentID: s.StudentID, Name: s.Name, Class: s.Class})
		}
	}
	return classes, out
}

// parseTeacherClasses turns "9, 10" into ["9" "10"], keeping only classes 1-10.
func parseTeacherClasses(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		if n, err := strconv.Atoi(part); err == nil && n >= 1 && n <= 10 {
			c := strconv.Itoa(n)
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	return out
}

// examMeans returns exam -> average marks over all subjects the user has results for.
func examMeans(u User) map[string]float64 {
	sum := map[string]float64{}
	n := map[string]int{}
	for _, r := range u.Results {
		sum[r.Exam] += float64(r.Marks)
		n[r.Exam]++
	}
	out := map[string]float64{}
	for e, s := range sum {
		out[e] = s / float64(n[e])
	}
	return out
}

func buildPoints(exams []string, marksByExam map[string]float64) []ProgressPoint {
	points := make([]ProgressPoint, 0, len(exams))
	var prev *float64
	for _, e := range exams {
		p := ProgressPoint{Exam: e}
		if m, ok := marksByExam[e]; ok {
			p.Marks = fptr(m)
			p.Grade = gradeForMarks(int(math.Round(m)))
			if prev != nil {
				p.Change = fptr(*p.Marks - *prev)
			}
			prev = p.Marks
		}
		points = append(points, p)
	}
	return points
}

// computeProgress builds the whole response. classmates must be the students of
// the same class (it may include the student). subject "" means all subjects.
func computeProgress(student User, classmates []User, subject string) ProgressResponse {
	resp := ProgressResponse{
		Student: ProgressStudent{StudentID: student.StudentID, Name: student.Name, Class: student.Class},
		Subject: subject,
		Series:  []ProgressSeries{},
	}

	// Subjects in a sensible order: the class syllabus first, then anything else.
	var subjects []string
	has := map[string]bool{}
	for _, r := range student.Results {
		has[r.Subject] = true
	}
	for _, s := range subjectsFor(student) {
		if has[s.Name] {
			subjects = append(subjects, s.Name)
			delete(has, s.Name)
		}
	}
	var extra []string
	for s := range has {
		extra = append(extra, s)
	}
	sort.Strings(extra)
	subjects = append(subjects, extra...)

	// Marks per subject per exam for the student.
	bySubject := map[string]map[string]float64{}
	examSet := map[string]bool{}
	for _, r := range student.Results {
		if bySubject[r.Subject] == nil {
			bySubject[r.Subject] = map[string]float64{}
		}
		bySubject[r.Subject][r.Exam] = float64(r.Marks)
		if subject == "" || subject == r.Subject {
			examSet[r.Exam] = true
		}
	}
	allExamSet := map[string]bool{}
	for _, r := range student.Results {
		allExamSet[r.Exam] = true
	}
	for e := range examSet {
		resp.Exams = append(resp.Exams, e)
	}
	sortExams(resp.Exams)
	if resp.Exams == nil {
		resp.Exams = []string{}
	}
	var allExams []string
	for e := range allExamSet {
		allExams = append(allExams, e)
	}
	sortExams(allExams)

	// Lines for the chart.
	for _, s := range subjects {
		if subject == "" || subject == s {
			resp.Series = append(resp.Series, ProgressSeries{Name: s, Points: buildPoints(resp.Exams, bySubject[s])})
		}
	}
	myMeans := examMeans(student)
	if subject == "" {
		resp.Overall = &ProgressSeries{Name: "Overall average", Points: buildPoints(resp.Exams, myMeans)}
	}

	// Class average per exam (same subject, or overall for All Subjects).
	resp.ClassAverage = make([]ClassAveragePoint, 0, len(resp.Exams))
	for _, e := range resp.Exams {
		var sum float64
		count := 0
		for _, c := range classmates {
			if c.Role != "student" || c.Class != student.Class {
				continue
			}
			if subject == "" {
				if m, ok := examMeans(c)[e]; ok {
					sum += m
					count++
				}
				continue
			}
			for _, r := range c.Results {
				if r.Exam == e && r.Subject == subject {
					sum += float64(r.Marks)
					count++
					break
				}
			}
		}
		pt := ClassAveragePoint{Exam: e}
		if count > 0 {
			pt.Average = fptr(sum / float64(count))
		}
		resp.ClassAverage = append(resp.ClassAverage, pt)
	}

	// Summary cards always use every subject.
	var overallValues []float64
	for _, e := range allExams {
		if m, ok := myMeans[e]; ok {
			overallValues = append(overallValues, m)
			resp.Summary.LatestOverall = fptr(m)
			resp.Summary.LatestExam = e
		}
	}
	resp.Summary.Trend = trendFor(overallValues)
	resp.Summary.Attendance = attendancePercent(student)
	bestAvg, worstAvg := -1.0, 101.0
	for _, s := range subjects {
		var sum float64
		for _, m := range bySubject[s] {
			sum += m
		}
		avg := sum / float64(len(bySubject[s]))
		if avg > bestAvg {
			bestAvg, resp.Summary.BestSubject = avg, s
		}
		if avg < worstAvg {
			worstAvg, resp.Summary.WeakestSubject = avg, s
		}
	}
	if len(subjects) < 2 {
		resp.Summary.WeakestSubject = ""
	}
	return resp
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// serveTeacherProgress answers GET /api/teacher/students/{studentId}/progress?subject=
// It is read-only and only for a teacher assigned to the student's class.
func (a *App) serveTeacherProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/teacher/students/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "progress" {
		writeJSONError(w, http.StatusNotFound, "Not found")
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _, u, logged := a.session(r)
	if !logged {
		writeJSONError(w, http.StatusUnauthorized, "Please log in.")
		return
	}
	if u.Role != "teacher" || u.MustChange {
		writeJSONError(w, http.StatusForbidden, "Teacher access required.")
		return
	}
	var student User
	found := false
	for _, acc := range a.users {
		if acc.Role == "student" && acc.StudentID == parts[0] {
			student, found = acc, true
			break
		}
	}
	if !found {
		writeJSONError(w, http.StatusNotFound, "Student not found.")
		return
	}
	if !teacherHasClass(u, student.Class) {
		writeJSONError(w, http.StatusForbidden, "You are not assigned to this student's class.")
		return
	}
	subject := strings.TrimSpace(r.URL.Query().Get("subject"))
	if subject != "" {
		valid := false
		for _, s := range subjectsFor(student) {
			valid = valid || s.Name == subject
		}
		for _, res := range student.Results {
			valid = valid || res.Subject == subject
		}
		if !valid {
			writeJSONError(w, http.StatusBadRequest, "Unknown subject.")
			return
		}
	}
	var classmates []User
	for _, acc := range a.users {
		if acc.Role == "student" && acc.Class == student.Class {
			classmates = append(classmates, acc)
		}
	}
	writeJSON(w, http.StatusOK, computeProgress(student, classmates, subject))
}

