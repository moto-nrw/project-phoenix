package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	CreateFirstOgsVersion     = "1.7.7"
	CreateFirstOgsDescription = "Create first OGS organization for existing data backfill"
)

// firstOgsId is a deterministic ID for the first organization.
// Using a fixed ID makes the migration idempotent and allows other migrations
// to reference it reliably. This will be the default organization for all
// existing data that predates multi-tenancy.
const firstOgsId = "first-ogs-organization"

func init() {
	MigrationRegistry[CreateFirstOgsVersion] = &Migration{
		Version:     CreateFirstOgsVersion,
		Description: CreateFirstOgsDescription,
		DependsOn:   []string{"1.7.6"}, // Depends on ogs_id columns
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createFirstOgs(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeFirstOgs(ctx, db)
		},
	)
}

// createFirstOgs creates the first OGS organization in the BetterAuth organization table.
// This organization serves as the default tenant for all pre-existing data.
// Note: traegerId is set to a placeholder since tenant.traeger table is created in WP4.
func createFirstOgs(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.7: Creating first OGS organization...")

	// Check if the organization table exists (BetterAuth should have created it)
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'organization'
		)
	`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking organization table: %w", err)
	}

	if !exists {
		return fmt.Errorf("public.organization table does not exist - BetterAuth service must be initialized first (WP1)")
	}

	// Check if any organization already exists
	var count int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM public.organization`).Scan(&count)
	if err != nil {
		return fmt.Errorf("error counting organizations: %w", err)
	}

	if count > 0 {
		fmt.Println("  Organization(s) already exist, skipping creation")

		// Get the first organization by creation date (for logging)
		var existingId string
		err = db.QueryRowContext(ctx, `
			SELECT id FROM public.organization
			ORDER BY "createdAt"
			LIMIT 1
		`).Scan(&existingId)
		if err != nil {
			return fmt.Errorf("error getting existing organization: %w", err)
		}

		fmt.Printf("  Using existing organization: %s\n", existingId)
		return nil
	}

	// Create the first OGS organization
	// Note: traegerId is a placeholder - the tenant.traeger table is created in WP4.
	// Once WP4 is complete, this can be updated to reference a real träger.
	_, err = db.ExecContext(ctx, `
		INSERT INTO public.organization (id, name, slug, "createdAt", "traegerId", metadata)
		VALUES (
			$1,
			'Erste OGS (Migration)',
			'erste-ogs',
			NOW(),
			'migration-placeholder-traeger',
			'{"source": "migration-1.7.7", "note": "Default organization for pre-existing data"}'
		)
		ON CONFLICT (id) DO NOTHING
	`, firstOgsId)
	if err != nil {
		return fmt.Errorf("error creating first OGS organization: %w", err)
	}

	fmt.Printf("  Created first OGS organization with ID: %s\n", firstOgsId)
	fmt.Println("Migration 1.7.7: Successfully created first OGS organization")
	return nil
}

// removeFirstOgs removes the first OGS organization.
// This should only be run if the organization was created by this migration
// and no data references it.
func removeFirstOgs(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.7: Removing first OGS organization...")

	// Only remove if it's the migration-created org with placeholder traeger
	result, err := db.ExecContext(ctx, `
		DELETE FROM public.organization
		WHERE id = $1
		AND "traegerId" = 'migration-placeholder-traeger'
	`, firstOgsId)
	if err != nil {
		return fmt.Errorf("error removing first OGS organization: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		fmt.Println("  Removed migration-created organization")
	} else {
		fmt.Println("  No migration-created organization found to remove (may have been updated)")
	}

	fmt.Println("Migration 1.7.7: Rollback complete")
	return nil
}
