package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerSettingOperatorChangesVersion = "1.15.753"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerSettingOperatorChangesVersion,
		Description: "Träger-Einstellungen: Änderungen eines Operators im Änderungsprotokoll (#5748)",
		DependsOn: []string{
			traegerDocumentTemplatesVersion, // the last migration before this one
			traegerSettingsVersion,          // the change log this extends
		},
	})
	Migrations.MustRegister(traegerSettingOperatorChangesUp, traegerSettingOperatorChangesDown)
}

// traegerSettingOperatorChangesUpSQL lets the change log of the Träger values
// name an operator (#5748). changed_by names an account of the Träger-Büro;
// an operator is no account, so its change names it in changed_by_operator.
// A change has at most one of the two.
const traegerSettingOperatorChangesUpSQL = `
ALTER TABLE config.traeger_setting_audit
	ADD COLUMN changed_by_operator BIGINT REFERENCES platform.operators(id) ON DELETE SET NULL;
ALTER TABLE config.traeger_setting_audit
	ADD CONSTRAINT chk_traeger_setting_audit_one_actor CHECK (changed_by IS NULL OR changed_by_operator IS NULL);
COMMENT ON COLUMN config.traeger_setting_audit.changed_by_operator IS
	'The operator who made the change from the operator dashboard (#5748); changed_by is then NULL.';
`

const traegerSettingOperatorChangesDownSQL = `
ALTER TABLE config.traeger_setting_audit
	DROP CONSTRAINT IF EXISTS chk_traeger_setting_audit_one_actor;
ALTER TABLE config.traeger_setting_audit
	DROP COLUMN IF EXISTS changed_by_operator;
`

func traegerSettingOperatorChangesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerSettingOperatorChangesUpSQL); err != nil {
		return fmt.Errorf("add the operator to the Träger setting change log: %w", err)
	}
	return nil
}

func traegerSettingOperatorChangesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerSettingOperatorChangesDownSQL); err != nil {
		return fmt.Errorf("drop the operator from the Träger setting change log: %w", err)
	}
	return nil
}
