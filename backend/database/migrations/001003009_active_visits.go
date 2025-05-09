package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ActiveVisitsVersion     = "1.4.0"
	ActiveVisitsDescription = "Create activities.active_visits table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ActiveVisitsVersion] = &Migration{
		Version:     ActiveVisitsVersion,
		Description: ActiveVisitsDescription,
		DependsOn:   []string{"1.3.6"}, // Depends on users.students AND activities.active_groups which are both 1.3.6
	}

	// Migration 1.4.0: Create activities.active_visits table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createActiveVisitsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropActiveVisitsTable(ctx, db)
		},
	)
}

// createActiveVisitsTable creates the activities.active_visits table
func createActiveVisitsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.0: Creating activities.active_visits table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the active_visits table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS activities.active_visits (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL,       -- Reference to users.students
			active_group_id BIGINT NOT NULL,  -- Reference to activities.active_groups
			entry_time TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- When student entered the group
			exit_time TIMESTAMPTZ,            -- When student left (NULL if still active)                      -- Optional notes about the visit
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			
			-- Foreign key constraints
			CONSTRAINT fk_active_visits_student FOREIGN KEY (student_id)
				REFERENCES users.students(id) ON DELETE CASCADE,
			CONSTRAINT fk_active_visits_active_group FOREIGN KEY (active_group_id)
				REFERENCES activities.active_groups(id) ON DELETE CASCADE,
				
			-- Business rule: entry time must be before exit time (if exit time exists)
			CONSTRAINT chk_entry_before_exit CHECK (
				exit_time IS NULL OR entry_time <= exit_time
			)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating activities.active_visits table: %w", err)
	}

	// Create indexes for active_visits - improve query performance
	_, err = tx.ExecContext(ctx, `
		-- Add indexes to speed up queries
		CREATE INDEX IF NOT EXISTS idx_active_visits_student_id ON activities.active_visits(student_id);
		CREATE INDEX IF NOT EXISTS idx_active_visits_active_group_id ON activities.active_visits(active_group_id);
		CREATE INDEX IF NOT EXISTS idx_active_visits_entry_time ON activities.active_visits(entry_time);
		CREATE INDEX IF NOT EXISTS idx_active_visits_exit_time ON activities.active_visits(exit_time);
		
		-- Index for finding active visits (where exit_time is null)
		CREATE INDEX IF NOT EXISTS idx_active_visits_currently_active ON activities.active_visits(student_id, active_group_id) 
		WHERE exit_time IS NULL;
		
		-- Composite index for common queries like finding all visits for a student within a time range
		CREATE INDEX IF NOT EXISTS idx_active_visits_student_timerange ON activities.active_visits(student_id, entry_time, exit_time);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for active_visits table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for active_visits
		DROP TRIGGER IF EXISTS update_active_visits_updated_at ON activities.active_visits;
		CREATE TRIGGER update_active_visits_updated_at
		BEFORE UPDATE ON activities.active_visits
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for active_visits: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropActiveVisitsTable drops the activities.active_visits table
func dropActiveVisitsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.0: Removing activities.active_visits table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_active_visits_updated_at ON activities.active_visits;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for active_visits table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS activities.active_visits CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping activities.active_visits table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
