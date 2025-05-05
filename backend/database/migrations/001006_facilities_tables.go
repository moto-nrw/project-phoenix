package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	FacilitiesTablesVersion     = "1.6.0"
	FacilitiesTablesDescription = "Create facilities tables for room management"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[FacilitiesTablesVersion] = &Migration{
		Version:     FacilitiesTablesVersion,
		Description: FacilitiesTablesDescription,
		DependsOn:   []string{"1.5.0"}, // Depends on activities tables
	}

	// Migration 1.6.0: Create facilities tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createFacilitiesTables(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropFacilitiesTables(ctx, db)
		},
	)
}

// createFacilitiesTables creates all the tables for the facilities schema
func createFacilitiesTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.0: Creating facilities tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the facilities.rooms table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS facilities.rooms (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			building TEXT,
			floor INT NOT NULL DEFAULT 0,
			capacity INT NOT NULL DEFAULT 0,
			category TEXT NOT NULL DEFAULT 'Other',
			color TEXT NOT NULL DEFAULT '#FFFFFF',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating facilities.rooms table: %w", err)
	}

	// Create the facilities.room_occupancy table
	_, err = tx.ExecContext(ctx, `
		DROP TYPE IF EXISTS occupancy_status CASCADE;
		CREATE TYPE occupancy_status AS ENUM ('active', 'inactive', 'maintenance');
		
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

	// Create the facilities.room_occupancy_teachers table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS facilities.room_occupancy_teachers (
			id BIGSERIAL PRIMARY KEY,
			room_occupancy_id BIGINT NOT NULL REFERENCES facilities.room_occupancy(id),
			teacher_id BIGINT NOT NULL REFERENCES users.teachers(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(room_occupancy_id, teacher_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating facilities.room_occupancy_teachers table: %w", err)
	}

	// Create the facilities.visits table - tracks all persons (students, teachers, guests)
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS facilities.visits (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL REFERENCES users.persons(id),
			room_occupancy_id BIGINT NOT NULL REFERENCES facilities.room_occupancy(id),
			entry_time TIMESTAMPTZ NOT NULL,
			exit_time TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating facilities.visits table: %w", err)
	}

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
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating facilities.room_history table: %w", err)
	}

	// Create indexes for performance
	_, err = tx.ExecContext(ctx, `
		-- Indexes for facilities.room_occupancy
		CREATE INDEX idx_room_occupancy_room_id ON facilities.room_occupancy(room_id);
		CREATE INDEX idx_room_occupancy_timeframe_id ON facilities.room_occupancy(timeframe_id);
		CREATE INDEX idx_room_occupancy_activity_group_id ON facilities.room_occupancy(activity_group_id) WHERE activity_group_id IS NOT NULL;
		CREATE INDEX idx_room_occupancy_group_id ON facilities.room_occupancy(group_id) WHERE group_id IS NOT NULL;
		
		-- Indexes for facilities.room_occupancy_teachers
		CREATE INDEX idx_room_occupancy_teachers_room_occupancy_id ON facilities.room_occupancy_teachers(room_occupancy_id);
		CREATE INDEX idx_room_occupancy_teachers_teacher_id ON facilities.room_occupancy_teachers(teacher_id);
		
		-- Indexes for facilities.visits
		CREATE INDEX idx_visits_person_id ON facilities.visits(person_id);
		CREATE INDEX idx_visits_room_occupancy_id ON facilities.visits(room_occupancy_id);
		CREATE INDEX idx_visits_entry_time ON facilities.visits(entry_time);
		CREATE INDEX idx_visits_exit_time ON facilities.visits(exit_time) WHERE exit_time IS NOT NULL;
		
		-- Indexes for facilities.room_history
		CREATE INDEX idx_room_history_room_id ON facilities.room_history(room_id);
		CREATE INDEX idx_room_history_day ON facilities.room_history(day);
		CREATE INDEX idx_room_history_timeframe_id ON facilities.room_history(timeframe_id);
		CREATE INDEX idx_room_history_teacher_id ON facilities.room_history(teacher_id);
		CREATE INDEX idx_room_history_group_id ON facilities.room_history(group_id) WHERE group_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for facilities tables: %w", err)
	}

	// Create triggers for updating updated_at columns
	_, err = tx.ExecContext(ctx, `
		-- Trigger for facilities.rooms
		CREATE TRIGGER update_facilities_rooms_updated_at
		BEFORE UPDATE ON facilities.rooms
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for facilities.room_occupancy
		CREATE TRIGGER update_facilities_room_occupancy_updated_at
		BEFORE UPDATE ON facilities.room_occupancy
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for facilities.visits
		CREATE TRIGGER update_facilities_visits_updated_at
		BEFORE UPDATE ON facilities.visits
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating triggers for facilities tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropFacilitiesTables drops all the tables in the facilities schema
func dropFacilitiesTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.0: Removing facilities tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		-- Drop triggers first
		DROP TRIGGER IF EXISTS update_facilities_rooms_updated_at ON facilities.rooms;
		DROP TRIGGER IF EXISTS update_facilities_room_occupancy_updated_at ON facilities.room_occupancy;
		DROP TRIGGER IF EXISTS update_facilities_visits_updated_at ON facilities.visits;
		
		-- Drop tables in order of dependencies
		DROP TABLE IF EXISTS facilities.room_history;
		DROP TABLE IF EXISTS facilities.visits;
		DROP TABLE IF EXISTS facilities.room_occupancy_teachers;
		DROP TABLE IF EXISTS facilities.room_occupancy;
		DROP TABLE IF EXISTS facilities.rooms;
		
		-- Drop custom types
		DROP TYPE IF EXISTS occupancy_status;
	`)
	if err != nil {
		return fmt.Errorf("error dropping facilities tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
