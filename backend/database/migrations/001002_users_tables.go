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

	// 1. Create the users_persons table - base entity for all individuals
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.users_persons (
			id BIGSERIAL PRIMARY KEY,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			tag_id TEXT UNIQUE,
			account_id BIGINT UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_users_persons_account FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating users_persons table: %w", err)
	}

	// Create indexes for users_persons
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_users_persons_tag_id ON users.users_persons(tag_id);
		CREATE INDEX IF NOT EXISTS idx_users_persons_names ON users.users_persons(first_name, last_name);
		CREATE INDEX IF NOT EXISTS idx_users_persons_account_id ON users.users_persons(account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for users_persons table: %w", err)
	}

	// 2. Create the users_profiles table - user profile information
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.users_profiles (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			account_id BIGINT NOT NULL UNIQUE,
			avatar TEXT,
			bio TEXT,
			settings JSONB DEFAULT '{}'::jsonb,
			CONSTRAINT fk_users_profiles_account FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating users_profiles table: %w", err)
	}

	// Create indexes for users_profiles
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_users_profiles_account_id ON users.users_profiles(account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for users_profiles table: %w", err)
	}

	// 3. Create the users_teachers table - for pedagogical specialists
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.users_teachers (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL UNIQUE,
			specialization TEXT NOT NULL,
			role TEXT,
			is_password_otp BOOLEAN DEFAULT FALSE,
			qualifications TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_users_teachers_person FOREIGN KEY (person_id) 
				REFERENCES users.users_persons(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating users_teachers table: %w", err)
	}

	// Create indexes for users_teachers
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_users_teachers_person_id ON users.users_teachers(person_id);
		CREATE INDEX IF NOT EXISTS idx_users_teachers_specialization ON users.users_teachers(specialization);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for users_teachers table: %w", err)
	}

	// Create a separate table for guest instructors
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.users_guests (
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
			CONSTRAINT fk_users_guests_person FOREIGN KEY (person_id) 
				REFERENCES users.users_persons(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating users_guests table: %w", err)
	}

	// Create indexes for users_guests
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_users_guests_person_id ON users.users_guests(person_id);
		CREATE INDEX IF NOT EXISTS idx_users_guests_organization ON users.users_guests(organization);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for users_guests table: %w", err)
	}

	// 4. Create the users_students table - for students/children
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.users_students (
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
			CONSTRAINT fk_users_students_person FOREIGN KEY (person_id) 
				REFERENCES users.users_persons(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating users_students table: %w", err)
	}

	// Create indexes for users_students
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_users_students_person_id ON users.users_students(person_id);
		CREATE INDEX IF NOT EXISTS idx_users_students_school_class ON users.users_students(school_class);
		CREATE INDEX IF NOT EXISTS idx_users_students_group_id ON users.users_students(group_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for users_students table: %w", err)
	}

	// 5. Create the users_rfid_cards table - for physical tracking
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.users_rfid_cards (
			id TEXT PRIMARY KEY,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating users_rfid_cards table: %w", err)
	}

	// Create updated_at timestamp triggers
	_, err = tx.ExecContext(ctx, `
		-- Trigger for users_persons
		CREATE TRIGGER update_users_persons_updated_at
		BEFORE UPDATE ON users.users_persons
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for users_profiles
		CREATE TRIGGER update_users_profiles_updated_at
		BEFORE UPDATE ON users.users_profiles
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for users_teachers
		CREATE TRIGGER update_users_teachers_updated_at
		BEFORE UPDATE ON users.users_teachers
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for users_guests
		CREATE TRIGGER update_users_guests_updated_at
		BEFORE UPDATE ON users.users_guests
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for users_students
		CREATE TRIGGER update_users_students_updated_at
		BEFORE UPDATE ON users.users_students
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for users_rfid_cards
		CREATE TRIGGER update_users_rfid_cards_updated_at
		BEFORE UPDATE ON users.users_rfid_cards
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
		DROP TABLE IF EXISTS users.users_students;
		DROP TABLE IF EXISTS users.users_teachers;
		DROP TABLE IF EXISTS users.users_guests;
		DROP TABLE IF EXISTS users.users_rfid_cards;
		DROP TABLE IF EXISTS users.users_profiles;
		DROP TABLE IF EXISTS users.users_persons;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users schema tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
