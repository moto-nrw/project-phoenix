package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{Version: "1.15.374", Description: "Lease durable staff-offboarding file cleanup (#2709)", DependsOn: []string{"1.15.1", createTenantRolesVersion}})
	Migrations.MustRegister(staffOffboardingCleanupUp, staffOffboardingCleanupDown)
}

func staffOffboardingCleanupUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		CREATE TABLE users.staff_offboarding_cleanup (
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id BIGINT NOT NULL,
			lease_token TEXT,
			lease_expires_at TIMESTAMPTZ,
			next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			attempts INTEGER NOT NULL DEFAULT 0,
			completed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (tenant_id, staff_id),
			CHECK ((lease_token IS NULL) = (lease_expires_at IS NULL))
		);
		CREATE INDEX staff_offboarding_cleanup_pending ON users.staff_offboarding_cleanup
			(tenant_id, next_retry_at, staff_id) WHERE completed_at IS NULL;
		GRANT SELECT, INSERT, UPDATE, DELETE ON users.staff_offboarding_cleanup TO phoenix_tenant;
		GRANT ALL ON users.staff_offboarding_cleanup TO phoenix_admin;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create staff offboarding cleanup: %w", err)
	}
	return provisionTenantRLS(ctx, db, "users.staff_offboarding_cleanup")
}

func staffOffboardingCleanupDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw("DROP TABLE IF EXISTS users.staff_offboarding_cleanup").Exec(ctx)
	return err
}
