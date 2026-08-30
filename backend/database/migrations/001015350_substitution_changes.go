package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	substitutionChangesVersion     = "1.15.350"
	substitutionChangesDescription = "Type group substitutions and add their audit trail"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: substitutionChangesVersion, Description: substitutionChangesDescription,
		DependsOn: []string{pwaUsageWriteCapabilityVersion},
	})
	Migrations.MustRegister(substitutionChangesUp, substitutionChangesDown)
}

func substitutionChangesUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE education.group_substitution ADD COLUMN target_type TEXT;
		UPDATE education.group_substitution
		SET target_type = CASE
			WHEN regular_staff_id IS NULL THEN 'group_handover'
			ELSE 'legacy_personnel_substitution'
		END;
		ALTER TABLE education.group_substitution ALTER COLUMN target_type SET NOT NULL;
		ALTER TABLE education.group_substitution
			ADD CONSTRAINT chk_group_substitution_target_type
			CHECK (target_type IN ('group_handover', 'legacy_personnel_substitution'));
		CREATE INDEX idx_group_substitution_target_active
			ON education.group_substitution (tenant_id, target_type, end_date);

		CREATE TABLE audit.substitution_changes (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			substitution_id BIGINT NOT NULL,
			target_type TEXT NOT NULL CHECK (target_type IN ('group_handover')),
			action TEXT NOT NULL CHECK (action IN ('assigned', 'ended')),
			group_id BIGINT NOT NULL,
			target_staff_id BIGINT NOT NULL,
			actor_account_id BIGINT NOT NULL,
			start_date DATE NOT NULL,
			end_date DATE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX idx_substitution_changes_lookup
			ON audit.substitution_changes (tenant_id, substitution_id, created_at);
		GRANT SELECT, INSERT ON audit.substitution_changes TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.substitution_changes_id_seq TO phoenix_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create substitution audit trail: %w", err)
	}
	return provisionTenantRLS(ctx, db, "audit.substitution_changes")
}

func substitutionChangesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		DROP TABLE IF EXISTS audit.substitution_changes;
		ALTER TABLE education.group_substitution DROP CONSTRAINT IF EXISTS chk_group_substitution_target_type;
		ALTER TABLE education.group_substitution DROP COLUMN IF EXISTS target_type;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("drop substitution audit trail: %w", err)
	}
	return nil
}
