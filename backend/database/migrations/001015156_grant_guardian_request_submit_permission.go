package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	grantGuardianRequestSubmitPermissionVersion     = "1.15.156"
	grantGuardianRequestSubmitPermissionDescription = "Grant parent_portal.request.submit to guardians that already hold full parent-portal access (#1672 review finding #4)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     grantGuardianRequestSubmitPermissionVersion,
		Description: grantGuardianRequestSubmitPermissionDescription,
		DependsOn:   []string{parentMessagingRequestsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.156: Granting parent_portal.request.submit to full-access guardians...")
			// #1672 review finding #4 split structured change-requests off plain chat:
			// they now require parent_portal.request.submit instead of
			// parent_portal.notes.write. New full-guardian relationships receive the key
			// from the preset (authorize.fullParentPortalPermissions); this backfills the
			// EXISTING rows so guardians who could already submit requests under
			// notes.write do not silently lose that ability.
			//
			// Target the full guardian presets by guardian_role directly — the exact same
			// predicate migrations 1.15.137 / 1.15.143 used to grant access and the
			// edit/pickup keys, so the request.submit grant lands on precisely the rows
			// that already carry notes.write. Idempotent; COALESCE guards a stray NULL
			// permissions column (`NULL || jsonb` is NULL in Postgres).
			_, err := db.NewRaw(`
				UPDATE users.students_guardians
				SET permissions = COALESCE(permissions, '{}'::jsonb)
					|| '{"parent_portal.request.submit": true}'::jsonb
				WHERE guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian')
			`).Exec(ctx)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.156: removing parent_portal.request.submit key...")
			// Mirror the up's WHERE clause so the rollback is the inverse of the
			// grant, not a broader revoke. The original down stripped the key from
			// EVERY row, which also de-authorized full-guardian relationships that
			// received request.submit from the app preset (not this migration) — a
			// forward→back→forward cycle would then silently leave newer guardians
			// without it until their next write. Scoping to the same preset roles
			// keeps the rollback symmetric with the grant.
			_, err := db.NewRaw(`
				UPDATE users.students_guardians
				SET permissions = permissions - 'parent_portal.request.submit'
				WHERE guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian')
			`).Exec(ctx)
			return err
		},
	)
}
