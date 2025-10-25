package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	UpdateGuardianForeignKeysVersion     = "1.4.3"
	UpdateGuardianForeignKeysDescription = "Update students_guardians FK and drop legacy columns"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UpdateGuardianForeignKeysVersion] = &Migration{
		Version:     UpdateGuardianForeignKeysVersion,
		Description: UpdateGuardianForeignKeysDescription,
		DependsOn:   []string{"1.4.2"}, // Depends on data migration
	}

	// Migration 1.4.3: Update foreign keys and remove legacy columns
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return updateGuardianForeignKeysUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return updateGuardianForeignKeysDown(ctx, db)
		},
	)
}

// updateGuardianForeignKeysUp updates the foreign key references
func updateGuardianForeignKeysUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.3: Updating foreign keys and removing legacy columns...")

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

	// Step 1: Drop persons_guardians table (no longer needed)
	fmt.Println("  - Dropping users.persons_guardians table...")
	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS users.persons_guardians CASCADE`)
	if err != nil {
		return fmt.Errorf("error dropping users.persons_guardians table: %w", err)
	}

	// Step 2: Rename column in students_guardians table
	fmt.Println("  - Renaming guardian_account_id to guardian_id...")
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students_guardians
		RENAME COLUMN guardian_account_id TO guardian_id
	`)
	if err != nil {
		// Check if column already renamed (idempotent)
		count, checkErr := tx.NewSelect().
			TableExpr("information_schema.columns").
			Where("table_schema = 'users'").
			Where("table_name = 'students_guardians'").
			Where("column_name = 'guardian_id'").
			Count(ctx)

		if checkErr == nil && count > 0 {
			fmt.Println("  - Column already renamed, skipping...")
		} else {
			return fmt.Errorf("error renaming column: %w", err)
		}
	}

	// Step 3: Drop old foreign key constraint
	fmt.Println("  - Updating foreign key constraint...")
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students_guardians
		DROP CONSTRAINT IF EXISTS fk_students_guardians_guardian
	`)
	if err != nil {
		log.Printf("Warning: Could not drop old constraint (may not exist): %v", err)
	}

	// Step 4: Add new foreign key constraint
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students_guardians
		ADD CONSTRAINT fk_students_guardians_guardian
		FOREIGN KEY (guardian_id) REFERENCES users.guardians(id) ON DELETE CASCADE
	`)
	if err != nil {
		return fmt.Errorf("error adding new foreign key constraint: %w", err)
	}

	// Step 5: Update indexes
	fmt.Println("  - Updating indexes...")
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_students_guardians_guardian_account_id;
		CREATE INDEX IF NOT EXISTS idx_students_guardians_guardian_id ON users.students_guardians(guardian_id);
	`)
	if err != nil {
		return fmt.Errorf("error updating indexes: %w", err)
	}

	// Step 6: Update trigger function to use new column name
	fmt.Println("  - Updating trigger function...")
	_, err = tx.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION users.enforce_single_primary_student_guardian()
		RETURNS TRIGGER AS $$
		BEGIN
			IF NEW.is_primary = TRUE THEN
				UPDATE users.students_guardians
				SET is_primary = FALSE
				WHERE student_id = NEW.student_id
				AND id != NEW.id;
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		return fmt.Errorf("error updating trigger function: %w", err)
	}

	// Step 7: Drop legacy columns from students table
	fmt.Println("  - Dropping legacy guardian columns from students table...")
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students
		DROP COLUMN IF EXISTS guardian_name,
		DROP COLUMN IF EXISTS guardian_contact,
		DROP COLUMN IF EXISTS guardian_email,
		DROP COLUMN IF EXISTS guardian_phone
	`)
	if err != nil {
		return fmt.Errorf("error dropping legacy columns from students table: %w", err)
	}

	fmt.Println("  - Migration completed successfully!")

	// Commit the transaction
	return tx.Commit()
}

// updateGuardianForeignKeysDown rolls back the migration
func updateGuardianForeignKeysDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.3...")
	fmt.Println("WARNING: This rollback will require manual intervention to restore data")

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

	// Add back legacy columns to students table
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS guardian_name TEXT,
		ADD COLUMN IF NOT EXISTS guardian_contact TEXT,
		ADD COLUMN IF NOT EXISTS guardian_email TEXT,
		ADD COLUMN IF NOT EXISTS guardian_phone TEXT
	`)
	if err != nil {
		return fmt.Errorf("error adding back legacy columns: %w", err)
	}

	// Rename column back
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students_guardians
		RENAME COLUMN guardian_id TO guardian_account_id
	`)
	if err != nil {
		return fmt.Errorf("error renaming column back: %w", err)
	}

	// Update foreign key constraint back to accounts_parents
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students_guardians
		DROP CONSTRAINT IF EXISTS fk_students_guardians_guardian;

		ALTER TABLE users.students_guardians
		ADD CONSTRAINT fk_students_guardians_guardian
		FOREIGN KEY (guardian_account_id) REFERENCES auth.accounts_parents(id) ON DELETE CASCADE
	`)
	if err != nil {
		return fmt.Errorf("error updating foreign key constraint: %w", err)
	}

	// Recreate persons_guardians table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.persons_guardians (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL,
			guardian_account_id BIGINT NOT NULL,
			relationship_type TEXT NOT NULL,
			is_primary BOOLEAN NOT NULL DEFAULT FALSE,
			permissions JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_persons_guardians_person FOREIGN KEY (person_id) REFERENCES users.persons(id) ON DELETE CASCADE,
			CONSTRAINT fk_persons_guardians_guardian FOREIGN KEY (guardian_account_id) REFERENCES auth.accounts_parents(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error recreating persons_guardians table: %w", err)
	}

	return tx.Commit()
}
