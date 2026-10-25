package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerSettingsVersion = "1.15.552"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerSettingsVersion,
		Description: "Träger-Einstellungen: Werte und Änderungen des Trägers zwischen Standard und Schule (#4652, ADR 0086)",
		DependsOn: []string{
			accountAnalyticsConsentVersion, // the last migration before this one
			configSettingValuesVersion,     // config.setting_values, the school level
			traegerPortalVersion,           // the Träger session that writes the values
		},
	})
	Migrations.MustRegister(traegerSettingsUp, traegerSettingsDown)
}

// traegerSettingsUpSQL creates the Träger level of the settings system (ADR
// 0086 sections 5 and 7):
//
//   - config.traeger_setting_values holds one value per Träger and key. A
//     school resolves registry default, then this value, then its own
//     override in config.setting_values, live on every read.
//   - config.traeger_setting_audit is the append-only change log of the
//     Träger level, written in the same transaction as the change.
//
// Both belong to the Träger, not to a school. The Träger transaction sets
// app.current_org_id under the tenant role; its policy admits only the rows
// of that Träger, for reading and writing. A school's tenant transaction may
// read the values of its own Träger through a second, read-only policy and
// can neither write them nor read the change log. School tables get no second
// policy family (ADR 0084).
const traegerSettingsUpSQL = `
CREATE TABLE config.traeger_setting_values (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	setting_key VARCHAR(255) NOT NULL,
	value JSONB NOT NULL,
	updated_by BIGINT REFERENCES auth.accounts(id),
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT uq_traeger_setting UNIQUE (organization_id, setting_key)
);
COMMENT ON TABLE config.traeger_setting_values IS
	'Träger value of a setting (ADR 0086): between the registry default and the school override, resolved live for every school of the Träger.';

CREATE TRIGGER update_traeger_setting_values_updated_at
	BEFORE UPDATE ON config.traeger_setting_values
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

CREATE TABLE config.traeger_setting_audit (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	setting_key VARCHAR(255) NOT NULL,
	old_value JSONB,
	new_value JSONB,
	action VARCHAR(20) NOT NULL CHECK (action IN ('set', 'reset')),
	changed_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_traeger_setting_audit_key ON config.traeger_setting_audit (organization_id, setting_key, changed_at);
COMMENT ON TABLE config.traeger_setting_audit IS
	'Append-only change log of the Träger values (ADR 0086 section 7); kept three years like config.setting_audit.';

ALTER TABLE config.traeger_setting_values ENABLE ROW LEVEL SECURITY;
ALTER TABLE config.traeger_setting_values FORCE ROW LEVEL SECURITY;
ALTER TABLE config.traeger_setting_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE config.traeger_setting_audit FORCE ROW LEVEL SECURITY;

CREATE POLICY traeger_isolation_config_traeger_setting_values ON config.traeger_setting_values
	FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

CREATE POLICY school_reads_traeger_config_traeger_setting_values ON config.traeger_setting_values
	FOR SELECT
	USING (organization_id = (
		SELECT school.organization_id FROM platform.schools AS school
		WHERE school.id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
	));

CREATE POLICY traeger_isolation_config_traeger_setting_audit ON config.traeger_setting_audit
	FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON config.traeger_setting_values, config.traeger_setting_audit FROM PUBLIC, phoenix_auth;
GRANT SELECT, INSERT, UPDATE, DELETE ON config.traeger_setting_values TO phoenix_tenant;
-- Append-only for the Träger: no UPDATE. DELETE serves only the three-year
-- retention, which runs in the Träger transaction of the next change.
REVOKE UPDATE ON config.traeger_setting_audit FROM phoenix_tenant;
GRANT SELECT, INSERT, DELETE ON config.traeger_setting_audit TO phoenix_tenant;
GRANT USAGE ON SEQUENCE config.traeger_setting_values_id_seq, config.traeger_setting_audit_id_seq TO phoenix_tenant;
GRANT ALL ON config.traeger_setting_values, config.traeger_setting_audit TO phoenix_admin;
GRANT USAGE ON SEQUENCE config.traeger_setting_values_id_seq, config.traeger_setting_audit_id_seq TO phoenix_admin;
`

const traegerSettingsDownSQL = `
DROP TABLE IF EXISTS config.traeger_setting_audit;
DROP TABLE IF EXISTS config.traeger_setting_values;
`

func traegerSettingsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerSettingsUpSQL); err != nil {
		return fmt.Errorf("create Träger settings tables: %w", err)
	}
	return nil
}

func traegerSettingsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerSettingsDownSQL); err != nil {
		return fmt.Errorf("drop Träger settings tables: %w", err)
	}
	return nil
}
