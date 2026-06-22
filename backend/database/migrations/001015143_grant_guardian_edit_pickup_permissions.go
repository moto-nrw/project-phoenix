package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	grantGuardianEditPickupPermissionsVersion     = "1.15.143"
	grantGuardianEditPickupPermissionsDescription = "Grant parent_portal.guardian.edit + parent_portal.pickup.manage to guardians that already hold full parent-portal access (#1667)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     grantGuardianEditPickupPermissionsVersion,
		Description: grantGuardianEditPickupPermissionsDescription,
		DependsOn:   []string{backfillStudentEnrollmentRequestChildSourceVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.143: Granting guardian.edit + pickup.manage to full-access guardians...")
			// Every students_guardians row that already grants parent_portal.access
			// is, under the V1 role-derived permission model, a "full" guardian
			// (primary/legal/co preset). Those are exactly the guardians who should
			// gain the new edit + pickup-management capabilities. New relationships
			// receive the keys from the preset (authorize.fullParentPortalPermissions);
			// this backfills the existing ones. Idempotent: re-running just re-sets
			// the same two keys to true.
			_, err := db.NewRaw(`
				UPDATE users.students_guardians
				SET permissions = permissions
					|| '{"parent_portal.guardian.edit": true, "parent_portal.pickup.manage": true}'::jsonb
				WHERE COALESCE((permissions->>'parent_portal.access')::boolean, false) = true
			`).Exec(ctx)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.143: removing guardian.edit + pickup.manage keys...")
			_, err := db.NewRaw(`
				UPDATE users.students_guardians
				SET permissions = permissions
					- 'parent_portal.guardian.edit'
					- 'parent_portal.pickup.manage'
			`).Exec(ctx)
			return err
		},
	)
}
