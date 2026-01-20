package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	BackfillOgsIdVersion     = "1.7.8"
	BackfillOgsIdDescription = "Backfill ogs_id for all existing data"
)

func init() {
	MigrationRegistry[BackfillOgsIdVersion] = &Migration{
		Version:     BackfillOgsIdVersion,
		Description: BackfillOgsIdDescription,
		DependsOn:   []string{"1.7.7"}, // Depends on first OGS creation
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return backfillOgsId(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return clearBackfilledOgsId(ctx, db)
		},
	)
}

// backfillOgsId updates all existing records to use the first OGS organization ID.
// This ensures all pre-existing data belongs to a tenant before making ogs_id NOT NULL.
func backfillOgsId(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.8: Backfilling ogs_id for existing data...")

	// Get the first organization ID (created in 1.7.7 or existing)
	var firstOgsId string
	err := db.QueryRowContext(ctx, `
		SELECT id FROM public.organization
		ORDER BY "createdAt"
		LIMIT 1
	`).Scan(&firstOgsId)
	if err != nil {
		return fmt.Errorf("error getting first OGS ID (ensure migration 1.7.7 ran successfully): %w", err)
	}

	fmt.Printf("  Using OGS ID for backfill: %s\n", firstOgsId)

	// Tables to backfill (same as ogsIdTables from 1.7.6)
	tables := []string{
		"users.persons",
		"users.students",
		"users.staff",
		"users.teachers",
		"education.groups",
		"facilities.rooms",
		"iot.devices",
		"active.visits",
		"active.groups",
		"activities.groups",
		"activities.categories",
	}

	totalBackfilled := int64(0)
	for _, table := range tables {
		// Use string concatenation for the value since fmt.Sprintf and $1 don't mix
		// The firstOgsId is a known safe value from the database
		query := fmt.Sprintf(`UPDATE %s SET ogs_id = '%s' WHERE ogs_id IS NULL`, table, firstOgsId)
		result, err := db.ExecContext(ctx, query)
		if err != nil {
			return fmt.Errorf("error backfilling ogs_id in %s: %w", table, err)
		}

		rowsAffected, _ := result.RowsAffected()
		totalBackfilled += rowsAffected
		fmt.Printf("  Backfilled %d rows in %s\n", rowsAffected, table)
	}

	fmt.Printf("Migration 1.7.8: Successfully backfilled %d total rows across all tables\n", totalBackfilled)
	return nil
}

// clearBackfilledOgsId sets ogs_id back to NULL for all records.
// This is the inverse of backfill - used when rolling back migrations.
// Note: This will fail if ogs_id has been made NOT NULL (1.7.9).
// Roll back 1.7.9 first before rolling back this migration.
func clearBackfilledOgsId(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.8: Clearing backfilled ogs_id values...")

	// Get the first organization ID to only clear records we backfilled
	var firstOgsId string
	err := db.QueryRowContext(ctx, `
		SELECT id FROM public.organization
		ORDER BY "createdAt"
		LIMIT 1
	`).Scan(&firstOgsId)
	if err != nil {
		fmt.Println("  Warning: Could not find first OGS ID, clearing all ogs_id values")
		firstOgsId = ""
	}

	tables := []string{
		"users.persons",
		"users.students",
		"users.staff",
		"users.teachers",
		"education.groups",
		"facilities.rooms",
		"iot.devices",
		"active.visits",
		"active.groups",
		"activities.groups",
		"activities.categories",
	}

	for _, table := range tables {
		var query string
		if firstOgsId != "" {
			// Only clear records that were backfilled with the first OGS ID
			query = fmt.Sprintf(`UPDATE %s SET ogs_id = NULL WHERE ogs_id = '%s'`, table, firstOgsId)
		} else {
			// Fallback: clear all ogs_id values
			query = fmt.Sprintf(`UPDATE %s SET ogs_id = NULL`, table)
		}

		result, queryErr := db.ExecContext(ctx, query)
		if queryErr != nil {
			// This might fail if ogs_id is NOT NULL - that's expected
			// User should roll back 1.7.9 first
			return fmt.Errorf("error clearing ogs_id in %s (is ogs_id NOT NULL? Roll back 1.7.9 first): %w", table, queryErr)
		}

		rowsAffected, _ := result.RowsAffected()
		fmt.Printf("  Cleared %d rows in %s\n", rowsAffected, table)
	}

	fmt.Println("Migration 1.7.8: Rollback complete")
	return nil
}
