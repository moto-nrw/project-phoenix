package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	StudentTablesVersion     = "1.7.0"
	StudentTablesDescription = "Student and feedback tables"
)

func init() {
	// Migration 7: students and feedback tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentTablesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentTablesDown(ctx, db)
		},
	)
}

// studentTablesUp creates the student, feedback, and student_ags tables
func studentTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Creating students and feedback tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the students table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS students (
			id BIGSERIAL PRIMARY KEY,
			school_class TEXT NOT NULL,
			bus BOOLEAN NOT NULL DEFAULT false,
			name_lg TEXT NOT NULL,
			contact_lg TEXT NOT NULL,
			in_house BOOLEAN NOT NULL DEFAULT false,
			wc BOOLEAN NOT NULL DEFAULT false,
			school_yard BOOLEAN NOT NULL DEFAULT false,
			custom_users_id BIGINT NOT NULL UNIQUE,
			group_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_student_user FOREIGN KEY (custom_users_id) REFERENCES custom_users(id) ON DELETE CASCADE,
			CONSTRAINT fk_student_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE RESTRICT
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating students table: %w", err)
	}

	// 2. Create the feedback table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS feedback (
			id BIGSERIAL PRIMARY KEY,
			feedback_value TEXT NOT NULL,
			day DATE NOT NULL,
			time TIME NOT NULL,
			student_id BIGINT NOT NULL,
			mensa_feedback BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_feedback_student FOREIGN KEY (student_id) REFERENCES  students(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating feedback table: %w", err)
	}

	// 3. Create student_ags junction table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS student_ags (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL,
			ag_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_student_ags_student FOREIGN KEY (student_id) REFERENCES  students(id) ON DELETE CASCADE,
			CONSTRAINT fk_student_ags_ags FOREIGN KEY (ag_id) REFERENCES ags(id) ON DELETE CASCADE,
			CONSTRAINT uq_student_ags UNIQUE(student_id, ag_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating student_ags table: %w", err)
	}

	// Create indexes for student
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_student_custom_users_id ON  students(custom_users_id);
		CREATE INDEX IF NOT EXISTS idx_student_group_id ON  students(group_id);
		CREATE INDEX IF NOT EXISTS idx_student_school_class ON  students(school_class);
		CREATE INDEX IF NOT EXISTS idx_student_in_house ON  students(in_house);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for students table: %w", err)
	}

	// Create indexes for feedback
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_feedback_student_id ON feedback(student_id);
		CREATE INDEX IF NOT EXISTS idx_feedback_day ON feedback(day);
		CREATE INDEX IF NOT EXISTS idx_feedback_mensa ON feedback(mensa_feedback);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for feedback table: %w", err)
	}

	// Create indexes for student_ags
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_student_ags_student_id ON student_ags(student_id);
		CREATE INDEX IF NOT EXISTS idx_student_ags_ag_id ON student_ags(ag_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for student_ags table: %w", err)
	}

	// Create trigger for updated_at column in students table
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_student_modified_at ON students;
		CREATE TRIGGER update_student_modified_at
		BEFORE UPDATE ON students
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for students table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// studentTablesDown removes the student, feedback, and student_ags tables
func studentTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back students and feedback tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS student_ags;
		DROP TABLE IF EXISTS feedback;
		DROP TABLE IF EXISTS students;
	`)
	if err != nil {
		return fmt.Errorf("error dropping students tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
