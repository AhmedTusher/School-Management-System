-- ============================================================================
-- S. M. Pilot Model School — Notices, Achievements & Contact Schema
-- Author: tahsin-rahman2004
-- Database: PostgreSQL / SQLite compatible SQL
-- ============================================================================

CREATE TABLE IF NOT EXISTS notices (
    id           VARCHAR(40)  PRIMARY KEY,
    title        VARCHAR(200) NOT NULL,
    category     VARCHAR(40)  NOT NULL CHECK (category IN ('academic', 'exam', 'event', 'holiday', 'admission', 'general')),
    publish_date DATE         NOT NULL,
    summary      TEXT         NOT NULL,
    attachment   VARCHAR(255) DEFAULT '',
    is_pinned    BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notices_category ON notices(category);
CREATE INDEX IF NOT EXISTS idx_notices_pinned ON notices(is_pinned, publish_date DESC);

CREATE TABLE IF NOT EXISTS achievements (
    id             VARCHAR(40)  PRIMARY KEY,
    title          VARCHAR(200) NOT NULL,
    category       VARCHAR(50)  NOT NULL CHECK (category IN ('board_exam', 'scholarship', 'science_olympiad', 'sports', 'cultural', 'debate')),
    award_year     INTEGER      NOT NULL,
    recipient_name VARCHAR(120) NOT NULL,
    description    TEXT         NOT NULL,
    image_path     VARCHAR(255) DEFAULT '',
    created_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_achievements_year ON achievements(award_year DESC);

CREATE TABLE IF NOT EXISTS contact_messages (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    sender_name  VARCHAR(100) NOT NULL,
    sender_email VARCHAR(254) NOT NULL,
    sender_phone VARCHAR(30)  DEFAULT '',
    subject      VARCHAR(150) NOT NULL,
    message_body TEXT         NOT NULL,
    is_resolved  BOOLEAN      NOT NULL DEFAULT FALSE,
    submitted_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed Data for School Notice Board
INSERT INTO notices (id, title, category, publish_date, summary, attachment, is_pinned) VALUES
    ('nt-1', 'Class 6 to 9 Admission Open for 2027 Session', 'admission', '2026-11-01', 'Online and offline admission forms are now available for Classes 6 through 9. Review the admission policy and submit applications before the deadline.', 'downloads/admission-policy-2027.pdf', TRUE),
    ('nt-2', 'Annual Examination Routine Published', 'exam', '2026-10-25', 'The schedule for the upcoming Annual Examinations for Classes 1 to 10 has been published by the academic office.', '', TRUE),
    ('nt-3', 'Inter-House Sports & Cultural Week', 'event', '2026-10-18', 'Students interested in athletics, debate, and cultural competitions should register their names with their respective class teachers.', '', FALSE),
    ('nt-4', 'Science Fair Project Submission', 'academic', '2026-10-10', 'Class 8, 9, and 10 Science group students are invited to submit project proposals for the District Science & Technology Fair.', '', FALSE)
ON CONFLICT (id) DO NOTHING;

-- Seed Data for School Achievements
INSERT INTO achievements (id, title, category, award_year, recipient_name, description, image_path) VALUES
    ('ach-1', '100% Pass Rate & District Top GPA-5 in SSC Examination', 'board_exam', 2026, 'SSC Batch 2026', 'Achieved outstanding board examination results in Comilla Education Board with record GPA-5 scholars.', 'images/ach-ssc-topper.png'),
    ('ach-2', 'National Science & Mathematics Olympiad Regional Champions', 'science_olympiad', 2026, 'Senior Science Club', 'First place in the regional Science and Mathematics Olympiad with three students qualifying for the national round.', 'images/ach-math-olympiad.png'),
    ('ach-3', 'Inter-School Debate & Cultural Competition Trophy', 'debate', 2025, 'School Debate Team', 'Awarded Best Team and Best Speaker honours at the Upazila Inter-School Debate Championship.', 'images/ach-debate.png'),
    ('ach-4', 'District Inter-School Football & Athletics Championship', 'sports', 2025, 'School Sports Team', 'Crowned champions in the Upazila and District School Football Tournament.', 'images/ach-football.png')
ON CONFLICT (id) DO NOTHING;

