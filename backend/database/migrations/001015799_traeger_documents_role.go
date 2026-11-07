package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerDocumentsRoleVersion = "1.15.799"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerDocumentsRoleVersion,
		Description: "Träger-Rolle Dokumente mit eigenem Recht für die Träger-Dokumente (#5992)",
		DependsOn: []string{
			traegerDocumentDeletionVersion, // the last migration before this one
			traegerLeadershipRolesVersion,  // the role checks this one widens
		},
	})
	Migrations.MustRegister(traegerDocumentsRoleUp, traegerDocumentsRoleDown)
}

// traegerDocumentsRoleUpSQL adds the Träger role documents („Dokumente“,
// right traeger_documents:manage) of #5992. Until now the right "Anmeldung
// verwalten" of Verwaltung managed the documents. Nobody loses a right: every
// holder of administration, and every open invitation naming it, gets the
// new role at the same Träger. Sessions pick it up at their next refresh,
// which re-reads the roles.
const traegerDocumentsRoleUpSQL = `
ALTER TABLE auth.organization_account_roles DROP CONSTRAINT chk_organization_account_roles_role;
ALTER TABLE auth.organization_account_roles ADD CONSTRAINT chk_organization_account_roles_role
	CHECK (role IN ('administration', 'billing', 'personnel', 'read_only', 'leadership', 'school_access', 'documents'));
ALTER TABLE auth.traeger_invitations DROP CONSTRAINT chk_traeger_invitations_roles;
ALTER TABLE auth.traeger_invitations ADD CONSTRAINT chk_traeger_invitations_roles CHECK (
	cardinality(roles) > 0
	AND roles <@ ARRAY['administration', 'billing', 'personnel', 'read_only', 'leadership', 'school_access', 'documents']::VARCHAR(32)[]
);

INSERT INTO auth.organization_account_roles (organization_id, account_id, role)
SELECT held.organization_id, held.account_id, 'documents'
FROM auth.organization_account_roles AS held
WHERE held.role = 'administration'
ON CONFLICT (organization_id, account_id, role) DO NOTHING;

UPDATE auth.traeger_invitations
SET roles = ARRAY(
	SELECT DISTINCT role FROM unnest(roles || ARRAY['documents']::VARCHAR(32)[]) AS role ORDER BY role
)
WHERE 'administration' = ANY(roles);
`

// traegerDocumentsRoleDownSQL restores the role set before #5992. The role
// documents goes; an invitation holding only it is withdrawn, since it would
// grant nothing. Without the role the documents fall back to "Anmeldung
// verwalten", so the down path widens no right: a person with documents
// alone does not get Verwaltung.
const traegerDocumentsRoleDownSQL = `
DELETE FROM auth.organization_account_roles WHERE role = 'documents';
DELETE FROM auth.traeger_invitations WHERE roles = ARRAY['documents']::VARCHAR(32)[];
UPDATE auth.traeger_invitations
SET roles = array_remove(roles, 'documents')
WHERE 'documents' = ANY(roles);

ALTER TABLE auth.traeger_invitations DROP CONSTRAINT chk_traeger_invitations_roles;
ALTER TABLE auth.traeger_invitations ADD CONSTRAINT chk_traeger_invitations_roles CHECK (
	cardinality(roles) > 0
	AND roles <@ ARRAY['administration', 'billing', 'personnel', 'read_only', 'leadership', 'school_access']::VARCHAR(32)[]
);
ALTER TABLE auth.organization_account_roles DROP CONSTRAINT chk_organization_account_roles_role;
ALTER TABLE auth.organization_account_roles ADD CONSTRAINT chk_organization_account_roles_role
	CHECK (role IN ('administration', 'billing', 'personnel', 'read_only', 'leadership', 'school_access'));
`

func traegerDocumentsRoleUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerDocumentsRoleUpSQL); err != nil {
		return fmt.Errorf("add the Träger role documents: %w", err)
	}
	return nil
}

func traegerDocumentsRoleDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerDocumentsRoleDownSQL); err != nil {
		return fmt.Errorf("remove the Träger role documents: %w", err)
	}
	return nil
}
