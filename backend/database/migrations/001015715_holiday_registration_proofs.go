package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayRegistrationProofsVersion = "1.15.715"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayRegistrationProofsVersion,
		Description: "Ferienanmeldung: Nachweis für den Leistungsbezug direkt hochladen (#5238)",
		DependsOn: []string{
			staffGroupChatsVersion,                   // the last migration before this one
			holidayRegistrationsVersion,              // the registrations and the abuse counters
			holidayRegistrationReviewVersion,         // the audit trail that records each opened proof
			parentBillingHolidayContributionsVersion, // holiday_registrations.benefits_declared
		},
	})
	Migrations.MustRegister(holidayRegistrationProofsUp, holidayRegistrationProofsDown)
}

// holidayRegistrationProofsUpSQL lets parents attach the proof of their
// benefits to a holiday registration (#5238):
//
//   - enrollment.holiday_registration_proofs: one file (PDF, JPEG or PNG,
//     at most 10 MB) a parent uploads through the public link. It belongs to
//     the Träger, not to a school, so the bytes live in the row like the
//     confirmation letters and the logo of a Träger: the private object
//     store only knows school namespaces, which the school erasure and the
//     school export walk. A row starts unclaimed with the SHA-256 of the
//     token the uploader holds; the submission claims it for its
//     registration and clears the hash. Unclaimed rows expire after a day,
//     claimed rows go with their registration, their period or the Träger.
//     The Träger policy keeps every school transaction away from them, so
//     the hosting OGS never sees a proof.
//   - enrollment.holiday_registration_rate_limits: the upload bucket of the
//     public link counts per IP next to the submission buckets.
//   - enrollment.holiday_registration_audit: the action 'proof' records
//     every proof the Träger-Büro opens.
const holidayRegistrationProofsUpSQL = `
CREATE TABLE enrollment.holiday_registration_proofs (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	holiday_period_id BIGINT NOT NULL,
	holiday_registration_id BIGINT,
	token_hash TEXT,
	filename TEXT NOT NULL,
	content_type TEXT NOT NULL,
	size_bytes INT NOT NULL,
	content BYTEA NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	claimed_at TIMESTAMPTZ,
	CONSTRAINT fk_holiday_registration_proofs_period FOREIGN KEY (organization_id, holiday_period_id)
		REFERENCES enrollment.holiday_periods (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_holiday_registration_proofs_registration
		FOREIGN KEY (organization_id, holiday_period_id, holiday_registration_id)
		REFERENCES enrollment.holiday_registrations (organization_id, holiday_period_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_holiday_registration_proofs_token UNIQUE (token_hash),
	CONSTRAINT chk_holiday_registration_proofs_file CHECK (
		BTRIM(filename) <> '' AND char_length(filename) <= 130
		AND content_type IN ('application/pdf', 'image/jpeg', 'image/png')
		AND size_bytes = octet_length(content) AND size_bytes BETWEEN 1 AND 10485760),
	CONSTRAINT chk_holiday_registration_proofs_claim CHECK (
		(holiday_registration_id IS NULL AND claimed_at IS NULL AND token_hash IS NOT NULL)
		OR (holiday_registration_id IS NOT NULL AND claimed_at IS NOT NULL AND token_hash IS NULL))
);
CREATE INDEX idx_holiday_registration_proofs_registration
	ON enrollment.holiday_registration_proofs (holiday_registration_id)
	WHERE holiday_registration_id IS NOT NULL;
CREATE INDEX idx_holiday_registration_proofs_unclaimed
	ON enrollment.holiday_registration_proofs (created_at)
	WHERE holiday_registration_id IS NULL;
CREATE INDEX idx_holiday_registration_proofs_period
	ON enrollment.holiday_registration_proofs (organization_id, holiday_period_id);
COMMENT ON TABLE enrollment.holiday_registration_proofs IS
	'Proof of benefits a parent attaches to a holiday registration (#5238); belongs to the Träger, unclaimed rows expire after a day, claimed rows go with their registration.';

ALTER TABLE enrollment.holiday_registration_proofs ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_registration_proofs FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_registration_proofs ON enrollment.holiday_registration_proofs FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON enrollment.holiday_registration_proofs FROM PUBLIC, phoenix_tenant;
GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.holiday_registration_proofs TO phoenix_tenant;
GRANT USAGE ON SEQUENCE enrollment.holiday_registration_proofs_id_seq TO phoenix_tenant;

ALTER TABLE enrollment.holiday_registration_rate_limits
	DROP CONSTRAINT chk_holiday_registration_rate_limits_key_type;
ALTER TABLE enrollment.holiday_registration_rate_limits
	ADD CONSTRAINT chk_holiday_registration_rate_limits_key_type CHECK (key_type IN ('ip', 'email', 'upload_ip'));

ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof'));
`

// holidayRegistrationProofsDownSQL drops the proofs and the entries the
// restored checks would reject.
const holidayRegistrationProofsDownSQL = `
DELETE FROM enrollment.holiday_registration_audit WHERE action = 'proof';
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen'));

DELETE FROM enrollment.holiday_registration_rate_limits WHERE key_type = 'upload_ip';
ALTER TABLE enrollment.holiday_registration_rate_limits
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_rate_limits_key_type;
ALTER TABLE enrollment.holiday_registration_rate_limits
	ADD CONSTRAINT chk_holiday_registration_rate_limits_key_type CHECK (key_type IN ('ip', 'email'));

DROP TABLE IF EXISTS enrollment.holiday_registration_proofs;
`

func holidayRegistrationProofsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationProofsUpSQL); err != nil {
		return fmt.Errorf("add holiday registration proofs: %w", err)
	}
	return nil
}

func holidayRegistrationProofsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationProofsDownSQL); err != nil {
		return fmt.Errorf("drop holiday registration proofs: %w", err)
	}
	return nil
}
