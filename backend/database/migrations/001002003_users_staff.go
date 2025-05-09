package migrations

import (
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
	defer tx.Rollback()

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

	// Modify the existing teachers and guests schema to reference staff
	// First, insert existing teachers and guests into the staff table
	_, err = tx.ExecContext(ctx, `
		-- Insert all persons from teachers as staff
		INSERT INTO users.staff (person_id)
		SELECT person_id FROM users.teachers
		ON CONFLICT (person_id) DO NOTHING;
		
		-- Insert all persons from guests as staff
		INSERT INTO users.staff (person_id)
		SELECT person_id FROM users.guests
		ON CONFLICT (person_id) DO NOTHING;
	`)
	if err != nil {
		return fmt.Errorf("error migrating existing data to staff table: %w", err)
	}

	// Now modify the teachers table to reference staff instead of directly to persons
	_, err = tx.ExecContext(ctx, `
		-- First create a temporary column
		ALTER TABLE users.teachers ADD COLUMN staff_id BIGINT;
		
		-- Update the staff_id column based on the person_id
		UPDATE users.teachers t
		SET staff_id = s.id
		FROM users.staff s
		WHERE t.person_id = s.person_id;
		
		-- Add foreign key constraint
		ALTER TABLE users.teachers
		ADD CONSTRAINT fk_teachers_staff
		FOREIGN KEY (staff_id) REFERENCES users.staff(id) ON DELETE CASCADE;
		
		-- Create index
		CREATE INDEX IF NOT EXISTS idx_teachers_staff_id ON users.teachers(staff_id);
		
		-- Make sure all rows have a valid staff_id
		ALTER TABLE users.teachers ALTER COLUMN staff_id SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error modifying teachers table: %w", err)
	}

	// Modify the guests table to reference staff instead of directly to persons
	_, err = tx.ExecContext(ctx, `
		-- First create a temporary column
		ALTER TABLE users.guests ADD COLUMN staff_id BIGINT;
		
		-- Update the staff_id column based on the person_id
		UPDATE users.guests g
		SET staff_id = s.id
		FROM users.staff s
		WHERE g.person_id = s.person_id;
		
		-- Add foreign key constraint
		ALTER TABLE users.guests
		ADD CONSTRAINT fk_guests_staff
		FOREIGN KEY (staff_id) REFERENCES users.staff(id) ON DELETE CASCADE;
		
		-- Create index
		CREATE INDEX IF NOT EXISTS idx_guests_staff_id ON users.guests(staff_id);
		
		-- Make sure all rows have a valid staff_id
		ALTER TABLE users.guests ALTER COLUMN staff_id SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error modifying guests table: %w", err)
	}

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
	defer tx.Rollback()

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
