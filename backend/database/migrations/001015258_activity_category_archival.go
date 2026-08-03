package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	activityCategoryArchivalVersion     = "1.15.258"
	activityCategoryArchivalDescription = "Add archived_at to activities.categories and scope the tenant/name unique index to active rows (issue #2131)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activityCategoryArchivalVersion,
		Description: activityCategoryArchivalDescription,
		DependsOn:   []string{addIsSystemFlagsVersion},
	})

	Migrations.MustRegister(activityCategoryArchivalUp, activityCategoryArchivalDown)
}

func activityCategoryArchivalUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.258: Adding archived_at to activities.categories...")

	if _, err := db.NewRaw(`
				ALTER TABLE activities.categories
				ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
			`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding archived_at column to activities.categories: %w", err)
	}

	// Scope uniqueness to active rows so a school can archive
	// "Kochen" and later create a fresh category under the same
	// name. Restoring an archived row whose name is taken by an
	// active one still fails on this index — that is the intended
	// guard, surfaced to the user as a name conflict.
	if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS activities.idx_categories_tenant_name;
			`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping idx_categories_tenant_name: %w", err)
	}
	if _, err := db.NewRaw(`
				CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_tenant_name_active
				ON activities.categories(tenant_id, name)
				WHERE archived_at IS NULL;
			`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating idx_categories_tenant_name_active: %w", err)
	}

	return nil
}

func activityCategoryArchivalDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.258...")

	// The old schema cannot represent two categories with the same name.
	// Preserve every category and its references by giving only conflicting
	// archived rows an explicit rollback suffix before restoring the
	// unconditional index. The loop handles the unlikely case where a user
	// already chose the generated suffix for another category.
	if _, err := db.NewRaw(`
				DO $migration$
				BEGIN
					WHILE EXISTS (
						SELECT 1
						FROM activities.categories
						GROUP BY tenant_id, name
						HAVING COUNT(*) > 1
					) LOOP
						UPDATE activities.categories AS c
						SET name = c.name || ' (archiviert #' || c.id || ')'
						WHERE c.archived_at IS NOT NULL
						  AND EXISTS (
							SELECT 1
							FROM activities.categories AS duplicate
							WHERE duplicate.tenant_id = c.tenant_id
							  AND duplicate.name = c.name
							  AND duplicate.id <> c.id
						  );
					END LOOP;
				END
				$migration$;
			`).Exec(ctx); err != nil {
		return fmt.Errorf("failed renaming conflicting archived categories: %w", err)
	}

	if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS activities.idx_categories_tenant_name_active;
			`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping idx_categories_tenant_name_active: %w", err)
	}
	if _, err := db.NewRaw(`
				CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_tenant_name
				ON activities.categories(tenant_id, name);
			`).Exec(ctx); err != nil {
		return fmt.Errorf("failed restoring idx_categories_tenant_name: %w", err)
	}
	if _, err := db.NewRaw(`
				ALTER TABLE activities.categories
				DROP COLUMN IF EXISTS archived_at;
			`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping archived_at column: %w", err)
	}

	return nil
}
