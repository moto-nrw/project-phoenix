package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	groupSeriesRootVersion     = "1.15.183"
	groupSeriesRootDescription = "Track recurring activity-template split lineage for care-offering roster materialization"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     groupSeriesRootVersion,
		Description: groupSeriesRootDescription,
		DependsOn: []string{
			groupTargetGroupVersion,
			compositePKIndexesVersion, // 1.14.4 owns UNIQUE activities.groups(tenant_id, id)
		},
	})
	Migrations.MustRegister(groupSeriesRootUp, groupSeriesRootDown)
}

func groupSeriesRootUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.183: Adding split-series lineage to activities.groups...")
	// The pre-1.15.183 schema stored no predecessor/successor identifier, so an
	// automatic historical backfill would have to guess from mutable template
	// fields or coincident validity boundaries. Keep existing rows unmodified;
	// operators can review the conservative provenance-based candidates in
	// backend/database/audits/legacy_group_series_lineage.sql.
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS series_root_id BIGINT,
			ADD CONSTRAINT chk_activities_groups_series_root_not_self
				CHECK (series_root_id IS NULL OR series_root_id <> id),
			ADD CONSTRAINT fk_activities_groups_series_root_tenant
				FOREIGN KEY (tenant_id, series_root_id)
				REFERENCES activities.groups (tenant_id, id)
				ON DELETE SET NULL (series_root_id);

		CREATE INDEX IF NOT EXISTS idx_activities_groups_tenant_series_root
			ON activities.groups (tenant_id, series_root_id)
			WHERE series_root_id IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding activities.groups split-series lineage: %w", err)
	}
	fmt.Println("Migration 1.15.183: Completed successfully")
	return nil
}

func groupSeriesRootDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.183: Dropping activities.groups split-series lineage...")
	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_activities_groups_tenant_series_root;
		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS fk_activities_groups_series_root_tenant,
			DROP CONSTRAINT IF EXISTS chk_activities_groups_series_root_not_self,
			DROP COLUMN IF EXISTS series_root_id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping activities.groups split-series lineage: %w", err)
	}
	return nil
}
