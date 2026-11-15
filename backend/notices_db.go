// notices_db.go - official school notices.
// Notices are kept in data/notices.json. Admins add or delete them in the
// /admin panel and the public notice.html page reads them from /api/notices.
// (The SQL repository code below is prepared for a future real database.)
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
)

// One notice shown on the Notice board.
type NoticeRecord struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	PublishDate string `json:"publishDate"`
	Summary     string `json:"summary"`
	Attachment  string `json:"attachment"`
	IsPinned    bool   `json:"isPinned"`
}

type AchievementRecord struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Category      string `json:"category"`
	AwardYear     int    `json:"awardYear"`
	RecipientName string `json:"recipientName"`
	Description   string `json:"description"`
	ImagePath     string `json:"imagePath"`
}

type ContactMessage struct {
	ID          int    `json:"id"`
	SenderName  string `json:"senderName"`
	SenderEmail string `json:"senderEmail"`
	SenderPhone string `json:"senderPhone"`
	Subject     string `json:"subject"`
	MessageBody string `json:"messageBody"`
	IsResolved  bool   `json:"isResolved"`
}

// NoticesContactsSQLRepository manages SQL queries for notices, achievements, and contact messages.
type NoticesContactsSQLRepository struct {
	db *sql.DB
}

func NewNoticesContactsSQLRepository(db *sql.DB) *NoticesContactsSQLRepository {
	return &NoticesContactsSQLRepository{db: db}
}

func (r *NoticesContactsSQLRepository) InitSchema(ctx context.Context, schemaPath string) error {
	if r == nil || r.db == nil {
		return nil
	}
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, string(raw))
	return err
}

func (r *NoticesContactsSQLRepository) ListNoticesSQL(ctx context.Context) ([]NoticeRecord, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	const query = `
SELECT id, title, category, publish_date, summary, attachment, is_pinned
FROM notices
ORDER BY is_pinned DESC, publish_date DESC;`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []NoticeRecord
	for rows.Next() {
		var n NoticeRecord
		if err := rows.Scan(&n.ID, &n.Title, &n.Category, &n.PublishDate, &n.Summary, &n.Attachment, &n.IsPinned); err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	return items, rows.Err()
}

func (r *NoticesContactsSQLRepository) SaveNoticeSQL(ctx context.Context, n NoticeRecord) error {
	if r == nil || r.db == nil {
		return nil
	}
	const query = `
INSERT INTO notices (id, title, category, publish_date, summary, attachment, is_pinned)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    category = EXCLUDED.category,
    publish_date = EXCLUDED.publish_date,
    summary = EXCLUDED.summary,
    attachment = EXCLUDED.attachment,
    is_pinned = EXCLUDED.is_pinned;`
	_, err := r.db.ExecContext(ctx, query, n.ID, n.Title, n.Category, n.PublishDate, n.Summary, n.Attachment, n.IsPinned)
	return err
}

func (r *NoticesContactsSQLRepository) DeleteNoticeSQL(ctx context.Context, id string) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM notices WHERE id = $1`, id)
	return err
}

func (r *NoticesContactsSQLRepository) ListAchievementsSQL(ctx context.Context) ([]AchievementRecord, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	const query = `
SELECT id, title, category, award_year, recipient_name, description, image_path
FROM achievements
ORDER BY award_year DESC, id ASC;`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []AchievementRecord
	for rows.Next() {
		var a AchievementRecord
		if err := rows.Scan(&a.ID, &a.Title, &a.Category, &a.AwardYear, &a.RecipientName, &a.Description, &a.ImagePath); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r *NoticesContactsSQLRepository) SaveContactMessageSQL(ctx context.Context, m ContactMessage) error {
	if r == nil || r.db == nil {
		return nil
	}
	const query = `
INSERT INTO contact_messages (sender_name, sender_email, sender_phone, subject, message_body)
VALUES ($1, $2, $3, $4, $5);`
	_, err := r.db.ExecContext(ctx, query, m.SenderName, m.SenderEmail, m.SenderPhone, m.Subject, m.MessageBody)
	return err
}

// Starter notices used the first time the server runs (before any file exists).
func defaultNotices() []NoticeRecord {
	return []NoticeRecord{
		{
			ID:          "n-1",
			Title:       "2027 Academic Session Admission Schedule",
			Category:    "Admission",
			PublishDate: "2026-11-05",
			Summary:     "Applications for Classes 1–9 are now open. Class 1 selection is by digital lottery; Classes 2–9 require a written admission test.",
			Attachment:  "downloads/admission-policy-2027.pdf",
			IsPinned:    true,
		},
		{
			ID:          "n-2",
			Title:       "Annual Examination 2026 Routine & Guidelines",
			Category:    "Examination",
			PublishDate: "2026-11-01",
			Summary:     "Annual examinations for Classes 1–9 will commence from November 24, 2026. Students are requested to collect their admit cards from the school office.",
			Attachment:  "",
			IsPinned:    false,
		},
		{
			ID:          "n-3",
			Title:       "Winter Timings & Morning Assembly Notice",
			Category:    "General",
			PublishDate: "2026-10-28",
			Summary:     "Starting November 15, morning assembly will begin at 8:30 AM and regular classes will commence at 8:45 AM.",
			Attachment:  "",
			IsPinned:    false,
		},
	}
}

// Only these categories are accepted, so nobody can send a made-up one.
func validNoticeCategory(cat string) bool {
	for _, allowed := range []string{"Admission", "Examination", "Academic", "Event", "General", "Urgent"} {
		if cat == allowed {
			return true
		}
	}
	return false
}

// The notices file lives next to the accounts file.
func (a *App) noticesFile() string {
	return filepath.Join(filepath.Dir(a.file), "notices.json")
}

// Reads notices.json at start-up. If it does not exist, use the starter notices.
func (a *App) loadNotices() error {
	data, err := os.ReadFile(a.noticesFile())
	if err == nil {
		if err = json.Unmarshal(data, &a.notices); err != nil {
			return err
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	a.notices = defaultNotices()
	return nil
}

// Saves notices safely (write to a temporary file, then rename).
func (a *App) saveNotices() error {
	dir := filepath.Dir(a.noticesFile())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a.notices, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "notices-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, a.noticesFile())
}


