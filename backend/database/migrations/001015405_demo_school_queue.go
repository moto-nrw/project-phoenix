package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.405",
		Description: "Queue one demo school per demo access in the demo school states (#3463)",
		DependsOn:   []string{"1.15.403", "1.15.404"},
	})
	Migrations.MustRegister(demoSchoolQueueUp, demoSchoolQueueDown)
}

// A row is a queued order until the demo process seeded its school. The
// serving backend may only queue an order and read its progress: column
// grants keep seed_state, which holds credentials, and status out of its
// reach. The demo process sees every demo school instead of one literal slug.
func demoSchoolQueueUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE platform.demo_school_states
			ALTER COLUMN tenant_id DROP NOT NULL,
			ALTER COLUMN seed_state DROP NOT NULL,
			ADD COLUMN status TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('preparing', 'ready', 'failed')),
			ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0,
			ADD COLUMN school_name TEXT NOT NULL DEFAULT '',
			ADD COLUMN person_name TEXT NOT NULL DEFAULT '',
			ADD COLUMN visitor_account_id BIGINT,
			ADD COLUMN claimed_at TIMESTAMPTZ,
			ADD CONSTRAINT demo_school_states_ready_is_seeded
				CHECK (status <> 'ready' OR (tenant_id IS NOT NULL AND seed_state IS NOT NULL));
		CREATE INDEX idx_demo_school_states_queue ON platform.demo_school_states (created_at) WHERE status = 'preparing';

		GRANT UPDATE, DELETE ON platform.demo_school_states TO phoenix_demo;
		DROP POLICY demo_runtime_state ON platform.demo_school_states;
		CREATE POLICY demo_runtime_state ON platform.demo_school_states
			FOR ALL TO phoenix_demo USING (true) WITH CHECK (true);

		-- phoenix_admin bypasses row security, so its reach is cut by columns: a
		-- new row is always a waiting order, because status cannot be written.
		ALTER TABLE platform.demo_school_states ALTER COLUMN status SET DEFAULT 'preparing';
		GRANT SELECT (name, status, tenant_id, visitor_account_id), INSERT (name, school_name, person_name)
			ON platform.demo_school_states TO phoenix_admin;

		ALTER TABLE auth.demo_accesses ADD COLUMN school_slug TEXT NOT NULL DEFAULT 'messe-demo';
		CREATE INDEX idx_demo_accesses_email ON auth.demo_accesses (email);

		DROP POLICY demo_runtime_visits ON active.visits;
		CREATE POLICY demo_runtime_visits ON active.visits
			AS RESTRICTIVE FOR SELECT TO phoenix_demo
			USING (tenant_id IN (SELECT tenant_id FROM platform.demo_school_states));
		DROP POLICY demo_runtime_groups ON active.groups;
		CREATE POLICY demo_runtime_groups ON active.groups
			AS RESTRICTIVE FOR SELECT TO phoenix_demo
			USING (tenant_id IN (SELECT tenant_id FROM platform.demo_school_states));
		DROP POLICY demo_runtime_attendance ON active.attendance;
		CREATE POLICY demo_runtime_attendance ON active.attendance
			AS RESTRICTIVE FOR SELECT TO phoenix_demo
			USING (tenant_id IN (SELECT tenant_id FROM platform.demo_school_states));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("queue demo schools: %w", err)
	}
	return nil
}

func demoSchoolQueueDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		DROP POLICY IF EXISTS demo_runtime_visits ON active.visits;
		CREATE POLICY demo_runtime_visits ON active.visits
			AS RESTRICTIVE FOR SELECT TO phoenix_demo
			USING (tenant_id = (SELECT tenant_id FROM platform.demo_school_states WHERE name = 'messe-demo'));
		DROP POLICY IF EXISTS demo_runtime_groups ON active.groups;
		CREATE POLICY demo_runtime_groups ON active.groups
			AS RESTRICTIVE FOR SELECT TO phoenix_demo
			USING (tenant_id = (SELECT tenant_id FROM platform.demo_school_states WHERE name = 'messe-demo'));
		DROP POLICY IF EXISTS demo_runtime_attendance ON active.attendance;
		CREATE POLICY demo_runtime_attendance ON active.attendance
			AS RESTRICTIVE FOR SELECT TO phoenix_demo
			USING (tenant_id = (SELECT tenant_id FROM platform.demo_school_states WHERE name = 'messe-demo'));
		DROP INDEX IF EXISTS auth.idx_demo_accesses_email;
		ALTER TABLE auth.demo_accesses DROP COLUMN IF EXISTS school_slug;
		REVOKE SELECT (name, status, tenant_id, visitor_account_id), INSERT (name, school_name, person_name)
			ON platform.demo_school_states FROM phoenix_admin;
		DROP POLICY IF EXISTS demo_runtime_state ON platform.demo_school_states;
		CREATE POLICY demo_runtime_state ON platform.demo_school_states
			FOR ALL TO phoenix_demo USING (name = 'messe-demo') WITH CHECK (name = 'messe-demo');
		REVOKE UPDATE, DELETE ON platform.demo_school_states FROM phoenix_demo;
		DELETE FROM platform.demo_school_states WHERE tenant_id IS NULL OR seed_state IS NULL;
		DROP INDEX IF EXISTS platform.idx_demo_school_states_queue;
		ALTER TABLE platform.demo_school_states
			DROP CONSTRAINT IF EXISTS demo_school_states_ready_is_seeded,
			DROP COLUMN IF EXISTS claimed_at,
			DROP COLUMN IF EXISTS visitor_account_id,
			DROP COLUMN IF EXISTS person_name,
			DROP COLUMN IF EXISTS school_name,
			DROP COLUMN IF EXISTS attempts,
			DROP COLUMN IF EXISTS status,
			ALTER COLUMN seed_state SET NOT NULL,
			ALTER COLUMN tenant_id SET NOT NULL;
	`).Exec(ctx)
	return err
}
