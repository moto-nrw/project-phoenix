package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayRegistrationRetentionDaysVersion = "1.15.921"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayRegistrationRetentionDaysVersion,
		Description: "Ferienanmeldung: zugesagte Aufbewahrungsfrist je Anmeldung, Gesundheitsangaben im Löschprotokoll (#6691, ADR 0118)",
		DependsOn: []string{
			holidayRegistrationDeletionsVersion, // the deletion log this extends
			holidayHealthDataVersion,            // the sealed health information the log counts
			dataReturnErasureOrderLockVersion,   // the newest on development (1.15.919)
		},
	})
	Migrations.MustRegister(holidayRegistrationRetentionDaysUp, holidayRegistrationRetentionDaysDown)
}

// holidayRegistrationRetentionDaysUpSQL keeps on every registration the
// retention the parents agreed to (#6691, ADR 0118 addendum): the days
// after the last week of the period its consent text names. Existing
// registrations agreed to "drei Monate" and keep 92 days; the default stays
// 92 so a row that names none never outlives that promise. A new
// registration names the retention of its Träger at the submission.
//
// The health information goes 92 days after the period whatever the
// registration keeps. The deletion log counts those values in
// health_records; such a row deletes no registration, so the CHECK on the
// counts admits it.
const holidayRegistrationRetentionDaysUpSQL = `
ALTER TABLE enrollment.holiday_registrations
	ADD COLUMN retention_days INT NOT NULL DEFAULT 92,
	ADD CONSTRAINT chk_holiday_registrations_retention_days CHECK (retention_days BETWEEN 92 AND 1095);
COMMENT ON COLUMN enrollment.holiday_registrations.retention_days IS
	'Days after the last week of the period the registration stays, as its consent text says (#6691); 92 for a registration before.';

ALTER TABLE enrollment.holiday_registration_deletions
	ADD COLUMN health_records INT NOT NULL DEFAULT 0,
	DROP CONSTRAINT chk_holiday_registration_deletions_counts,
	ADD CONSTRAINT chk_holiday_registration_deletions_counts CHECK (
		registrations >= 0 AND children >= 0 AND audit_entries >= 0 AND health_records >= 0
		AND registrations + audit_entries + health_records > 0);
COMMENT ON COLUMN enrollment.holiday_registration_deletions.health_records IS
	'Sealed health values of registrations and children deleted 92 days after the period while their registration stays (#6691).';
`

const holidayRegistrationRetentionDaysDownSQL = `
DELETE FROM enrollment.holiday_registration_deletions WHERE registrations + audit_entries = 0;
ALTER TABLE enrollment.holiday_registration_deletions
	DROP CONSTRAINT chk_holiday_registration_deletions_counts,
	DROP COLUMN health_records,
	ADD CONSTRAINT chk_holiday_registration_deletions_counts CHECK (
		registrations >= 0 AND children >= 0 AND audit_entries >= 0 AND registrations + audit_entries > 0);
ALTER TABLE enrollment.holiday_registrations
	DROP CONSTRAINT chk_holiday_registrations_retention_days,
	DROP COLUMN retention_days;
`

func holidayRegistrationRetentionDaysUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationRetentionDaysUpSQL); err != nil {
		return fmt.Errorf("add holiday registration retention days: %w", err)
	}
	return nil
}

func holidayRegistrationRetentionDaysDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationRetentionDaysDownSQL); err != nil {
		return fmt.Errorf("drop holiday registration retention days: %w", err)
	}
	return nil
}
