package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const retireDirectPermissionsVersion = "1.15.646"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     retireDirectPermissionsVersion,
		Description: "Remove retired direct account permissions and their cleanup paths (#4485)",
		DependsOn:   []string{"1.15.468", signaturesVersion, traegerPortalVersion, mealCaterersVersion, surveyErasureAfterMealsVersion, "1.15.645"},
	})
	Migrations.MustRegister(retireDirectPermissionsUp, retireDirectPermissionsDown)
}

// PR 1 (#4489) must already be deployed: old application images still use
// this table for cleanup. DROP RESTRICT deliberately refuses unknown SQL
// dependencies instead of silently removing a caller through CASCADE.
func retireDirectPermissionsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := installDirectPermissionCleanup(ctx, tx, true); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
   DROP FUNCTION auth.has_permission(BIGINT, TEXT) RESTRICT;
   DROP FUNCTION auth.has_resource_permission(BIGINT, TEXT, TEXT) RESTRICT;
   DROP TABLE auth.account_permissions RESTRICT;
  `)
		if err != nil {
			return fmt.Errorf("drop retired direct account permissions: %w", err)
		}
		return nil
	})
}

func retireDirectPermissionsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := restoreDirectPermissionsStorage(ctx, tx); err != nil {
			return fmt.Errorf("restore empty direct permission storage: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "auth.account_permissions"); err != nil {
			return err
		}
		return installDirectPermissionCleanup(ctx, tx, false)
	})
}

// Reinstall frozen definitions so erasure guards, security-definer settings
// and grants remain unchanged except for the retired table's statements.
func installDirectPermissionCleanup(ctx context.Context, tx bun.Tx, retire bool) error {
	steps := []func(context.Context, bun.Tx) error{
		installDirectPermissionAccountUnusedUp, installDirectPermissionAccountErasureUp,
		installDirectPermissionDemoErasureUp, installRetiredDirectPermissionsPlan,
		installRetiredDirectPermissionsPreflight, installRetiredDirectPermissionsResidual,
		installRetiredDirectPermissionsDelete, installRetiredDirectPermissionsPreview,
	}
	if !retire {
		steps = []func(context.Context, bun.Tx) error{
			installDirectPermissionAccountUnusedDown, installDirectPermissionAccountErasureDown,
			installDirectPermissionDemoErasureDown, installMealOrderErasurePlan,
			installMealOrderErasurePreflight, installMealOrderErasureResidual,
			installMealOrderErasureDelete, installMealOrderErasurePreview,
		}
	}
	for _, step := range steps {
		if err := step(ctx, tx); err != nil {
			return fmt.Errorf("install direct permission cleanup: %w", err)
		}
	}
	return nil
}
