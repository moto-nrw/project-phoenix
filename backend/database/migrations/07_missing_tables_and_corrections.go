package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 7: Add missing tables and fix issues
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 7: Adding missing tables and fixing issues...")

			// 1. Create Room_history table
			_, err := db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS room_history (
					id SERIAL PRIMARY KEY,
					room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
					ag_name TEXT NOT NULL,
					day DATE NOT NULL,
					timespan_id INTEGER NOT NULL REFERENCES timespans(id) ON DELETE CASCADE,
					ag_category_id INTEGER REFERENCES ag_categories(id) ON DELETE SET NULL,
					supervisor_id INTEGER NOT NULL,
					max_participant INTEGER NOT NULL DEFAULT 0,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
				)
			`)
			if err != nil {
				return err
			}

			// 2. Create indexes for room_history
			_, err = db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_room_history_day ON room_history(day);
				CREATE INDEX IF NOT EXISTS idx_room_history_room_id ON room_history(room_id);
			`)
			if err != nil {
				return err
			}

			// 3. Update Visit table
			_, err = db.ExecContext(ctx, `
				ALTER TABLE visits 
				ADD COLUMN IF NOT EXISTS day DATE NOT NULL DEFAULT CURRENT_DATE,
				ADD COLUMN IF NOT EXISTS room_id INTEGER REFERENCES rooms(id) ON DELETE CASCADE;
				
				CREATE INDEX IF NOT EXISTS idx_visits_day ON visits(day);
			`)
			if err != nil {
				return err
			}

			// 4. Update Feedback table
			_, err = db.ExecContext(ctx, `
				-- First check if we need to rename
				DO $$
				BEGIN
					IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'feedbacks' AND column_name = 'content') THEN
						ALTER TABLE feedbacks RENAME COLUMN content TO feedback_value;
					END IF;
				END $$;
				
				ALTER TABLE feedbacks
				ADD COLUMN IF NOT EXISTS day DATE NOT NULL DEFAULT CURRENT_DATE,
				ADD COLUMN IF NOT EXISTS time TIME NOT NULL DEFAULT CURRENT_TIME,
				ADD COLUMN IF NOT EXISTS mensa_feedback BOOLEAN NOT NULL DEFAULT FALSE;
				
				-- Only drop type if it exists
				DO $$
				BEGIN
					IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'feedbacks' AND column_name = 'type') THEN
						ALTER TABLE feedbacks DROP COLUMN type;
					END IF;
				END $$;
				
				CREATE INDEX IF NOT EXISTS idx_feedbacks_day ON feedbacks(day);
			`)
			if err != nil {
				return err
			}

			// 5. Update Settings table
			_, err = db.ExecContext(ctx, `
				ALTER TABLE settings
				ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'system',
				ADD COLUMN IF NOT EXISTS requires_restart BOOLEAN NOT NULL DEFAULT FALSE,
				ADD COLUMN IF NOT EXISTS requires_db_reset BOOLEAN NOT NULL DEFAULT FALSE;
			`)
			if err != nil {
				return err
			}

			// 6. Create additional indexes for performance
			_, err = db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_students_school_class ON students(school_class);
				CREATE INDEX IF NOT EXISTS idx_students_in_house ON students(in_house);
				CREATE INDEX IF NOT EXISTS idx_room_name ON rooms(room_name);
				CREATE INDEX IF NOT EXISTS idx_ag_name ON ags(name);
				CREATE INDEX IF NOT EXISTS idx_ag_category_name ON ag_categories(name);
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 7: Dropping added tables and reverting updates...")

			// 1. Drop Room_history table
			_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS room_history CASCADE`)
			if err != nil {
				return err
			}

			// 2. Drop indexes
			_, err = db.ExecContext(ctx, `
				DROP INDEX IF EXISTS idx_room_history_day;
				DROP INDEX IF EXISTS idx_room_history_room_id;
				DROP INDEX IF EXISTS idx_students_school_class;
				DROP INDEX IF EXISTS idx_students_in_house;
				DROP INDEX IF EXISTS idx_room_name;
				DROP INDEX IF EXISTS idx_ag_name;
				DROP INDEX IF EXISTS idx_ag_category_name;
				DROP INDEX IF EXISTS idx_visits_day;
				DROP INDEX IF EXISTS idx_feedbacks_day;
			`)
			if err != nil {
				return err
			}

			// 3. Revert Settings table changes
			_, err = db.ExecContext(ctx, `
				ALTER TABLE settings
				DROP COLUMN IF EXISTS category,
				DROP COLUMN IF EXISTS requires_restart, 
				DROP COLUMN IF EXISTS requires_db_reset;
			`)
			if err != nil {
				return err
			}

			// 4. Revert Feedback table changes
			_, err = db.ExecContext(ctx, `
				ALTER TABLE feedbacks
				RENAME COLUMN feedback_value TO content;
				
				ALTER TABLE feedbacks
				DROP COLUMN IF EXISTS day,
				DROP COLUMN IF EXISTS time,
				DROP COLUMN IF EXISTS mensa_feedback;
				
				ALTER TABLE feedbacks
				ADD COLUMN IF NOT EXISTS type TEXT NOT NULL DEFAULT 'general';
			`)
			if err != nil {
				return err
			}

			// 5. Revert Visit table changes
			_, err = db.ExecContext(ctx, `
				ALTER TABLE visits
				DROP COLUMN IF EXISTS day,
				DROP COLUMN IF EXISTS room_id;
			`)

			return err
		},
	)
}
