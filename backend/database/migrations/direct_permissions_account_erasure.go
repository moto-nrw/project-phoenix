package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// Frozen definitions for the direct-permission retirement (1.15.646).
func installDirectPermissionAccountErasureUp(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, directPermissionAccountErasureUpSQL)
	return err
}

const directPermissionAccountErasureUpSQL = `		CREATE OR REPLACE FUNCTION auth.erase_unused_accounts()
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
		$$;`

func installDirectPermissionAccountErasureDown(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, directPermissionAccountErasureDownSQL)
	return err
}

const directPermissionAccountErasureDownSQL = `		CREATE OR REPLACE FUNCTION auth.erase_unused_accounts()
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
		$$;`
