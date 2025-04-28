package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 12: Align foreign keys with ER diagram
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 12: Aligning foreign keys with ER diagram...")

			// Change students.group_id from ON DELETE RESTRICT to ON DELETE CASCADE
			_, err := db.ExecContext(ctx, `
				-- First drop the existing foreign key constraint
				ALTER TABLE students DROP CONSTRAINT IF EXISTS students_group_id_fkey;
				
				-- Then add the constraint with ON DELETE CASCADE
				ALTER TABLE students 
				ADD CONSTRAINT students_group_id_fkey
				FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE;
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 12: Reverting foreign key changes...")

			// Change students.group_id back to ON DELETE RESTRICT
			_, err := db.ExecContext(ctx, `
				-- Drop the CASCADE constraint
				ALTER TABLE students DROP CONSTRAINT IF EXISTS students_group_id_fkey;
				
				-- Add back the RESTRICT constraint
				ALTER TABLE students 
				ADD CONSTRAINT students_group_id_fkey
				FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE RESTRICT;
			`)

			return err
		},
	)
}
