package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	activityCategoryManagePermissionVersion     = "1.15.258"
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
	fmt.Println("Migration 1.15.258: Adding activities:manage_categories permission...")

	if err := grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        activityCategoryManagePermissionName,
		Description: "Manage activity categories (school Stammdaten)",
		Resource:    "activities",
		Action:      "manage_categories",
	}, "admin"); err != nil {
		return err
	}

	fmt.Println("Migration 1.15.258: Successfully granted activities:manage_categories to admin role")
	return nil
}

func removeActivityCategoryManagePermission(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.258: Removing activities:manage_categories...")

	if err := dropPermission(ctx, db, activityCategoryManagePermissionName); err != nil {
		return err
	}

	fmt.Println("Migration 1.15.258: Successfully rolled back")
	return nil
}
