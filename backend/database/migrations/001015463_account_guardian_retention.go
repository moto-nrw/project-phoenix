package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const accountGuardianRetentionVersion = "1.15.463"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     accountGuardianRetentionVersion,
		Description: "Elternkontakte ohne Kind nach der Frist löschen und ungenutzte Konten anonymisieren (#4043)",
		DependsOn: []string{
			attendanceRetentionVersion,      // previous subject check of audit.data_deletions; the last migration before this one
			demoVisitorErasureVersion,       // the account tombstone this one follows
			operatorAccountRetentionVersion, // the hourly account retention beside this one
		},
	})
	Migrations.MustRegister(accountGuardianRetentionUp, accountGuardianRetentionDown)
}

// guardianRetentionKind is the data kind of the deletion log this migration
// adds.
const guardianRetentionKind = "guardian_retention"

// accountGuardianRetentionUpSQL prepares the two deletions of #4043.
//
// Parent contacts without a child (users.guardian_profiles):
//   - childless_since records since when a contact has no relationship to a
//     child. A new contact starts childless at its creation. The nightly run
//     of the school keeps the column in step before it deletes: it clears
//     the mark of a contact with a relationship and marks a contact without
//     one, so a contact counts from the first run that sees it without a
//     child. No trigger does this: the relationship table keeps its
//     no-trigger contract of 1.15.387. Contacts that are childless today
//     count from this migration, because nobody recorded when they lost
//     their last child.
//   - The nightly run of the school deletes a contact once childless_since
//     lies further back than gdpr.guardian_retention_days; it logs the data
//     kind guardian_retention.
//
// Accounts no school needs any more (auth.accounts):
//   - auth.erase_unused_accounts() tombstones every account that no school
//     uses: no pending school access, no active school access unless its
//     only roles are guardian, no person, no parent contact, no enrollment
//     request names it, no open invitation for its e-mail address and no
//     demo access. An active access without any role counts as used. The
//     rows that would go with the account through ON DELETE CASCADE go
//     (sessions, credentials, roles, school mappings, profile, preferences,
//     push subscriptions, read markers, its import protocol); the row stays
//     with a placeholder address and personal_data_erased_at, because
//     messages, protocols and other people's records keep pointing at it
//     (ADR 0065). It returns the avatar files of the erased accounts, so the
//     caller removes them.
//   - The hourly token cleanup calls it as phoenix_admin. The function runs
//     as its owner with a fixed search path; only phoenix_admin may call it.
const accountGuardianRetentionUpSQL = `
		ALTER TABLE users.guardian_profiles ADD COLUMN IF NOT EXISTS childless_since TIMESTAMPTZ DEFAULT NOW();
		UPDATE users.guardian_profiles AS guardian SET childless_since = NULL
		WHERE EXISTS (
			SELECT 1 FROM users.student_guardian_relationships AS link
			WHERE link.guardian_profile_id = guardian.id
		);
		CREATE INDEX IF NOT EXISTS idx_guardian_profiles_childless_since
			ON users.guardian_profiles (tenant_id, childless_since)
			WHERE childless_since IS NOT NULL;

		ALTER TABLE auth.accounts ADD COLUMN IF NOT EXISTS personal_data_erased_at TIMESTAMPTZ;

		-- Every account reference keeps an account. Take the same account-row
		-- lock as erasure before writing one, then verify that the account still
		-- is live. This lives at the database boundary so account administration,
		-- provisioning and both People Directory writers cannot bypass the fence.
		CREATE OR REPLACE FUNCTION auth.lock_live_account_reference()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		SECURITY DEFINER
		SET search_path = pg_catalog, auth
		AS $$
		BEGIN
			IF NEW.account_id IS NULL THEN
				RETURN NEW;
			END IF;
			PERFORM 1 FROM auth.accounts
			WHERE id = NEW.account_id AND personal_data_erased_at IS NULL
			FOR KEY SHARE;
			IF NOT FOUND THEN
				RAISE EXCEPTION 'cannot reference an erased account'
					USING ERRCODE = '23503';
			END IF;
			RETURN NEW;
		END;
		$$;
		REVOKE ALL ON FUNCTION auth.lock_live_account_reference() FROM PUBLIC;
		DROP TRIGGER IF EXISTS lock_live_account_reference ON users.persons;
		CREATE TRIGGER lock_live_account_reference
			BEFORE INSERT OR UPDATE OF account_id ON users.persons
			FOR EACH ROW EXECUTE FUNCTION auth.lock_live_account_reference();
		DROP TRIGGER IF EXISTS lock_live_account_reference ON users.guardian_profiles;
		CREATE TRIGGER lock_live_account_reference
			BEFORE INSERT OR UPDATE OF account_id ON users.guardian_profiles
			FOR EACH ROW EXECUTE FUNCTION auth.lock_live_account_reference();
		DROP TRIGGER IF EXISTS lock_live_account_reference ON auth.account_roles;
		CREATE TRIGGER lock_live_account_reference
			BEFORE INSERT OR UPDATE ON auth.account_roles
			FOR EACH ROW EXECUTE FUNCTION auth.lock_live_account_reference();
		DROP TRIGGER IF EXISTS lock_live_account_reference ON auth.account_permissions;
		CREATE TRIGGER lock_live_account_reference
			BEFORE INSERT OR UPDATE ON auth.account_permissions
			FOR EACH ROW EXECUTE FUNCTION auth.lock_live_account_reference();
		DROP TRIGGER IF EXISTS lock_live_account_reference ON auth.account_tenants;
		CREATE TRIGGER lock_live_account_reference
			BEFORE INSERT OR UPDATE ON auth.account_tenants
			FOR EACH ROW EXECUTE FUNCTION auth.lock_live_account_reference();
		DROP TRIGGER IF EXISTS lock_live_account_reference ON auth.guardian_student_access;
		CREATE TRIGGER lock_live_account_reference
			BEFORE INSERT OR UPDATE OF account_id ON auth.guardian_student_access
			FOR EACH ROW EXECUTE FUNCTION auth.lock_live_account_reference();

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

		REVOKE ALL ON FUNCTION auth.account_is_unused(BIGINT, TEXT) FROM PUBLIC;
		REVOKE ALL ON FUNCTION auth.erase_unused_accounts() FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION auth.erase_unused_accounts() TO phoenix_admin;

		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS chk_data_deletions_subject;
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT chk_data_deletions_subject CHECK (
				(student_id IS NULL OR staff_id IS NULL)
				AND (
					student_id IS NOT NULL
					OR staff_id IS NOT NULL
					OR deletion_type IN (
						'unregistered_tag_scan_retention',
						'status_day_retention',
						'attendance_retention',
						'guardian_retention',
						'parent_message_retention',
						'auth_event_retention',
						'data_access_log_retention',
						'deletion_log_retention',
						'delivery_retention',
						'file_cleanup_retention',
						'staff_cleanup_retention',
						'student_document_cleanup_retention',
						'care_withdrawal_retention',
						'submission_rate_limit_retention',
						'settled_enrollment_retention',
						'staff_sick_note_retention',
						'student_deletion_log_retention',
						'enrollment_deletion_log_retention',
						'enrollment_restoration_log_retention',
						'time_tracking_deletion_log_retention',
						'staff_master_data_change_retention',
						'personnel_number_change_retention',
						'guardian_financial_change_retention',
						'class_list_change_retention',
						'substitution_change_retention',
						'file_event_retention',
						'export_transfer_retention',
						'setting_audit_retention'
					)
				)
			);
`

// accountGuardianRetentionDownSQL restores the subject check of 1.15.460 and
// removes the functions and both columns. Erased accounts keep
// their placeholder address.
const accountGuardianRetentionDownSQL = `
		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS chk_data_deletions_subject;
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT chk_data_deletions_subject CHECK (
				(student_id IS NULL OR staff_id IS NULL)
				AND (
					student_id IS NOT NULL
					OR staff_id IS NOT NULL
					OR deletion_type IN (
						'unregistered_tag_scan_retention',
						'status_day_retention',
						'attendance_retention',
						'parent_message_retention',
						'auth_event_retention',
						'data_access_log_retention',
						'deletion_log_retention',
						'delivery_retention',
						'file_cleanup_retention',
						'staff_cleanup_retention',
						'student_document_cleanup_retention',
						'care_withdrawal_retention',
						'submission_rate_limit_retention',
						'settled_enrollment_retention',
						'staff_sick_note_retention',
						'student_deletion_log_retention',
						'enrollment_deletion_log_retention',
						'enrollment_restoration_log_retention',
						'time_tracking_deletion_log_retention',
						'staff_master_data_change_retention',
						'personnel_number_change_retention',
						'guardian_financial_change_retention',
						'class_list_change_retention',
						'substitution_change_retention',
						'file_event_retention',
						'export_transfer_retention',
						'setting_audit_retention'
					)
				)
			);

		DROP TRIGGER IF EXISTS lock_live_account_reference ON auth.guardian_student_access;
		DROP TRIGGER IF EXISTS lock_live_account_reference ON auth.account_tenants;
		DROP TRIGGER IF EXISTS lock_live_account_reference ON auth.account_permissions;
		DROP TRIGGER IF EXISTS lock_live_account_reference ON auth.account_roles;
		DROP TRIGGER IF EXISTS lock_live_account_reference ON users.guardian_profiles;
		DROP TRIGGER IF EXISTS lock_live_account_reference ON users.persons;
		DROP FUNCTION IF EXISTS auth.lock_live_account_reference();
		DROP FUNCTION IF EXISTS auth.erase_unused_accounts();
		DROP FUNCTION IF EXISTS auth.account_is_unused(BIGINT, TEXT);
		ALTER TABLE auth.accounts DROP COLUMN IF EXISTS personal_data_erased_at;

		DROP INDEX IF EXISTS users.idx_guardian_profiles_childless_since;
		ALTER TABLE users.guardian_profiles DROP COLUMN IF EXISTS childless_since;
`

func accountGuardianRetentionUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackAccountGuardianRetention(ctx, tx)
	if _, err := tx.ExecContext(ctx, accountGuardianRetentionUpSQL); err != nil {
		return fmt.Errorf("prepare account and guardian retention: %w", err)
	}
	return tx.Commit()
}

// accountGuardianRetentionDown refuses while guardian retention rows are
// logged: the restored check would reject them.
func accountGuardianRetentionDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type = ? AND student_id IS NULL AND staff_id IS NULL`, guardianRetentionKind).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check guardian retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d guardian retention rows exist", logged)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackAccountGuardianRetention(ctx, tx)
	if _, err := tx.ExecContext(ctx, accountGuardianRetentionDownSQL); err != nil {
		return fmt.Errorf("revert account and guardian retention: %w", err)
	}
	return tx.Commit()
}

func rollbackAccountGuardianRetention(ctx context.Context, tx bun.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logRollbackFailure(ctx, err)
	}
}
