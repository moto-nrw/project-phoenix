package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayRegistrationReviewVersion = "1.15.620"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayRegistrationReviewVersion,
		Description: "Träger-Portal: Ferienanmeldungen prüfen, bestätigen, stornieren, umbuchen, einfrieren und wieder öffnen mit Protokoll (#4663)",
		DependsOn: []string{
			traegerMasterDataVersion,    // the last migration before this one
			holidayRegistrationsVersion, // the registrations this migration reviews
		},
	})
	Migrations.MustRegister(holidayRegistrationReviewUp, holidayRegistrationReviewDown)
}

// holidayRegistrationReviewUpSQL adds the review of the holiday registrations
// by the Träger-Büro (#4663, spec #4611 user stories 35 and 36, ADR 0114):
//
//   - enrollment.holiday_registrations.status: a registration is received,
//     confirmed or cancelled.
//   - enrollment.holiday_registrations.freeze_override: after the
//     registration deadline of its period a registration is frozen. 'frozen'
//     freezes it at once, 'reopened' opens it again; NULL follows the
//     deadline.
//   - idx_holiday_registration_children_duplicate: the lookup of the
//     duplicate hint by period, name and date of birth.
//   - enrollment.holiday_registration_audit: the append-only trail of every
//     read of the registrations' personal data and every change, written in
//     the same Träger transaction. The changes of a registration form its
//     history and go with it; the entries of a list or an export go with the
//     period, both 92 days after its last week (hourly retention).
//
// The table carries organization_id and the Träger policy of ADR 0105. The
// request role may insert and read, not update; it deletes only through the
// cascade of its registration.
const holidayRegistrationReviewUpSQL = `
ALTER TABLE enrollment.holiday_registrations DROP CONSTRAINT chk_holiday_registrations_status;
ALTER TABLE enrollment.holiday_registrations
	ADD CONSTRAINT chk_holiday_registrations_status CHECK (status IN ('received', 'confirmed', 'cancelled'));
ALTER TABLE enrollment.holiday_registrations
	ADD COLUMN freeze_override VARCHAR(16),
	ADD CONSTRAINT chk_holiday_registrations_freeze_override CHECK (freeze_override IN ('frozen', 'reopened'));

-- The duplicate hint looks up the children of a period by name and date of
-- birth for every child it lists; the index keeps that lookup flat.
CREATE INDEX idx_holiday_registration_children_duplicate ON enrollment.holiday_registration_children
	(organization_id, holiday_period_id, lower(first_name), lower(last_name), birth_date);

CREATE TABLE enrollment.holiday_registration_audit (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	holiday_period_id BIGINT NOT NULL,
	holiday_registration_id BIGINT,
	action VARCHAR(16) NOT NULL,
	details JSONB NOT NULL DEFAULT '{}'::jsonb,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_holiday_registration_audit_period FOREIGN KEY (organization_id, holiday_period_id)
		REFERENCES enrollment.holiday_periods (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_holiday_registration_audit_registration
		FOREIGN KEY (organization_id, holiday_period_id, holiday_registration_id)
		REFERENCES enrollment.holiday_registrations (organization_id, holiday_period_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen')),
	CONSTRAINT chk_holiday_registration_audit_subject CHECK (
		(action IN ('list', 'export')) = (holiday_registration_id IS NULL))
);
CREATE INDEX idx_holiday_registration_audit_registration
	ON enrollment.holiday_registration_audit (holiday_registration_id, occurred_at)
	WHERE holiday_registration_id IS NOT NULL;
CREATE INDEX idx_holiday_registration_audit_period
	ON enrollment.holiday_registration_audit (organization_id, holiday_period_id, occurred_at);
COMMENT ON TABLE enrollment.holiday_registration_audit IS
	'Append-only trail of every read and change of the holiday registrations of a Träger (#4663, ADR 0114); goes with its registration or period.';

ALTER TABLE enrollment.holiday_registration_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_registration_audit FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_registration_audit ON enrollment.holiday_registration_audit FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON enrollment.holiday_registration_audit FROM PUBLIC, phoenix_tenant;
GRANT SELECT, INSERT ON enrollment.holiday_registration_audit TO phoenix_tenant;
GRANT USAGE ON SEQUENCE enrollment.holiday_registration_audit_id_seq TO phoenix_tenant;
`

const holidayRegistrationReviewDownSQL = `
DROP TABLE IF EXISTS enrollment.holiday_registration_audit;
DROP INDEX IF EXISTS enrollment.idx_holiday_registration_children_duplicate;
UPDATE enrollment.holiday_registrations SET status = 'received' WHERE status <> 'received';
ALTER TABLE enrollment.holiday_registrations DROP CONSTRAINT IF EXISTS chk_holiday_registrations_freeze_override;
ALTER TABLE enrollment.holiday_registrations DROP COLUMN IF EXISTS freeze_override;
ALTER TABLE enrollment.holiday_registrations DROP CONSTRAINT IF EXISTS chk_holiday_registrations_status;
ALTER TABLE enrollment.holiday_registrations
	ADD CONSTRAINT chk_holiday_registrations_status CHECK (status IN ('received'));
`

func holidayRegistrationReviewUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationReviewUpSQL); err != nil {
		return fmt.Errorf("add holiday registration review: %w", err)
	}
	return nil
}

func holidayRegistrationReviewDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationReviewDownSQL); err != nil {
		return fmt.Errorf("drop holiday registration review: %w", err)
	}
	return nil
}
