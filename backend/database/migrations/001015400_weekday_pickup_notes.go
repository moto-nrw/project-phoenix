package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	weekdayPickupNotesVersion     = "1.15.400"
	weekdayPickupNotesDescription = "Recurring weekday notes without a pickup time (#3369)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     weekdayPickupNotesVersion,
		Description: weekdayPickupNotesDescription,
		// 1.15.39 is the latest change to schedule.student_pickup_notes.
		DependsOn: []string{repairPickupScheduleTenantIDsVersion},
	})

	Migrations.MustRegister(weekdayPickupNotesUp, weekdayPickupNotesDown)
}

// weekdayPickupNotesUp lets a note recur on a weekday instead of belonging to
// one date (#3369). The weekly pickup row cannot carry such a note: its mere
// existence marks the child as expected that weekday, so a note for a day the
// child does not come needs a home that says nothing about attendance.
//
// A row is either dated (note_date) or recurring (weekday), never both. One
// recurring note per child and weekday keeps the weekly plan editor a plain
// field instead of a list.
func weekdayPickupNotesUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logRollbackFailure(ctx, rollbackErr)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE schedule.student_pickup_notes
			ADD COLUMN IF NOT EXISTS weekday SMALLINT;
		ALTER TABLE schedule.student_pickup_notes
			ALTER COLUMN note_date DROP NOT NULL;

		ALTER TABLE schedule.student_pickup_notes
			DROP CONSTRAINT IF EXISTS chk_pickup_note_date_or_weekday;
		ALTER TABLE schedule.student_pickup_notes
			ADD CONSTRAINT chk_pickup_note_date_or_weekday CHECK (
				(note_date IS NOT NULL AND weekday IS NULL)
				OR (note_date IS NULL AND weekday BETWEEN 1 AND 5)
			);

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_pickup_notes_weekday
			ON schedule.student_pickup_notes (tenant_id, student_id, weekday)
			WHERE weekday IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error adding weekday to schedule.student_pickup_notes: %w", err)
	}

	return tx.Commit()
}

// weekdayPickupNotesDown removes the recurring notes first: they have no date
// and would violate the restored NOT NULL.
func weekdayPickupNotesDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DROP INDEX IF EXISTS schedule.uniq_pickup_notes_weekday;
		ALTER TABLE schedule.student_pickup_notes
			DROP CONSTRAINT IF EXISTS chk_pickup_note_date_or_weekday;
		DELETE FROM schedule.student_pickup_notes WHERE note_date IS NULL;
		ALTER TABLE schedule.student_pickup_notes
			ALTER COLUMN note_date SET NOT NULL;
		ALTER TABLE schedule.student_pickup_notes
			DROP COLUMN IF EXISTS weekday;
	`)
	if err != nil {
		return fmt.Errorf("error removing weekday from schedule.student_pickup_notes: %w", err)
	}
	return nil
}
