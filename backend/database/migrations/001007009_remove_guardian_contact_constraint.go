package migrations

import (
	"context"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	RemoveGuardianContactConstraintVersion     = "1.7.9"
	RemoveGuardianContactConstraintDescription = "Remove check_contact_method constraint from guardian_profiles (phone numbers now in separate table)"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[RemoveGuardianContactConstraintVersion] = &Migration{
		Version:     RemoveGuardianContactConstraintVersion,
		Description: RemoveGuardianContactConstraintDescription,
		DependsOn:   []string{GuardianPhoneNumbersVersion}, // Depends on guardian_phone_numbers table (1.7.8)
	}

	// Migration 1.7.7: Remove check_contact_method constraint
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return removeGuardianContactConstraintUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeGuardianContactConstraintDown(ctx, db)
		},
	)
}

// removeGuardianContactConstraintUp removes the check_contact_method constraint
// This is needed because phone numbers are now stored in guardian_phone_numbers table
// and the guardian profile may be created first, then phone numbers added separately
func removeGuardianContactConstraintUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.7: Removing check_contact_method constraint from guardian_profiles...")

	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.guardian_profiles
		DROP CONSTRAINT IF EXISTS check_contact_method
	`)
	if err != nil {
		return fmt.Errorf("error dropping check_contact_method constraint: %w", err)
	}

	log.Println("Successfully removed check_contact_method constraint")
	return nil
}

// removeGuardianContactConstraintDown restores the check_contact_method constraint
func removeGuardianContactConstraintDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.7: Restoring check_contact_method constraint...")

	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.guardian_profiles
		ADD CONSTRAINT check_contact_method CHECK (
			email IS NOT NULL OR
			phone IS NOT NULL OR
			mobile_phone IS NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error restoring check_contact_method constraint: %w", err)
	}

	log.Println("Successfully restored check_contact_method constraint")
	return nil
}
