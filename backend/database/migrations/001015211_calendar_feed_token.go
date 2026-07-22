package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	calendarFeedTokenVersion     = "1.15.211"
	calendarFeedTokenDescription = "Add calendar_feed_token to auth.accounts for iCalendar subscription feeds"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     calendarFeedTokenVersion,
		Description: calendarFeedTokenDescription,
		DependsOn:   []string{staffShiftSeriesOccurrenceDateVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.211: Adding calendar_feed_token to auth.accounts...")
			if _, err := db.NewRaw(`
				ALTER TABLE auth.accounts
					ADD COLUMN IF NOT EXISTS calendar_feed_token TEXT;

				CREATE UNIQUE INDEX IF NOT EXISTS uq_accounts_calendar_feed_token
					ON auth.accounts (calendar_feed_token)
					WHERE calendar_feed_token IS NOT NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding calendar_feed_token: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.211: Dropping calendar_feed_token...")
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS auth.uq_accounts_calendar_feed_token;
				ALTER TABLE auth.accounts DROP COLUMN IF EXISTS calendar_feed_token;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping calendar_feed_token: %w", err)
			}
			return nil
		},
	)
}
