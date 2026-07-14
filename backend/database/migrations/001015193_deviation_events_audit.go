package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	deviationEventsAuditVersion     = "1.15.193"
	deviationEventsAuditDescription = "Create audit.deviation_events: append-only Änderungsprotokoll for planning deviations (#1886)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     deviationEventsAuditVersion,
		Description: deviationEventsAuditDescription,
		DependsOn:   []string{staffShiftFlexibleChangesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.193: Creating audit.deviation_events...")
			if _, err := db.NewRaw(`
				CREATE TABLE IF NOT EXISTS audit.deviation_events (
					id BIGSERIAL PRIMARY KEY,
					tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,

					-- Stable slot anchor across ReplanWeek/template splits. Deliberately
					-- NOT a FK to schedule.activity_instances: re-plans delete and
					-- recreate those rows, and the old #1868 design lost its history to
					-- exactly that cascade. (activity_group_id, occurrence_date,
					-- start_time) is the same slot key snapshotDeviations/
					-- matchRegeneratedInstance use to re-attach deviations after a
					-- re-plan. A deleted activity group legitimately voids its history.
					-- NULL for spontaneous instances (no template) — those are never
					-- deleted by a re-plan, so their instance_id pointer stays stable.
					activity_group_id BIGINT REFERENCES activities.groups(id) ON DELETE CASCADE,
					occurrence_date DATE NOT NULL,
					start_time TIME NOT NULL,

					-- Convenience pointer to the instance the event was written
					-- against; nulled when a re-plan deletes that row. The slot anchor
					-- above stays authoritative.
					instance_id BIGINT REFERENCES schedule.activity_instances(id) ON DELETE SET NULL,

					-- Staff the event is about / brought in as substitute. SET NULL so
					-- offboarding neither blocks nor erases the trail; no name
					-- snapshots — display names resolve at read time (GDPR: IDs only).
					subject_staff_id BIGINT REFERENCES users.staff(id) ON DELETE SET NULL,
					related_staff_id BIGINT REFERENCES users.staff(id) ON DELETE SET NULL,

					-- Acting account; nulled on account deletion, same as
					-- audit.guardian_changes.
					actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,

					-- Open vocabulary (Go constants in models/audit/deviation_event.go);
					-- no CHECK so #1841/#1843/#1884 add event types without a migration.
					event_type TEXT NOT NULL,

					old_value JSONB,
					new_value JSONB,
					reason TEXT,
					occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS idx_deviation_events_slot
					ON audit.deviation_events (tenant_id, activity_group_id, occurrence_date, start_time, occurred_at DESC);
				CREATE INDEX IF NOT EXISTS idx_deviation_events_date
					ON audit.deviation_events (tenant_id, occurrence_date, occurred_at DESC);
				CREATE INDEX IF NOT EXISTS idx_deviation_events_instance
					ON audit.deviation_events (instance_id) WHERE instance_id IS NOT NULL;
				CREATE INDEX IF NOT EXISTS idx_deviation_events_actor
					ON audit.deviation_events (actor_account_id, occurred_at DESC);

				ALTER TABLE audit.deviation_events ENABLE ROW LEVEL SECURITY;
				ALTER TABLE audit.deviation_events FORCE ROW LEVEL SECURITY;
				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1
						FROM pg_policies
						WHERE schemaname = 'audit'
							AND tablename = 'deviation_events'
							AND policyname = 'tenant_isolation_audit_deviation_events'
					) THEN
						CREATE POLICY tenant_isolation_audit_deviation_events
							ON audit.deviation_events
							FOR ALL
							USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
							WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
					END IF;
				END $$;

				-- Append-only at the role level: no UPDATE. DELETE is granted because
				-- the GDPR retention job (TimetableCleanupService) runs per tenant
				-- inside the server's phoenix_tenant transactions, same as
				-- audit.unregistered_tag_scans.
				GRANT SELECT, INSERT, DELETE ON audit.deviation_events TO phoenix_tenant;
				GRANT USAGE ON SEQUENCE audit.deviation_events_id_seq TO phoenix_tenant;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating audit.deviation_events: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.193: Dropping audit.deviation_events...")
			if _, err := db.NewRaw(`
				DROP TABLE IF EXISTS audit.deviation_events CASCADE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping audit.deviation_events: %w", err)
			}
			return nil
		},
	)
}
