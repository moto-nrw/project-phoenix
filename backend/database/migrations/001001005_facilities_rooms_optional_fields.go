package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	FacilitiesRoomsOptionalFieldsVersion     = "1.1.5"
	FacilitiesRoomsOptionalFieldsDescription = "Make floor, capacity, category, and color optional in facilities.rooms"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[FacilitiesRoomsOptionalFieldsVersion] = &Migration{
		Version:     FacilitiesRoomsOptionalFieldsVersion,
		Description: FacilitiesRoomsOptionalFieldsDescription,
		DependsOn:   []string{"1.1.4"}, // Depends on schedule.recurrence_rules (last 1.1.x migration)
	}

	// Migration 1.1.5: Make floor, capacity, category, and color optional
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return makeRoomFieldsOptional(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return restoreRoomFieldsNotNull(ctx, db)
		},
	)
}

// makeRoomFieldsOptional removes NOT NULL constraints and defaults from floor, capacity, category, and color
func makeRoomFieldsOptional(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.1.5: Making floor, capacity, category, and color optional in facilities.rooms...")

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

	// Make floor optional (drop NOT NULL and default)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE facilities.rooms
		ALTER COLUMN floor DROP NOT NULL,
		ALTER COLUMN floor DROP DEFAULT;
	`)
	if err != nil {
		return fmt.Errorf("error making floor optional: %w", err)
	}

	// Make capacity optional (drop NOT NULL and default)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE facilities.rooms
		ALTER COLUMN capacity DROP NOT NULL,
		ALTER COLUMN capacity DROP DEFAULT;
	`)
	if err != nil {
		return fmt.Errorf("error making capacity optional: %w", err)
	}

	// Make category optional (drop NOT NULL and default)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE facilities.rooms
		ALTER COLUMN category DROP NOT NULL,
		ALTER COLUMN category DROP DEFAULT;
	`)
	if err != nil {
		return fmt.Errorf("error making category optional: %w", err)
	}

	// Make color optional (drop NOT NULL and default)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE facilities.rooms
		ALTER COLUMN color DROP NOT NULL,
		ALTER COLUMN color DROP DEFAULT;
	`)
	if err != nil {
		return fmt.Errorf("error making color optional: %w", err)
	}

	fmt.Println("Successfully made floor, capacity, category, and color optional")

	// Commit the transaction
	return tx.Commit()
}

// restoreRoomFieldsNotNull restores the original NOT NULL constraints and defaults
func restoreRoomFieldsNotNull(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.1.5: Restoring NOT NULL constraints and defaults...")

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

	// Update any NULL values to defaults before adding NOT NULL constraint
	_, err = tx.ExecContext(ctx, `
		UPDATE facilities.rooms
		SET
			floor = COALESCE(floor, 0),
			capacity = COALESCE(capacity, 0),
			category = COALESCE(category, 'Other'),
			color = COALESCE(color, '#FFFFFF')
		WHERE floor IS NULL
		   OR capacity IS NULL
		   OR category IS NULL
		   OR color IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error updating NULL values: %w", err)
	}

	// Restore floor NOT NULL and default
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE facilities.rooms
		ALTER COLUMN floor SET DEFAULT 0,
		ALTER COLUMN floor SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error restoring floor NOT NULL: %w", err)
	}

	// Restore capacity NOT NULL and default
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE facilities.rooms
		ALTER COLUMN capacity SET DEFAULT 0,
		ALTER COLUMN capacity SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error restoring capacity NOT NULL: %w", err)
	}

	// Restore category NOT NULL and default
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE facilities.rooms
		ALTER COLUMN category SET DEFAULT 'Other',
		ALTER COLUMN category SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error restoring category NOT NULL: %w", err)
	}

	// Restore color NOT NULL and default
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE facilities.rooms
		ALTER COLUMN color SET DEFAULT '#FFFFFF',
		ALTER COLUMN color SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error restoring color NOT NULL: %w", err)
	}

	fmt.Println("Successfully restored NOT NULL constraints and defaults")

	// Commit the transaction
	return tx.Commit()
}
