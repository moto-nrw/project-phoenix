package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	normalizeRoomCapacityVersion     = "1.15.286"
	normalizeRoomCapacityDescription = "Normalize legacy room capacities and enforce positive optional limits (#2237)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     normalizeRoomCapacityVersion,
		Description: normalizeRoomCapacityDescription,
		DependsOn:   []string{wissingenDepartureModesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return normalizeRoomCapacityUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return normalizeRoomCapacityDown(ctx, db)
		},
	)
}

func normalizeRoomCapacityUp(ctx context.Context, db *bun.DB) error {
	result, err := db.NewRaw(`
		UPDATE facilities.rooms
		SET capacity = NULL
		WHERE capacity <= 0;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed normalizing legacy room capacities: %w", err)
	}

	if affected, rowsErr := result.RowsAffected(); rowsErr == nil {
		migrationLog().InfoContext(ctx, "non-positive room capacities normalized to NULL",
			"rows", affected,
		)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE facilities.rooms
			ADD CONSTRAINT rooms_capacity_positive_check
			CHECK (capacity IS NULL OR capacity > 0) NOT VALID;
		ALTER TABLE facilities.rooms
			VALIDATE CONSTRAINT rooms_capacity_positive_check;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enforcing positive room capacities: %w", err)
	}

	return nil
}

func normalizeRoomCapacityDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE facilities.rooms
			DROP CONSTRAINT IF EXISTS rooms_capacity_positive_check;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing positive room capacity constraint: %w", err)
	}

	return nil
}
