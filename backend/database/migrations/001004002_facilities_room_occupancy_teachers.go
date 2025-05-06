package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	FacilitiesRoomOccupancyTeachersVersion     = "1.4.2"
	FacilitiesRoomOccupancyTeachersDescription = "Create facilities.room_occupancy_teachers table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[FacilitiesRoomOccupancyTeachersVersion] = &Migration{
		Version:     FacilitiesRoomOccupancyTeachersVersion,
		Description: FacilitiesRoomOccupancyTeachersDescription,
		DependsOn:   []string{"1.4.1", "1.2.3"}, // Depends on room_occupancy and teachers
	}

	// Migration 1.4.2: Create facilities.room_occupancy_teachers table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createFacilitiesRoomOccupancyTeachersTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropFacilitiesRoomOccupancyTeachersTable(ctx, db)
		},
	)
}

// createFacilitiesRoomOccupancyTeachersTable creates the facilities.room_occupancy_teachers table
func createFacilitiesRoomOccupancyTeachersTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.2: Creating facilities.room_occupancy_teachers table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the facilities.room_occupancy_teachers table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS facilities.room_occupancy_teachers (
			id BIGSERIAL PRIMARY KEY,
			room_occupancy_id BIGINT NOT NULL REFERENCES facilities.room_occupancy(id) ON DELETE RESTRICT,
			teacher_id BIGINT REFERENCES users.teachers(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(room_occupancy_id, teacher_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating facilities.room_occupancy_teachers table: %w", err)
	}

	// Create indexes for room_occupancy_teachers
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX idx_room_occupancy_teachers_room_occupancy_id ON facilities.room_occupancy_teachers(room_occupancy_id);
		CREATE INDEX idx_room_occupancy_teachers_teacher_id ON facilities.room_occupancy_teachers(teacher_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for room_occupancy_teachers table: %w", err)
	}

	// No updated_at column in this table, so no trigger needed

	// Commit the transaction
	return tx.Commit()
}

// dropFacilitiesRoomOccupancyTeachersTable drops the facilities.room_occupancy_teachers table
func dropFacilitiesRoomOccupancyTeachersTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.2: Removing facilities.room_occupancy_teachers table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS facilities.room_occupancy_teachers;
	`)
	if err != nil {
		return fmt.Errorf("error dropping facilities.room_occupancy_teachers table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
