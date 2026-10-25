package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerSchoolSessionsVersion = "1.15.556"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerSchoolSessionsVersion,
		Description: "Träger-Portal: Schulsitzungen aus dem Absprung des Träger-Büros (#4654)",
		DependsOn: []string{
			traegerInvitationsVersion, // the last migration before this one
			traegerPortalVersion,      // chk_tokens_session_owner and auth.tokens.organization_id
		},
	})
	Migrations.MustRegister(traegerSchoolSessionsUp, traegerSchoolSessionsDown)
}

// traegerSchoolSessionsUpSQL lets a school session name the Träger it came
// through (ADR 0084 section 5, #4654). The Absprung of the Träger-Büro opens
// a normal OGS session (portal scope tenant) for exactly one school of the
// Träger, without a mapping in auth.account_tenants. The session keeps its
// school in tenant_id and the Träger in organization_id: every refresh
// re-checks the Träger, and ending the Träger's sessions ends it too. School
// and parent sessions still name no Träger; a Träger session no school.
const traegerSchoolSessionsUpSQL = `
ALTER TABLE auth.tokens DROP CONSTRAINT chk_tokens_session_owner;
ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_session_owner CHECK (
	(portal_scope = 'org' AND tenant_id IS NULL AND organization_id IS NOT NULL)
	OR (portal_scope = 'tenant' AND tenant_id IS NOT NULL)
	OR (portal_scope NOT IN ('org', 'tenant') AND tenant_id IS NOT NULL AND organization_id IS NULL)
);
COMMENT ON COLUMN auth.tokens.organization_id IS
	'Träger of an org-scope (Träger-Portal) session, which belongs to no school, or of a school session the Träger-Büro opened through the Absprung (ADR 0084).';
`

// The down migration removes the Absprung sessions before it restores the
// old constraint; they cannot be refreshed without the Träger anyway.
const traegerSchoolSessionsDownSQL = `
DELETE FROM auth.tokens WHERE portal_scope = 'tenant' AND organization_id IS NOT NULL;
ALTER TABLE auth.tokens DROP CONSTRAINT chk_tokens_session_owner;
ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_session_owner CHECK (
	(portal_scope = 'org' AND tenant_id IS NULL AND organization_id IS NOT NULL)
	OR (portal_scope <> 'org' AND tenant_id IS NOT NULL AND organization_id IS NULL)
);
COMMENT ON COLUMN auth.tokens.organization_id IS
	'Träger of an org-scope (Träger-Portal) session; such a session belongs to no school (ADR 0084).';
`

func traegerSchoolSessionsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerSchoolSessionsUpSQL); err != nil {
		return fmt.Errorf("let OGS sessions name the Träger of their Absprung: %w", err)
	}
	return nil
}

func traegerSchoolSessionsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerSchoolSessionsDownSQL); err != nil {
		return fmt.Errorf("restore the session owner check without Absprung sessions: %w", err)
	}
	return nil
}
