package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 6: Add indexes for performance optimization
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 6: Adding indexes and optimizations...")

			// Add indexes to improve query performance

			// Indexes for foreign keys in students table
			_, err := db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_students_group_id ON students(group_id);
				CREATE INDEX IF NOT EXISTS idx_students_custom_user_id ON students(custom_user_id);
			`)
			if err != nil {
				return err
			}

			// Indexes for foreign keys in ags table
			_, err = db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_ags_supervisor_id ON ags(supervisor_id);
				CREATE INDEX IF NOT EXISTS idx_ags_ag_category_id ON ags(ag_category_id);
				CREATE INDEX IF NOT EXISTS idx_ags_datespan_id ON ags(datespan_id);
			`)
			if err != nil {
				return err
			}

			// Indexes for junction tables
			_, err = db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_student_ags_student_id ON student_ags(student_id);
				CREATE INDEX IF NOT EXISTS idx_student_ags_ag_id ON student_ags(ag_id);
				
				CREATE INDEX IF NOT EXISTS idx_group_supervisors_group_id ON group_supervisors(group_id);
				CREATE INDEX IF NOT EXISTS idx_group_supervisors_supervisor_id ON group_supervisors(supervisor_id);
				
				CREATE INDEX IF NOT EXISTS idx_combined_group_groups_combined_group_id ON combined_group_groups(combined_group_id);
				CREATE INDEX IF NOT EXISTS idx_combined_group_groups_group_id ON combined_group_groups(group_id);
				
				CREATE INDEX IF NOT EXISTS idx_combined_group_specialists_combined_group_id ON combined_group_specialists(combined_group_id);
				CREATE INDEX IF NOT EXISTS idx_combined_group_specialists_specialist_id ON combined_group_specialists(specialist_id);
				
				CREATE INDEX IF NOT EXISTS idx_room_occupancy_supervisors_room_occupancy_id ON room_occupancy_supervisors(room_occupancy_id);
				CREATE INDEX IF NOT EXISTS idx_room_occupancy_supervisors_supervisor_id ON room_occupancy_supervisors(supervisor_id);
			`)
			if err != nil {
				return err
			}

			// Indexes for timespan related queries
			_, err = db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_ag_times_timespan_id ON ag_times(timespan_id);
				CREATE INDEX IF NOT EXISTS idx_ag_times_ag_id ON ag_times(ag_id);
				CREATE INDEX IF NOT EXISTS idx_room_occupancy_room_id ON room_occupancy(room_id);
				CREATE INDEX IF NOT EXISTS idx_room_occupancy_timespan_id ON room_occupancy(timespan_id);
			`)
			if err != nil {
				return err
			}

			// Add index for accounts email search
			_, err = db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_accounts_email ON accounts(email);
				CREATE INDEX IF NOT EXISTS idx_accounts_username ON accounts(username);
			`)
			if err != nil {
				return err
			}

			// Add settings for search
			_, err = db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_settings_key ON settings(key);
			`)
			if err != nil {
				return err
			}

			// Add device ID index
			_, err = db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_devices_device_id ON devices(device_id);
				CREATE INDEX IF NOT EXISTS idx_devices_room_id ON devices(room_id);
			`)
			if err != nil {
				return err
			}

			// Add missing constraints to match ER diagram
			fmt.Println("Adding missing constraints from ER diagram...")

			// Add UNIQUE constraint and INDEX to groups.name
			_, err = db.ExecContext(ctx, `
				-- Add unique constraint to group names if it doesn't exist
				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1 FROM pg_constraint WHERE conname = 'groups_name_key'
					) THEN
						ALTER TABLE groups ADD CONSTRAINT groups_name_key UNIQUE (name);
					END IF;
				END
				$$;

				-- Create index on groups.name if it doesn't exist
				CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);
			`)
			if err != nil {
				return fmt.Errorf("error adding constraints to groups: %w", err)
			}

			// Update room_occupancy.device_id to be NOT NULL, UNIQUE and INDEXED
			_, err = db.ExecContext(ctx, `
				-- Check if device_id column exists in room_occupancy
				DO $$
				BEGIN
					-- First update any NULL values with a placeholder
					UPDATE room_occupancy SET device_id = 'device_' || id WHERE device_id IS NULL;
							
					-- Make device_id NOT NULL if it's nullable
					IF EXISTS (
						SELECT 1 FROM information_schema.columns 
						WHERE table_name = 'room_occupancy' 
						AND column_name = 'device_id' 
						AND is_nullable = 'YES'
					) THEN
						ALTER TABLE room_occupancy ALTER COLUMN device_id SET NOT NULL;
					END IF;
				END
				$$;

				-- Add unique constraint if it doesn't exist
				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1 FROM pg_constraint WHERE conname = 'room_occupancy_device_id_key'
					) THEN
						ALTER TABLE room_occupancy ADD CONSTRAINT room_occupancy_device_id_key UNIQUE (device_id);
					END IF;
				END
				$$;

				-- Create index on device_id if it doesn't exist
				CREATE INDEX IF NOT EXISTS idx_room_occupancy_device_id ON room_occupancy(device_id);
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 6: Dropping indexes...")

			// Remove the constraints for ER diagram alignment
			_, err := db.ExecContext(ctx, `
				DROP INDEX IF EXISTS idx_groups_name;
				ALTER TABLE groups DROP CONSTRAINT IF EXISTS groups_name_key;
				
				DROP INDEX IF EXISTS idx_room_occupancy_device_id;
				ALTER TABLE room_occupancy DROP CONSTRAINT IF EXISTS room_occupancy_device_id_key;
				ALTER TABLE room_occupancy ALTER COLUMN device_id DROP NOT NULL;
			`)
			if err != nil {
				return err
			}

			// Drop all created indexes
			_, err = db.ExecContext(ctx, `
				DROP INDEX IF EXISTS idx_students_group_id;
				DROP INDEX IF EXISTS idx_students_custom_user_id;
				
				DROP INDEX IF EXISTS idx_ags_supervisor_id;
				DROP INDEX IF EXISTS idx_ags_ag_category_id;
				DROP INDEX IF EXISTS idx_ags_datespan_id;
				
				DROP INDEX IF EXISTS idx_student_ags_student_id;
				DROP INDEX IF EXISTS idx_student_ags_ag_id;
				
				DROP INDEX IF EXISTS idx_group_supervisors_group_id;
				DROP INDEX IF EXISTS idx_group_supervisors_supervisor_id;
				
				DROP INDEX IF EXISTS idx_combined_group_groups_combined_group_id;
				DROP INDEX IF EXISTS idx_combined_group_groups_group_id;
				
				DROP INDEX IF EXISTS idx_combined_group_specialists_combined_group_id;
				DROP INDEX IF EXISTS idx_combined_group_specialists_specialist_id;
				
				DROP INDEX IF EXISTS idx_room_occupancy_supervisors_room_occupancy_id;
				DROP INDEX IF EXISTS idx_room_occupancy_supervisors_supervisor_id;
				
				DROP INDEX IF EXISTS idx_ag_times_timespan_id;
				DROP INDEX IF EXISTS idx_ag_times_ag_id;
				DROP INDEX IF EXISTS idx_room_occupancy_room_id;
				DROP INDEX IF EXISTS idx_room_occupancy_timespan_id;
				
				DROP INDEX IF EXISTS idx_accounts_email;
				DROP INDEX IF EXISTS idx_accounts_username;
				
				DROP INDEX IF EXISTS idx_settings_key;
				
				DROP INDEX IF EXISTS idx_devices_device_id;
				DROP INDEX IF EXISTS idx_devices_room_id;
			`)

			return err
		},
	)
}
