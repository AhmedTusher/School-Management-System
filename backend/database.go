package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CoreSQLRepository manages SQL persistence for users, attendance, teachers, and admissions.
type CoreSQLRepository struct {
	db *sql.DB
}

func NewCoreSQLRepository(db *sql.DB) *CoreSQLRepository {
	return &CoreSQLRepository{db: db}
}

func (r *CoreSQLRepository) InitSchema(ctx context.Context, schemaPath string) error {
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

func (r *CoreSQLRepository) UpsertUserSQL(ctx context.Context, u User) error {
	if r == nil || r.db == nil {
		return nil
	}
	const query = `
INSERT INTO users (email, name, student_id, class, student_grp, role, salt, password_hash, must_change)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (email) DO UPDATE SET
    name = EXCLUDED.name,
    student_id = EXCLUDED.student_id,
    class = EXCLUDED.class,
    student_grp = EXCLUDED.student_grp,
    role = EXCLUDED.role,
    salt = EXCLUDED.salt,
    password_hash = EXCLUDED.password_hash,
    must_change = EXCLUDED.must_change;`
	_, err := r.db.ExecContext(ctx, query, u.Email, u.Name, u.StudentID, u.Class, u.Group, u.Role, u.Salt, u.Hash, u.MustChange)
	return err
}

func (r *CoreSQLRepository) DeleteUserSQL(ctx context.Context, email string) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE email = $1`, email)
	return err
}

func (r *CoreSQLRepository) LoadUsersSQL(ctx context.Context) ([]User, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT email, name, student_id, class, student_grp, role, salt, password_hash, must_change FROM users ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Email, &u.Name, &u.StudentID, &u.Class, &u.Group, &u.Role, &u.Salt, &u.Hash, &u.MustChange); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

func (r *CoreSQLRepository) UpsertTeacherSQL(ctx context.Context, t Teacher) error {
	if r == nil || r.db == nil {
		return nil
	}
	const query = `
INSERT INTO teachers (id, name, role, subject, qualification, email, image, is_head)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    role = EXCLUDED.role,
    subject = EXCLUDED.subject,
    qualification = EXCLUDED.qualification,
    email = EXCLUDED.email,
    image = EXCLUDED.image,
    is_head = EXCLUDED.is_head,
    updated_at = CURRENT_TIMESTAMP;`
	_, err := r.db.ExecContext(ctx, query, t.ID, t.Name, t.Role, t.Subject, t.Qualification, t.Email, t.Image, t.IsHead)
	return err
}

func (r *CoreSQLRepository) DeleteTeacherSQL(ctx context.Context, id string) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM teachers WHERE id = $1`, id)
	return err
}

func sqlQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// writeSQLSnapshot writes a live SQL dump file alongside the local data store.
func (a *App) writeSQLSnapshot() {
	dir := filepath.Dir(a.file)
	_ = os.MkdirAll(dir, 0700)
	var b strings.Builder
	b.WriteString("-- Auto-generated SQL snapshot for S. M. Pilot Model School\n")
	b.WriteString("BEGIN TRANSACTION;\n\n")
	for _, u := range a.users {
		must := "FALSE"
		if u.MustChange {
			must = "TRUE"
		}
		fmt.Fprintf(&b, "INSERT INTO users (email, name, student_id, class, student_grp, role, salt, password_hash, must_change) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s) ON CONFLICT (email) DO UPDATE SET name=EXCLUDED.name, student_grp=EXCLUDED.student_grp, role=EXCLUDED.role, salt=EXCLUDED.salt, password_hash=EXCLUDED.password_hash, must_change=EXCLUDED.must_change;\n",
			sqlQuote(u.Email), sqlQuote(u.Name), sqlQuote(u.StudentID), sqlQuote(u.Class), sqlQuote(u.Group), sqlQuote(u.Role), sqlQuote(u.Salt), sqlQuote(u.Hash), must)
	}
	b.WriteString("\n")
	for _, t := range a.teachers {
		head := "FALSE"
		if t.IsHead {
			head = "TRUE"
		}
		fmt.Fprintf(&b, "INSERT INTO teachers (id, name, role, subject, qualification, email, image, is_head) VALUES (%s, %s, %s, %s, %s, %s, %s, %s) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, role=EXCLUDED.role, subject=EXCLUDED.subject, qualification=EXCLUDED.qualification, email=EXCLUDED.email, image=EXCLUDED.image, is_head=EXCLUDED.is_head;\n",
			sqlQuote(t.ID), sqlQuote(t.Name), sqlQuote(t.Role), sqlQuote(t.Subject), sqlQuote(t.Qualification), sqlQuote(t.Email), sqlQuote(t.Image), head)
	}
	b.WriteString("\nCOMMIT;\n")
	_ = os.WriteFile(filepath.Join(dir, "school_dump.sql"), []byte(b.String()), 0600)
}

