package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	IndexesAndConstraintsVersion     = "1.11.0"
	IndexesAndConstraintsDescription = "Performance indexes and data integrity constraints"
)

func init() {
	// Migration 11: Performance indexes and data integrity constraints
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return indexesAndConstraintsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return indexesAndConstraintsDown(ctx, db)
		},
	)
}

// indexesAndConstraintsUp adds additional indexes and constraints for performance and data integrity
func indexesAndConstraintsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Adding performance indexes and data integrity constraints...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Adding NOT NULL constraints to important columns
	_, err = tx.ExecContext(ctx, `
		-- students table
		ALTER TABLE students 
		ALTER COLUMN name_lg SET NOT NULL,
		ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
		ALTER COLUMN modified_at SET DEFAULT CURRENT_TIMESTAMP;

		-- Groups table
		ALTER TABLE groups 
		ALTER COLUMN name SET NOT NULL,
		ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
		ALTER COLUMN modified_at SET DEFAULT CURRENT_TIMESTAMP;

		-- Rooms table
		ALTER TABLE rooms 
		ALTER COLUMN room_name SET NOT NULL,
		ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
		ALTER COLUMN modified_at SET DEFAULT CURRENT_TIMESTAMP;
		
		-- Room_occupancy table
		ALTER TABLE room_occupancy
		ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
		ALTER COLUMN modified_at SET DEFAULT CURRENT_TIMESTAMP;
	`)
	if err != nil {
		return fmt.Errorf("error adding NOT NULL constraints: %w", err)
	}

	// 2. Add performance-focused indexes
	_, err = tx.ExecContext(ctx, `
		-- students indexes for frequent queries
		CREATE INDEX IF NOT EXISTS idx_student_name_lg ON  students(name_lg);
		CREATE INDEX IF NOT EXISTS idx_student_group_id ON  students(group_id);
		CREATE INDEX IF NOT EXISTS idx_student_created_at ON  students(created_at);
		
		-- Groups indexes
		CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);
		CREATE INDEX IF NOT EXISTS idx_groups_created_at ON groups(created_at);
		
		-- Rooms indexes
		CREATE INDEX IF NOT EXISTS idx_rooms_room_name ON rooms(room_name);
		CREATE INDEX IF NOT EXISTS idx_rooms_category ON rooms(category);
		CREATE INDEX IF NOT EXISTS idx_rooms_created_at ON rooms(created_at);
		
		-- Room occupancy indexes for frequently accessed columns
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_room_id ON room_occupancy(room_id);
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_status ON room_occupancy(status);
		
		-- Room history indexes
		CREATE INDEX IF NOT EXISTS idx_room_history_room_id ON room_history(room_id);
		CREATE INDEX IF NOT EXISTS idx_room_history_day ON room_history(day);
		
	`)
	if err != nil {
		return fmt.Errorf("error creating performance indexes: %w", err)
	}

	// 3. Add composite indexes for frequently joined or filtered tables
	_, err = tx.ExecContext(ctx, `
		-- Composite indexes for room history queries
		CREATE INDEX IF NOT EXISTS idx_room_history_room_day ON room_history(room_id, day);
		
		-- Composite indexes for students queries
		CREATE INDEX IF NOT EXISTS idx_student_group_id_in_house ON  students(group_id, in_house);
	`)
	if err != nil {
		return fmt.Errorf("error creating composite indexes: %w", err)
	}

	// 4. Add check constraints for data integrity
	_, err = tx.ExecContext(ctx, `
		-- Check constraint for room occupancy
		ALTER TABLE room_occupancy
		ADD CONSTRAINT check_room_occupancy_status
		CHECK (status IN ('active', 'inactive'));
		
		-- Ensure days for room history are valid
		ALTER TABLE room_history
		ADD CONSTRAINT check_room_history_day
		CHECK (day <= CURRENT_DATE);
		
		-- Ensure valid occupancy values for room tables
		ALTER TABLE room_occupancy
		ADD CONSTRAINT check_occupancy_values
		CHECK (current_occupancy >= 0 AND current_occupancy <= max_capacity);
	`)
	if err != nil {
		return fmt.Errorf("error adding check constraints: %w", err)
	}

	// 6. Add unique constraints for data integrity
	_, err = tx.ExecContext(ctx, `
		-- Ensure unique room names
		ALTER TABLE rooms
		ADD CONSTRAINT unique_rooms_room_name
		UNIQUE (room_name);
		
		-- Ensure unique group names
		ALTER TABLE groups
		ADD CONSTRAINT unique_group_name
		UNIQUE (name);
	`)
	if err != nil {
		return fmt.Errorf("error adding unique constraints: %w", err)
	}

	// 7. Add updated_at triggers for tables missing them
	_, err = tx.ExecContext(ctx, `
		-- Ensure the update_updated_at_column function exists
		CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			-- Check which column exists in the table
			IF EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = TG_TABLE_NAME 
				AND column_name = 'modified_at'
			) THEN
				NEW.modified_at = CURRENT_TIMESTAMP;
			ELSE
				NEW.updated_at = CURRENT_TIMESTAMP;
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		
		-- Create function for modified_at column
		CREATE OR REPLACE FUNCTION update_modified_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.modified_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		
		-- students table trigger
		DROP TRIGGER IF EXISTS update_student_modified_at ON students;
		CREATE TRIGGER update_student_modified_at
		BEFORE UPDATE ON students
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_at_column();
		
		-- Groups table trigger
		DROP TRIGGER IF EXISTS update_groups_modified_at ON groups;
		CREATE TRIGGER update_groups_modified_at
		BEFORE UPDATE ON groups
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_at_column();
		
		-- Rooms table trigger
		DROP TRIGGER IF EXISTS update_rooms_modified_at ON rooms;
		CREATE TRIGGER update_rooms_modified_at
		BEFORE UPDATE ON rooms
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_at_column();
		
		-- Room occupancy table trigger
		DROP TRIGGER IF EXISTS update_room_occupancy_modified_at ON room_occupancy;
		CREATE TRIGGER update_room_occupancy_modified_at
		BEFORE UPDATE ON room_occupancy
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error adding updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// indexesAndConstraintsDown removes the indexes and constraints added in indexesAndConstraintsUp
func indexesAndConstraintsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back performance indexes and data integrity constraints...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Remove unique constraints
	_, err = tx.ExecContext(ctx, `
		-- Remove unique constraints
		ALTER TABLE rooms DROP CONSTRAINT IF EXISTS unique_rooms_room_name;
		ALTER TABLE groups DROP CONSTRAINT IF EXISTS unique_group_name;
	`)
	if err != nil {
		return fmt.Errorf("error removing unique constraints: %w", err)
	}

	// 2. Remove check constraints
	_, err = tx.ExecContext(ctx, `
		-- Remove check constraints
		ALTER TABLE room_occupancy DROP CONSTRAINT IF EXISTS check_room_occupancy_status;
		ALTER TABLE room_history DROP CONSTRAINT IF EXISTS check_room_history_day;
		ALTER TABLE room_occupancy DROP CONSTRAINT IF EXISTS check_occupancy_values;
	`)
	if err != nil {
		return fmt.Errorf("error removing check constraints: %w", err)
	}

	// 3. Remove composite indexes
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS idx_room_history_room_day;
		DROP INDEX IF EXISTS idx_student_group_id_in_house;
	`)
	if err != nil {
		return fmt.Errorf("error removing composite indexes: %w", err)
	}

	// 4. Remove performance indexes
	_, err = tx.ExecContext(ctx, `
		-- Remove students indexes
		DROP INDEX IF EXISTS idx_student_name_lg;
		DROP INDEX IF EXISTS idx_student_group_id;
		DROP INDEX IF EXISTS idx_student_created_at;
		
		-- Remove groups indexes
		DROP INDEX IF EXISTS idx_groups_name;
		DROP INDEX IF EXISTS idx_groups_created_at;
		
		-- Remove rooms indexes
		DROP INDEX IF EXISTS idx_rooms_room_name;
		DROP INDEX IF EXISTS idx_rooms_category;
		DROP INDEX IF EXISTS idx_rooms_created_at;
		
		-- Remove room occupancy indexes
		DROP INDEX IF EXISTS idx_room_occupancy_room_id;
		DROP INDEX IF EXISTS idx_room_occupancy_status;
		
		-- Remove room history indexes
		DROP INDEX IF EXISTS idx_room_history_room_id;
		DROP INDEX IF EXISTS idx_room_history_day;
		
	`)
	if err != nil {
		return fmt.Errorf("error removing performance indexes: %w", err)
	}

	// 5. Remove NOT NULL constraints and default values
	_, err = tx.ExecContext(ctx, `
		-- Remove NOT NULL constraints from students table
		ALTER TABLE students 
		ALTER COLUMN name_lg DROP NOT NULL,
		ALTER COLUMN created_at DROP DEFAULT,
		ALTER COLUMN modified_at DROP DEFAULT;
		
		-- Remove NOT NULL constraints from groups table
		ALTER TABLE groups 
		ALTER COLUMN name DROP NOT NULL,
		ALTER COLUMN created_at DROP DEFAULT,
		ALTER COLUMN modified_at DROP DEFAULT;
		
		-- Remove NOT NULL constraints from rooms table
		ALTER TABLE rooms 
		ALTER COLUMN room_name DROP NOT NULL,
		ALTER COLUMN created_at DROP DEFAULT,
		ALTER COLUMN modified_at DROP DEFAULT;
		
		-- Remove NOT NULL constraints from room_occupancy table
		ALTER TABLE room_occupancy
		ALTER COLUMN created_at DROP DEFAULT,
		ALTER COLUMN modified_at DROP DEFAULT;
	`)
	if err != nil {
		return fmt.Errorf("error removing NOT NULL constraints and default values: %w", err)
	}

	// 6. Remove triggers
	_, err = tx.ExecContext(ctx, `
		-- Remove triggers
		DROP TRIGGER IF EXISTS update_student_modified_at ON students;
		DROP TRIGGER IF EXISTS update_groups_modified_at ON groups;
		DROP TRIGGER IF EXISTS update_rooms_modified_at ON rooms;
		DROP TRIGGER IF EXISTS update_room_occupancy_modified_at ON room_occupancy;
		
		-- Remove functions
		DROP FUNCTION IF EXISTS update_modified_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error removing triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
