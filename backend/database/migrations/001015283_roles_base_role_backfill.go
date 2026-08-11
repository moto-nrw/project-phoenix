package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	rolesBaseRoleBackfillVersion     = "1.15.283"
	rolesBaseRoleBackfillDescription = "Backfill auth.roles.base_role for school roles created before the column existed (#2222)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     rolesBaseRoleBackfillVersion,
		Description: rolesBaseRoleBackfillDescription,
		DependsOn:   []string{mfaChallengePortalBindingVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return rolesBaseRoleBackfillUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rolesBaseRoleBackfillDown(ctx, db)
		},
	)
}

// rolesBaseRoleBackfillUp gives every school role a privilege tier.
//
// base_role arrived with migration 1.15.31 and was never backfilled. CreateRole
// has required it for new roles ever since, so the gap is exactly the roles a
// school created before that migration. Everything that asks "what tier is this
// role?" — announcement targeting, the grant policy, and since #2222 the
// question of whether the holder is personnel — reads NULL as "unknown" and has
// to guess.
//
// The tier is derived from what the role can actually do, which is the same
// standard CanGrantRole applies: a role that may manage users is an admin role,
// everything else is a staff role.
//
// Guardian is deliberately not derived. A guardian tier is never inferred from
// permissions, guardian access is granted exclusively through the guardian
// invitation flow with the platform guardian role, and no code path today
// recognises a school-defined guardian role without base_role. Classifying one
// as 'user' therefore changes nothing about who may log into the parents
// portal.
//
// System roles are left alone: they carry their tier in their name and
// EffectiveBaseRole reads it there.
func rolesBaseRoleBackfillUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.283: Backfilling auth.roles.base_role for school roles...")

	res, err := db.ExecContext(ctx, `
		UPDATE auth.roles r
		SET base_role = CASE
			WHEN EXISTS (
				SELECT 1
				FROM auth.role_permissions rp
				JOIN auth.permissions p ON p.id = rp.permission_id
				WHERE rp.role_id = r.id
				  AND (
					p.name = 'users:manage'
					OR (p.resource = 'users' AND p.action = 'manage')
					OR (p.resource = 'admin' AND p.action = '*')
					OR (p.resource = '*' AND p.action = '*')
				  )
			) THEN 'admin'
			ELSE 'user'
		END,
		updated_at = NOW()
		WHERE r.base_role IS NULL
		  AND r.is_system = FALSE
		  AND r.tenant_id IS NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("error backfilling base_role: %w", err)
	}

	if affected, affErr := res.RowsAffected(); affErr == nil {
		fmt.Printf("  Classified %d school role(s) by tier\n", affected)
	}

	return nil
}

// rolesBaseRoleBackfillDown is a no-op on purpose: the pre-migration state is
// "tier unknown", and there is no record of which roles were NULL before.
// Resetting every school role to NULL would destroy the tiers CreateRole has
// been requiring since 1.15.31.
func rolesBaseRoleBackfillDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.283: nothing to undo (backfill only, see doc comment)")
	return nil
}
