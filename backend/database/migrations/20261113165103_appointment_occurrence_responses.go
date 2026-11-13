package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const appointmentOccurrenceResponsesVersion = "20261113165103"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     appointmentOccurrenceResponsesVersion,
		Description: "Zusage oder Absage für einen einzelnen Termin einer Terminreihe (#6987)",
		DependsOn: []string{
			personalCalendarVersion, // calendar.appointment_recipients (1.15.173)
		},
	})
	Migrations.MustRegister(appointmentOccurrenceResponsesUp, appointmentOccurrenceResponsesDown)
}

// appointmentOccurrenceResponsesUpSQL lets a recipient answer one date of a
// series differently from the series (#6987).
//
//   - status stays the answer for the whole series and for every date
//     without an own answer. An answer stored before this migration therefore
//     keeps meaning what it meant: it holds for every date of the series.
//   - occurrence_responses maps an occurrence date (YYYY-MM-DD, the date the
//     series puts it on, also when an override moves it) to "accepted" or
//     "declined". A new answer for the whole series empties it.
//   - The answers live on the recipient row: they are read with it, go with
//     it (school erasure, guardian deletion, retention) and are exported with
//     it, so no further table, erasure path or read is needed.
const appointmentOccurrenceResponsesUpSQL = `
ALTER TABLE calendar.appointment_recipients
	ADD COLUMN occurrence_responses JSONB NOT NULL DEFAULT '{}'::jsonb,
	ADD CONSTRAINT chk_calendar_recipients_occurrence_responses CHECK (
		jsonb_typeof(occurrence_responses) = 'object'
		AND NOT jsonb_path_exists(occurrence_responses,
			'$.keyvalue() ? (!(@.key like_regex "^[0-9]{4}-[0-9]{2}-[0-9]{2}$") || (@.value != "accepted" && @.value != "declined"))')
	);
COMMENT ON COLUMN calendar.appointment_recipients.occurrence_responses IS
	'Answers for single dates of a series that differ from status, keyed by occurrence date (YYYY-MM-DD) with accepted or declined (#6987).';
`

const appointmentOccurrenceResponsesDownSQL = `
ALTER TABLE calendar.appointment_recipients
	DROP CONSTRAINT IF EXISTS chk_calendar_recipients_occurrence_responses,
	DROP COLUMN IF EXISTS occurrence_responses;
`

func appointmentOccurrenceResponsesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, appointmentOccurrenceResponsesUpSQL); err != nil {
		return fmt.Errorf("add occurrence responses to appointment recipients: %w", err)
	}
	return nil
}

func appointmentOccurrenceResponsesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, appointmentOccurrenceResponsesDownSQL); err != nil {
		return fmt.Errorf("drop occurrence responses from appointment recipients: %w", err)
	}
	return nil
}
