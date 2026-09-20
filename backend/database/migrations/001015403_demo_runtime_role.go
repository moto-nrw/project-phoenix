package migrations

import (
	"context"
	"fmt"
	"os"

	"github.com/uptrace/bun"
)

const demoRuntimeRoleVersion = "1.15.403"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoRuntimeRoleVersion,
		Description: "Create the least-privilege database role for the standing demo runtime (#3461)",
		DependsOn:   []string{"1.14.1", "1.15.402"},
	})
	Migrations.MustRegister(demoRuntimeRoleUp, demoRuntimeRoleDown)
}

func demoRuntimeRoleUp(ctx context.Context, db *bun.DB) error {
	password := os.Getenv("PHOENIX_DEMO_PASSWORD")
	if password == "" {
		return fmt.Errorf("PHOENIX_DEMO_PASSWORD is required for the demo runtime role")
	}
	_, err := db.NewRaw(`
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_demo') THEN
				CREATE ROLE phoenix_demo LOGIN NOINHERIT;
			END IF;
		END $$;
		GRANT USAGE ON SCHEMA active, platform TO phoenix_demo;
		GRANT SELECT ON active.visits, active.groups, active.attendance TO phoenix_demo;
		GRANT SELECT, INSERT ON platform.demo_school_states TO phoenix_demo;
		ALTER TABLE platform.demo_school_states ENABLE ROW LEVEL SECURITY;
		ALTER TABLE platform.demo_school_states FORCE ROW LEVEL SECURITY;
		CREATE POLICY demo_runtime_state ON platform.demo_school_states
			FOR ALL TO phoenix_demo
			USING (name = 'messe-demo')
			WITH CHECK (name = 'messe-demo');
		CREATE POLICY demo_runtime_visits ON active.visits
			AS RESTRICTIVE FOR SELECT TO phoenix_demo
			USING (tenant_id = (SELECT tenant_id FROM platform.demo_school_states WHERE name = 'messe-demo'));
		CREATE POLICY demo_runtime_groups ON active.groups
			AS RESTRICTIVE FOR SELECT TO phoenix_demo
			USING (tenant_id = (SELECT tenant_id FROM platform.demo_school_states WHERE name = 'messe-demo'));
		CREATE POLICY demo_runtime_attendance ON active.attendance
			AS RESTRICTIVE FOR SELECT TO phoenix_demo
			USING (tenant_id = (SELECT tenant_id FROM platform.demo_school_states WHERE name = 'messe-demo'));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create demo runtime role: %w", err)
	}
	if _, err := db.ExecContext(ctx, "ALTER ROLE phoenix_demo PASSWORD ?", password); err != nil {
		return fmt.Errorf("set demo runtime role password: %w", err)
	}
	return nil
}

func demoRuntimeRoleDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		DROP POLICY IF EXISTS demo_runtime_attendance ON active.attendance;
		DROP POLICY IF EXISTS demo_runtime_groups ON active.groups;
		DROP POLICY IF EXISTS demo_runtime_visits ON active.visits;
		DROP POLICY IF EXISTS demo_runtime_state ON platform.demo_school_states;
		ALTER TABLE platform.demo_school_states DISABLE ROW LEVEL SECURITY;
		REVOKE SELECT, INSERT ON platform.demo_school_states FROM phoenix_demo;
		REVOKE SELECT ON active.visits, active.groups, active.attendance FROM phoenix_demo;
		REVOKE USAGE ON SCHEMA active, platform FROM phoenix_demo;
		DROP ROLE IF EXISTS phoenix_demo;
	`).Exec(ctx)
	return err
}
