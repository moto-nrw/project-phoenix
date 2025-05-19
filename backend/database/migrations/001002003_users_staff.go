package migrations

import (
	"log"
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	UsersStaffVersion     = "1.2.3"
	UsersStaffDescription = "Users staff table as intermediate layer"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersStaffVersion] = &Migration{
		Version:     UsersStaffVersion,
		Description: UsersStaffDescription,
		DependsOn:   []string{"1.2.1"}, // Depends on persons table
	}

	// Migration 1.2.3: Users staff table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersStaffUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersStaffDown(ctx, db)
		},
	)
}

// usersStaffUp creates the users.staff table and modifies teachers and guests tables
func usersStaffUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.2.3: Creating users.staff table...")

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

	// Create the staff table - base entity for all staff members
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.staff (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL UNIQUE,
			staff_notes TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_staff_person FOREIGN KEY (person_id) 
				REFERENCES users.persons(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating staff table: %w", err)
	}

	// Create indexes for staff
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_staff_person_id ON users.staff(person_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for staff table: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for staff
		DROP TRIGGER IF EXISTS update_staff_updated_at ON users.staff;
		CREATE TRIGGER update_staff_updated_at
		BEFORE UPDATE ON users.staff
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Create staff table only
	// We don't reference teachers or guests here since they don't exist yet
	// Teachers and guests tables will be created in subsequent migrations
	// No data to migrate at this stage

	// No need to modify teachers table here as it will be created in migration 1.2.4

	// No need to modify guests table here as it will be created in migration 1.2.5

	// Commit the transaction
	return tx.Commit()
}

// usersStaffDown removes the users.staff table and restores original structure
func usersStaffDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.2.3: Removing users.staff table...")

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

	// Drop the foreign keys and columns first
	_, err = tx.ExecContext(ctx, `
		-- Remove staff_id from teachers
		ALTER TABLE users.teachers DROP CONSTRAINT IF EXISTS fk_teachers_staff;
		DROP INDEX IF EXISTS idx_teachers_staff_id;
		ALTER TABLE users.teachers DROP COLUMN IF EXISTS staff_id;
		
		-- Remove staff_id from guests
		ALTER TABLE users.guests DROP CONSTRAINT IF EXISTS fk_guests_staff;
		DROP INDEX IF EXISTS idx_guests_staff_id;
		ALTER TABLE users.guests DROP COLUMN IF EXISTS staff_id;
	`)
	if err != nil {
		return fmt.Errorf("error restoring original table structure: %w", err)
	}

	// Drop the staff table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.staff;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.staff table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
