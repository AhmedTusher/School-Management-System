-- ============================================================================
-- S. M. Pilot Model School — Core Database Schema (Users, Teachers, Attendance, Admissions)
-- Author: AhmedTusher
-- Database: PostgreSQL / SQLite compatible SQL
-- ============================================================================

CREATE TABLE IF NOT EXISTS users (
    email        VARCHAR(254) PRIMARY KEY,
    name         VARCHAR(100) NOT NULL,
    student_id   VARCHAR(50)  DEFAULT '',
    class        VARCHAR(10)  DEFAULT '',
    student_grp  VARCHAR(40)  DEFAULT '',
    role         VARCHAR(20)  NOT NULL CHECK (role IN ('superadmin', 'admin', 'student')),
    salt         VARCHAR(128) NOT NULL,
    password_hash VARCHAR(128) NOT NULL,
    must_change  BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_student_id ON users(student_id);

CREATE TABLE IF NOT EXISTS attendance (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    student_email VARCHAR(254) NOT NULL REFERENCES users(email) ON DELETE CASCADE,
    record_date   DATE         NOT NULL,
    subject       VARCHAR(80)  NOT NULL,
    status        VARCHAR(20)  NOT NULL CHECK (status IN ('present', 'absent', 'late', 'excused')),
    UNIQUE (student_email, record_date, subject)
);

CREATE INDEX IF NOT EXISTS idx_attendance_student ON attendance(student_email);

CREATE TABLE IF NOT EXISTS teachers (
    id            VARCHAR(40)  PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    role          VARCHAR(60)  NOT NULL,
    subject       VARCHAR(80)   NOT NULL DEFAULT '',
    qualification VARCHAR(100) NOT NULL,
    email         VARCHAR(254) NOT NULL,
    image         VARCHAR(255) NOT NULL,
    is_head       BOOLEAN      NOT NULL DEFAULT FALSE,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS admission_applications (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    application_no  VARCHAR(40)  UNIQUE NOT NULL,
    student_name    VARCHAR(100) NOT NULL,
    applying_class  VARCHAR(10)  NOT NULL,
    student_group   VARCHAR(40)  DEFAULT '',
    guardian_name   VARCHAR(100) NOT NULL,
    guardian_phone  VARCHAR(30)  NOT NULL,
    guardian_email  VARCHAR(254) DEFAULT '',
    previous_school VARCHAR(150) DEFAULT '',
    address         TEXT         NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    submitted_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Initial Seed Data for Teachers Directory
INSERT INTO teachers (id, name, role, subject, qualification, email, image, is_head) VALUES
    ('t-1', 'Teacher name', 'Head Teacher', 'Administration', 'M.A., M.Ed.', 'headteacher@example.com', 'images/teachers/sample-teacher-1.png', TRUE),
    ('t-2', 'Teacher name', 'Assistant teacher', 'Bangla', 'M.A., B.Ed.', 'bangla.teacher@example.com', 'images/teachers/sample-teacher-2-hijab.png', FALSE),
    ('t-3', 'Teacher name', 'Assistant teacher', 'English', 'M.A., B.Ed.', 'english.teacher@example.com', 'images/teachers/sample-teacher-3.png', FALSE),
    ('t-4', 'Teacher name', 'Assistant teacher', 'Mathematics', 'M.Sc., B.Ed.', 'math.teacher@example.com', 'images/teachers/sample-teacher-4.png', FALSE),
    ('t-5', 'Teacher name', 'Assistant teacher', 'Science', 'M.Sc., B.Ed.', 'science.teacher@example.com', 'images/teachers/sample-teacher-5.png', FALSE),
    ('t-6', 'Teacher name', 'Assistant teacher', 'ICT', 'B.Sc. (CSE)', 'ict.teacher@example.com', 'images/teachers/sample-teacher-6-hijab.png', FALSE),
    ('t-7', 'Teacher name', 'Assistant teacher', 'Social Studies', 'M.S.S., B.Ed.', 'socialstudies.teacher@example.com', 'images/teachers/sample-teacher-7.png', FALSE)
ON CONFLICT (id) DO NOTHING;

