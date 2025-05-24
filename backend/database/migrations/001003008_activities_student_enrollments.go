package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	ActivitiesStudentEnrollmentsVersion     = "1.3.8"
	ActivitiesStudentEnrollmentsDescription = "Create activities.student_enrollments table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ActivitiesStudentEnrollmentsVersion] = &Migration{
		Version:     ActivitiesStudentEnrollmentsVersion,
		Description: ActivitiesStudentEnrollmentsDescription,
		DependsOn:   []string{"1.3.2", "1.3.5"}, // Depends on activities.groups and users.students
	}

	// Migration 1.3.8: Create activities.student_enrollments table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createActivitiesStudentEnrollmentsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropActivitiesStudentEnrollmentsTable(ctx, db)
		},
	)
}

// createActivitiesStudentEnrollmentsTable creates the activities.student_enrollments table
func createActivitiesStudentEnrollmentsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.8: Creating activities.student_enrollments table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the student_enrollments table - for student participation in activities
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS activities.student_enrollments (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL,
			activity_group_id BIGINT NOT NULL,
			enrollment_date DATE NOT NULL DEFAULT CURRENT_DATE,
			attendance_status TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_student_enrollments_student FOREIGN KEY (student_id) 
				REFERENCES users.students(id) ON DELETE CASCADE,
			CONSTRAINT fk_student_enrollments_activity_group FOREIGN KEY (activity_group_id) 
				REFERENCES activities.groups(id) ON DELETE CASCADE,
			CONSTRAINT uk_student_activity_enrollment UNIQUE (student_id, activity_group_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating student_enrollments table: %w", err)
	}

	// Create indexes for student_enrollments
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_student_enrollments_student_id ON activities.student_enrollments(student_id);
		CREATE INDEX IF NOT EXISTS idx_student_enrollments_activity_group_id ON activities.student_enrollments(activity_group_id);
		CREATE INDEX IF NOT EXISTS idx_student_enrollments_date ON activities.student_enrollments(enrollment_date);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for student_enrollments table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for student_enrollments
		DROP TRIGGER IF EXISTS update_student_enrollments_updated_at ON activities.student_enrollments;
		CREATE TRIGGER update_student_enrollments_updated_at
		BEFORE UPDATE ON activities.student_enrollments
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating trigger for student_enrollments table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropActivitiesStudentEnrollmentsTable drops the activities.student_enrollments table
func dropActivitiesStudentEnrollmentsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.8: Removing activities.student_enrollments table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_student_enrollments_updated_at ON activities.student_enrollments;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for student_enrollments table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS activities.student_enrollments;
	`)
	if err != nil {
		return fmt.Errorf("error dropping activities.student_enrollments table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
