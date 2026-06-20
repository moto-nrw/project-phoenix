package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careOfferingAutoRulesVersion     = "1.15.139"
	careOfferingAutoRulesDescription = "Add care-offering statistics flags and automatic offering trigger rules for enrollment phases."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careOfferingAutoRulesVersion,
		Description: careOfferingAutoRulesDescription,
		DependsOn:   []string{careOfferingsSelectionRulesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.139: Adding care offering auto rules...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.care_offerings
					ADD COLUMN IF NOT EXISTS counts_as_care boolean NOT NULL DEFAULT true,
					ADD COLUMN IF NOT EXISTS auto_add_grade_levels jsonb NOT NULL DEFAULT '[]'::jsonb;

				CREATE TABLE IF NOT EXISTS enrollment.care_offering_auto_triggers (
					id BIGSERIAL PRIMARY KEY,
					tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
					target_care_offering_id BIGINT NOT NULL REFERENCES enrollment.care_offerings(id) ON DELETE CASCADE,
					trigger_care_offering_id BIGINT NOT NULL REFERENCES enrollment.care_offerings(id) ON DELETE CASCADE,
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					CONSTRAINT uq_care_offering_auto_trigger_pair
						UNIQUE (target_care_offering_id, trigger_care_offering_id),
					CONSTRAINT chk_care_offering_auto_trigger_no_self
						CHECK (target_care_offering_id <> trigger_care_offering_id)
				);

				CREATE INDEX IF NOT EXISTS idx_care_offering_auto_triggers_target
					ON enrollment.care_offering_auto_triggers (target_care_offering_id);
				CREATE INDEX IF NOT EXISTS idx_care_offering_auto_triggers_trigger
					ON enrollment.care_offering_auto_triggers (trigger_care_offering_id);

				ALTER TABLE enrollment.request_child_offerings
					ADD COLUMN IF NOT EXISTS manual_selected_days jsonb,
					ADD COLUMN IF NOT EXISTS automatic_selected_days jsonb;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding care offering auto rules: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.care_offering_auto_triggers ENABLE ROW LEVEL SECURITY;
				ALTER TABLE enrollment.care_offering_auto_triggers FORCE ROW LEVEL SECURITY;
				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1
						FROM pg_policies
						WHERE schemaname = 'enrollment'
							AND tablename = 'care_offering_auto_triggers'
							AND policyname = 'tenant_isolation_enrollment_care_offering_auto_triggers'
					) THEN
						CREATE POLICY tenant_isolation_enrollment_care_offering_auto_triggers
							ON enrollment.care_offering_auto_triggers
							USING (tenant_id = current_setting('app.current_tenant_id')::bigint)
							WITH CHECK (tenant_id = current_setting('app.current_tenant_id')::bigint);
					END IF;
				END $$;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed enabling RLS for care offering auto triggers: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.139: Dropping care offering auto rules...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.request_child_offerings
					DROP COLUMN IF EXISTS automatic_selected_days,
					DROP COLUMN IF EXISTS manual_selected_days;
				DROP TABLE IF EXISTS enrollment.care_offering_auto_triggers;
				ALTER TABLE enrollment.care_offerings
					DROP COLUMN IF EXISTS auto_add_grade_levels,
					DROP COLUMN IF EXISTS counts_as_care;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping care offering auto rules: %w", err)
			}
			return nil
		},
	)
}
