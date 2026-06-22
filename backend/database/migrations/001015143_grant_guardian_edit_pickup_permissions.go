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
			// The "full" guardian presets (primary/legal/co) are exactly the
			// guardians who should gain the new edit + pickup-management capabilities.
			// We target them by guardian_role directly — the same predicate migration
			// 1.15.137 used to grant parent_portal.access in the first place (it set
			// access only for these three roles), so this neither over-grants to a
			// stray non-full row that somehow carries access nor under-grants to a
			// full guardian: guardian_role is NOT NULL and deterministically
			// backfilled. New relationships receive the keys from the preset
			// (authorize.fullParentPortalPermissions); this backfills existing ones.
			// Idempotent: re-running just re-sets the same two keys to true.
			// COALESCE the left side: in Postgres `NULL || jsonb` is NULL, so a row
			// with a NULL permissions column would be silently skipped rather than
			// granted. Migration 1.15.137 set these three roles non-null, but
			// COALESCE makes the backfill robust against any stray NULL.
			_, err := db.NewRaw(`
				UPDATE users.students_guardians
				SET permissions = COALESCE(permissions, '{}'::jsonb)
					|| '{"parent_portal.guardian.edit": true, "parent_portal.pickup.manage": true}'::jsonb
				WHERE guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian')
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
