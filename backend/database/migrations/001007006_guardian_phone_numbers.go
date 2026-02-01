package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	GuardianPhoneNumbersVersion     = "1.7.6.1"
	GuardianPhoneNumbersDescription = "Create users.guardian_phone_numbers table for flexible phone storage"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[GuardianPhoneNumbersVersion] = &Migration{
		Version:     GuardianPhoneNumbersVersion,
		Description: GuardianPhoneNumbersDescription,
		DependsOn:   []string{"1.3.5.1"}, // Depends on guardian_profiles
	}

	// Migration 1.7.6: Create guardian_phone_numbers table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return guardianPhoneNumbersUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return guardianPhoneNumbersDown(ctx, db)
		},
	)
}

// guardianPhoneNumbersUp creates the users.guardian_phone_numbers table
func guardianPhoneNumbersUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.6: Creating users.guardian_phone_numbers table...")

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

	// Create the phone_type enum
	_, err = tx.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'phone_type' AND typnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'users')) THEN
				CREATE TYPE users.phone_type AS ENUM ('mobile', 'home', 'work', 'other');
			END IF;
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("error creating phone_type enum: %w", err)
	}

	// Create the guardian_phone_numbers table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.guardian_phone_numbers (
			id BIGSERIAL PRIMARY KEY,
			guardian_profile_id BIGINT NOT NULL,
			phone_number TEXT NOT NULL,
			phone_type users.phone_type NOT NULL DEFAULT 'mobile',
			label TEXT,
			is_primary BOOLEAN NOT NULL DEFAULT FALSE,
			priority INT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

			-- Foreign Key
			CONSTRAINT fk_guardian_phone_guardian_profile
				FOREIGN KEY (guardian_profile_id) REFERENCES users.guardian_profiles(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating guardian_phone_numbers table: %w", err)
	}

	// Create indexes
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_guardian_phone_guardian_id
			ON users.guardian_phone_numbers(guardian_profile_id);
		CREATE INDEX IF NOT EXISTS idx_guardian_phone_primary
			ON users.guardian_phone_numbers(guardian_profile_id, is_primary) WHERE is_primary;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for guardian_phone_numbers: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_guardian_phone_numbers_updated_at ON users.guardian_phone_numbers;
		CREATE TRIGGER update_guardian_phone_numbers_updated_at
		BEFORE UPDATE ON users.guardian_phone_numbers
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Migrate existing phone data from guardian_profiles
	// First: Migrate phone as home type (priority 1, primary if it's the only number)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users.guardian_phone_numbers (guardian_profile_id, phone_number, phone_type, is_primary, priority)
		SELECT id, phone, 'home'::users.phone_type, TRUE, 1
		FROM users.guardian_profiles
		WHERE phone IS NOT NULL AND phone != ''
	`)
	if err != nil {
		return fmt.Errorf("error migrating phone numbers (home): %w", err)
	}

	// Second: Migrate mobile_phone as mobile type
	// Set as primary only if there was no home phone migrated
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users.guardian_phone_numbers (guardian_profile_id, phone_number, phone_type, is_primary, priority)
		SELECT gp.id, gp.mobile_phone, 'mobile'::users.phone_type,
			NOT EXISTS (
				SELECT 1 FROM users.guardian_phone_numbers gpn
				WHERE gpn.guardian_profile_id = gp.id
			),
			2
		FROM users.guardian_profiles gp
		WHERE gp.mobile_phone IS NOT NULL AND gp.mobile_phone != ''
	`)
	if err != nil {
		return fmt.Errorf("error migrating phone numbers (mobile): %w", err)
	}

	// Create backward-compat trigger to sync new table -> old fields
	// This ensures existing code still works during transition period
	_, err = tx.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION users.sync_guardian_phone_to_legacy()
		RETURNS TRIGGER AS $$
		DECLARE
			primary_home TEXT;
			primary_mobile TEXT;
		BEGIN
			-- Get the primary home phone
			SELECT phone_number INTO primary_home
			FROM users.guardian_phone_numbers
			WHERE guardian_profile_id = COALESCE(NEW.guardian_profile_id, OLD.guardian_profile_id)
				AND phone_type = 'home'
			ORDER BY is_primary DESC, priority ASC
			LIMIT 1;

			-- Get the primary mobile phone
			SELECT phone_number INTO primary_mobile
			FROM users.guardian_phone_numbers
			WHERE guardian_profile_id = COALESCE(NEW.guardian_profile_id, OLD.guardian_profile_id)
				AND phone_type = 'mobile'
			ORDER BY is_primary DESC, priority ASC
			LIMIT 1;

			-- Update the legacy fields in guardian_profiles
			UPDATE users.guardian_profiles
			SET phone = primary_home,
				mobile_phone = primary_mobile
			WHERE id = COALESCE(NEW.guardian_profile_id, OLD.guardian_profile_id);

			RETURN COALESCE(NEW, OLD);
		END;
		$$ LANGUAGE plpgsql;

		DROP TRIGGER IF EXISTS sync_phone_to_legacy ON users.guardian_phone_numbers;
		CREATE TRIGGER sync_phone_to_legacy
		AFTER INSERT OR UPDATE OR DELETE ON users.guardian_phone_numbers
		FOR EACH ROW
		EXECUTE FUNCTION users.sync_guardian_phone_to_legacy();
	`)
	if err != nil {
		return fmt.Errorf("error creating backward-compat trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// guardianPhoneNumbersDown removes the users.guardian_phone_numbers table
func guardianPhoneNumbersDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.6: Removing users.guardian_phone_numbers table...")

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

	// Drop triggers first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_guardian_phone_numbers_updated_at ON users.guardian_phone_numbers;
		DROP TRIGGER IF EXISTS sync_phone_to_legacy ON users.guardian_phone_numbers;
		DROP FUNCTION IF EXISTS users.sync_guardian_phone_to_legacy();
	`)
	if err != nil {
		return fmt.Errorf("error dropping triggers: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.guardian_phone_numbers CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.guardian_phone_numbers table: %w", err)
	}

	// Drop the enum type
	_, err = tx.ExecContext(ctx, `
		DROP TYPE IF EXISTS users.phone_type;
	`)
	if err != nil {
		return fmt.Errorf("error dropping phone_type enum: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
