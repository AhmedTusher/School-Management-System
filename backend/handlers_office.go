/*
 * handlers_office.go
 * Office content management actions for /admin
 *
 * Covers teachers, public notices, gallery uploads, class seat counts,
 * admissions review, achievements, home stats, About page quotes,
 * routine/calendar PDFs, and super-admin account controls
 */
package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

/*
 * handleOfficeAction routes admin/super-admin POST actions
 * returns (handled, httpSent)
 */
func (a *App) handleOfficeAction(w http.ResponseWriter, r *http.Request, u User, v *View, action string, now time.Time, tempPass, tempEmail, tempAccountType *string) (bool, bool) {

	// ----------------------------------------------------------------
	// Teachers directory & login accounts
	// ----------------------------------------------------------------
	if action == "teacher-add" {

		// read submitted teacher info
		tName := strings.TrimSpace(r.FormValue("teacher_name"))
		tRole := strings.TrimSpace(r.FormValue("teacher_role"))
		tSubject := strings.TrimSpace(r.FormValue("teacher_subject"))
		tQual := strings.TrimSpace(r.FormValue("teacher_qualification"))
		tEmail := strings.ToLower(strings.TrimSpace(r.FormValue("teacher_email")))
		tImage := strings.TrimSpace(r.FormValue("teacher_image"))

		if tImage == "" {
			tImage = "images/teachers/sample-teacher-3.png"
		}

		if tName == "" || len(tName) > 100 || tRole == "" || len(tRole) > 60 || tQual == "" || len(tQual) > 100 || !validEmail(tEmail) || !validTeacherImage(tImage) {
			v.Error = "Enter a valid teacher name, role, qualification, email and portrait."
		} else {
			head := isHeadRole(tRole)

			if !head && (tSubject == "" || len(tSubject) > 80) {
				v.Error = "Enter the subject taught by this teacher."
			} else {
				// demote previous head teacher if a new head teacher is added
				if head {
					for i := range a.teachers {
						if a.teachers[i].IsHead {
							a.teachers[i].IsHead = false

							if isHeadRole(a.teachers[i].Role) {
								a.teachers[i].Role = "Senior teacher"
							}
						}
					}
				}

				newT := Teacher{
					ID:            "t-" + token()[:8],
					Name:          tName,
					Role:          tRole,
					Subject:       tSubject,
					Qualification: tQual,
					Email:         tEmail,
					Image:         tImage,
					IsHead:        head,
				}

				a.teachers = append(a.teachers, newT)

				if err := a.saveTeachers(); err != nil {
					v.Error = "Unable to save new teacher."
				} else {
					// auto create login account if email is free
					if _, existsAcc := a.users[tEmail]; !existsAcc {
						temporary := token()[:20]

						tAcc := User{
							Email:          tEmail,
							Name:           tName,
							Role:           "teacher",
							TeacherSubject: tSubject,
							TeacherClasses: parseTeacherClasses(r.FormValue("teacher_classes")),
							MustChange:     true,
						}

						setPassword(&tAcc, temporary)

						if a.put(tAcc) == nil {
							*tempPass = temporary
							*tempEmail = tEmail
							*tempAccountType = "teacher"
						}
					}

					v.Success = "New teacher (" + tName + ") added and login account created."
				}
			}
		}

		return true, false

	} else if action == "teacher-password" {

		// reset or generate a teacher portal password
		tID := strings.TrimSpace(r.FormValue("teacher_id"))
		var found *Teacher

		for i := range a.teachers {
			if a.teachers[i].ID == tID {
				found = &a.teachers[i]
				break
			}
		}

		if found == nil || !validEmail(found.Email) {
			v.Error = "Teacher profile with a valid email not found."
		} else if existing, ok := a.users[found.Email]; ok && existing.Role != "teacher" {
			v.Error = "That email is already used by a " + existing.Role + " account."
		} else {
			temporary := token()[:20]

			tAcc := a.users[found.Email]
			tAcc.Email = found.Email
			tAcc.Name = found.Name
			tAcc.Role = "teacher"
			tAcc.TeacherSubject = found.Subject
			tAcc.MustChange = true

			setPassword(&tAcc, temporary)

			if err := a.put(tAcc); err != nil {
				v.Error = "Unable to issue teacher password."
			} else {
				// clear old active sessions for this teacher
				for key, session := range a.sessions {
					if session.Email == found.Email {
						delete(a.sessions, key)
					}
				}

				*tempPass = temporary
				*tempEmail = found.Email
				*tempAccountType = "teacher"
				v.Success = "New temporary login password issued for teacher " + found.Name + " (" + found.Email + ")."
			}
		}

		return true, false

	} else if action == "teacher-update" {

		tID := strings.TrimSpace(r.FormValue("teacher_id"))
		tName := strings.TrimSpace(r.FormValue("teacher_name"))
		tRole := strings.TrimSpace(r.FormValue("teacher_role"))
		tSubject := strings.TrimSpace(r.FormValue("teacher_subject"))
		tQual := strings.TrimSpace(r.FormValue("teacher_qualification"))
		tEmail := strings.ToLower(strings.TrimSpace(r.FormValue("teacher_email")))
		tImage := strings.TrimSpace(r.FormValue("teacher_image"))

		idx := -1
		for i, t := range a.teachers {
			if t.ID == tID {
				idx = i
				break
			}
		}

		if idx == -1 {
			v.Error = "Teacher profile not found."
		} else if tName == "" || len(tName) > 100 || tRole == "" || len(tRole) > 60 || tQual == "" || len(tQual) > 100 || !validEmail(tEmail) || !validTeacherImage(tImage) {
			v.Error = "Enter a valid teacher name, role, qualification, email and portrait."
		} else {
			head := isHeadRole(tRole)

			if head {
				for i := range a.teachers {
					if i != idx && a.teachers[i].IsHead {
						a.teachers[i].IsHead = false

						if isHeadRole(a.teachers[i].Role) {
							a.teachers[i].Role = "Senior teacher"
						}
					}
				}
			}

			a.teachers[idx] = Teacher{
				ID:            tID,
				Name:          tName,
				Role:          tRole,
				Subject:       tSubject,
				Qualification: tQual,
				Email:         tEmail,
				Image:         tImage,
				IsHead:        head,
			}

			if err := a.saveTeachers(); err != nil {
				v.Error = "Unable to update teacher profile."
			} else {
				// sync changes to linked teacher login account if one exists
				if tAcc, ok := a.users[tEmail]; ok && tAcc.Role == "teacher" {
					tAcc.Name = tName
					tAcc.TeacherSubject = tSubject
					tAcc.TeacherClasses = parseTeacherClasses(r.FormValue("teacher_classes"))
					_ = a.put(tAcc)
				}

				v.Success = "Teacher profile (" + tName + ") updated."
			}
		}

		return true, false

	} else if action == "teacher-delete" {

		tID := strings.TrimSpace(r.FormValue("teacher_id"))
		idx := -1

		for i, t := range a.teachers {
			if t.ID == tID {
				idx = i
				break
			}
		}

		if idx == -1 {
			v.Error = "Teacher profile not found."
		} else {
			removedName := a.teachers[idx].Name
			removedEmail := strings.ToLower(strings.TrimSpace(a.teachers[idx].Email))

			a.teachers = append(a.teachers[:idx], a.teachers[idx+1:]...)

			if err := a.saveTeachers(); err != nil {
				v.Error = "Unable to delete teacher profile."
			} else {
				// also remove teacher login account and active sessions
				if tAcc, ok := a.users[removedEmail]; ok && tAcc.Role == "teacher" {
					delete(a.users, removedEmail)
					_ = a.save()

					for key, session := range a.sessions {
						if session.Email == removedEmail {
							delete(a.sessions, key)
						}
					}
				}

				v.Success = "Teacher (" + removedName + ") removed from the Teachers' Information page."
			}
		}

		return true, false
	}

	// ----------------------------------------------------------------
	// Notice Board (add / delete)
	// ----------------------------------------------------------------
	if action == "notice-add" {

		// publish a new notice straight to notice.html
		nTitle := strings.TrimSpace(r.FormValue("notice_title"))
		nCategory := strings.TrimSpace(r.FormValue("notice_category"))
		nDate := strings.TrimSpace(r.FormValue("notice_date"))
		nSummary := strings.TrimSpace(r.FormValue("notice_summary"))
		nPinned := r.FormValue("notice_pinned") == "on" || r.FormValue("notice_pinned") == "true"

		if nDate == "" {
			nDate = now.Format("2006-01-02")
		}

		if nTitle == "" || len(nTitle) > 140 || !validNoticeCategory(nCategory) || nSummary == "" || len(nSummary) > 600 {
			v.Error = "Enter a valid notice title, category and summary (up to 600 characters)."
		} else {
			newN := NoticeRecord{
				ID:          "n-" + token()[:8],
				Title:       nTitle,
				Category:    nCategory,
				PublishDate: nDate,
				Summary:     nSummary,
				IsPinned:    nPinned,
			}

			a.notices = append([]NoticeRecord{newN}, a.notices...)

			if err := a.saveNotices(); err != nil {
				v.Error = "Unable to publish notice."
			} else {
				v.Success = "Notice (" + nTitle + ") published to the public Notice Board."
			}
		}

		return true, false

	} else if action == "notice-delete" {

		nID := strings.TrimSpace(r.FormValue("notice_id"))
		idx := -1

		for i, n := range a.notices {
			if n.ID == nID {
				idx = i
				break
			}
		}

		if idx == -1 {
			v.Error = "Notice not found."
		} else {
			removedTitle := a.notices[idx].Title
			a.notices = append(a.notices[:idx], a.notices[idx+1:]...)

			if err := a.saveNotices(); err != nil {
				v.Error = "Unable to remove notice."
			} else {
				v.Success = "Notice (" + removedTitle + ") removed from the Notice Board."
			}
		}

		return true, false
	}

	// ----------------------------------------------------------------
	// School Gallery photos
	// ----------------------------------------------------------------
	if action == "gallery-add" {

		// upload photo and caption for gallery.html
		gTitle := strings.TrimSpace(r.FormValue("gallery_title"))

		if gTitle == "" || len(gTitle) > 160 {
			v.Error = "Enter a photo caption (up to 160 characters)."
		} else if imgPath, err := a.saveUploadedGalleryPhoto(r, "gallery_photo"); err != nil {
			v.Error = "Choose a valid JPEG, PNG or WebP image file (up to 5 MB)."
		} else {
			item := GalleryItem{
				ID:    "g-" + token()[:8],
				Title: gTitle,
				Image: imgPath,
			}

			a.gallery = append([]GalleryItem{item}, a.gallery...)

			if err := a.saveGallery(); err != nil {
				v.Error = "Unable to save gallery photo."
			} else {
				v.Success = "Photo (" + gTitle + ") added to the public Gallery page."
			}
		}

		return true, false

	} else if action == "gallery-delete" {

		// remove gallery item and uploaded file
		gID := strings.TrimSpace(r.FormValue("gallery_id"))
		idx := -1

		for i, g := range a.gallery {
			if g.ID == gID {
				idx = i
				break
			}
		}

		if idx == -1 {
			v.Error = "Gallery photo not found."
		} else {
			removed := a.gallery[idx]
			a.gallery = append(a.gallery[:idx], a.gallery[idx+1:]...)

			if err := a.saveGallery(); err != nil {
				v.Error = "Unable to remove gallery photo."
			} else {
				if strings.HasPrefix(removed.Image, "images/gallery/") {
					_ = os.Remove(filepath.Join(a.frontendDir, filepath.FromSlash(removed.Image)))
				}

				v.Success = "Photo (" + removed.Title + ") removed from the Gallery."
			}
		}

		return true, false
	}

	// ----------------------------------------------------------------
	// Class seat capacities
	// ----------------------------------------------------------------
	if action == "seats-update" {

		// update seat counts for classes 2-8 and 10
		for _, cls := range []string{"2", "3", "4", "5", "6", "7", "8", "10"} {
			if n, err := strconv.Atoi(strings.TrimSpace(r.FormValue("seat_" + cls))); err == nil && n >= 0 && n <= 500 {
				a.seats.Classes[cls] = n
			}
		}

		// update class 9 group seats and sum total
		for _, grp := range []string{"science", "humanities", "business"} {
			if n, err := strconv.Atoi(strings.TrimSpace(r.FormValue("seat_9_" + grp))); err == nil && n >= 0 && n <= 500 {
				a.seats.Groups[grp] = n
			}
		}

		a.seats.Classes["9"] = a.seats.Groups["science"] + a.seats.Groups["humanities"] + a.seats.Groups["business"]

		if err := a.saveSeats(); err != nil {
			v.Error = "Unable to save seat availability."
		} else {
			v.Success = "Class seat availability updated on the public Admission pages."
		}

		return true, false
	}

	// ----------------------------------------------------------------
	// Online Admission applications (approve / reject / delete)
	// ----------------------------------------------------------------
	if action == "admission-status" {

		appID := strings.TrimSpace(r.FormValue("admission_id"))
		status := strings.TrimSpace(r.FormValue("status"))

		if status != "Approved" && status != "Rejected" && status != "Pending" {
			v.Error = "Invalid application status."
		} else {
			found := false
			var targetApp AdmissionApplication

			for i := range a.admissions {
				if a.admissions[i].ID == appID {
					a.admissions[i].Status = status
					a.notifyAdmissionGuardian(&a.admissions[i])
					targetApp = a.admissions[i]
					found = true
					break
				}
			}

			if !found {
				v.Error = "Admission application not found."
			} else if err := a.saveAdmissions(); err != nil {
				v.Error = "Unable to update application status."
			} else {
				v.Success = "Application " + appID + " marked as " + status + " — notification sent to guardian phone (" + targetApp.GuardianPhone + ")"

				if targetApp.GuardianEmail != "" {
					v.Success += " and email (" + targetApp.GuardianEmail + ")."
				} else {
					v.Success += "."
				}
			}
		}

		return true, false

	} else if action == "admission-delete" {

		appID := strings.TrimSpace(r.FormValue("admission_id"))
		idx := -1

		for i, app := range a.admissions {
			if app.ID == appID {
				idx = i
				break
			}
		}

		if idx == -1 {
			v.Error = "Admission application not found."
		} else {
			removed := a.admissions[idx]
			a.admissions = append(a.admissions[:idx], a.admissions[idx+1:]...)

			if err := a.saveAdmissions(); err != nil {
				v.Error = "Unable to remove application."
			} else {
				// delete uploaded applicant docs and photo from disk
				for _, fp := range []string{removed.StudentPhoto, removed.TransferCertificate, removed.AcademicCertificate} {
					if fp != "" {
						_ = os.Remove(filepath.Join(a.frontendDir, filepath.FromSlash(fp)))
					}
				}

				v.Success = "Application " + appID + " removed."
			}
		}

		return true, false
	}

	// ----------------------------------------------------------------
	// Achievements page cards
	// ----------------------------------------------------------------
	if action == "achievement-add" {

		aTitle := strings.TrimSpace(r.FormValue("achievement_title"))
		aCat := strings.TrimSpace(r.FormValue("achievement_category"))
		aDesc := strings.TrimSpace(r.FormValue("achievement_desc"))

		if aTitle == "" || len(aTitle) > 140 || aCat == "" || len(aCat) > 60 || aDesc == "" || len(aDesc) > 600 {
			v.Error = "Enter a valid achievement title, category/year badge and description."
		} else if imgPath, err := a.saveUploadedGalleryPhoto(r, "achievement_photo"); err != nil {
			v.Error = "Choose a valid JPEG, PNG or WebP image file (up to 5 MB)."
		} else {
			rec := AchievementRecord{
				ID:          "ach-" + token()[:8],
				Title:       aTitle,
				Category:    aCat,
				Description: aDesc,
				ImagePath:   imgPath,
			}

			a.achievements = append([]AchievementRecord{rec}, a.achievements...)

			if err := a.saveAchievements(); err != nil {
				v.Error = "Unable to save achievement."
			} else {
				v.Success = "Achievement (" + aTitle + ") published to the public Achievements page."
			}
		}

		return true, false

	} else if action == "achievement-delete" {

		aID := strings.TrimSpace(r.FormValue("achievement_id"))
		idx := -1

		for i, ach := range a.achievements {
			if ach.ID == aID {
				idx = i
				break
			}
		}

		if idx == -1 {
			v.Error = "Achievement not found."
		} else {
			removed := a.achievements[idx]
			a.achievements = append(a.achievements[:idx], a.achievements[idx+1:]...)

			if err := a.saveAchievements(); err != nil {
				v.Error = "Unable to remove achievement."
			} else {
				if strings.HasPrefix(removed.ImagePath, "images/gallery/") {
					_ = os.Remove(filepath.Join(a.frontendDir, filepath.FromSlash(removed.ImagePath)))
				}

				v.Success = "Achievement (" + removed.Title + ") removed."
			}
		}

		return true, false
	}

	// ----------------------------------------------------------------
	// Home stats & About page leadership quotes
	// ----------------------------------------------------------------
	if action == "stats-update" {

		// update "School at a glance" counters on home.html
		students, sErr := strconv.Atoi(strings.TrimSpace(r.FormValue("total_students")))
		teachers, tErr := strconv.Atoi(strings.TrimSpace(r.FormValue("total_teachers")))
		staff, stErr := strconv.Atoi(strings.TrimSpace(r.FormValue("total_staff")))

		if sErr != nil || tErr != nil || stErr != nil || students < 0 || students > 50000 || teachers < 0 || teachers > 5000 || staff < 0 || staff > 5000 {
			v.Error = "Enter valid numbers for total students, teachers and staff."
		} else {
			a.stats.TotalStudents = students
			a.stats.TotalTeachers = teachers
			a.stats.TotalStaff = staff

			if err := a.saveStats(); err != nil {
				v.Error = "Unable to save home page statistics."
			} else {
				v.Success = "Home page statistics updated."
			}
		}

		return true, false

	} else if action == "leadership-update" {

		// update UNO and Head Teacher portraits, names, roles, and quotes on about.html
		chairName := strings.TrimSpace(r.FormValue("chairperson_name"))
		chairRole := strings.TrimSpace(r.FormValue("chairperson_role"))
		chairMsg := strings.TrimSpace(r.FormValue("chairperson_message"))

		headName := strings.TrimSpace(r.FormValue("headteacher_name"))
		headRole := strings.TrimSpace(r.FormValue("headteacher_role"))
		headMsg := strings.TrimSpace(r.FormValue("headteacher_message"))

		if len(chairName) > 100 || len(headName) > 100 || len(chairRole) > 120 || len(headRole) > 120 || len(chairMsg) > 1200 || len(headMsg) > 1200 {
			v.Error = "Names (<=100), roles (<=120), and messages (<=1200) exceeded length limits."
			return true, false
		}

		if imgPath, err := a.saveUploadedGalleryPhoto(r, "chairperson_photo"); err == nil {
			if strings.HasPrefix(a.stats.ChairpersonImage, "images/gallery/") {
				_ = os.Remove(filepath.Join(a.frontendDir, filepath.FromSlash(a.stats.ChairpersonImage)))
			}

			a.stats.ChairpersonImage = imgPath
		} else if err == os.ErrInvalid {
			v.Error = "Choose a valid JPEG, PNG or WebP image file (up to 5 MB) for the Upazila Nirbahi Officer."
			return true, false
		}

		if imgPath, err := a.saveUploadedGalleryPhoto(r, "headteacher_photo"); err == nil {
			if strings.HasPrefix(a.stats.HeadTeacherImage, "images/gallery/") {
				_ = os.Remove(filepath.Join(a.frontendDir, filepath.FromSlash(a.stats.HeadTeacherImage)))
			}

			a.stats.HeadTeacherImage = imgPath
		} else if err == os.ErrInvalid {
			v.Error = "Choose a valid JPEG, PNG or WebP image file (up to 5 MB) for the Head Teacher."
			return true, false
		}

		if chairName != "" {
			a.stats.ChairpersonName = chairName
		}
		if chairRole != "" {
			a.stats.ChairpersonRole = chairRole
		}
		if chairMsg != "" {
			a.stats.ChairpersonMessage = chairMsg
		}

		if headName != "" {
			a.stats.HeadTeacherName = headName
		}
		if headRole != "" {
			a.stats.HeadTeacherRole = headRole
		}
		if headMsg != "" {
			a.stats.HeadTeacherMessage = headMsg
		}

		// keep Head Teacher synced in teachers list too
		syncedTeachers := false
		for i := range a.teachers {
			if a.teachers[i].IsHead {
				if a.stats.HeadTeacherName != "" {
					a.teachers[i].Name = a.stats.HeadTeacherName
				}
				if a.stats.HeadTeacherImage != "" {
					a.teachers[i].Image = a.stats.HeadTeacherImage
				}

				syncedTeachers = true
			}
		}

		if syncedTeachers {
			_ = a.saveTeachers()
		}

		if err := a.saveStats(); err != nil {
			v.Error = "Unable to save About page leadership updates."
		} else {
			v.Success = "About page leadership portraits, names, and messages updated."
		}

		return true, false

	} else if action == "leadership-quote-add" {

		// add another quote card on about.html
		qBadge := strings.TrimSpace(r.FormValue("quote_badge"))
		qName := strings.TrimSpace(r.FormValue("quote_name"))
		qRole := strings.TrimSpace(r.FormValue("quote_role"))
		qMsg := strings.TrimSpace(r.FormValue("quote_message"))

		if qBadge == "" {
			qBadge = "Leadership message"
		}

		if qName == "" || len(qName) > 100 || len(qBadge) > 60 || qRole == "" || len(qRole) > 120 || qMsg == "" || len(qMsg) > 1200 {
			v.Error = "Enter a valid name, designation/role, and message (up to 1200 characters)."
			return true, false
		}

		imgPath := "images/teachers/sample-teacher-3.png"

		if uploaded, err := a.saveUploadedGalleryPhoto(r, "quote_photo"); err == nil {
			imgPath = uploaded
		} else if err == os.ErrInvalid {
			v.Error = "Choose a valid JPEG, PNG or WebP image file (up to 5 MB)."
			return true, false
		}

		q := LeadershipQuote{
			ID:      "lq-" + token()[:8],
			Badge:   qBadge,
			Name:    qName,
			Role:    qRole,
			Message: qMsg,
			Image:   imgPath,
		}

		a.stats.ExtraLeadershipQuotes = append(a.stats.ExtraLeadershipQuotes, q)

		if err := a.saveStats(); err != nil {
			v.Error = "Unable to save leadership quote."
		} else {
			v.Success = "Added message from " + qName + " to the public About page."
		}

		return true, false

	} else if action == "leadership-quote-delete" {

		qID := strings.TrimSpace(r.FormValue("quote_id"))
		idx := -1

		for i, q := range a.stats.ExtraLeadershipQuotes {
			if q.ID == qID {
				idx = i
				break
			}
		}

		if idx == -1 {
			v.Error = "Leadership quote not found."
		} else {
			removed := a.stats.ExtraLeadershipQuotes[idx]
			a.stats.ExtraLeadershipQuotes = append(a.stats.ExtraLeadershipQuotes[:idx], a.stats.ExtraLeadershipQuotes[idx+1:]...)

			if err := a.saveStats(); err != nil {
				v.Error = "Unable to remove leadership quote."
			} else {
				if strings.HasPrefix(removed.Image, "images/gallery/") {
					_ = os.Remove(filepath.Join(a.frontendDir, filepath.FromSlash(removed.Image)))
				}

				v.Success = "Removed message from " + removed.Name + "."
			}
		}

		return true, false
	}

	// ----------------------------------------------------------------
	// Class Routine & Academic Calendar PDFs
	// ----------------------------------------------------------------
	if action == "routine-pdf-upload" {

		rClass := strings.TrimSpace(r.FormValue("routine_class"))
		validClass := rClass == "All"

		for _, c := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"} {
			if rClass == c {
				validClass = true
			}
		}

		if !validClass {
			v.Error = "Select a valid target class for the routine PDF."
		} else if pdfPath, err := a.saveUploadedResultPDF(r, "routine_pdf"); err != nil {
			v.Error = "Choose a valid PDF file (up to 5 MB)."
		} else {
			if a.stats.RoutinePDFs == nil {
				a.stats.RoutinePDFs = map[string]string{}
			}

			if oldPath := a.stats.RoutinePDFs[rClass]; oldPath != "" {
				_ = os.Remove(filepath.Join(a.frontendDir, filepath.FromSlash(oldPath)))
			}

			a.stats.RoutinePDFs[rClass] = pdfPath

			if err := a.saveStats(); err != nil {
				v.Error = "Unable to save class routine PDF."
			} else if rClass == "All" {
				v.Success = "Class Routine PDF published for all classes."
			} else {
				v.Success = "Class Routine PDF published for Class " + rClass + "."
			}
		}

		return true, false

	} else if action == "routine-pdf-delete" {

		rClass := strings.TrimSpace(r.FormValue("routine_class"))

		if a.stats.RoutinePDFs == nil || a.stats.RoutinePDFs[rClass] == "" {
			v.Error = "Class routine PDF not found."
		} else {
			oldPath := a.stats.RoutinePDFs[rClass]
			delete(a.stats.RoutinePDFs, rClass)

			if err := a.saveStats(); err != nil {
				v.Error = "Unable to remove class routine PDF."
			} else {
				_ = os.Remove(filepath.Join(a.frontendDir, filepath.FromSlash(oldPath)))
				v.Success = "Class Routine PDF removed."
			}
		}

		return true, false

	} else if action == "calendar-pdf-upload" {

		if pdfPath, err := a.saveUploadedResultPDF(r, "calendar_pdf"); err != nil {
			v.Error = "Choose a valid PDF file (up to 5 MB)."
		} else {
			if a.stats.CalendarPDF != "" {
				_ = os.Remove(filepath.Join(a.frontendDir, filepath.FromSlash(a.stats.CalendarPDF)))
			}

			a.stats.CalendarPDF = pdfPath

			if err := a.saveStats(); err != nil {
				v.Error = "Unable to save academic calendar PDF."
			} else {
				v.Success = "Academic Calendar PDF published to Student Dashboards."
			}
		}

		return true, false

	} else if action == "calendar-pdf-delete" {

		if a.stats.CalendarPDF == "" {
			v.Error = "Academic Calendar PDF not found."
		} else {
			oldPath := a.stats.CalendarPDF
			a.stats.CalendarPDF = ""

			if err := a.saveStats(); err != nil {
				v.Error = "Unable to remove academic calendar PDF."
			} else {
				_ = os.Remove(filepath.Join(a.frontendDir, filepath.FromSlash(oldPath)))
				v.Success = "Academic Calendar PDF removed."
			}
		}

		return true, false
	}

	// ----------------------------------------------------------------
	// School Admin accounts (Super Admin only)
	// ----------------------------------------------------------------
	if action == "admin-create" || action == "admin-reset" || action == "admin-delete" {

		// server-side role guard so normal admins or students get 403
		if u.Role != "superadmin" {
			http.Error(w, "Super administrator access required", 403)
			return true, true
		}

		if action == "admin-create" {

			adminName := strings.TrimSpace(r.FormValue("admin_name"))
			adminEmail := strings.ToLower(strings.TrimSpace(r.FormValue("admin_email")))
			_, already := a.users[adminEmail]

			if already || !validEmail(adminEmail) || adminName == "" || len(adminName) > 100 {
				v.Error = "Enter a unique valid email and name for the new School Admin."
			} else {
				temporary := token()[:20]

				newAdmin := User{
					Email:      adminEmail,
					Name:       adminName,
					Role:       "admin",
					MustChange: true,
				}

				setPassword(&newAdmin, temporary)

				if err := a.put(newAdmin); err != nil {
					v.Error = "Unable to create School Admin account."
				} else {
					*tempPass = temporary
					*tempEmail = adminEmail
					*tempAccountType = "admin"
					v.Success = "School Admin account created: " + adminEmail
				}
			}

		} else if action == "admin-reset" {

			adminEmail := strings.ToLower(strings.TrimSpace(r.FormValue("admin_email")))
			target, ok := a.users[adminEmail]

			if !ok || target.Role != "admin" {
				v.Error = "School Admin account not found."
			} else {
				temporary := token()[:20]
				setPassword(&target, temporary)
				target.MustChange = true

				if err := a.put(target); err != nil {
					v.Error = "Unable to reset School Admin password."
				} else {
					for key, session := range a.sessions {
						if session.Email == adminEmail {
							delete(a.sessions, key)
						}
					}

					*tempPass = temporary
					*tempEmail = adminEmail
					*tempAccountType = "admin"
					v.Success = "School Admin (" + adminEmail + ") password has been reset."
				}
			}

		} else if action == "admin-delete" {

			adminEmail := strings.ToLower(strings.TrimSpace(r.FormValue("admin_email")))
			target, ok := a.users[adminEmail]

			if !ok || target.Role != "admin" {
				v.Error = "School Admin account not found."
			} else {
				delete(a.users, adminEmail)

				if err := a.save(); err != nil {
					a.users[adminEmail] = target
					v.Error = "Unable to remove School Admin account."
				} else {
					for key, session := range a.sessions {
						if session.Email == adminEmail {
							delete(a.sessions, key)
						}
					}

					v.Success = "School Admin (" + adminEmail + ") has been removed."
				}
			}
		}

		return true, false
	}

	return false, false
}
