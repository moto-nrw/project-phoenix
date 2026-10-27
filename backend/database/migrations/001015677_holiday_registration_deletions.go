package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayRegistrationDeletionsVersion = "1.15.677"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayRegistrationDeletionsVersion,
		Description: "Ferienanmeldungen: Löschung drei Monate nach den Ferien steht im Löschprotokoll des Trägers (#4667)",
		DependsOn: []string{
			parentBillingRunExportEventVersion, // the last migration before this one
			holidayRegistrationReviewVersion,   // the registrations and the trail the deletion counts
		},
	})
	Migrations.MustRegister(holidayRegistrationDeletionsUp, holidayRegistrationDeletionsDown)
}

// holidayRegistrationDeletionsUpSQL adds the deletion log of the holiday
// registrations of a Träger (#4667, spec #4611 user story 121, ADR 0118).
// The hourly retention deletes the registrations of a period 92 days after
// its last week and, in the same statement, writes one row per period with
// the counts of what went: no name, no content. audit.data_deletions cannot
// hold it, because it belongs to a school and a registration does not.
//
// The row names the period only by ID, without a foreign key, so it stays
// when the Träger deletes the period; it goes with the Träger or after three
// years, like the deletion log of a school. Only the administrative
// transaction of the hourly job writes and prunes it; the request role has
// no privilege on it. The Träger policy keeps a later reader of the
// Träger-Portal to its own rows.
const holidayRegistrationDeletionsUpSQL = `
CREATE TABLE enrollment.holiday_registration_deletions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	holiday_period_id BIGINT NOT NULL,
	period_ended_on DATE NOT NULL,
	registrations INT NOT NULL,
	children INT NOT NULL,
	audit_entries INT NOT NULL,
	deleted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_holiday_registration_deletions_counts CHECK (
		registrations >= 0 AND children >= 0 AND audit_entries >= 0 AND registrations + audit_entries > 0)
);
CREATE INDEX idx_holiday_registration_deletions_organization
	ON enrollment.holiday_registration_deletions (organization_id, deleted_at);
CREATE INDEX idx_holiday_registration_deletions_deleted_at
	ON enrollment.holiday_registration_deletions (deleted_at);
COMMENT ON TABLE enrollment.holiday_registration_deletions IS
	'Deletion log of the holiday registrations of a Träger (#4667, ADR 0118): counts per period, no names; goes after three years or with the Träger.';

ALTER TABLE enrollment.holiday_registration_deletions ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_registration_deletions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_registration_deletions ON enrollment.holiday_registration_deletions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON enrollment.holiday_registration_deletions FROM PUBLIC, phoenix_tenant;
REVOKE ALL ON SEQUENCE enrollment.holiday_registration_deletions_id_seq FROM PUBLIC, phoenix_tenant;
`

const holidayRegistrationDeletionsDownSQL = `
DROP TABLE IF EXISTS enrollment.holiday_registration_deletions;
`

func holidayRegistrationDeletionsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationDeletionsUpSQL); err != nil {
		return fmt.Errorf("add holiday registration deletion log: %w", err)
	}
	return nil
}

func holidayRegistrationDeletionsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationDeletionsDownSQL); err != nil {
		return fmt.Errorf("drop holiday registration deletion log: %w", err)
	}
	return nil
}
