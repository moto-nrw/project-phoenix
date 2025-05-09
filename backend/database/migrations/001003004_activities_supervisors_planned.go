package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ActivitySupervisorsVersion     = "1.3.4"
	ActivitySupervisorsDescription = "Create activities.supervisors table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ActivitySupervisorsVersion] = &Migration{
		Version:     ActivitySupervisorsVersion,
		Description: ActivitySupervisorsDescription,
		DependsOn:   []string{"1.3.2", "1.2.3"}, // Depends on activity groups and staff tables
	}

	// Migration 1.3.4: Create activities.supervisors table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createActivitySupervisorsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropActivitySupervisorsTable(ctx, db)
		},
	)
}

// createActivitySupervisorsTable creates the activities.supervisors table
func createActivitySupervisorsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.4: Creating activities.supervisors table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the supervisors table - many-to-many relation between staff and activity groups //
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS activities.supervisors (
			id BIGSERIAL PRIMARY KEY,
			staff_id BIGINT NOT NULL,
			group_id BIGINT NOT NULL,
			is_primary BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_activity_supervisors_staff FOREIGN KEY (staff_id) 
				REFERENCES users.staff(id) ON DELETE CASCADE,
			CONSTRAINT fk_activity_supervisors_group FOREIGN KEY (group_id) 
				REFERENCES activities.groups(id) ON DELETE CASCADE, 
			CONSTRAINT uq_activity_supervisors_staff_group UNIQUE (staff_id, group_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating supervisors table: %w", err)
	}

	// Create indexes for supervisors
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_activity_supervisors_staff ON activities.supervisors(staff_id);
		CREATE INDEX IF NOT EXISTS idx_activity_supervisors_group ON activities.supervisors(group_id);
		CREATE INDEX IF NOT EXISTS idx_activity_supervisors_primary ON activities.supervisors(is_primary);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for supervisors table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for supervisors
		DROP TRIGGER IF EXISTS update_activity_supervisors_updated_at ON activities.supervisors;
		CREATE TRIGGER update_activity_supervisors_updated_at
		BEFORE UPDATE ON activities.supervisors
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating trigger for supervisors table: %w", err)
	}

	// Create trigger to ensure only one primary supervisor per group
	_, err = tx.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION activities.ensure_single_primary_supervisor()
		RETURNS TRIGGER AS $$
		BEGIN
			-- If the new/updated row is a primary supervisor
			IF NEW.is_primary = TRUE THEN
				-- Set all other supervisors for this group to not primary
				UPDATE activities.supervisors
				SET is_primary = FALSE
				WHERE group_id = NEW.group_id
				AND id != NEW.id;
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		DROP TRIGGER IF EXISTS ensure_single_primary_supervisor_trigger ON activities.supervisors;
		CREATE TRIGGER ensure_single_primary_supervisor_trigger
		BEFORE INSERT OR UPDATE ON activities.supervisors
		FOR EACH ROW
		EXECUTE FUNCTION activities.ensure_single_primary_supervisor();
	`)
	if err != nil {
		return fmt.Errorf("error creating single primary supervisor constraint: %w", err)
	}

	// Remove supervisor_id column from activities.groups if it exists (moving to the new relation table)
	_, err = tx.ExecContext(ctx, `
		-- Check if supervisor_id column exists in activities.groups
		DO $$
		BEGIN
			IF EXISTS (
				SELECT FROM information_schema.columns 
				WHERE table_schema = 'activities' 
				AND table_name = 'groups' 
				AND column_name = 'supervisor_id'
			) THEN
				-- Drop the index first if it exists
				DROP INDEX IF EXISTS idx_activity_groups_supervisor;
				
				-- Then drop the column
				ALTER TABLE activities.groups DROP COLUMN supervisor_id;
			END IF;
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("error removing supervisor_id from groups table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropActivitySupervisorsTable drops the activities.supervisors table
func dropActivitySupervisorsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.4: Removing activities.supervisors table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop triggers first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_activity_supervisors_updated_at ON activities.supervisors;
		DROP TRIGGER IF EXISTS ensure_single_primary_supervisor_trigger ON activities.supervisors;
		DROP FUNCTION IF EXISTS activities.ensure_single_primary_supervisor();
	`)
	if err != nil {
		return fmt.Errorf("error dropping triggers for supervisors table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS activities.supervisors;
	`)
	if err != nil {
		return fmt.Errorf("error dropping activities.supervisors table: %w", err)
	}

	// Add back the supervisor_id column to activities.groups
	_, err = tx.ExecContext(ctx, `
		-- Add supervisor_id back to groups table
		ALTER TABLE activities.groups ADD COLUMN IF NOT EXISTS supervisor_id BIGINT;
		
		-- Add the index back
		CREATE INDEX IF NOT EXISTS idx_activity_groups_supervisor ON activities.groups(supervisor_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring supervisor_id column: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
