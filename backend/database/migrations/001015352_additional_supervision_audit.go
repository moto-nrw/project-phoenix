package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	additionalSupervisionAuditVersion     = "1.15.352"
	additionalSupervisionAuditDescription = "Audit additional active supervisors"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: additionalSupervisionAuditVersion, Description: additionalSupervisionAuditDescription,
		DependsOn: []string{operationalOverviewTwoModesVersion},
	})
	Migrations.MustRegister(additionalSupervisionAuditUp, additionalSupervisionAuditDown)
}

func additionalSupervisionAuditUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE audit.substitution_changes
			DROP CONSTRAINT IF EXISTS substitution_changes_target_type_check;
		ALTER TABLE audit.substitution_changes
			DROP CONSTRAINT IF EXISTS chk_substitution_changes_target_type;
		ALTER TABLE audit.substitution_changes
			ADD CONSTRAINT chk_substitution_changes_target_type
			CHECK (target_type IN ('group_handover', 'additional_supervision'));
		ALTER TABLE audit.substitution_changes
			ALTER COLUMN end_date DROP NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("extend substitution audit types: %w", err)
	}
	return nil
}

func additionalSupervisionAuditDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		DELETE FROM audit.substitution_changes
		WHERE target_type = 'additional_supervision';
		ALTER TABLE audit.substitution_changes
			ALTER COLUMN end_date SET NOT NULL;
		ALTER TABLE audit.substitution_changes
			DROP CONSTRAINT IF EXISTS chk_substitution_changes_target_type;
		ALTER TABLE audit.substitution_changes
			ADD CONSTRAINT substitution_changes_target_type_check
			CHECK (target_type IN ('group_handover'));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("restore substitution audit types: %w", err)
	}
	return nil
}
