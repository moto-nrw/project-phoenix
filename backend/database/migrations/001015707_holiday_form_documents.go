package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayFormDocumentsVersion = "1.15.707"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayFormDocumentsVersion,
		Description: "Ferienanmeldung: Hinweise, PDF-Dokumente und Zustimmungsnachweise (#5199)",
		DependsOn:   []string{holidayPeriodFormsVersion},
	})
	Migrations.MustRegister(holidayFormDocumentsUp, holidayFormDocumentsDown)
}

const holidayFormDocumentsUpSQL = `
ALTER TABLE enrollment.holiday_period_forms
	ADD COLUMN legal_blocks JSONB NOT NULL DEFAULT '[]'::jsonb,
	ADD CONSTRAINT chk_holiday_form_legal_blocks CHECK (jsonb_typeof(legal_blocks) = 'array');
ALTER TABLE enrollment.holiday_registrations
	ADD COLUMN legal_acceptances JSONB NOT NULL DEFAULT '{}'::jsonb,
	ADD CONSTRAINT chk_holiday_legal_acceptances CHECK (jsonb_typeof(legal_acceptances) = 'object');
CREATE TABLE enrollment.holiday_form_documents (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	holiday_period_id BIGINT NOT NULL,
	filename TEXT NOT NULL,
	content BYTEA NOT NULL,
	size INT NOT NULL CHECK (size = octet_length(content) AND size <= 10485760 AND size > 0),
	created_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_holiday_form_documents_period FOREIGN KEY (organization_id, holiday_period_id)
		REFERENCES enrollment.holiday_periods(organization_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_holiday_form_documents_period ON enrollment.holiday_form_documents(organization_id, holiday_period_id);
ALTER TABLE enrollment.holiday_form_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_form_documents FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_holiday_form_documents ON enrollment.holiday_form_documents FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
REVOKE ALL ON enrollment.holiday_form_documents FROM PUBLIC, phoenix_tenant;
GRANT SELECT, INSERT, DELETE ON enrollment.holiday_form_documents TO phoenix_tenant;
GRANT USAGE ON SEQUENCE enrollment.holiday_form_documents_id_seq TO phoenix_tenant;
COMMENT ON TABLE enrollment.holiday_form_documents IS
	'Immutable PDFs of a holiday period, including historic form versions; Enrollment owns the lifecycle (#5199, ADR 0105).';
`

func holidayFormDocumentsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayFormDocumentsUpSQL); err != nil {
		return fmt.Errorf("add holiday form documents: %w", err)
	}
	return nil
}
func holidayFormDocumentsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS enrollment.holiday_form_documents;
		ALTER TABLE enrollment.holiday_registrations DROP CONSTRAINT IF EXISTS chk_holiday_legal_acceptances, DROP COLUMN IF EXISTS legal_acceptances;
		ALTER TABLE enrollment.holiday_period_forms DROP CONSTRAINT IF EXISTS chk_holiday_form_legal_blocks, DROP COLUMN IF EXISTS legal_blocks;`)
	return err
}
