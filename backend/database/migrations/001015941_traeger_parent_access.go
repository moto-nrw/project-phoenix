package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerParentAccessVersion = "1.15.941"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerParentAccessVersion,
		Description: "Ferienanmeldung: Elternkonto am Träger ohne Schule (#6673, ADR 0216)",
		DependsOn: []string{
			holidayKnownChildrenVersion,    // the newest on development (1.15.940)
			traegerSchoolSessionsVersion,   // chk_tokens_session_owner as it stands
			retireDirectPermissionsVersion, // auth.account_is_unused as it stands
		},
	})
	Migrations.MustRegister(traegerParentAccessUp, traegerParentAccessDown)
}

// traegerParentAccessUpSQL gives a parent without a school an account at the
// Träger of their holiday registration (ADR 0216 points 6, 7 and 10):
//
//   - auth.traeger_parent_access names the account and the Träger it may
//     open a parents-portal session at (Identity & Access). Setting up the
//     account through the holiday registration's link writes it. Like the
//     Träger roles it has no tenant_id and no RLS policy family: only the
//     administrative role reaches it, and it goes with the account or the
//     Träger.
//   - auth.tokens lets a parent session name that Träger and no school: the
//     session kind traeger_parent.
//   - auth.account_is_unused keeps an account that holds such an access: it
//     has no school, no profile and no child, and the hourly erasure would
//     otherwise anonymize it right after it was set up.
const traegerParentAccessUpSQL = `
CREATE TABLE auth.traeger_parent_access (
	account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (account_id, organization_id)
);
CREATE INDEX idx_traeger_parent_access_organization ON auth.traeger_parent_access (organization_id);
COMMENT ON TABLE auth.traeger_parent_access IS
	'Parents-portal access of an account at a Träger without a school (ADR 0216): set up through the link of a holiday registration; goes with the account or the Träger.';

REVOKE ALL ON auth.traeger_parent_access FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
GRANT SELECT, INSERT, UPDATE, DELETE ON auth.traeger_parent_access TO phoenix_admin;

ALTER TABLE auth.tokens DROP CONSTRAINT chk_tokens_session_owner;
ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_session_owner CHECK (
	(portal_scope = 'org' AND tenant_id IS NULL AND organization_id IS NOT NULL)
	OR (portal_scope = 'tenant' AND tenant_id IS NOT NULL)
	OR (portal_scope = 'parent' AND tenant_id IS NULL AND organization_id IS NOT NULL)
	OR (portal_scope NOT IN ('org', 'tenant') AND tenant_id IS NOT NULL AND organization_id IS NULL)
);
COMMENT ON COLUMN auth.tokens.organization_id IS
	'Träger of an org-scope (Träger-Portal) session or of a parent session at the Träger, both without a school, or of a school session the Träger-Büro opened through the Absprung (ADR 0084, ADR 0216).';
`

// The down migration removes the parent sessions at a Träger before it
// restores the old constraint; without the table they cannot be refreshed.
const traegerParentAccessDownSQL = `
DELETE FROM auth.tokens WHERE portal_scope = 'parent' AND organization_id IS NOT NULL;
ALTER TABLE auth.tokens DROP CONSTRAINT chk_tokens_session_owner;
ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_session_owner CHECK (
	(portal_scope = 'org' AND tenant_id IS NULL AND organization_id IS NOT NULL)
	OR (portal_scope = 'tenant' AND tenant_id IS NOT NULL)
	OR (portal_scope NOT IN ('org', 'tenant') AND tenant_id IS NOT NULL AND organization_id IS NULL)
);
COMMENT ON COLUMN auth.tokens.organization_id IS
	'Träger of an org-scope (Träger-Portal) session, which belongs to no school, or of a school session the Träger-Büro opened through the Absprung (ADR 0084).';
DROP TABLE auth.traeger_parent_access;
`

// traegerParentAccessAccountUnusedSQL is the frozen definition of 1.15.646
// with one more clause at its end: an access at a Träger keeps its account
// only while the Träger is active and not deleted, and only for the 30 days
// its setup link is valid (identityaccess.HolidayParentSetupTTL), the grace
// an open invitation gives an account that has not signed in yet. 1.15.942
// adds the lasting reason, a holiday registration of the account at that
// Träger, once the registration can name the account.
const traegerParentAccessAccountUnusedSQL = `
		CREATE OR REPLACE FUNCTION auth.account_is_unused(target_id BIGINT, target_email TEXT)
		RETURNS BOOLEAN
		LANGUAGE sql
		STABLE
		AS $$
			SELECT NOT EXISTS (
				SELECT 1 FROM auth.account_tenants AS membership
				WHERE membership.account_id = target_id
					AND (
						membership.status = 'pending'
						OR (
							membership.status = 'active'
							AND NOT EXISTS (
								SELECT 1 FROM auth.account_roles AS grant_row
								WHERE grant_row.account_id = membership.account_id
									AND grant_row.tenant_id = membership.tenant_id
							)
						)
						OR (
							membership.status = 'active'
							AND EXISTS (
								SELECT 1 FROM auth.account_roles AS grant_row
								JOIN auth.roles AS granted ON granted.id = grant_row.role_id
								WHERE grant_row.account_id = membership.account_id
									AND grant_row.tenant_id = membership.tenant_id
									AND LOWER(granted.name) <> 'guardian'
							)
						)
					)
			)
			AND NOT EXISTS (SELECT 1 FROM users.persons AS person WHERE person.account_id = target_id)
			AND NOT EXISTS (SELECT 1 FROM users.guardian_profiles AS guardian WHERE guardian.account_id = target_id)
			AND NOT EXISTS (SELECT 1 FROM enrollment.requests AS request WHERE request.guardian_account_id = target_id)
			AND NOT EXISTS (
				SELECT 1 FROM auth.invitation_tokens AS invitation
				WHERE LOWER(invitation.email) = LOWER(target_email)
					AND invitation.used_at IS NULL
					AND invitation.expires_at > statement_timestamp()
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.guardian_invitations AS invitation
				JOIN users.guardian_profiles AS invited ON invited.id = invitation.guardian_profile_id
				WHERE LOWER(invited.email) = LOWER(target_email)
					AND invitation.accepted_at IS NULL
					AND invitation.expires_at > statement_timestamp()
					AND invitation.approval_status <> 'rejected'
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.demo_accesses AS demo
				WHERE demo.account_id = target_id OR demo.parent_account_id = target_id
			)
			AND NOT EXISTS (
				SELECT 1 FROM platform.organization_accounts AS traeger
				WHERE traeger.account_id = target_id AND traeger.status IN ('pending', 'active')
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.caterer_accounts AS caterer_account
				JOIN auth.caterer_schools AS school
					ON school.caterer_id = caterer_account.caterer_id AND school.ended_at IS NULL
				WHERE caterer_account.account_id = target_id
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.caterer_invitations AS invitation
				WHERE LOWER(invitation.email) = LOWER(target_email)
					AND invitation.accepted_at IS NULL
					AND invitation.revoked_at IS NULL
					AND invitation.expires_at > statement_timestamp()
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.traeger_parent_access AS parent_access
				JOIN platform.organizations AS traeger ON traeger.id = parent_access.organization_id
				WHERE parent_access.account_id = target_id
					AND traeger.active AND traeger.deleted_at IS NULL
					AND parent_access.created_at > statement_timestamp() - INTERVAL '30 days'
			);
		$$;
		REVOKE ALL ON FUNCTION auth.account_is_unused(BIGINT, TEXT) FROM PUBLIC;
`

func traegerParentAccessUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, traegerParentAccessUpSQL); err != nil {
			return fmt.Errorf("add the parents-portal access at a Träger: %w", err)
		}
		return installTraegerParentAccessAccountUnused(ctx, tx)
	})
}

// installTraegerParentAccessAccountUnused installs auth.account_is_unused of
// traegerParentAccessVersion; the rollback of 1.15.942 writes it back.
func installTraegerParentAccessAccountUnused(ctx context.Context, tx bun.IDB) error {
	if _, err := tx.ExecContext(ctx, traegerParentAccessAccountUnusedSQL); err != nil {
		return fmt.Errorf("keep accounts with parents-portal access at a Träger: %w", err)
	}
	return nil
}

func traegerParentAccessDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := installDirectPermissionAccountUnusedUp(ctx, tx); err != nil {
			return fmt.Errorf("restore auth.account_is_unused of 1.15.646: %w", err)
		}
		if _, err := tx.ExecContext(ctx, traegerParentAccessDownSQL); err != nil {
			return fmt.Errorf("remove the parents-portal access at a Träger: %w", err)
		}
		return nil
	})
}
