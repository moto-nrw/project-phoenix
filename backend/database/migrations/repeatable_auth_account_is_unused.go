package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	RegisterRepeatable("auth.account_is_unused", func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, repeatableAuthAccountIsUnusedSQL)
		return err
	})
}

// repeatableAuthAccountIsUnusedSQL is the current definition of
// auth.account_is_unused(bigint,text) (ADR 0223). Change the function
// here; numbered migrations no longer define it.
const repeatableAuthAccountIsUnusedSQL = `CREATE OR REPLACE FUNCTION auth.account_is_unused(target_id bigint, target_email text)
 RETURNS boolean
 LANGUAGE sql
 STABLE
AS $function$
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
					AND (
						parent_access.created_at > statement_timestamp() - INTERVAL '30 days'
						OR EXISTS (
							SELECT 1 FROM enrollment.holiday_registrations AS registration
							WHERE registration.organization_id = parent_access.organization_id
								AND (registration.parent_account_id = target_id
									OR (registration.parent_account_id IS NULL AND LOWER(registration.guardian_email) = LOWER(target_email)))
						)
					)
			);
		$function$
`
