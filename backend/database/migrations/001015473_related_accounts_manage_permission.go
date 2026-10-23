package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	relatedAccountsManagePermissionVersion     = "1.15.473"
	relatedAccountsManagePermissionDescription = "Grant parent_portal.related_accounts.manage to full guardians (#4603)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     relatedAccountsManagePermissionVersion,
		Description: relatedAccountsManagePermissionDescription,
		DependsOn: []string{
			guardianOwnerCutoverVersion,            // auth.guardian_student_access holds the permissions
			careOfferingSelectionRulePerDayVersion, // previous head
		},
	})
	Migrations.MustRegister(relatedAccountsManagePermissionUp, relatedAccountsManagePermissionDown)
}

// Inviting a further account to a child or taking one's access away needed
// only parent_portal.access until #4603. The new permission keeps that power
// with the roles that held it in practice: only the full guardian presets
// carry portal access, and they receive the permission from the presets in
// auth/authorize from now on. Existing relationships of those roles get it
// here, so no family loses the action with the deploy.
func relatedAccountsManagePermissionUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		UPDATE auth.guardian_student_access AS access
		SET permissions = access.permissions || '{"parent_portal.related_accounts.manage": true}'::jsonb
		FROM users.student_guardian_relationships AS relationship
		WHERE relationship.tenant_id = access.tenant_id AND relationship.id = access.relationship_id
			AND relationship.guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian')
			AND NOT (access.permissions ? 'parent_portal.related_accounts.manage');
	`)
	if err != nil {
		return fmt.Errorf("grant parent_portal.related_accounts.manage: %w", err)
	}
	return nil
}

// relatedAccountsManagePermissionDown is a deliberate no-op. Code before this
// version never reads the key, so leaving it changes nothing, and a later Up
// skips every relationship that already carries it. Removing it would also
// remove a grant made after the deploy, which no rollback may undo.
func relatedAccountsManagePermissionDown(_ context.Context, _ *bun.DB) error {
	return nil
}
