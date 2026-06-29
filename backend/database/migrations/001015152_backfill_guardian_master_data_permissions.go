package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	backfillGuardianMasterDataPermsVersion     = "1.15.152"
	backfillGuardianMasterDataPermsDescription = "Grant parent_portal.master_data.{edit,request} to existing primary/legal/co guardians"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     backfillGuardianMasterDataPermsVersion,
		Description: backfillGuardianMasterDataPermsDescription,
		DependsOn: []string{
			UsersStudentsGuardiansVersion,          // users.students_guardians.permissions
			studentGuardianRolesVersion,            // guardian_role column populated
			createStudentDataChangeRequestsVersion, // previous branch migration after development
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return backfillGuardianMasterDataPermsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return backfillGuardianMasterDataPermsDown(ctx, db)
		},
	)
}

// backfillGuardianMasterDataPermsUp adds the two new master-data permission keys
// to the stored permissions JSONB of every guardian relationship whose role
// preset already grants the full parent-portal set (primary/legal/co guardian).
// New relationships pick the keys up automatically via
// authorize.StudentGuardianPermissionSet; this migration only repairs rows that
// predate the keys. Superuser connection bypasses RLS, so all tenants update.
func backfillGuardianMasterDataPermsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.152: Backfilling master-data permissions for existing guardians...")

	if _, err := db.NewRaw(`
		UPDATE users.students_guardians
		SET permissions = COALESCE(permissions, '{}'::jsonb)
			|| '{"parent_portal.master_data.edit": true, "parent_portal.master_data.request": true}'::jsonb
		WHERE guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian');
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling guardian master-data permissions: %w", err)
	}

	return nil
}

func backfillGuardianMasterDataPermsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.152: Removing master-data permissions from guardians...")

	if _, err := db.NewRaw(`
		UPDATE users.students_guardians
		SET permissions = (permissions - 'parent_portal.master_data.edit') - 'parent_portal.master_data.request'
		WHERE permissions ? 'parent_portal.master_data.edit'
		   OR permissions ? 'parent_portal.master_data.request';
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing guardian master-data permissions: %w", err)
	}

	return nil
}
