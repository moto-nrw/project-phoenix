package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AddBetterAuthUserIDVersion     = "1.8.6"
	AddBetterAuthUserIDDescription = "Add betterauth_user_id column to users.staff for BetterAuth linkage"
)

func init() {
	MigrationRegistry[AddBetterAuthUserIDVersion] = &Migration{
		Version:     AddBetterAuthUserIDVersion,
		Description: AddBetterAuthUserIDDescription,
		DependsOn:   []string{"1.8.5"}, // After ogs_id trigger
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addBetterAuthUserID(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropBetterAuthUserID(ctx, db)
		},
	)
}

// addBetterAuthUserID adds a column to link staff records to BetterAuth users.
//
// This enables the Go backend to:
// 1. Validate BetterAuth session and get user_id
// 2. Look up the corresponding staff record via this column
// 3. Load permissions and apply authorization rules
//
// The column is nullable because:
// - Existing staff may not yet have BetterAuth accounts
// - Staff can be created before their BetterAuth invitation is accepted
// - Migration must not fail on existing data
func addBetterAuthUserID(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.6: Adding betterauth_user_id to users.staff...")

	// Add nullable column (TEXT to match BetterAuth's UUID format)
	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.staff
		ADD COLUMN IF NOT EXISTS betterauth_user_id TEXT
	`)
	if err != nil {
		return fmt.Errorf("error adding betterauth_user_id column: %w", err)
	}
	fmt.Println("  Added betterauth_user_id column")

	// Add unique constraint (each BetterAuth user maps to exactly one staff)
	_, err = db.ExecContext(ctx, `
		ALTER TABLE users.staff
		ADD CONSTRAINT staff_betterauth_user_id_unique UNIQUE (betterauth_user_id)
	`)
	if err != nil {
		// Constraint may already exist from a previous partial run
		fmt.Printf("  Note: Unique constraint may already exist: %v\n", err)
	} else {
		fmt.Println("  Added unique constraint on betterauth_user_id")
	}

	// Add index for fast lookups during session validation
	_, err = db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_staff_betterauth_user_id
		ON users.staff(betterauth_user_id)
		WHERE betterauth_user_id IS NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("error creating betterauth_user_id index: %w", err)
	}
	fmt.Println("  Created partial index on betterauth_user_id")

	// Add comment for documentation
	_, err = db.ExecContext(ctx, `
		COMMENT ON COLUMN users.staff.betterauth_user_id IS
			'Links this staff record to a BetterAuth user account.
			 Used by Go backend to resolve staff from BetterAuth session.
			 NULL until the staff member accepts their invitation and creates an account.'
	`)
	if err != nil {
		return fmt.Errorf("error adding column comment: %w", err)
	}

	fmt.Println("Migration 1.8.6: Successfully added betterauth_user_id to users.staff")
	return nil
}

// dropBetterAuthUserID removes the betterauth_user_id column.
func dropBetterAuthUserID(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.6: Removing betterauth_user_id from users.staff...")

	// Drop index first
	_, _ = db.ExecContext(ctx, `DROP INDEX IF EXISTS users.idx_staff_betterauth_user_id`)
	fmt.Println("  Dropped index idx_staff_betterauth_user_id")

	// Drop unique constraint
	_, _ = db.ExecContext(ctx, `
		ALTER TABLE users.staff
		DROP CONSTRAINT IF EXISTS staff_betterauth_user_id_unique
	`)
	fmt.Println("  Dropped unique constraint")

	// Drop column
	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.staff
		DROP COLUMN IF EXISTS betterauth_user_id
	`)
	if err != nil {
		return fmt.Errorf("error dropping betterauth_user_id column: %w", err)
	}
	fmt.Println("  Dropped betterauth_user_id column")

	fmt.Println("Migration 1.8.6: Successfully removed betterauth_user_id from users.staff")
	return nil
}
