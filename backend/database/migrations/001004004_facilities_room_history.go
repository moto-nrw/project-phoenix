package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	FacilitiesRoomHistoryVersion     = "1.4.4"
	FacilitiesRoomHistoryDescription = "Create facilities.room_history table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[FacilitiesRoomHistoryVersion] = &Migration{
		Version:     FacilitiesRoomHistoryVersion,
		Description: FacilitiesRoomHistoryDescription,
		DependsOn:   []string{"1.1.1", "1.1.2", "1.3.1", "1.2.3", "1.2.6"}, // Depends on rooms, timeframes, categories, teachers, and groups
	}

	// Migration 1.4.4: Create facilities.room_history table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createFacilitiesRoomHistoryTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropFacilitiesRoomHistoryTable(ctx, db)
		},
	)
}

// createFacilitiesRoomHistoryTable creates the facilities.room_history table
func createFacilitiesRoomHistoryTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.4: Creating facilities.room_history table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the facilities.room_history table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS facilities.room_history (
			id BIGSERIAL PRIMARY KEY,
			room_id BIGINT NOT NULL REFERENCES facilities.rooms(id),
			activity_group_name TEXT NOT NULL,
			day DATE NOT NULL,
			timeframe_id BIGINT NOT NULL REFERENCES schedule.timeframes(id),
			category_id BIGINT REFERENCES activities.categories(id),
			teacher_id BIGINT NOT NULL REFERENCES users.teachers(id),
			max_participants INT NOT NULL DEFAULT 0,
			group_id BIGINT REFERENCES education.groups(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			-- Ensure consistent handling of category_id and group_id relationship
			CONSTRAINT only_one_group_type CHECK (
				(category_id IS NULL AND group_id IS NOT NULL) OR
				(category_id IS NOT NULL AND group_id IS NULL) OR
				(category_id IS NULL AND group_id IS NULL)
			)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating facilities.room_history table: %w", err)
	}

	// Create indexes for room_history
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX idx_room_history_room_id ON facilities.room_history(room_id);
		CREATE INDEX idx_room_history_day ON facilities.room_history(day);
		CREATE INDEX idx_room_history_timeframe_id ON facilities.room_history(timeframe_id);
		CREATE INDEX idx_room_history_teacher_id ON facilities.room_history(teacher_id);
		CREATE INDEX idx_room_history_group_id ON facilities.room_history(group_id) WHERE group_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for room_history table: %w", err)
	}

	// No updated_at column in this table, so no trigger needed

	// Commit the transaction
	return tx.Commit()
}

// dropFacilitiesRoomHistoryTable drops the facilities.room_history table
func dropFacilitiesRoomHistoryTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.4: Removing facilities.room_history table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS facilities.room_history;
	`)
	if err != nil {
		return fmt.Errorf("error dropping facilities.room_history table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
