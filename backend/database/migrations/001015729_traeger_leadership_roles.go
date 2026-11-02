package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerLeadershipRolesVersion = "1.15.729"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerLeadershipRolesVersion,
		Description: "Träger-Rollen Leitung und Schulzugang aus Verwaltung herauslösen (#5471)",
		DependsOn: []string{
			holidayHealthDataVersion,           // the last migration before this one
			authTokenSwitchedFromFamilyVersion, // the other migration before this one
			traegerPortalVersion,               // the Träger role rows
			traegerInvitationsVersion,          // the roles of the open invitations
		},
	})
	Migrations.MustRegister(traegerLeadershipRolesUp, traegerLeadershipRolesDown)
}

// traegerLeadershipRolesUpSQL adds the Träger roles leadership („Leitung“,
// "Konten verwalten") and school_access („Schulzugang“, "In OGS wechseln")
// that #5471 takes out of administration. Nobody loses a right: every
// holder of administration, and every invitation naming it, gets both new
// roles too. The Träger-Büro then decides itself who keeps what. Sessions
// pick the new role set up at their next refresh, which re-reads the roles.
const traegerLeadershipRolesUpSQL = `
ALTER TABLE auth.organization_account_roles DROP CONSTRAINT chk_organization_account_roles_role;
ALTER TABLE auth.organization_account_roles ADD CONSTRAINT chk_organization_account_roles_role
	CHECK (role IN ('administration', 'billing', 'personnel', 'read_only', 'leadership', 'school_access'));
ALTER TABLE auth.traeger_invitations DROP CONSTRAINT chk_traeger_invitations_roles;
ALTER TABLE auth.traeger_invitations ADD CONSTRAINT chk_traeger_invitations_roles CHECK (
	cardinality(roles) > 0
	AND roles <@ ARRAY['administration', 'billing', 'personnel', 'read_only', 'leadership', 'school_access']::VARCHAR(32)[]
);

INSERT INTO auth.organization_account_roles (organization_id, account_id, role)
SELECT held.organization_id, held.account_id, added.role
FROM auth.organization_account_roles AS held
CROSS JOIN (VALUES ('leadership'), ('school_access')) AS added(role)
WHERE held.role = 'administration'
ON CONFLICT (organization_id, account_id, role) DO NOTHING;

UPDATE auth.traeger_invitations
SET roles = ARRAY(
	SELECT DISTINCT role FROM unnest(roles || ARRAY['leadership', 'school_access']::VARCHAR(32)[]) AS role ORDER BY role
)
WHERE 'administration' = ANY(roles);
`

// traegerLeadershipRolesDownSQL restores the role set before #5471. A person
// or invitation with one of the new roles gets administration, which held
// both rights, so the down path takes no right away either.
const traegerLeadershipRolesDownSQL = `
INSERT INTO auth.organization_account_roles (organization_id, account_id, role)
SELECT DISTINCT organization_id, account_id, 'administration'
FROM auth.organization_account_roles
WHERE role IN ('leadership', 'school_access')
ON CONFLICT (organization_id, account_id, role) DO NOTHING;
DELETE FROM auth.organization_account_roles WHERE role IN ('leadership', 'school_access');

UPDATE auth.traeger_invitations
SET roles = ARRAY(
	SELECT DISTINCT role
	FROM unnest(array_remove(array_remove(roles, 'leadership'), 'school_access') || ARRAY['administration']::VARCHAR(32)[]) AS role
	ORDER BY role
)
WHERE roles && ARRAY['leadership', 'school_access']::VARCHAR(32)[];

ALTER TABLE auth.traeger_invitations DROP CONSTRAINT chk_traeger_invitations_roles;
ALTER TABLE auth.traeger_invitations ADD CONSTRAINT chk_traeger_invitations_roles CHECK (
	cardinality(roles) > 0
	AND roles <@ ARRAY['administration', 'billing', 'personnel', 'read_only']::VARCHAR(32)[]
);
ALTER TABLE auth.organization_account_roles DROP CONSTRAINT chk_organization_account_roles_role;
ALTER TABLE auth.organization_account_roles ADD CONSTRAINT chk_organization_account_roles_role
	CHECK (role IN ('administration', 'billing', 'personnel', 'read_only'));
`

func traegerLeadershipRolesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerLeadershipRolesUpSQL); err != nil {
		return fmt.Errorf("add the Träger roles leadership and school_access: %w", err)
	}
	return nil
}

func traegerLeadershipRolesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerLeadershipRolesDownSQL); err != nil {
		return fmt.Errorf("remove the Träger roles leadership and school_access: %w", err)
	}
	return nil
}
