package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerMasterDataVersion = "1.15.612"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerMasterDataVersion,
		Description: "Träger-Portal: Stammdaten des Trägers mit Logo, Ansprechpartner und Bankverbindungen in der Träger-Transaktion (#4653)",
		DependsOn: []string{
			mealCombinedRolePermissionsVersion, // the last migration before this one
			traegerPortalVersion,               // the Träger-Portal this table family serves
		},
	})
	Migrations.MustRegister(traegerMasterDataUp, traegerMasterDataDown)
}

// traegerMasterDataUpSQL creates the master data of a Träger (#4653, spec
// #4611 user story 9, ADR 0112):
//
//   - platform.traeger_profiles: one row per Träger with its logo (PNG or
//     JPEG, at most 1 MiB, ADR 0086 section 1) and its contact person for
//     documents.
//   - platform.traeger_bank_accounts: the bank accounts of the Träger, each
//     with a label; at most one is the default.
//   - platform.traeger_master_data_audit: the append-only log of every read
//     and change of these rows, written in the same transaction.
//
// Colours and the texts of the letterhead and footer are Träger settings
// (config.traeger_setting_values), not columns here.
//
// The tables belong to the Träger, not to a school: they carry
// organization_id and no tenant_id and follow the Träger policy of ADR 0105.
// The request role in the Träger transaction sees and writes only the rows
// of the Träger in app.current_org_id; a school transaction never sets it
// and sees none. The administrative role bypasses RLS as for every table.
// All rows go with the Träger.
const traegerMasterDataUpSQL = `
CREATE TABLE platform.traeger_profiles (
	organization_id BIGINT PRIMARY KEY REFERENCES platform.organizations(id) ON DELETE CASCADE,
	contact_name TEXT NOT NULL DEFAULT '',
	contact_phone TEXT NOT NULL DEFAULT '',
	contact_email TEXT NOT NULL DEFAULT '',
	logo BYTEA,
	logo_content_type VARCHAR(16),
	logo_width INTEGER,
	logo_height INTEGER,
	logo_updated_at TIMESTAMPTZ,
	updated_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_traeger_profiles_contact CHECK (
		char_length(contact_name) <= 120 AND char_length(contact_phone) <= 40 AND char_length(contact_email) <= 254
	),
	CONSTRAINT chk_traeger_profiles_logo CHECK (
		(logo IS NULL AND logo_content_type IS NULL AND logo_width IS NULL AND logo_height IS NULL AND logo_updated_at IS NULL)
		OR (logo IS NOT NULL AND octet_length(logo) BETWEEN 1 AND 1048576
			AND logo_content_type IN ('image/png', 'image/jpeg')
			AND logo_width > 0 AND logo_height > 0 AND logo_updated_at IS NOT NULL)
	)
);
COMMENT ON TABLE platform.traeger_profiles IS
	'Master data of a Träger (#4653, ADR 0112): logo and contact person for documents; read and written in the Träger transaction (ADR 0105).';

CREATE TABLE platform.traeger_bank_accounts (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	label TEXT NOT NULL,
	account_holder TEXT NOT NULL,
	iban VARCHAR(34) NOT NULL,
	bic VARCHAR(11) NOT NULL DEFAULT '',
	bank_name TEXT NOT NULL DEFAULT '',
	is_default BOOLEAN NOT NULL DEFAULT false,
	created_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	updated_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_traeger_bank_accounts_label CHECK (BTRIM(label) <> '' AND char_length(label) <= 80),
	CONSTRAINT chk_traeger_bank_accounts_holder CHECK (BTRIM(account_holder) <> '' AND char_length(account_holder) <= 140),
	CONSTRAINT chk_traeger_bank_accounts_iban CHECK (iban ~ '^[A-Z]{2}[0-9]{2}[A-Z0-9]{11,30}$'),
	CONSTRAINT chk_traeger_bank_accounts_bic CHECK (bic = '' OR bic ~ '^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$'),
	CONSTRAINT chk_traeger_bank_accounts_bank_name CHECK (char_length(bank_name) <= 120)
);
CREATE UNIQUE INDEX uq_traeger_bank_accounts_label ON platform.traeger_bank_accounts (organization_id, lower(label));
CREATE UNIQUE INDEX uq_traeger_bank_accounts_default ON platform.traeger_bank_accounts (organization_id) WHERE is_default;
COMMENT ON TABLE platform.traeger_bank_accounts IS
	'Bank account of a Träger with its label (#4653, ADR 0112); at most one is the default.';

CREATE TABLE platform.traeger_master_data_audit (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	subject VARCHAR(16) NOT NULL,
	subject_id BIGINT,
	action VARCHAR(16) NOT NULL,
	old_value JSONB,
	new_value JSONB,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_traeger_master_data_audit_subject CHECK (subject IN ('master_data', 'contact', 'logo', 'bank_account')),
	CONSTRAINT chk_traeger_master_data_audit_action CHECK (action IN ('read', 'set', 'delete', 'create', 'update'))
);
CREATE INDEX idx_traeger_master_data_audit_organization ON platform.traeger_master_data_audit (organization_id, occurred_at);
COMMENT ON TABLE platform.traeger_master_data_audit IS
	'Append-only log of every read and change of the master data of a Träger (#4653, ADR 0112); kept three years like config.traeger_setting_audit.';

CREATE TRIGGER update_platform_traeger_profiles_updated_at
	BEFORE UPDATE ON platform.traeger_profiles
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();
CREATE TRIGGER update_platform_traeger_bank_accounts_updated_at
	BEFORE UPDATE ON platform.traeger_bank_accounts
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

ALTER TABLE platform.traeger_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.traeger_profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_platform_traeger_profiles ON platform.traeger_profiles FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE platform.traeger_bank_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.traeger_bank_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_platform_traeger_bank_accounts ON platform.traeger_bank_accounts FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE platform.traeger_master_data_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.traeger_master_data_audit FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_platform_traeger_master_data_audit ON platform.traeger_master_data_audit FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON platform.traeger_profiles, platform.traeger_bank_accounts, platform.traeger_master_data_audit
	FROM PUBLIC, phoenix_auth;
GRANT SELECT, INSERT, UPDATE, DELETE ON platform.traeger_profiles, platform.traeger_bank_accounts TO phoenix_tenant;
-- Append-only for the Träger: no UPDATE. DELETE serves only the three-year
-- retention, which runs in the Träger transaction of the next entry.
REVOKE UPDATE ON platform.traeger_master_data_audit FROM phoenix_tenant;
GRANT SELECT, INSERT, DELETE ON platform.traeger_master_data_audit TO phoenix_tenant;
GRANT USAGE ON SEQUENCE platform.traeger_bank_accounts_id_seq, platform.traeger_master_data_audit_id_seq TO phoenix_tenant;
GRANT ALL ON platform.traeger_profiles, platform.traeger_bank_accounts, platform.traeger_master_data_audit TO phoenix_admin;
GRANT USAGE ON SEQUENCE platform.traeger_bank_accounts_id_seq, platform.traeger_master_data_audit_id_seq TO phoenix_admin;
`

const traegerMasterDataDownSQL = `
DROP TABLE IF EXISTS platform.traeger_master_data_audit;
DROP TABLE IF EXISTS platform.traeger_bank_accounts;
DROP TABLE IF EXISTS platform.traeger_profiles;
`

func traegerMasterDataUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerMasterDataUpSQL); err != nil {
		return fmt.Errorf("create Träger master data: %w", err)
	}
	return nil
}

func traegerMasterDataDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerMasterDataDownSQL); err != nil {
		return fmt.Errorf("drop Träger master data: %w", err)
	}
	return nil
}
