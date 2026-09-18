package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentAnnouncementRemindersVersion     = "1.15.386"
	parentAnnouncementRemindersDescription = "Add a scheduled one-off reminder to parent announcements"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentAnnouncementRemindersVersion,
		Description: parentAnnouncementRemindersDescription,
		DependsOn: []string{
			parentAnnouncementsVersion, // users.parent_announcements
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return parentAnnouncementRemindersUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return parentAnnouncementRemindersDown(ctx, db)
		},
	)
}

func parentAnnouncementRemindersUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	// A reminder (#3162) is a second delivery of the SAME announcement at a
	// moment the school picks when writing it: the announcement goes out weeks
	// before the holidays, the reminder the day before the early pick-up.
	//
	//   reminder_at      - the instant the reminder is due (the school enters a
	//                      Berlin calendar day plus a clock time; stored as the
	//                      resulting instant, never as a date)
	//   reminder_text    - optional short wording for the reminder; NULL means
	//                      the reminder repeats the announcement text
	//   reminder_sent_at - stamped exactly once by the scheduler tick that
	//                      claimed the reminder; the idempotency mark that makes
	//                      overlapping ticks and restarts harmless
	//
	// All three are NULL for every existing row: an announcement without a
	// reminder behaves exactly as before.
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.parent_announcements
			ADD COLUMN IF NOT EXISTS reminder_at      TIMESTAMPTZ,
			ADD COLUMN IF NOT EXISTS reminder_text    TEXT,
			ADD COLUMN IF NOT EXISTS reminder_sent_at TIMESTAMPTZ;
	`)
	if err != nil {
		return fmt.Errorf("error adding reminder columns to users.parent_announcements: %w", err)
	}

	// Constraints are added separately (ADD CONSTRAINT has no IF NOT EXISTS) so
	// a re-run on a partially applied schema does not fail. Text and sent mark
	// only make sense next to a reminder moment.
	_, err = tx.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'chk_parent_announcements_reminder_text_needs_at'
			) THEN
				ALTER TABLE users.parent_announcements
					ADD CONSTRAINT chk_parent_announcements_reminder_text_needs_at
					CHECK (reminder_text IS NULL OR reminder_at IS NOT NULL);
			END IF;

			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'chk_parent_announcements_reminder_sent_needs_at'
			) THEN
				ALTER TABLE users.parent_announcements
					ADD CONSTRAINT chk_parent_announcements_reminder_sent_needs_at
					CHECK (reminder_sent_at IS NULL OR reminder_at IS NOT NULL);
			END IF;
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("error adding reminder constraints to users.parent_announcements: %w", err)
	}

	// The scheduler scans every few minutes for reminders that fell due and
	// were not sent yet. The partial index keeps that scan to the handful of
	// rows that can still fire instead of every announcement ever written.
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_parent_announcements_reminder_due
			ON users.parent_announcements (tenant_id, reminder_at)
			WHERE reminder_at IS NOT NULL AND reminder_sent_at IS NULL AND published_at IS NOT NULL AND active;
	`)
	if err != nil {
		return fmt.Errorf("error creating reminder index on users.parent_announcements: %w", err)
	}

	return tx.Commit()
}

func parentAnnouncementRemindersDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	if _, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_parent_announcements_reminder_due;
		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_reminder_sent_needs_at,
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_reminder_text_needs_at,
			DROP COLUMN IF EXISTS reminder_sent_at,
			DROP COLUMN IF EXISTS reminder_text,
			DROP COLUMN IF EXISTS reminder_at;
	`); err != nil {
		return fmt.Errorf("error dropping reminder columns from users.parent_announcements: %w", err)
	}

	return tx.Commit()
}
