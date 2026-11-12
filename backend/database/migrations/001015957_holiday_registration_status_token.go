package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayRegistrationStatusTokenVersion = "1.15.957"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayRegistrationStatusTokenVersion,
		Description: "Ferienanmeldung: geheimer Status-Link je Anmeldung (#6676)",
		DependsOn: []string{
			studentNoteTeamNoticesVersion,           // the newest migration on development (1.15.956)
			holidayRegistrationParentAccountVersion, // the last change of the table (1.15.945)
		},
	})
	Migrations.MustRegister(holidayRegistrationStatusTokenUp, holidayRegistrationStatusTokenDown)
}

// holidayRegistrationStatusTokenUpSQL gives a holiday registration the token
// of its status link (#6676). The registration sets it at its submission; a
// registration from before has none and no link. The token lives in the row
// and goes with it when the retention deletes the registration (ADR 0118),
// which ends the link. The unique index serves the lookup by token across
// Träger in the administrative transaction.
const holidayRegistrationStatusTokenUpSQL = `
ALTER TABLE enrollment.holiday_registrations ADD COLUMN status_token TEXT;
CREATE UNIQUE INDEX idx_holiday_registrations_status_token ON enrollment.holiday_registrations (status_token)
	WHERE status_token IS NOT NULL;
COMMENT ON COLUMN enrollment.holiday_registrations.status_token IS
	'Random token of the parents'' status link (#6676); set at the submission, goes with the registration.';
`

const holidayRegistrationStatusTokenDownSQL = `ALTER TABLE enrollment.holiday_registrations DROP COLUMN status_token;`

func holidayRegistrationStatusTokenUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationStatusTokenUpSQL); err != nil {
		return fmt.Errorf("add the status token of holiday registrations: %w", err)
	}
	return nil
}

func holidayRegistrationStatusTokenDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationStatusTokenDownSQL); err != nil {
		return fmt.Errorf("drop the status token of holiday registrations: %w", err)
	}
	return nil
}
