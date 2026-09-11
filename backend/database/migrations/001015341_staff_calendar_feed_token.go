package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffCalendarFeedTokenVersion     = "1.15.341"
	staffCalendarFeedTokenDescription = "Add tenant-bound staff iCalendar feed tokens"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffCalendarFeedTokenVersion,
		Description: staffCalendarFeedTokenDescription,
		DependsOn:   []string{parentLetterDeliveryVersion, calendarFeedTokenVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.341: Adding staff calendar feed tokens...")
			if _, err := db.NewRaw(`
				ALTER TABLE auth.account_tenants
					ADD COLUMN IF NOT EXISTS staff_calendar_feed_token TEXT;

				CREATE UNIQUE INDEX IF NOT EXISTS uq_account_tenants_staff_calendar_feed_token
					ON auth.account_tenants (staff_calendar_feed_token)
					WHERE staff_calendar_feed_token IS NOT NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding staff calendar feed token: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.341: Dropping staff calendar feed tokens...")
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS auth.uq_account_tenants_staff_calendar_feed_token;
				ALTER TABLE auth.account_tenants DROP COLUMN IF EXISTS staff_calendar_feed_token;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping staff calendar feed token: %w", err)
			}
			return nil
		},
	)
}
