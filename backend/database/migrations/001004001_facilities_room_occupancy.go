package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	FacilitiesRoomOccupancyVersion     = "1.4.1"
	FacilitiesRoomOccupancyDescription = "Create facilities.room_occupancy table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[FacilitiesRoomOccupancyVersion] = &Migration{
		Version:     FacilitiesRoomOccupancyVersion,
		Description: FacilitiesRoomOccupancyDescription,
		DependsOn:   []string{"1.1.1", "1.1.2", "1.3.2", "1.2.6"}, // Depends on rooms, timeframes, activities.groups, and education.groups
	}

	// Migration 1.4.1: Create facilities.room_occupancy table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createFacilitiesRoomOccupancyTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropFacilitiesRoomOccupancyTable(ctx, db)
		},
	)
}

// createFacilitiesRoomOccupancyTable creates the facilities.room_occupancy table
func createFacilitiesRoomOccupancyTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.1: Creating facilities.room_occupancy table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the room_occupancy status type
	_, err = tx.ExecContext(ctx, `
		DROP TYPE IF EXISTS occupancy_status CASCADE;
		CREATE TYPE occupancy_status AS ENUM ('active', 'inactive', 'maintenance');
	`)
	if err != nil {
		return fmt.Errorf("error creating occupancy_status type: %w", err)
	}

	// Create the facilities.room_occupancy table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS facilities.room_occupancy (
			id BIGSERIAL PRIMARY KEY,
			device_id TEXT NOT NULL UNIQUE,
			room_id BIGINT NOT NULL REFERENCES facilities.rooms(id),
			timeframe_id BIGINT NOT NULL REFERENCES schedule.timeframes(id),
			status occupancy_status NOT NULL DEFAULT 'active',
			max_capacity INT NOT NULL DEFAULT 0,
			current_occupancy INT NOT NULL DEFAULT 0,
			activity_group_id BIGINT REFERENCES activities.groups(id),
			group_id BIGINT REFERENCES education.groups(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			-- Ensure only one of activity_group_id or group_id is set
			CONSTRAINT only_one_group_type CHECK (
				(activity_group_id IS NULL AND group_id IS NOT NULL) OR
				(activity_group_id IS NOT NULL AND group_id IS NULL)
			)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating facilities.room_occupancy table: %w", err)
	}

	// Create indexes for room_occupancy
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX idx_room_occupancy_room_id ON facilities.room_occupancy(room_id);
		CREATE INDEX idx_room_occupancy_timeframe_id ON facilities.room_occupancy(timeframe_id);
		CREATE INDEX idx_room_occupancy_activity_group_id ON facilities.room_occupancy(activity_group_id) WHERE activity_group_id IS NOT NULL;
		CREATE INDEX idx_room_occupancy_group_id ON facilities.room_occupancy(group_id) WHERE group_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for room_occupancy table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for room_occupancy
		CREATE TRIGGER update_facilities_room_occupancy_updated_at
		BEFORE UPDATE ON facilities.room_occupancy
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating trigger for room_occupancy table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropFacilitiesRoomOccupancyTable drops the facilities.room_occupancy table
func dropFacilitiesRoomOccupancyTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.1: Removing facilities.room_occupancy table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_facilities_room_occupancy_updated_at ON facilities.room_occupancy;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for room_occupancy table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS facilities.room_occupancy;
	`)
	if err != nil {
		return fmt.Errorf("error dropping facilities.room_occupancy table: %w", err)
	}

	// Drop the type
	_, err = tx.ExecContext(ctx, `
		DROP TYPE IF EXISTS occupancy_status;
	`)
	if err != nil {
		return fmt.Errorf("error dropping occupancy_status type: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
