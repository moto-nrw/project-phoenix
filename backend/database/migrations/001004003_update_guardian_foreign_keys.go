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

// updateGuardianForeignKeysUp removes legacy tables and columns
func updateGuardianForeignKeysUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.3: Removing legacy tables and columns...")

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

	// Step 2: Drop legacy columns from students table
	// Note: Migration 1.3.6 already creates students_guardians with correct structure
	// (guardian_id column, FK to users.guardians, correct indexes)
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

	// Recreate persons_guardians table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.persons_guardians (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL,
			guardian_id BIGINT NOT NULL,
			relationship_type TEXT NOT NULL,
			is_primary BOOLEAN NOT NULL DEFAULT FALSE,
			permissions JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_persons_guardians_person FOREIGN KEY (person_id) REFERENCES users.persons(id) ON DELETE CASCADE,
			CONSTRAINT fk_persons_guardians_guardian FOREIGN KEY (guardian_id) REFERENCES users.guardians(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error recreating persons_guardians table: %w", err)
	}

	return tx.Commit()
}
