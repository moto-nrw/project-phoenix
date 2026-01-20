package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	CreateRLSFunctionVersion     = "1.8.2"
	CreateRLSFunctionDescription = "Create current_ogs_id() function for RLS policies"
)

func init() {
	MigrationRegistry[CreateRLSFunctionVersion] = &Migration{
		Version:     CreateRLSFunctionVersion,
		Description: CreateRLSFunctionDescription,
		DependsOn:   []string{"1.8.1"}, // After seed_first_traeger
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createRLSFunction(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropRLSFunction(ctx, db)
		},
	)
}

// createRLSFunction creates the current_ogs_id() function used by RLS policies.
// This function reads the app.ogs_id session variable set by tenant middleware.
// Returns a non-matching UUID if not set (ensures no data leakage).
func createRLSFunction(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.2: Creating current_ogs_id() function for RLS...")

	// Create function that reads app.ogs_id session variable.
	// Returns '00000000-0000-0000-0000-000000000000' if not set (matches nothing).
	// This is a security feature - ensures queries return empty results rather
	// than all data if the tenant context is not properly set.
	//
	// IMPORTANT: After transaction ends, PostgreSQL returns '' not NULL,
	// so we must check for both conditions.
	_, err := db.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION current_ogs_id() RETURNS TEXT AS $$
		DECLARE
			ogs_id TEXT;
		BEGIN
			-- Get the session variable, returns NULL if not set
			-- The second parameter (true) means return NULL if not set instead of error
			ogs_id := current_setting('app.ogs_id', true);

			-- If not set OR empty string, return a UUID that will never match real data
			-- This ensures queries return empty results rather than all data
			-- The '' check is critical because PostgreSQL resets to '' after transaction
			IF ogs_id IS NULL OR ogs_id = '' THEN
				RETURN '00000000-0000-0000-0000-000000000000';
			END IF;

			RETURN ogs_id;
		END;
		$$ LANGUAGE plpgsql STABLE;

		COMMENT ON FUNCTION current_ogs_id() IS
			'Returns the current tenant OGS ID from app.ogs_id session variable.
			 Used by RLS policies for tenant isolation. Returns a non-matching UUID if not set.
			 Set via: SET LOCAL app.ogs_id = ''org-id''; (within transaction)';
	`)
	if err != nil {
		return fmt.Errorf("error creating current_ogs_id function: %w", err)
	}

	fmt.Println("Migration 1.8.2: Successfully created current_ogs_id() function")
	return nil
}

// dropRLSFunction removes the current_ogs_id() function.
// CASCADE ensures any dependent policies are also dropped.
func dropRLSFunction(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.2: Dropping current_ogs_id() function...")

	_, err := db.ExecContext(ctx, `DROP FUNCTION IF EXISTS current_ogs_id() CASCADE`)
	if err != nil {
		return fmt.Errorf("error dropping current_ogs_id function: %w", err)
	}

	fmt.Println("Migration 1.8.2: Successfully dropped current_ogs_id() function")
	return nil
}
