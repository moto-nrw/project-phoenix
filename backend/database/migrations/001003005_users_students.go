package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	UsersStudentsVersion     = "1.3.5" // Changed from 1.3.5 to resolve version conflict with IoT devices
	UsersStudentsDescription = "Users students table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersStudentsVersion] = &Migration{
		Version:     UsersStudentsVersion,
		Description: UsersStudentsDescription,
		DependsOn:   []string{"1.2.1", "1.2.7"}, // Depends on persons table AND groups table
	}

	// Migration 1.3.5: Users students table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersStudentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersStudentsDown(ctx, db)
		},
	)
}

// usersStudentsUp creates the users.students table
func usersStudentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.5: Creating users.students table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the students table - for students/children
	_, err = tx.ExecContext(ctx, `
		-- Create email format validation function if it doesn't exist
		CREATE OR REPLACE FUNCTION is_valid_email(email TEXT) RETURNS BOOLEAN AS $$
		BEGIN
			RETURN email ~* '^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$';
		END;
		$$ LANGUAGE plpgsql IMMUTABLE;

		-- Create phone format validation function if it doesn't exist
		CREATE OR REPLACE FUNCTION is_valid_phone(phone TEXT) RETURNS BOOLEAN AS $$
		BEGIN
			-- Validates international format +XX XXXXXXXX or local format with optional spaces/dashes
			RETURN phone ~* '^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$';
		END;
		$$ LANGUAGE plpgsql IMMUTABLE;

		CREATE TABLE IF NOT EXISTS users.students (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL UNIQUE,
			school_class TEXT NOT NULL,
			-- Boolean fields indicating current student location
			-- bus: Student is in the bus
			bus BOOLEAN NOT NULL DEFAULT FALSE,
			-- in_house: Student is inside the building
			in_house BOOLEAN NOT NULL DEFAULT FALSE,
			-- wc: Student is in bathroom
			wc BOOLEAN NOT NULL DEFAULT FALSE,
			-- school_yard: Student is in school yard
			school_yard BOOLEAN NOT NULL DEFAULT FALSE,
			guardian_name TEXT NOT NULL,
			-- Legacy field maintained for backward compatibility
			guardian_contact TEXT NOT NULL,
			-- New structured contact fields with validation
			guardian_email TEXT,
			guardian_phone TEXT,
			group_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_students_person FOREIGN KEY (person_id) 
				REFERENCES users.persons(id) ON DELETE CASCADE,
			CONSTRAINT fk_students_group FOREIGN KEY (group_id)
				REFERENCES education.groups(id) ON DELETE SET NULL,
			-- Email format validation
			CONSTRAINT chk_valid_guardian_email CHECK (
				guardian_email IS NULL OR is_valid_email(guardian_email)
			),
			-- Phone format validation
			CONSTRAINT chk_valid_guardian_phone CHECK (
				guardian_phone IS NULL OR is_valid_phone(guardian_phone)
			),
			-- Ensure only one location is set at a time
			CONSTRAINT chk_one_location_only CHECK (
				(bus::int + in_house::int + wc::int + school_yard::int) <= 1
			)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating students table: %w", err)
	}

	// Create indexes for students
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_students_person_id ON users.students(person_id);
		CREATE INDEX IF NOT EXISTS idx_students_school_class ON users.students(school_class);
		CREATE INDEX IF NOT EXISTS idx_students_group_id ON users.students(group_id);
		CREATE INDEX IF NOT EXISTS idx_students_guardian_email ON users.students(guardian_email);
		CREATE INDEX IF NOT EXISTS idx_students_guardian_phone ON users.students(guardian_phone);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for students table: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for students
		DROP TRIGGER IF EXISTS update_students_updated_at ON users.students;
		CREATE TRIGGER update_students_updated_at
		BEFORE UPDATE ON users.students
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersStudentsDown removes the users.students table
func usersStudentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.5: Removing users.students table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop the students table and validation functions
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.students;
		
		-- Only drop validation functions if no other objects depend on them
		DO $$
		BEGIN
			-- Check if any objects depend on is_valid_email function
			IF NOT EXISTS (
				SELECT 1 FROM pg_depend d
				JOIN pg_proc p ON d.objid = p.oid
				WHERE p.proname = 'is_valid_email' AND d.deptype = 'n'
				  AND d.objid <> d.refobjid  -- exclude self-references
			) THEN
				DROP FUNCTION IF EXISTS is_valid_email(TEXT);
			END IF;
			
			-- Check if any objects depend on is_valid_phone function
			IF NOT EXISTS (
				SELECT 1 FROM pg_depend d
				JOIN pg_proc p ON d.objid = p.oid
				WHERE p.proname = 'is_valid_phone' AND d.deptype = 'n'
				  AND d.objid <> d.refobjid  -- exclude self-references
			) THEN
				DROP FUNCTION IF EXISTS is_valid_phone(TEXT);
			END IF;
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.students table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
