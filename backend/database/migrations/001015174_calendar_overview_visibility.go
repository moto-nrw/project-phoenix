package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	calendarOverviewVisibilityVersion     = "1.15.174"
	calendarOverviewVisibilityDescription = "Add attendee overview visibility to personal calendar appointments"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     calendarOverviewVisibilityVersion,
		Description: calendarOverviewVisibilityDescription,
		DependsOn:   []string{personalCalendarVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.174: Adding calendar overview visibility...")
			if _, err := db.NewRaw(`
				ALTER TABLE calendar.appointments
					ADD COLUMN IF NOT EXISTS overview_visibility TEXT NOT NULL DEFAULT 'organizer';

				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1
						FROM pg_constraint
						WHERE conname = 'chk_calendar_appointments_overview_visibility'
							AND conrelid = 'calendar.appointments'::regclass
					) THEN
						ALTER TABLE calendar.appointments
							ADD CONSTRAINT chk_calendar_appointments_overview_visibility
							CHECK (overview_visibility IN ('organizer', 'staff', 'all'));
					END IF;
				END $$;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding calendar overview visibility: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.174: Removing calendar overview visibility...")
			if _, err := db.NewRaw(`
				ALTER TABLE calendar.appointments
					DROP CONSTRAINT IF EXISTS chk_calendar_appointments_overview_visibility;
				ALTER TABLE calendar.appointments
					DROP COLUMN IF EXISTS overview_visibility;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed removing calendar overview visibility: %w", err)
			}
			return nil
		},
	)
}
