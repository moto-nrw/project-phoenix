package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerPortalVersion = "1.15.514"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerPortalVersion,
		Description: "Träger-Portal: Zuordnung der Konten zum Träger, Träger-Rollen und Träger-Sitzungen (#4649)",
		DependsOn: []string{
			schoolPortalClassTripsVersion, // the last migration before this one
			createOrgsAndSchoolsVersion,   // platform.organizations, the Träger
			authTokenPortalScopeVersion,   // the portal scope the Träger session is stored under
		},
	})
	Migrations.MustRegister(traegerPortalUp, traegerPortalDown)
}

// traegerPortalUpSQL creates the Träger-Portal foundation of ADR 0084 (#4649):
//
//   - platform.organization_accounts is the Träger membership of an account
//     (Organisation & Tenancy, #2811, #935). It has the lifecycle of
//     auth.account_tenants: pending, active, inactive with timestamps. An
//     account may belong to several Träger.
//   - auth.organization_account_roles holds the Träger roles of a member
//     (Identity & Access). They are separate from school roles: no school
//     role with tenant_id NULL serves as a Träger role. The rights of each
//     role are a fixed bundle in code (ADR 0084 section 4).
//   - auth.tokens carries the Träger session: portal scope org, the Träger in
//     organization_id and no school. Every other portal keeps its school.
//     Org sessions were never minted before, so no row is affected; any such
//     row could not be refreshed anyway and is removed.
//
// The new tables have no tenant_id: the Träger is not a tenant and gets no
// RLS policy family (ADR 0084). Only the administrative role reaches them.
const traegerPortalUpSQL = `
CREATE TABLE platform.organization_accounts (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
	status VARCHAR(16) NOT NULL DEFAULT 'pending',
	activated_at TIMESTAMPTZ,
	deactivated_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_organization_accounts_member UNIQUE (organization_id, account_id),
	CONSTRAINT chk_organization_accounts_status CHECK (status IN ('pending', 'active', 'inactive')),
	CONSTRAINT chk_organization_accounts_activated CHECK (status <> 'active' OR activated_at IS NOT NULL)
);
CREATE INDEX idx_organization_accounts_account ON platform.organization_accounts (account_id);
COMMENT ON TABLE platform.organization_accounts IS
	'Träger membership of an account (ADR 0084, #4649): the account reaches the Träger-Portal of this organization.';

CREATE TRIGGER update_platform_organization_accounts_updated_at
	BEFORE UPDATE ON platform.organization_accounts
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

CREATE TABLE auth.organization_account_roles (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
	role VARCHAR(32) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_organization_account_roles_role UNIQUE (organization_id, account_id, role),
	CONSTRAINT chk_organization_account_roles_role CHECK (role IN ('administration', 'billing', 'personnel', 'read_only'))
);
CREATE INDEX idx_organization_account_roles_account ON auth.organization_account_roles (account_id);
COMMENT ON TABLE auth.organization_account_roles IS
	'Träger roles of a Träger member (ADR 0084, #4649); the rights of each role are a fixed bundle in Identity & Access.';

REVOKE ALL ON platform.organization_accounts, auth.organization_account_roles
	FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
GRANT SELECT, INSERT, UPDATE, DELETE ON platform.organization_accounts, auth.organization_account_roles TO phoenix_admin;
GRANT USAGE ON SEQUENCE platform.organization_accounts_id_seq, auth.organization_account_roles_id_seq TO phoenix_admin;

DELETE FROM auth.tokens WHERE portal_scope = 'org';
ALTER TABLE auth.tokens
	ADD COLUMN organization_id BIGINT REFERENCES platform.organizations(id) ON DELETE CASCADE;
ALTER TABLE auth.tokens ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE auth.tokens DROP CONSTRAINT chk_tokens_tenant_id_not_null;
ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_session_owner CHECK (
	(portal_scope = 'org' AND tenant_id IS NULL AND organization_id IS NOT NULL)
	OR (portal_scope <> 'org' AND tenant_id IS NOT NULL AND organization_id IS NULL)
);
CREATE INDEX idx_tokens_organization ON auth.tokens (organization_id) WHERE organization_id IS NOT NULL;
COMMENT ON COLUMN auth.tokens.organization_id IS
	'Träger of an org-scope (Träger-Portal) session; such a session belongs to no school (ADR 0084).';

-- An active or pending Träger membership keeps the account in use, like a
-- school mapping: the nightly erasure must not anonymize a person who works
-- only in the Träger-Büro. An ended membership goes with the account.
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
						OR (
							membership.status = 'active'
							AND EXISTS (
								SELECT 1 FROM auth.account_permissions AS permission
								WHERE permission.account_id = membership.account_id
									AND permission.tenant_id = membership.tenant_id
									AND permission.granted
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
			);
		$$;

		CREATE OR REPLACE FUNCTION auth.erase_unused_accounts()
		RETURNS TABLE(account_id BIGINT, avatar TEXT)
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		AS $$
		#variable_conflict use_column
		DECLARE
			erased_ids BIGINT[];
		BEGIN
			-- Lock the candidates; an account a concurrent transaction holds
			-- (an acceptance writing its school access) waits for the next run.
			SELECT COALESCE(array_agg(locked.id), '{}') INTO erased_ids
			FROM (
				SELECT account.id FROM auth.accounts AS account
				WHERE account.personal_data_erased_at IS NULL
					AND auth.account_is_unused(account.id, account.email)
				ORDER BY account.id
				FOR UPDATE SKIP LOCKED
			) AS locked;
			-- Check again under the lock with a fresh snapshot: a reference
			-- committed between the first read and the lock keeps the account.
			SELECT COALESCE(array_agg(account.id), '{}') INTO erased_ids
			FROM auth.accounts AS account
			WHERE account.id = ANY (erased_ids)
				AND auth.account_is_unused(account.id, account.email);
			IF cardinality(erased_ids) = 0 THEN
				RETURN;
			END IF;

			-- Persist both current and legacy profile paths before either reference
			-- disappears. Global accounts have no single school owner.
			INSERT INTO auth.account_avatar_file_cleanup (avatar)
			SELECT files.avatar FROM (
				SELECT account.avatar FROM auth.accounts AS account WHERE account.id = ANY (erased_ids)
				UNION
				SELECT profile.avatar FROM users.profiles AS profile WHERE profile.account_id = ANY (erased_ids)
			) AS files
			WHERE files.avatar LIKE '/uploads/avatars/%'
			ON CONFLICT (avatar) DO NOTHING;

			-- One row per erased account, one per avatar file when it has any.
			RETURN QUERY
				SELECT erased.id, files.avatar
				FROM unnest(erased_ids) AS erased(id)
				LEFT JOIN LATERAL (
					SELECT account.avatar FROM auth.accounts AS account
					WHERE account.id = erased.id AND NULLIF(account.avatar, '') IS NOT NULL
					UNION
					SELECT profile.avatar FROM users.profiles AS profile
					WHERE profile.account_id = erased.id AND NULLIF(profile.avatar, '') IS NOT NULL
				) AS files ON TRUE;

			DELETE FROM auth.tokens AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.password_reset_tokens AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.password_reset_rate_limits AS target
			WHERE LOWER(target.email) IN (SELECT LOWER(account.email) FROM auth.accounts AS account WHERE account.id = ANY (erased_ids));
			DELETE FROM auth.account_roles AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.account_permissions AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.account_tenants AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.organization_account_roles AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM platform.organization_accounts AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.mfa_credentials AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.mfa_email_challenges AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.mfa_trusted_devices AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.mfa_overrides AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.passkey_credentials AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.passkey_sessions AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.profiles AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.notification_preferences AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.parent_request_rss_feeds AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.parent_message_count_preferences AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM iot.push_subscriptions AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM iot.pwa_standalone_usage AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM config.home_layouts AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM config.school_setup_dismissals AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM config.staff_onboardings AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM documents.folder_accounts AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM enrollment.request_reads AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM platform.announcement_views AS target WHERE target.user_id = ANY (erased_ids);
			DELETE FROM schedule.timetable_conflict_acks AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.parent_announcement_reads AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.staff_notice_acks AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM audit.data_imports AS target WHERE target.imported_by = ANY (erased_ids);
			UPDATE auth.accounts AS account SET
				email = 'geloescht-' || account.id || '@example.invalid',
				username = NULL, active = FALSE, password_hash = NULL, is_password_otp = FALSE,
				last_login = NULL, avatar = NULL, calendar_feed_token = NULL,
				mfa_attempts = 0, mfa_locked_until = NULL,
				personal_data_erased_at = statement_timestamp(), updated_at = statement_timestamp()
			WHERE account.id = ANY (erased_ids);
		END;
		$$;


`

const traegerPortalDownSQL = `
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
						OR (
							membership.status = 'active'
							AND EXISTS (
								SELECT 1 FROM auth.account_permissions AS permission
								WHERE permission.account_id = membership.account_id
									AND permission.tenant_id = membership.tenant_id
									AND permission.granted
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
			);
		$$;

		CREATE OR REPLACE FUNCTION auth.erase_unused_accounts()
		RETURNS TABLE(account_id BIGINT, avatar TEXT)
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		AS $$
		#variable_conflict use_column
		DECLARE
			erased_ids BIGINT[];
		BEGIN
			-- Lock the candidates; an account a concurrent transaction holds
			-- (an acceptance writing its school access) waits for the next run.
			SELECT COALESCE(array_agg(locked.id), '{}') INTO erased_ids
			FROM (
				SELECT account.id FROM auth.accounts AS account
				WHERE account.personal_data_erased_at IS NULL
					AND auth.account_is_unused(account.id, account.email)
				ORDER BY account.id
				FOR UPDATE SKIP LOCKED
			) AS locked;
			-- Check again under the lock with a fresh snapshot: a reference
			-- committed between the first read and the lock keeps the account.
			SELECT COALESCE(array_agg(account.id), '{}') INTO erased_ids
			FROM auth.accounts AS account
			WHERE account.id = ANY (erased_ids)
				AND auth.account_is_unused(account.id, account.email);
			IF cardinality(erased_ids) = 0 THEN
				RETURN;
			END IF;

			-- Persist both current and legacy profile paths before either reference
			-- disappears. Global accounts have no single school owner.
			INSERT INTO auth.account_avatar_file_cleanup (avatar)
			SELECT files.avatar FROM (
				SELECT account.avatar FROM auth.accounts AS account WHERE account.id = ANY (erased_ids)
				UNION
				SELECT profile.avatar FROM users.profiles AS profile WHERE profile.account_id = ANY (erased_ids)
			) AS files
			WHERE files.avatar LIKE '/uploads/avatars/%'
			ON CONFLICT (avatar) DO NOTHING;

			-- One row per erased account, one per avatar file when it has any.
			RETURN QUERY
				SELECT erased.id, files.avatar
				FROM unnest(erased_ids) AS erased(id)
				LEFT JOIN LATERAL (
					SELECT account.avatar FROM auth.accounts AS account
					WHERE account.id = erased.id AND NULLIF(account.avatar, '') IS NOT NULL
					UNION
					SELECT profile.avatar FROM users.profiles AS profile
					WHERE profile.account_id = erased.id AND NULLIF(profile.avatar, '') IS NOT NULL
				) AS files ON TRUE;

			DELETE FROM auth.tokens AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.password_reset_tokens AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.password_reset_rate_limits AS target
			WHERE LOWER(target.email) IN (SELECT LOWER(account.email) FROM auth.accounts AS account WHERE account.id = ANY (erased_ids));
			DELETE FROM auth.account_roles AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.account_permissions AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.account_tenants AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.mfa_credentials AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.mfa_email_challenges AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.mfa_trusted_devices AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.mfa_overrides AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.passkey_credentials AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM auth.passkey_sessions AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.profiles AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.notification_preferences AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.parent_request_rss_feeds AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.parent_message_count_preferences AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM iot.push_subscriptions AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM iot.pwa_standalone_usage AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM config.home_layouts AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM config.school_setup_dismissals AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM config.staff_onboardings AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM documents.folder_accounts AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM enrollment.request_reads AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM platform.announcement_views AS target WHERE target.user_id = ANY (erased_ids);
			DELETE FROM schedule.timetable_conflict_acks AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.parent_announcement_reads AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM users.staff_notice_acks AS target WHERE target.account_id = ANY (erased_ids);
			DELETE FROM audit.data_imports AS target WHERE target.imported_by = ANY (erased_ids);
			UPDATE auth.accounts AS account SET
				email = 'geloescht-' || account.id || '@example.invalid',
				username = NULL, active = FALSE, password_hash = NULL, is_password_otp = FALSE,
				last_login = NULL, avatar = NULL, calendar_feed_token = NULL,
				mfa_attempts = 0, mfa_locked_until = NULL,
				personal_data_erased_at = statement_timestamp(), updated_at = statement_timestamp()
			WHERE account.id = ANY (erased_ids);
		END;
		$$;

DELETE FROM auth.tokens WHERE portal_scope = 'org';
ALTER TABLE auth.tokens DROP CONSTRAINT IF EXISTS chk_tokens_session_owner;
ALTER TABLE auth.tokens DROP COLUMN IF EXISTS organization_id;
ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_tenant_id_not_null CHECK (tenant_id IS NOT NULL);
ALTER TABLE auth.tokens ALTER COLUMN tenant_id SET NOT NULL;
DROP TABLE IF EXISTS auth.organization_account_roles;
DROP TABLE IF EXISTS platform.organization_accounts;
`

func traegerPortalUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerPortalUpSQL); err != nil {
		return fmt.Errorf("create Träger portal foundation: %w", err)
	}
	return nil
}

func traegerPortalDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerPortalDownSQL); err != nil {
		return fmt.Errorf("drop Träger portal foundation: %w", err)
	}
	return nil
}
