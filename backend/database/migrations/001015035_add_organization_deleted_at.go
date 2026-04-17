package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	addOrganizationDeletedAtVersion     = "1.15.35"
	addOrganizationDeletedAtDescription = "Add deleted_at column to platform.organizations and partial indexes for fast non-deleted lookups (orgs + schools by organization_id)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addOrganizationDeletedAtVersion,
		Description: addOrganizationDeletedAtDescription,
		DependsOn:   []string{"1.15.34"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			if _, err := db.ExecContext(ctx, `
				ALTER TABLE platform.organizations
				ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ
			`); err != nil {
				return err
			}
			// Keep CountByIDs(orgs) and "list active organizations" fast by
			// letting the planner skip tombstoned rows via a partial index.
			if _, err := db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_organizations_not_deleted
				ON platform.organizations (id)
				WHERE deleted_at IS NULL
			`); err != nil {
				return err
			}
			// Speeds up SoftDeleteOrganization's pre-check
			// CountNonDeletedByOrganizationID (organization_id = ? AND
			// deleted_at IS NULL) which runs inside a locked transaction.
			_, err := db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_schools_org_not_deleted
				ON platform.schools (organization_id)
				WHERE deleted_at IS NULL
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			if _, err := db.ExecContext(ctx, `
				DROP INDEX IF EXISTS platform.idx_schools_org_not_deleted
			`); err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `
				DROP INDEX IF EXISTS platform.idx_organizations_not_deleted
			`); err != nil {
				return err
			}
			_, err := db.ExecContext(ctx, `
				ALTER TABLE platform.organizations
				DROP COLUMN IF EXISTS deleted_at
			`)
			return err
		},
	)
}
