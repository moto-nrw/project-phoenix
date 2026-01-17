package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/uptrace/bun"
)

const (
	UsersStudentsGuardiansVersion     = "1.3.6"
	UsersStudentsGuardiansDescription = "Link students directly to guardians"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersStudentsGuardiansVersion] = &Migration{
		Version:     UsersStudentsGuardiansVersion,
		Description: UsersStudentsGuardiansDescription,
		DependsOn:   []string{"1.3.5", "1.3.5.1"}, // Depends on students and guardian_profiles tables
	}

	// Migration 1.3.6: Students to guardians relationship
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersStudentsGuardiansUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersStudentsGuardiansDown(ctx, db)
		},
	)
}

// usersStudentsGuardiansUp creates the relationship table between students and guardians
func usersStudentsGuardiansUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.6: Creating users.students_guardians relationship table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logger.Logger.Warnf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the students_guardians table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.students_guardians (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL,
			guardian_profile_id BIGINT NOT NULL,
			relationship_type TEXT NOT NULL, -- e.g., 'parent', 'guardian', 'relative', 'other'
			is_primary BOOLEAN NOT NULL DEFAULT FALSE,
			is_emergency_contact BOOLEAN NOT NULL DEFAULT FALSE,
			can_pickup BOOLEAN NOT NULL DEFAULT FALSE,
			pickup_notes TEXT,
			emergency_priority INT DEFAULT 1,
			permissions JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_students_guardians_student FOREIGN KEY (student_id)
				REFERENCES users.students(id) ON DELETE CASCADE,
			CONSTRAINT fk_students_guardians_guardian FOREIGN KEY (guardian_profile_id)
				REFERENCES users.guardian_profiles(id) ON DELETE CASCADE,
			CONSTRAINT unique_student_guardian UNIQUE (student_id, guardian_profile_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating students_guardians table: %w", err)
	}

	// Create indexes for students_guardians
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_students_guardians_student_id ON users.students_guardians(student_id);
		CREATE INDEX IF NOT EXISTS idx_students_guardians_guardian_profile_id ON users.students_guardians(guardian_profile_id);
		CREATE INDEX IF NOT EXISTS idx_students_guardians_relationship_type ON users.students_guardians(relationship_type);
		CREATE INDEX IF NOT EXISTS idx_students_guardians_is_primary ON users.students_guardians(is_primary);
		CREATE INDEX IF NOT EXISTS idx_students_guardians_is_emergency_contact ON users.students_guardians(is_emergency_contact);
		CREATE INDEX IF NOT EXISTS idx_students_guardians_can_pickup ON users.students_guardians(can_pickup);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for students_guardians table: %w", err)
	}

	// Create trigger that enforces only one primary guardian per student
	_, err = tx.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION users.enforce_single_primary_student_guardian()
		RETURNS TRIGGER AS $$
		BEGIN
			-- If this is being set as primary
			IF NEW.is_primary = TRUE THEN
				-- Set any other relationships for this student to non-primary
				UPDATE users.students_guardians
				SET is_primary = FALSE
				WHERE student_id = NEW.student_id
				AND id != NEW.id;
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		DROP TRIGGER IF EXISTS enforce_single_primary_student_guardian_trigger ON users.students_guardians;
		CREATE TRIGGER enforce_single_primary_student_guardian_trigger
		BEFORE INSERT OR UPDATE OF is_primary ON users.students_guardians
		FOR EACH ROW
		WHEN (NEW.is_primary = TRUE)
		EXECUTE FUNCTION users.enforce_single_primary_student_guardian();
	`)
	if err != nil {
		return fmt.Errorf("error creating single primary guardian trigger: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for students_guardians
		DROP TRIGGER IF EXISTS update_students_guardians_updated_at ON users.students_guardians;
		CREATE TRIGGER update_students_guardians_updated_at
		BEFORE UPDATE ON users.students_guardians
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersStudentsGuardiansDown removes the users.students_guardians relationship table
func usersStudentsGuardiansDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.6: Removing users.students_guardians relationship table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logger.Logger.Warnf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop the triggers first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS enforce_single_primary_student_guardian_trigger ON users.students_guardians;
		DROP TRIGGER IF EXISTS update_students_guardians_updated_at ON users.students_guardians;
		DROP FUNCTION IF EXISTS users.enforce_single_primary_student_guardian();
	`)
	if err != nil {
		return fmt.Errorf("error dropping triggers and functions: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.students_guardians CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.students_guardians table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
