package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	activityCategoryManagePermissionVersion     = "1.15.259"
	activityCategoryManagePermissionDescription = "Add activities:manage_categories permission for school-admin category Stammdaten (issue #2131)"

	activityCategoryManagePermissionName = "activities:manage_categories"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activityCategoryManagePermissionVersion,
		Description: activityCategoryManagePermissionDescription,
		DependsOn:   []string{activityCategoryArchivalVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addActivityCategoryManagePermission(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeActivityCategoryManagePermission(ctx, db)
		},
	)
}

// addActivityCategoryManagePermission introduces activities:manage_categories
// and grants it to the admin role only. The existing activities:* permissions
// cannot serve as the gate: migration 1.9.4 granted activities:manage (and
// create/update/delete) to the plain `user` role, so every Betreuer holds
// them. Category Stammdaten are school-wide configuration and must stay with
// the OGS-Leitung, hence a dedicated admin-only permission.
func addActivityCategoryManagePermission(ctx context.Context, db *bun.DB) error {
	if err := grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        activityCategoryManagePermissionName,
		Description: "Manage activity categories (school Stammdaten)",
		Resource:    "activities",
		Action:      "manage_categories",
	}, "admin"); err != nil {
		return err
	}

	return nil
}

func removeActivityCategoryManagePermission(ctx context.Context, db *bun.DB) error {
	if err := dropPermission(ctx, db, activityCategoryManagePermissionName); err != nil {
		return err
	}

	return nil
}
