package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	UsersTablesVersion     = "1.2.0"
	UsersTablesDescription = "Users schema tables"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersTablesVersion] = &Migration{
		Version:     UsersTablesVersion,
		Description: UsersTablesDescription,
		DependsOn:   []string{"1.0.0"}, // Depends on infrastructure
	}

	// Migration 1.2.0: Users schema tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersTablesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersTablesDown(ctx, db)
		},
	)
}

// usersTablesUp creates the users schema tables
func usersTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.2.0: Creating users schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the persons table - base entity for all individuals
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.persons (
			id BIGSERIAL PRIMARY KEY,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			tag_id TEXT UNIQUE,
			account_id BIGINT UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_persons_account FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating persons table: %w", err)
	}

	// Create indexes for persons
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_persons_tag_id ON users.persons(tag_id);
		CREATE INDEX IF NOT EXISTS idx_persons_names ON users.persons(first_name, last_name);
		CREATE INDEX IF NOT EXISTS idx_persons_account_id ON users.persons(account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for persons table: %w", err)
	}

	// 2. Create the profiles table - user profile information
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.profiles (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			account_id BIGINT NOT NULL UNIQUE,
			avatar TEXT,
			bio TEXT,
			settings JSONB DEFAULT '{}'::jsonb,
			CONSTRAINT fk_profiles_account FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating profiles table: %w", err)
	}

	// Create indexes for profiles
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_profiles_account_id ON users.profiles(account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for profiles table: %w", err)
	}

	// 3. Create the teachers table - for pedagogical specialists
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.teachers (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL UNIQUE,
			specialization TEXT NOT NULL,
			role TEXT,
			is_password_otp BOOLEAN DEFAULT FALSE,
			qualifications TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_teachers_person FOREIGN KEY (person_id) 
				REFERENCES users.persons(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating teachers table: %w", err)
	}

	// Create indexes for teachers
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_teachers_person_id ON users.teachers(person_id);
		CREATE INDEX IF NOT EXISTS idx_teachers_specialization ON users.teachers(specialization);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for teachers table: %w", err)
	}

	// Create a separate table for guest instructors
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.guests (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL UNIQUE,
			organization TEXT,
			contact_email TEXT,
			contact_phone TEXT,
			activity_expertise TEXT NOT NULL,
			start_date DATE,
			end_date DATE,
			notes TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_guests_person FOREIGN KEY (person_id) 
				REFERENCES users.persons(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating guests table: %w", err)
	}

	// Create indexes for guests
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_guests_person_id ON users.guests(person_id);
		CREATE INDEX IF NOT EXISTS idx_guests_organization ON users.guests(organization);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for guests table: %w", err)
	}

	// 4. Create the students table - for students/children
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.students (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL UNIQUE,
			school_class TEXT NOT NULL,
			bus BOOLEAN NOT NULL DEFAULT FALSE,
			guardian_name TEXT NOT NULL,
			guardian_contact TEXT NOT NULL,
			in_house BOOLEAN NOT NULL DEFAULT FALSE,
			wc BOOLEAN NOT NULL DEFAULT FALSE,
			school_yard BOOLEAN NOT NULL DEFAULT FALSE,
			group_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_students_person FOREIGN KEY (person_id) 
				REFERENCES users.persons(id) ON DELETE CASCADE
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
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for students table: %w", err)
	}

	// 5. Create the rfid_cards table - for physical tracking
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.rfid_cards (
			id TEXT PRIMARY KEY,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating rfid_cards table: %w", err)
	}

	// Create updated_at timestamp triggers
	_, err = tx.ExecContext(ctx, `
		-- Trigger for persons
		CREATE TRIGGER update_persons_updated_at
		BEFORE UPDATE ON users.persons
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for profiles
		CREATE TRIGGER update_profiles_updated_at
		BEFORE UPDATE ON users.profiles
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for teachers
		CREATE TRIGGER update_teachers_updated_at
		BEFORE UPDATE ON users.teachers
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for guests
		CREATE TRIGGER update_guests_updated_at
		BEFORE UPDATE ON users.guests
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for students
		CREATE TRIGGER update_students_updated_at
		BEFORE UPDATE ON users.students
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for rfid_cards
		CREATE TRIGGER update_rfid_cards_updated_at
		BEFORE UPDATE ON users.rfid_cards
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersTablesDown removes the users schema tables
func usersTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.2.0: Removing users schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.students;
		DROP TABLE IF EXISTS users.teachers;
		DROP TABLE IF EXISTS users.guests;
		DROP TABLE IF EXISTS users.rfid_cards;
		DROP TABLE IF EXISTS users.profiles;
		DROP TABLE IF EXISTS users.persons;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users schema tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
