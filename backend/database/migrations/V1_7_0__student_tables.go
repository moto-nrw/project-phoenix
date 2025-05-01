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
	// Register the migration
	migration := &Migration{
		Version:     StudentTablesVersion,
		Description: StudentTablesDescription,
		DependsOn:   []string{"1.4.0", "1.6.0"}, // Depends on group foundation and activity tables
		Up:          studentTablesUp,
		Down:        studentTablesDown,
	}

	registerMigration(migration)
}

// studentTablesUp creates the student, feedback, and student_ags tables
func studentTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Creating student and feedback tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the student table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS student (
			id BIGSERIAL PRIMARY KEY,
			school_class TEXT NOT NULL,
			bus BOOLEAN NOT NULL DEFAULT false,
			name_lg TEXT NOT NULL,
			contact_lg TEXT NOT NULL,
			in_house BOOLEAN NOT NULL DEFAULT false,
			wc BOOLEAN NOT NULL DEFAULT false,
			school_yard BOOLEAN NOT NULL DEFAULT false,
			custom_user_id BIGINT NOT NULL UNIQUE,
			group_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_student_user FOREIGN KEY (custom_user_id) REFERENCES custom_user(id) ON DELETE CASCADE,
			CONSTRAINT fk_student_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE RESTRICT
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating student table: %w", err)
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
			CONSTRAINT fk_feedback_student FOREIGN KEY (student_id) REFERENCES student(id) ON DELETE CASCADE
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
			CONSTRAINT fk_student_ags_student FOREIGN KEY (student_id) REFERENCES student(id) ON DELETE CASCADE,
			CONSTRAINT fk_student_ags_ag FOREIGN KEY (ag_id) REFERENCES ag(id) ON DELETE CASCADE,
			CONSTRAINT uq_student_ag UNIQUE(student_id, ag_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating student_ags table: %w", err)
	}

	// Create indexes for student
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_student_custom_user_id ON student(custom_user_id);
		CREATE INDEX IF NOT EXISTS idx_student_group_id ON student(group_id);
		CREATE INDEX IF NOT EXISTS idx_student_school_class ON student(school_class);
		CREATE INDEX IF NOT EXISTS idx_student_in_house ON student(in_house);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for student table: %w", err)
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

	// Create trigger for updated_at column in student table
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_student_modified_at ON student;
		CREATE TRIGGER update_student_modified_at
		BEFORE UPDATE ON student
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for student table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// studentTablesDown removes the student, feedback, and student_ags tables
func studentTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back student and feedback tables...")

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
		DROP TABLE IF EXISTS student;
	`)
	if err != nil {
		return fmt.Errorf("error dropping student tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
