// handlers_admin.go - handles /admin role checks, student records, exam results,
// report card PDFs, and teacher classroom attendance roll.
// Office content management actions (teachers, notices, gallery, admissions,
// achievements, stats, routine/calendar PDFs, school admins) live in handlers_office.go.
package main

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// serveAdmin handles GET and POST requests to /admin.
func (a *App) serveAdmin(w http.ResponseWriter, r *http.Request, id string, u User, v *View, now time.Time) {
	// The staff/admin console. Only teachers, school admins and the super admin may enter (students get 403).
	if !isStaffRole(u.Role) {
		w.WriteHeader(http.StatusForbidden)
		v.Mode = "forbidden"
		v.Title = "School administrator access required"
		v.Error = "School administrator access required — your browser is currently signed in with a Student account (" + u.Email + ")."
		a.render(w, *v)
		return
	}
	v.Mode = "admin"
	v.Title = "School office"
	v.IsSuperAdmin = u.Role == "superadmin"
	v.IsTeacher = u.Role == "teacher"
	if v.IsTeacher {
		v.Title = "Teacher portal"
		_, v.TeacherSubjectLabel = teacherSubjectsFor(u, a.teachers, "9")
	}
	// Work out which section is open:
	// - Teachers only have "attendance" and the read-only "progress" (Student Progress).
	// - School Admins have "students", "student", "results", "admissions", "notices", "teachers", "achievements", "gallery" (plus overview).
	// - Super Admin additionally has "admins".
	tab := r.URL.Query().Get("tab")
	if v.IsTeacher {
		if tab != "attendance" && tab != "progress" {
			tab = "attendance"
		}
	} else {
		switch tab {
		case "students", "student", "notices", "admissions", "results":
		case "teachers", "gallery", "achievements", "about":
		case "admins":
			if !v.IsSuperAdmin {
				tab = ""
			}
		default:
			tab = ""
		}
	}
	v.Tab = tab
	profileEmail := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
	v.BloodGroups = bloodGroups
	if r.Method == "GET" {
		if flash, ok := a.flashes[id]; ok {
			v.Success = flash.Success
			v.Error = flash.Error
			v.TempPassword = flash.TempPass
			v.TempUserEmail = flash.TempEmail
			v.TempAccountType = flash.AccountType
			delete(a.flashes, id)
		}
	}
	if r.Method == "POST" {
		var tempPass, tempEmail, tempAccountType string
		email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		student, exists := a.users[email]
		action := r.FormValue("action")
		// Role boundary check: Teachers may ONLY take attendance. They can never enter or
		// publish marks - only school admins publish results (403 for every other action).
		if v.IsTeacher && action != "class-attendance-save" {
			if strings.Contains(action, "marks") || strings.Contains(action, "result") {
				http.Error(w, "Teachers cannot enter or publish marks. Only the school admin can publish results.", 403)
			} else {
				http.Error(w, "School administrator access required", 403)
			}
			return
		}
		// Conversely, taking class attendance is a teacher duty.
		if !v.IsTeacher && action == "class-attendance-save" {
			http.Error(w, "Teacher access required to take attendance", 403)
			return
		}
		// Only the Super Admin may create, reset or delete School Admin accounts.
		// Everything else in the office (teachers, students, gallery, achievements,
		// home page statistics, notices, results...) is open to School Admins too.
		switch action {
		case "admin-create", "admin-reset", "admin-delete":
			if !v.IsSuperAdmin {
				http.Error(w, "Super administrator access required", 403)
				return
			}
		}
		if action == "group" {
			group := r.FormValue("group")
			if !exists || student.Role != "student" || (student.Class != "9" && student.Class != "10") || !validGroup(group) {
				v.Error = "Select a valid group for a Class 9–10 student."
			} else {
				student.Group = group
				if err := a.put(student); err != nil {
					v.Error = "Unable to save group."
				} else {
					v.Success = "Student group updated."
				}
			}
		} else if action == "reset" {
			if !exists || student.Role != "student" {
				v.Error = "Student account not found."
			}
		} else if action == "create" {
			class := r.FormValue("class")
			name := strings.TrimSpace(r.FormValue("name"))
			studentID := strings.TrimSpace(r.FormValue("student_id"))
			group := ""
			if class == "9" || class == "10" {
				group = r.FormValue("group")
				if !validGroup(group) {
					v.Error = "Select Science, Humanities or Business Studies for Class 9–10."
				}
			}
			classOK := false
			for _, c := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"} {
				if class == c {
					classOK = true
				}
			}
			if exists || !validEmail(email) || name == "" || len(name) > 100 || studentID == "" || len(studentID) > 50 || !classOK {
				v.Error = "Enter a unique email, student ID, name and class (1–10)."
			}
			for _, other := range a.users {
				if other.StudentID == studentID && studentID != "" {
					v.Error = "This student ID already has an account."
				}
			}
			student = User{Email: email, Name: name, StudentID: studentID, Class: class, Group: group, Role: "student"}
		} else if action == "profile-update" {
			// Save a student's profile: check every field first, and save only if all are valid.
			name := strings.TrimSpace(r.FormValue("name"))
			gender := strings.TrimSpace(r.FormValue("gender"))
			dob := strings.TrimSpace(r.FormValue("date_of_birth"))
			blood := strings.TrimSpace(r.FormValue("blood_group"))
			address := strings.TrimSpace(r.FormValue("address"))
			father := strings.TrimSpace(r.FormValue("father_name"))
			mother := strings.TrimSpace(r.FormValue("mother_name"))
			phone := strings.TrimSpace(r.FormValue("parent_phone"))
			emergency := strings.TrimSpace(r.FormValue("emergency_phone"))
			dobOK := dob == ""
			if !dobOK {
				if d, err := time.Parse("2006-01-02", dob); err == nil && d.Before(now) && d.Year() > 1990 {
					dobOK = true
				}
			}
			switch {
			case !exists || student.Role != "student":
				v.Error = "Student account not found."
			case name == "" || len(name) > 100 || len(father) > 100 || len(mother) > 100 || len(address) > 300:
				v.Error = "Check the name, parent names and address lengths."
			case gender != "" && gender != "Male" && gender != "Female":
				v.Error = "Select a valid gender."
			case !validBloodGroup(blood):
				v.Error = "Select a valid blood group."
			case !dobOK:
				v.Error = "Enter a valid date of birth."
			case (phone != "" && !phonePattern.MatchString(phone)) || (emergency != "" && !phonePattern.MatchString(emergency)):
				v.Error = "Enter valid phone numbers (digits, +, -, spaces; 6–20 characters)."
			default:
				student.Name, student.Gender, student.DateOfBirth, student.BloodGroup = name, gender, dob, blood
				student.Address, student.FatherName, student.MotherName = address, father, mother
				student.ParentPhone, student.EmergencyPhone = phone, emergency
				if val := strings.TrimSpace(r.FormValue("name_bangla")); val != "" || r.Form.Has("name_bangla") {
					student.NameBangla = val
				}
				if val := strings.TrimSpace(r.FormValue("section")); val != "" || r.Form.Has("section") {
					student.Section = val
				}
				if val := strings.TrimSpace(r.FormValue("roll_no")); val != "" || r.Form.Has("roll_no") {
					student.RollNo = val
				}
				if val := strings.TrimSpace(r.FormValue("session")); val != "" || r.Form.Has("session") {
					student.Session = val
				}
				if val := strings.TrimSpace(r.FormValue("religion")); val != "" || r.Form.Has("religion") {
					student.Religion = val
				}
				if val := strings.TrimSpace(r.FormValue("nationality")); val != "" || r.Form.Has("nationality") {
					student.Nationality = val
				}
				if val := strings.TrimSpace(r.FormValue("birth_registration")); val != "" || r.Form.Has("birth_registration") {
					student.BirthRegistration = val
				}
				if val := strings.TrimSpace(r.FormValue("education_board")); val != "" || r.Form.Has("education_board") {
					student.EducationBoard = val
				}
				if val := strings.TrimSpace(r.FormValue("admission_date")); val != "" || r.Form.Has("admission_date") {
					student.AdmissionDate = val
				}
				if val := strings.TrimSpace(r.FormValue("father_occupation")); val != "" || r.Form.Has("father_occupation") {
					student.FatherOccupation = val
				}
				if val := strings.TrimSpace(r.FormValue("mother_occupation")); val != "" || r.Form.Has("mother_occupation") {
					student.MotherOccupation = val
				}
				if val := strings.TrimSpace(r.FormValue("guardian_name")); val != "" || r.Form.Has("guardian_name") {
					student.GuardianName = val
				}
				if val := strings.TrimSpace(r.FormValue("permanent_address")); val != "" || r.Form.Has("permanent_address") {
					student.PermanentAddress = val
				}
				if val := strings.TrimSpace(r.FormValue("tuition_status")); val == "Paid" || val == "Unpaid" {
					student.TuitionStatus = val
				}
				if err := a.put(student); err != nil {
					v.Error = "Unable to save the profile."
				} else {
					v.Success = "Profile saved for " + name + "."
				}
			}
		} else if action == "correction-status" {
			// Admin marks a student's profile correction request as Approved (corrected) or Rejected.
			reqID := strings.TrimSpace(r.FormValue("correction_id"))
			status := strings.TrimSpace(r.FormValue("status"))
			if !exists || student.Role != "student" {
				v.Error = "Student account not found."
			} else if status != "Approved" && status != "Rejected" {
				v.Error = "Invalid correction status."
			} else {
				found := false
				for i := range student.CorrectionRequests {
					if student.CorrectionRequests[i].ID == reqID {
						student.CorrectionRequests[i].Status = status
						found = true
						break
					}
				}
				if !found {
					v.Error = "Correction request not found."
				} else if err := a.put(student); err != nil {
					v.Error = "Unable to update correction request."
				} else {
					v.Success = "Correction request " + reqID + " marked as " + status + "."
				}
			}
		} else if action == "result-save" {
			// Record or update a student's exam marks (0-100) and compute their letter grade.
			exam := strings.TrimSpace(r.FormValue("exam"))
			subj := strings.TrimSpace(r.FormValue("subject"))
			marks, mErr := strconv.Atoi(strings.TrimSpace(r.FormValue("marks")))
			examOK := exam == "Half-Yearly Examination" || exam == "Annual Examination" || exam == "Class Test"
			subjOK := false
			for _, s := range subjectsFor(student) {
				if s.Name == subj {
					subjOK = true
				}
			}
			if !exists || student.Role != "student" {
				v.Error = "Student account not found."
			} else if !examOK || !subjOK || mErr != nil || marks < 0 || marks > 100 {
				v.Error = "Select a valid examination, subject and marks between 0 and 100."
			} else {
				rec := ResultRecord{Exam: exam, Subject: subj, Marks: marks, Grade: gradeForMarks(marks)}
				replaced := false
				for i := range student.Results {
					if student.Results[i].Exam == exam && student.Results[i].Subject == subj {
						student.Results[i] = rec
						replaced = true
						break
					}
				}
				if !replaced {
					student.Results = append(student.Results, rec)
				}
				if err := a.put(student); err != nil {
					v.Error = "Unable to save exam result."
				} else {
					v.Success = "Result saved: " + subj + " (" + exam + ") — " + strconv.Itoa(marks) + " (" + rec.Grade + ")."
				}
			}
		} else if action == "result-delete" {
			// Remove one exam result entry from a student's record.
			exam := strings.TrimSpace(r.FormValue("exam"))
			subj := strings.TrimSpace(r.FormValue("subject"))
			if !exists || student.Role != "student" {
				v.Error = "Student account not found."
			} else {
				var kept []ResultRecord
				for _, r := range student.Results {
					if !(r.Exam == exam && r.Subject == subj) {
						kept = append(kept, r)
					}
				}
				student.Results = kept
				if err := a.put(student); err != nil {
					v.Error = "Unable to remove exam result."
				} else {
					v.Success = "Exam result removed."
				}
			}
		} else if action == "result-save-bulk" {
			// Save marks for all subjects of a student at once in one click.
			exam := strings.TrimSpace(r.FormValue("exam"))
			examOK := exam == "Half-Yearly Examination" || exam == "Annual Examination" || exam == "Class Test"
			if !exists || student.Role != "student" {
				v.Error = "Student account not found."
			} else if !examOK {
				v.Error = "Select a valid examination."
			} else {
				savedCount := 0
				for _, s := range subjectsFor(student) {
					raw := strings.TrimSpace(r.FormValue("marks_" + s.Name))
					if raw == "" {
						continue
					}
					marks, mErr := strconv.Atoi(raw)
					if mErr != nil || marks < 0 || marks > 100 {
						v.Error = "Marks for " + s.Name + " must be between 0 and 100."
						break
					}
					rec := ResultRecord{Exam: exam, Subject: s.Name, Marks: marks, Grade: gradeForMarks(marks)}
					replaced := false
					for i := range student.Results {
						if student.Results[i].Exam == exam && student.Results[i].Subject == s.Name {
							student.Results[i] = rec
							replaced = true
							break
						}
					}
					if !replaced {
						student.Results = append(student.Results, rec)
					}
					savedCount++
				}
				if v.Error == "" {
					if savedCount == 0 {
						v.Error = "Enter marks for at least one subject."
					} else if err := a.put(student); err != nil {
						v.Error = "Unable to save results."
					} else {
						v.Success = "Published marks for " + strconv.Itoa(savedCount) + " subject(s) (" + exam + ")."
					}
				}
			}
		} else if action == "student-report-pdf" {
			// Upload an individual report card PDF for a single student.
			exam := strings.TrimSpace(r.FormValue("report_exam"))
			if exam == "" {
				exam = strings.TrimSpace(r.FormValue("exam"))
			}
			if !exists || student.Role != "student" {
				v.Error = "Student account not found."
			} else if pdfPath, err := a.saveUploadedResultPDF(r, "report_pdf"); err != nil {
				v.Error = "Choose a valid PDF file (up to 5 MB)."
			} else {
				if exam == "" {
					exam = "Official Report Card"
				}
				student.ReportCardPDF = pdfPath
				student.ReportCardExam = exam
				if err := a.put(student); err != nil {
					v.Error = "Unable to save report card PDF."
				} else {
					v.Success = "Personal report card PDF uploaded for " + student.Name + "."
				}
			}
		} else if action == "result-pdf-add" {
			// Publish an official result sheet PDF for a whole class.
			rExam := strings.TrimSpace(r.FormValue("result_exam"))
			rClass := strings.TrimSpace(r.FormValue("result_class"))
			rTitle := strings.TrimSpace(r.FormValue("result_title"))
			if rTitle == "" {
				rTitle = rExam + " — Class " + rClass + " Result Sheet"
			}
			if rExam == "" || rClass == "" || len(rTitle) > 160 {
				v.Error = "Select an examination and class for the result PDF."
			} else if pdfPath, err := a.saveUploadedResultPDF(r, "result_pdf"); err != nil {
				v.Error = "Choose a valid PDF file (up to 5 MB)."
			} else {
				rec := ResultPDFRecord{
					ID:          "rpdf-" + token()[:8],
					Exam:        rExam,
					Class:       rClass,
					Title:       rTitle,
					FilePath:    pdfPath,
					PublishedAt: now.Format("2006-01-02"),
				}
				a.resultPDFs = append([]ResultPDFRecord{rec}, a.resultPDFs...)
				if err := a.saveResultPDFs(); err != nil {
					v.Error = "Unable to save result PDF."
				} else {
					v.Success = "Result PDF (" + rTitle + ") published for Class " + rClass + "."
				}
			}
		} else if action == "result-pdf-delete" {
			rID := strings.TrimSpace(r.FormValue("result_pdf_id"))
			idx := -1
			for i, rp := range a.resultPDFs {
				if rp.ID == rID {
					idx = i
					break
				}
			}
			if idx == -1 {
				v.Error = "Result PDF not found."
			} else {
				removed := a.resultPDFs[idx]
				a.resultPDFs = append(a.resultPDFs[:idx], a.resultPDFs[idx+1:]...)
				if err := a.saveResultPDFs(); err != nil {
					v.Error = "Unable to remove result PDF."
				} else {
					_ = os.Remove(filepath.Join(a.frontendDir, filepath.FromSlash(removed.FilePath)))
					v.Success = "Result PDF (" + removed.Title + ") removed."
				}
			}
		} else if action == "class-attendance-save" {
			// Teacher marks attendance for an entire class at once (Present or Absent) for their subject.
			rClass := strings.TrimSpace(r.FormValue("roll_class"))
			rSubj := strings.TrimSpace(r.FormValue("roll_subject"))
			rDate := strings.TrimSpace(r.FormValue("roll_date"))
			if rDate == "" {
				rDate = now.Format("2006-01-02")
			}
			_, dErr := time.Parse("2006-01-02", rDate)
			allowedSubjs, _ := teacherSubjectsFor(u, a.teachers, rClass)
			subjAllowed := false
			for _, s := range allowedSubjs {
				if s.Name == rSubj {
					subjAllowed = true
					break
				}
			}
			if rClass == "" || rSubj == "" || dErr != nil || !subjAllowed {
				v.Error = "Select a valid class, date and your assigned subject."
			} else {
				marked := 0
				for k, acc := range a.users {
					if acc.Role != "student" || acc.Class != rClass {
						continue
					}
					st := strings.TrimSpace(r.FormValue("status_" + acc.Email))
					if st != "present" && st != "absent" {
						continue
					}
					rec := AttendanceRecord{Date: rDate, Subject: rSubj, Status: st}
					replaced := false
					for i := range acc.Attendance {
						if acc.Attendance[i].Date == rDate && acc.Attendance[i].Subject == rSubj {
							acc.Attendance[i] = rec
							replaced = true
							break
						}
					}
					if !replaced {
						acc.Attendance = append([]AttendanceRecord{rec}, acc.Attendance...)
					}
					a.users[k] = acc
					marked++
				}
				if marked == 0 {
					v.Error = "No students found in Class " + rClass + " to mark."
				} else if err := a.save(); err != nil {
					v.Error = "Unable to save class attendance."
				} else {
					v.Success = "Attendance saved for " + strconv.Itoa(marked) + " student(s) in Class " + rClass + " (" + rSubj + ", " + rDate + ")."
				}
			}
		} else if action == "attendance-save" {
			// Record daily subject attendance for a student from the admin profile page.
			dateStr := strings.TrimSpace(r.FormValue("date"))
			if dateStr == "" {
				dateStr = now.Format("2006-01-02")
			}
			subj := strings.TrimSpace(r.FormValue("subject"))
			status := strings.TrimSpace(r.FormValue("status"))
			_, dErr := time.Parse("2006-01-02", dateStr)
			statusOK := status == "present" || status == "absent"
			subjOK := false
			for _, s := range subjectsFor(student) {
				if s.Name == subj {
					subjOK = true
				}
			}
			if !exists || student.Role != "student" {
				v.Error = "Student account not found."
			} else if dErr != nil || !subjOK || !statusOK {
				v.Error = "Select a valid date, subject and attendance status."
			} else {
				rec := AttendanceRecord{Date: dateStr, Subject: subj, Status: status}
				replaced := false
				for i := range student.Attendance {
					if student.Attendance[i].Date == dateStr && student.Attendance[i].Subject == subj {
						student.Attendance[i] = rec
						replaced = true
						break
					}
				}
				if !replaced {
					student.Attendance = append([]AttendanceRecord{rec}, student.Attendance...)
				}
				if err := a.put(student); err != nil {
					v.Error = "Unable to save attendance."
				} else {
					v.Success = "Attendance recorded for " + subj + " on " + dateStr + " (" + status + ")."
				}
			}
		} else if action == "student-delete" {
			// Remove a student account and end any active login sessions for that student.
			if !exists || student.Role != "student" {
				v.Error = "Student account not found."
			} else {
				delete(a.users, email)
				if err := a.save(); err != nil {
					a.users[email] = student
					v.Error = "Unable to remove student account."
				} else {
					for key, session := range a.sessions {
						if session.Email == email {
							delete(a.sessions, key)
						}
					}
					v.Success = "Student account (" + email + ") removed."
					tab = "students"
					profileEmail = ""
				}
			}
		} else if handled, sent := a.handleOfficeAction(w, r, u, v, action, now, &tempPass, &tempEmail, &tempAccountType); handled {
			if sent {
				return
			}
		} else {
			v.Error = "Unknown action."
		}
		if v.Error == "" && (action == "create" || action == "reset") {
			// New or reset student account: make a temporary password. The student must
			// change it at first login and all their old sessions are ended.
			temporary := token()[:20]
			setPassword(&student, temporary)
			student.MustChange = true
			if err := a.put(student); err != nil {
				v.Error = "Unable to save the account."
			} else {
				for key, session := range a.sessions {
					if session.Email == email {
						delete(a.sessions, key)
					}
				}
				tempPass = temporary
				tempEmail = email
				tempAccountType = "student"
				if action == "create" {
					v.Success = "Student account created: " + email
				} else {
					v.Success = "New temporary password issued for student: " + email
				}
			}
		}
		// Keep the message (and temporary password) for ONE page load, then redirect.
		// Post -> Redirect -> Get means a browser refresh cannot re-send the form
		// and the password is not shown again.
		a.flashes[id] = FlashData{
			Success:     v.Success,
			Error:       v.Error,
			TempPass:    tempPass,
			TempEmail:   tempEmail,
			AccountType: tempAccountType,
		}
		dest := "/admin"
		if tab != "" {
			q := url.Values{"tab": {tab}}
			if tab == "student" && profileEmail != "" {
				q.Set("email", profileEmail)
			}
			if tab == "attendance" {
				if rc := strings.TrimSpace(r.FormValue("roll_class")); rc != "" {
					q.Set("roll_class", rc)
				}
				if rs := strings.TrimSpace(r.FormValue("roll_subject")); rs != "" {
					q.Set("roll_subject", rs)
				}
				if rd := strings.TrimSpace(r.FormValue("roll_date")); rd != "" {
					q.Set("roll_date", rd)
				}
				if re := strings.TrimSpace(r.FormValue("roll_exam")); re != "" {
					q.Set("roll_exam", re)
				}
			}
			dest += "?" + q.Encode()
		}
		http.Redirect(w, r, dest, 303)
		return
	}
	// Collect students and admins for the page, sorted so the list is stable.
	for _, account := range a.users {
		if account.Role == "student" {
			v.Students = append(v.Students, account)
		} else if account.Role == "admin" {
			v.Admins = append(v.Admins, account)
		}
	}
	sort.Slice(v.Students, func(i, j int) bool {
		ci, _ := strconv.Atoi(v.Students[i].Class)
		cj, _ := strconv.Atoi(v.Students[j].Class)
		if ci != cj {
			return ci < cj
		}
		if v.Students[i].StudentID != v.Students[j].StudentID {
			return v.Students[i].StudentID < v.Students[j].StudentID
		}
		return v.Students[i].Email < v.Students[j].Email
	})
	sort.Slice(v.Admins, func(i, j int) bool { return v.Admins[i].Email < v.Admins[j].Email })
	v.Teachers = append([]Teacher(nil), a.teachers...)
	{
		v.TeacherClassesByEmail = map[string]string{}
		for _, acc := range a.users {
			if acc.Role == "teacher" && len(acc.TeacherClasses) > 0 {
				v.TeacherClassesByEmail[acc.Email] = strings.Join(acc.TeacherClasses, ", ")
			}
		}
	}
	v.Notices = append([]NoticeRecord(nil), a.notices...)
	v.Gallery = append([]GalleryItem(nil), a.gallery...)
	v.Seats = a.seats
	v.Admissions = append([]AdmissionApplication(nil), a.admissions...)
	v.Achievements = append([]AchievementRecord(nil), a.achievements...)
	v.Stats = a.stats
	v.ResultPDFs = append([]ResultPDFRecord(nil), a.resultPDFs...)
	if tab == "attendance" {
		rc := strings.TrimSpace(r.URL.Query().Get("roll_class"))
		if rc == "" {
			rc = strings.TrimSpace(r.URL.Query().Get("class"))
		}
		if rc == "" {
			rc = "9"
		}
		v.RollClass = rc
		v.RollDate = strings.TrimSpace(r.URL.Query().Get("roll_date"))
		if v.RollDate == "" {
			v.RollDate = strings.TrimSpace(r.URL.Query().Get("date"))
		}
		if v.RollDate == "" {
			v.RollDate = now.Format("2006-01-02")
		}
		v.RollExam = strings.TrimSpace(r.URL.Query().Get("roll_exam"))
		if v.RollExam == "" {
			v.RollExam = "Half-Yearly Examination"
		}
		v.RollSubjects, v.TeacherSubjectLabel = teacherSubjectsFor(u, a.teachers, rc)
		v.RollSubject = strings.TrimSpace(r.URL.Query().Get("roll_subject"))
		if v.RollSubject == "" {
			v.RollSubject = strings.TrimSpace(r.URL.Query().Get("subject"))
		}
		validRollSubj := false
		for _, s := range v.RollSubjects {
			if s.Name == v.RollSubject {
				validRollSubj = true
				break
			}
		}
		if !validRollSubj && len(v.RollSubjects) > 0 {
			v.RollSubject = v.RollSubjects[0].Name
		}
		for _, st := range v.Students {
			if st.Class == rc {
				for _, att := range st.Attendance {
					if att.Date == v.RollDate && att.Subject == v.RollSubject {
						st.CurrentStatus = att.Status
						break
					}
				}
				for _, res := range st.Results {
					if res.Exam == v.RollExam && res.Subject == v.RollSubject {
						st.CurrentMarks = strconv.Itoa(res.Marks)
						st.CurrentGrade = res.Grade
						break
					}
				}
				v.RollStudents = append(v.RollStudents, st)
			}
		}
	}
	if tab == "progress" && v.IsTeacher {
		// Read-only Student Progress: only students of classes assigned to this teacher.
		v.ProgressClasses, v.ProgressStudents = progressRoster(u, v.Students)
	}
	if tab == "student" {
		if p, ok := a.users[profileEmail]; ok && p.Role == "student" {
			v.Profile = p
			v.ProfileSubjects = subjectsFor(p)
			if r := []rune(strings.TrimSpace(p.Name)); len(r) > 0 {
				v.ProfileInitial = strings.ToUpper(string(r[0]))
			}
		} else if v.Error == "" {
			v.Error = "Student not found."
		}
	}
	a.render(w, *v)
}
