package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.402",
		Description: "Persist synthetic demo school state outside the sidecar filesystem (#3461)",
		DependsOn:   []string{"1.13.1", "1.14.1"},
	})
	Migrations.MustRegister(demoSchoolStatesUp, demoSchoolStatesDown)
}

func demoSchoolStatesUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
  CREATE TABLE platform.demo_school_states (
   name TEXT PRIMARY KEY,
   tenant_id BIGINT NOT NULL UNIQUE REFERENCES platform.schools(id) ON DELETE CASCADE,
   seed_state JSONB NOT NULL,
   created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
  );
  REVOKE ALL ON platform.demo_school_states FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
 `).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create demo school states: %w", err)
	}
	return nil
}

func demoSchoolStatesDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`DROP TABLE IF EXISTS platform.demo_school_states`).Exec(ctx)
	return err
}
