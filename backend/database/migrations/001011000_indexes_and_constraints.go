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
		-- Student table
		ALTER TABLE student 
		ALTER COLUMN name SET NOT NULL,
		ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
		ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;

		-- Groups table
		ALTER TABLE groups 
		ALTER COLUMN name SET NOT NULL,
		ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
		ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;

		-- Room table
		ALTER TABLE room 
		ALTER COLUMN name SET NOT NULL,
		ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
		ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
		
		-- Room_occupancy table
		ALTER TABLE room_occupancy
		ALTER COLUMN entered_at SET NOT NULL,
		ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
		ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
	`)
	if err != nil {
		return fmt.Errorf("error adding NOT NULL constraints: %w", err)
	}

	// 2. Add performance-focused indexes
	_, err = tx.ExecContext(ctx, `
		-- Student indexes for frequent queries
		CREATE INDEX IF NOT EXISTS idx_student_name ON student(name);
		CREATE INDEX IF NOT EXISTS idx_student_active ON student(active);
		CREATE INDEX IF NOT EXISTS idx_student_group_id ON student(group_id);
		CREATE INDEX IF NOT EXISTS idx_student_created_at ON student(created_at);
		
		-- Groups indexes
		CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);
		CREATE INDEX IF NOT EXISTS idx_groups_active ON groups(active);
		CREATE INDEX IF NOT EXISTS idx_groups_created_at ON groups(created_at);
		
		-- Room indexes
		CREATE INDEX IF NOT EXISTS idx_room_name ON room(name);
		CREATE INDEX IF NOT EXISTS idx_room_active ON room(active);
		CREATE INDEX IF NOT EXISTS idx_room_created_at ON room(created_at);
		
		-- Room occupancy indexes for frequently accessed columns
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_room_id ON room_occupancy(room_id);
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_student_id ON room_occupancy(student_id);
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_entered_at ON room_occupancy(entered_at);
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_exited_at ON room_occupancy(exited_at);
		
		-- Room history indexes
		CREATE INDEX IF NOT EXISTS idx_room_history_room_id ON room_history(room_id);
		CREATE INDEX IF NOT EXISTS idx_room_history_timestamp ON room_history(timestamp);
		
		-- Activity tables (if they exist)
		CREATE INDEX IF NOT EXISTS idx_activity_logs_timestamp ON activity_logs(timestamp);
		CREATE INDEX IF NOT EXISTS idx_activity_logs_user_id ON activity_logs(user_id);
		CREATE INDEX IF NOT EXISTS idx_activity_logs_action_type ON activity_logs(action_type);
	`)
	if err != nil {
		return fmt.Errorf("error creating performance indexes: %w", err)
	}

	// 3. Add composite indexes for frequently joined or filtered tables
	_, err = tx.ExecContext(ctx, `
		-- Composite indexes for room occupancy queries
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_room_student ON room_occupancy(room_id, student_id);
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_room_entered ON room_occupancy(room_id, entered_at);
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_student_entered ON room_occupancy(student_id, entered_at);
		
		-- Composite indexes for room history queries
		CREATE INDEX IF NOT EXISTS idx_room_history_room_timestamp ON room_history(room_id, timestamp);
		
		-- Composite indexes for student queries
		CREATE INDEX IF NOT EXISTS idx_student_group_active ON student(group_id, active);
	`)
	if err != nil {
		return fmt.Errorf("error creating composite indexes: %w", err)
	}

	// 4. Add check constraints for data integrity
	_, err = tx.ExecContext(ctx, `
		-- Ensure entered_at is before exited_at in room_occupancy
		ALTER TABLE room_occupancy
		ADD CONSTRAINT check_room_occupancy_time_order 
		CHECK (exited_at IS NULL OR entered_at <= exited_at);
		
		-- Ensure timestamps for room history are valid
		ALTER TABLE room_history
		ADD CONSTRAINT check_room_history_timestamp
		CHECK (timestamp <= CURRENT_TIMESTAMP);
		
		-- Ensure valid percentages for relevant tables
		ALTER TABLE room_occupancy
		ADD CONSTRAINT check_occupancy_percentage
		CHECK (occupancy_percentage >= 0 AND occupancy_percentage <= 100);
	`)
	if err != nil {
		return fmt.Errorf("error adding check constraints: %w", err)
	}

	// 5. Add default values for important columns
	_, err = tx.ExecContext(ctx, `
		-- Default values for student table
		ALTER TABLE student
		ALTER COLUMN active SET DEFAULT TRUE;
		
		-- Default values for groups table
		ALTER TABLE groups
		ALTER COLUMN active SET DEFAULT TRUE;
		
		-- Default values for room table
		ALTER TABLE room
		ALTER COLUMN active SET DEFAULT TRUE;
	`)
	if err != nil {
		return fmt.Errorf("error setting default values: %w", err)
	}

	// 6. Add unique constraints for data integrity
	_, err = tx.ExecContext(ctx, `
		-- Ensure unique identifiers for students
		ALTER TABLE student
		ADD CONSTRAINT unique_student_external_id
		UNIQUE (external_id);
		
		-- Ensure unique room names
		ALTER TABLE room
		ADD CONSTRAINT unique_room_name
		UNIQUE (name);
		
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
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		
		-- Student table trigger
		DROP TRIGGER IF EXISTS update_student_updated_at ON student;
		CREATE TRIGGER update_student_updated_at
		BEFORE UPDATE ON student
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
		
		-- Groups table trigger
		DROP TRIGGER IF EXISTS update_groups_updated_at ON groups;
		CREATE TRIGGER update_groups_updated_at
		BEFORE UPDATE ON groups
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
		
		-- Room table trigger
		DROP TRIGGER IF EXISTS update_room_updated_at ON room;
		CREATE TRIGGER update_room_updated_at
		BEFORE UPDATE ON room
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
		
		-- Room occupancy table trigger
		DROP TRIGGER IF EXISTS update_room_occupancy_updated_at ON room_occupancy;
		CREATE TRIGGER update_room_occupancy_updated_at
		BEFORE UPDATE ON room_occupancy
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
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
		ALTER TABLE student DROP CONSTRAINT IF EXISTS unique_student_external_id;
		ALTER TABLE room DROP CONSTRAINT IF EXISTS unique_room_name;
		ALTER TABLE groups DROP CONSTRAINT IF EXISTS unique_group_name;
	`)
	if err != nil {
		return fmt.Errorf("error removing unique constraints: %w", err)
	}

	// 2. Remove check constraints
	_, err = tx.ExecContext(ctx, `
		-- Remove check constraints
		ALTER TABLE room_occupancy DROP CONSTRAINT IF EXISTS check_room_occupancy_time_order;
		ALTER TABLE room_history DROP CONSTRAINT IF EXISTS check_room_history_timestamp;
		ALTER TABLE room_occupancy DROP CONSTRAINT IF EXISTS check_occupancy_percentage;
	`)
	if err != nil {
		return fmt.Errorf("error removing check constraints: %w", err)
	}

	// 3. Remove composite indexes
	_, err = tx.ExecContext(ctx, `
		-- Remove composite indexes
		DROP INDEX IF EXISTS idx_room_occupancy_room_student;
		DROP INDEX IF EXISTS idx_room_occupancy_room_entered;
		DROP INDEX IF EXISTS idx_room_occupancy_student_entered;
		DROP INDEX IF EXISTS idx_room_history_room_timestamp;
		DROP INDEX IF EXISTS idx_student_group_active;
	`)
	if err != nil {
		return fmt.Errorf("error removing composite indexes: %w", err)
	}

	// 4. Remove performance indexes
	_, err = tx.ExecContext(ctx, `
		-- Remove student indexes
		DROP INDEX IF EXISTS idx_student_name;
		DROP INDEX IF EXISTS idx_student_active;
		DROP INDEX IF EXISTS idx_student_group_id;
		DROP INDEX IF EXISTS idx_student_created_at;
		
		-- Remove groups indexes
		DROP INDEX IF EXISTS idx_groups_name;
		DROP INDEX IF EXISTS idx_groups_active;
		DROP INDEX IF EXISTS idx_groups_created_at;
		
		-- Remove room indexes
		DROP INDEX IF EXISTS idx_room_name;
		DROP INDEX IF EXISTS idx_room_active;
		DROP INDEX IF EXISTS idx_room_created_at;
		
		-- Remove room occupancy indexes
		DROP INDEX IF EXISTS idx_room_occupancy_room_id;
		DROP INDEX IF EXISTS idx_room_occupancy_student_id;
		DROP INDEX IF EXISTS idx_room_occupancy_entered_at;
		DROP INDEX IF EXISTS idx_room_occupancy_exited_at;
		
		-- Remove room history indexes
		DROP INDEX IF EXISTS idx_room_history_room_id;
		DROP INDEX IF EXISTS idx_room_history_timestamp;
		
		-- Remove activity indexes
		DROP INDEX IF EXISTS idx_activity_logs_timestamp;
		DROP INDEX IF EXISTS idx_activity_logs_user_id;
		DROP INDEX IF EXISTS idx_activity_logs_action_type;
	`)
	if err != nil {
		return fmt.Errorf("error removing performance indexes: %w", err)
	}

	// 5. Remove NOT NULL constraints and default values
	_, err = tx.ExecContext(ctx, `
		-- Remove NOT NULL constraints from student table
		ALTER TABLE student 
		ALTER COLUMN name DROP NOT NULL,
		ALTER COLUMN created_at DROP DEFAULT,
		ALTER COLUMN updated_at DROP DEFAULT,
		ALTER COLUMN active DROP DEFAULT;
		
		-- Remove NOT NULL constraints from groups table
		ALTER TABLE groups 
		ALTER COLUMN name DROP NOT NULL,
		ALTER COLUMN created_at DROP DEFAULT,
		ALTER COLUMN updated_at DROP DEFAULT,
		ALTER COLUMN active DROP DEFAULT;
		
		-- Remove NOT NULL constraints from room table
		ALTER TABLE room 
		ALTER COLUMN name DROP NOT NULL,
		ALTER COLUMN created_at DROP DEFAULT,
		ALTER COLUMN updated_at DROP DEFAULT,
		ALTER COLUMN active DROP DEFAULT;
		
		-- Remove NOT NULL constraints from room_occupancy table
		ALTER TABLE room_occupancy
		ALTER COLUMN entered_at DROP NOT NULL,
		ALTER COLUMN created_at DROP DEFAULT,
		ALTER COLUMN updated_at DROP DEFAULT;
	`)
	if err != nil {
		return fmt.Errorf("error removing NOT NULL constraints and default values: %w", err)
	}

	// 6. Remove triggers
	_, err = tx.ExecContext(ctx, `
		-- Remove triggers
		DROP TRIGGER IF EXISTS update_student_updated_at ON student;
		DROP TRIGGER IF EXISTS update_groups_updated_at ON groups;
		DROP TRIGGER IF EXISTS update_room_updated_at ON room;
		DROP TRIGGER IF EXISTS update_room_occupancy_updated_at ON room_occupancy;
	`)
	if err != nil {
		return fmt.Errorf("error removing triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
