package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	makeGuardianContactsOptionalVersion     = "1.6.12"
	makeGuardianContactsOptionalDescription = "Make guardian email and phone optional (only name and relationship required)"
)

var makeGuardianContactsOptionalDependencies = []string{
	"1.6.11",
}

func init() {
	// Register migration metadata
	MigrationRegistry[makeGuardianContactsOptionalVersion] = &Migration{
		Version:     makeGuardianContactsOptionalVersion,
		Description: makeGuardianContactsOptionalDescription,
		DependsOn:   makeGuardianContactsOptionalDependencies,
	}

	// Register migration functions
	Migrations.MustRegister(
		makeGuardianContactsOptionalUp,
		makeGuardianContactsOptionalDown,
	)
}

// makeGuardianContactsOptionalUp makes email and phone optional in guardians table
// Only first_name, last_name are required - contact info is optional for pickup authorization
func makeGuardianContactsOptionalUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Running migration 1.6.12: Making guardian email and phone optional...")

	// Begin transaction
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("error beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Make email nullable and remove UNIQUE constraint
	// We'll recreate the unique constraint to allow nulls
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardians
		ALTER COLUMN email DROP NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error making email nullable: %w", err)
	}

	// Drop the existing UNIQUE constraint on email
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardians
		DROP CONSTRAINT IF EXISTS guardians_email_key;
	`)
	if err != nil {
		return fmt.Errorf("error dropping email unique constraint: %w", err)
	}

	// Create a partial unique index that only enforces uniqueness for non-NULL emails
	_, err = tx.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_guardians_email_unique
		ON users.guardians(email)
		WHERE email IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating partial unique index on email: %w", err)
	}

	// Drop the email format check constraint (we'll re-add it with null handling)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardians
		DROP CONSTRAINT IF EXISTS chk_email_format;
	`)
	if err != nil {
		return fmt.Errorf("error dropping email format constraint: %w", err)
	}

	// Re-add email format check that allows NULL
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardians
		ADD CONSTRAINT chk_email_format
		CHECK (email IS NULL OR email ~* '`+EmailValidationRegex+`');
	`)
	if err != nil {
		return fmt.Errorf("error adding email format constraint: %w", err)
	}

	// Make phone nullable
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardians
		ALTER COLUMN phone DROP NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error making phone nullable: %w", err)
	}

	// Add check constraint to ensure at least first_name and last_name are present
	// (Email and phone are now optional - guardians can be for pickup authorization only)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardians
		ADD CONSTRAINT chk_guardian_required_fields
		CHECK (
			first_name IS NOT NULL AND TRIM(first_name) != '' AND
			last_name IS NOT NULL AND TRIM(last_name) != ''
		);
	`)
	if err != nil {
		return fmt.Errorf("error adding required fields constraint: %w", err)
	}

	return tx.Commit()
}

// makeGuardianContactsOptionalDown reverts the changes
func makeGuardianContactsOptionalDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.12: Reverting guardian contact fields to required...")

	// Begin transaction
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("error beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// WARNING: This will fail if there are guardians with NULL email or phone
	// Set default values for NULL contacts before making them NOT NULL
	_, err = tx.ExecContext(ctx, `
		UPDATE users.guardians
		SET email = 'noemail' || id || '@placeholder.local'
		WHERE email IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error setting default emails: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE users.guardians
		SET phone = '+49000000' || LPAD(id::TEXT, 4, '0')
		WHERE phone IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error setting default phones: %w", err)
	}

	// Drop the partial unique index
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_guardians_email_unique;
	`)
	if err != nil {
		return fmt.Errorf("error dropping partial unique index: %w", err)
	}

	// Make email NOT NULL again
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardians
		ALTER COLUMN email SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error making email NOT NULL: %w", err)
	}

	// Add back UNIQUE constraint on email
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardians
		ADD CONSTRAINT guardians_email_key UNIQUE (email);
	`)
	if err != nil {
		return fmt.Errorf("error adding email unique constraint: %w", err)
	}

	// Make phone NOT NULL again
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardians
		ALTER COLUMN phone SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error making phone NOT NULL: %w", err)
	}

	// Drop the required fields constraint
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardians
		DROP CONSTRAINT IF EXISTS chk_guardian_required_fields;
	`)
	if err != nil {
		return fmt.Errorf("error dropping required fields constraint: %w", err)
	}

	return tx.Commit()
}
