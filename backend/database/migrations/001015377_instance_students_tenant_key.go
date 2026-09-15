package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const instanceStudentsTenantKeyVersion = "1.15.377"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     instanceStudentsTenantKeyVersion,
		Description: "Prepare the planned participant tenant key for Presence Expand (#2718)",
		DependsOn:   []string{createActivityInstancesVersion},
	})
	Migrations.MustRegister(instanceStudentsTenantKeyUp, instanceStudentsTenantKeyDown)
}

// This separately approved prerequisite is the only change to old storage.
// The existing global primary key already guarantees uniqueness; no rows or
// columns change. Keep the lock wait bounded and roll back a failed build.
func instanceStudentsTenantKeyUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			SET LOCAL statement_timeout = '60s';
			CREATE UNIQUE INDEX IF NOT EXISTS uq_instance_students_tenant_id
				ON schedule.instance_students (tenant_id, id);
		`)
		if err != nil {
			return fmt.Errorf("prepare planned participant tenant key: %w", err)
		}
		return nil
	})
}

func instanceStudentsTenantKeyDown(ctx context.Context, db *bun.DB) error {
	// No CASCADE: refuse to remove a key still used by the expanded schema.
	_, err := db.ExecContext(ctx, `DROP INDEX IF EXISTS schedule.uq_instance_students_tenant_id`)
	return err
}
