package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// The erasure of 1.15.736 with the Träger parent access introduced in
// 1.15.944: accounts are anonymized, so ON DELETE CASCADE does not run.
// Keep literal SQL so the architecture check sees every table it writes.
const traegerParentAccessAccountErasureSQL = `
		CREATE OR REPLACE FUNCTION auth.erase_unused_accounts()
		RETURNS TABLE(account_id BIGINT, avatar TEXT)
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		-- Each lock wait ends after 100 ms, far below deadlock_timeout (1 s),
		-- before PostgreSQL could pick the other transaction of a cycle as
		-- its victim (#5546).
		SET lock_timeout = '100ms'
		AS $$
		#variable_conflict use_column
		DECLARE
			candidates BIGINT[];
			candidate BIGINT;
			files TEXT[];
			erased INTEGER := 0;
			tried INTEGER := 0;
			started TIMESTAMPTZ;
		BEGIN
			-- The scan reads tables another transaction may hold exclusively.
			-- Its lock error skips this run with one log line; every account
			-- stays for the next run.
			BEGIN
				-- Random order: no locked account is first in every run.
				SELECT COALESCE(array_agg(account.id ORDER BY random()), '{}') INTO candidates
				FROM auth.accounts AS account
				WHERE account.personal_data_erased_at IS NULL
					AND auth.account_is_unused(account.id, account.email);
			EXCEPTION WHEN lock_not_available OR deadlock_detected THEN
				RAISE LOG 'unused account erasure skipped this run (SQLSTATE %): a table it reads is locked', SQLSTATE;
				RETURN;
			END;

			-- The budget counts from here: the scan reads but locks nothing,
			-- and a slow scan must not use up the time for the accounts.
			started := clock_timestamp();
			FOREACH candidate IN ARRAY candidates
			LOOP
				-- The budget: at most 200 accounts and none started later than
				-- 500 ms after the scan. An account already started is finished.
				IF tried >= 200 OR clock_timestamp() - started > INTERVAL '500 milliseconds' THEN
					EXIT;
				END IF;
				-- One subtransaction per account. When a lock it needs is in use
				-- longer than lock_timeout, or a deadlock is detected anyway, the
				-- account is rolled back untouched, its own lock included, and a
				-- later run erases it (#5546). Neither the run nor the other
				-- transaction fails.
				BEGIN
					-- An account another transaction holds (a login, a role
					-- change, a new reference) is passed over without a wait.
					PERFORM 1 FROM auth.accounts AS account
					WHERE account.id = candidate AND account.personal_data_erased_at IS NULL
					FOR UPDATE SKIP LOCKED;
					IF NOT FOUND THEN
						CONTINUE;
					END IF;
					tried := tried + 1;
					-- Check again under the lock with a fresh snapshot: a reference
					-- committed between the scan and the lock keeps the account.
					IF NOT EXISTS (
						SELECT 1 FROM auth.accounts AS account
						WHERE account.id = candidate
							AND auth.account_is_unused(account.id, account.email)
					) THEN
						CONTINUE;
					END IF;

					-- Both current and legacy profile paths, before either
					-- reference disappears. Global accounts have no single school
					-- owner.
					SELECT COALESCE(array_agg(file.avatar), '{}') INTO files
					FROM (
						SELECT account.avatar FROM auth.accounts AS account WHERE account.id = candidate
						UNION
						SELECT profile.avatar FROM users.profiles AS profile WHERE profile.account_id = candidate
					) AS file
					WHERE NULLIF(file.avatar, '') IS NOT NULL;
					INSERT INTO auth.account_avatar_file_cleanup (avatar)
					SELECT file.avatar FROM unnest(files) AS file(avatar)
					WHERE file.avatar LIKE '/uploads/avatars/%'
					ON CONFLICT (avatar) DO NOTHING;

					DELETE FROM auth.tokens AS target WHERE target.account_id = candidate;
					DELETE FROM auth.traeger_parent_access AS target WHERE target.account_id = candidate;
					DELETE FROM auth.password_reset_tokens AS target WHERE target.account_id = candidate;
					DELETE FROM auth.password_reset_rate_limits AS target
					WHERE LOWER(target.email) IN (SELECT LOWER(account.email) FROM auth.accounts AS account WHERE account.id = candidate);
					DELETE FROM auth.account_roles AS target WHERE target.account_id = candidate;
					DELETE FROM auth.account_tenants AS target WHERE target.account_id = candidate;
					DELETE FROM auth.organization_account_roles AS target WHERE target.account_id = candidate;
					DELETE FROM platform.organization_accounts AS target WHERE target.account_id = candidate;
					DELETE FROM auth.mfa_credentials AS target WHERE target.account_id = candidate;
					DELETE FROM auth.mfa_email_challenges AS target WHERE target.account_id = candidate;
					DELETE FROM auth.mfa_trusted_devices AS target WHERE target.account_id = candidate;
					DELETE FROM auth.mfa_overrides AS target WHERE target.account_id = candidate;
					DELETE FROM auth.passkey_credentials AS target WHERE target.account_id = candidate;
					DELETE FROM auth.passkey_sessions AS target WHERE target.account_id = candidate;
					DELETE FROM users.profiles AS target WHERE target.account_id = candidate;
					DELETE FROM users.notification_preferences AS target WHERE target.account_id = candidate;
					DELETE FROM users.parent_request_rss_feeds AS target WHERE target.account_id = candidate;
					DELETE FROM users.parent_message_count_preferences AS target WHERE target.account_id = candidate;
					DELETE FROM iot.push_subscriptions AS target WHERE target.account_id = candidate;
					DELETE FROM iot.pwa_standalone_usage AS target WHERE target.account_id = candidate;
					DELETE FROM config.home_layouts AS target WHERE target.account_id = candidate;
					DELETE FROM config.school_setup_dismissals AS target WHERE target.account_id = candidate;
					DELETE FROM config.staff_onboardings AS target WHERE target.account_id = candidate;
					DELETE FROM documents.folder_accounts AS target WHERE target.account_id = candidate;
					DELETE FROM enrollment.request_reads AS target WHERE target.account_id = candidate;
					DELETE FROM platform.announcement_views AS target WHERE target.user_id = candidate;
					DELETE FROM schedule.timetable_conflict_acks AS target WHERE target.account_id = candidate;
					DELETE FROM users.parent_announcement_reads AS target WHERE target.account_id = candidate;
					DELETE FROM users.staff_notice_acks AS target WHERE target.account_id = candidate;
					DELETE FROM audit.data_imports AS target WHERE target.imported_by = candidate;
					UPDATE auth.accounts AS account SET
						email = 'geloescht-' || account.id || '@example.invalid',
						username = NULL, active = FALSE, password_hash = NULL, is_password_otp = FALSE,
						last_login = NULL, avatar = NULL, calendar_feed_token = NULL,
						mfa_attempts = 0, mfa_locked_until = NULL,
						personal_data_erased_at = statement_timestamp(), updated_at = statement_timestamp()
					WHERE account.id = candidate;
					erased := erased + 1;

					-- Last, so that no row of a rolled-back account is returned:
					-- one row per erased account, one per avatar file when it has
					-- any.
					RETURN QUERY
						SELECT candidate, file.avatar
						FROM (
							SELECT NULL::TEXT AS avatar WHERE cardinality(files) = 0
							UNION ALL
							SELECT unnest(files)
						) AS file;
				EXCEPTION WHEN lock_not_available OR deadlock_detected THEN
					RAISE LOG 'unused account erasure deferred account % to the next run (SQLSTATE %): a lock it needs is in use', candidate, SQLSTATE;
				END;
			END LOOP;
			-- One line for what this run leaves: deferred, passed over, beyond
			-- the budget, or used again since the scan.
			IF erased < cardinality(candidates) THEN
				RAISE LOG 'unused account erasure erased % of % candidates; the next run checks the rest again', erased, cardinality(candidates);
			END IF;
		END;
		$$;
`

// installTraegerParentAccessAccountErasure installs auth.erase_unused_accounts
// of traegerParentAccessVersion.
func installTraegerParentAccessAccountErasure(ctx context.Context, tx bun.IDB) error {
	if _, err := tx.ExecContext(ctx, traegerParentAccessAccountErasureSQL); err != nil {
		return fmt.Errorf("erase Träger parent access with unused accounts: %w", err)
	}
	return nil
}
