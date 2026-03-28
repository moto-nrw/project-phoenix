package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	announcementOrgTenantTargetingVersion     = "1.15.21"
	announcementOrgTenantTargetingDescription = "Add target_org_ids and target_tenant_ids columns to platform.announcements for org/tenant targeting"
)

func init() {
	MigrationRegistry[announcementOrgTenantTargetingVersion] = &Migration{
		Version:     announcementOrgTenantTargetingVersion,
		Description: announcementOrgTenantTargetingDescription,
		DependsOn:   []string{"1.15.20"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE platform.announcements
				ADD COLUMN target_org_ids BIGINT[] NOT NULL DEFAULT '{}',
				ADD COLUMN target_tenant_ids BIGINT[] NOT NULL DEFAULT '{}';

				CREATE INDEX idx_announcements_target_org_ids
				ON platform.announcements USING GIN (target_org_ids)
				WHERE target_org_ids != '{}';

				CREATE INDEX idx_announcements_target_tenant_ids
				ON platform.announcements USING GIN (target_tenant_ids)
				WHERE target_tenant_ids != '{}';
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				DROP INDEX IF EXISTS platform.idx_announcements_target_tenant_ids;
				DROP INDEX IF EXISTS platform.idx_announcements_target_org_ids;
				ALTER TABLE platform.announcements
				DROP COLUMN IF EXISTS target_tenant_ids,
				DROP COLUMN IF EXISTS target_org_ids;
			`)
			return err
		},
	)
}
