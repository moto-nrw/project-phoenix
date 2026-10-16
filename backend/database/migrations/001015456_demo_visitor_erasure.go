package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const demoVisitorErasureVersion = "1.15.456"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoVisitorErasureVersion,
		Description: "Erase the visitor of a retired demo school (#4048)",
		DependsOn: []string{
			demoVisitorFirstLastNameVersion, // first_name and last_name of platform.demo_school_states
			protocolRetentionVersion,        // the last migration before this one
		},
	})
	Migrations.MustRegister(demoVisitorErasureUp, demoVisitorErasureDown)
}

// demoVisitorErasureUpSQL erases the visitor of the public demo once its
// demo school is out of use (#4048). The demo process calls the function
// every hour, right after the expiry of #3470 has deleted the demo accesses
// 14 days after their last use. The companion function hides schools and
// erases visitors under the same per-school lock as access creation and use.
// It also takes the "demo school seed:" lease held by the complete seed run,
// so a failed or deferred seed cannot write visitor data after erasure.
//
// An order is out of use when it is closed (ready with a hidden school, or
// failed) and no demo access names it any more: a visitor whose order failed
// keeps seeing that until the access expires.
//
// For each such order the function
//   - replaces the visitor's name with "Demo Besuch" wherever the seed of
//     the school copied it: the caregiver (users.persons), the parent
//     (users.guardian_profiles), the enrollment request, the guardian change
//     log and the parent messages;
//   - deletes the declarations the parent signed with the name, since their
//     record hash covers it, and the mails that carried it;
//   - deletes the order itself with the name, the account ids and the
//     authentication data of its seed state.
//
// The visitor is matched by the caregiver and parent account IDs, never by a
// name. Their account and school histories cover a seed that failed and was
// repeated: that seed left its first school behind
// (cmd/devtools/demo_scheduler.go, ReplaceAbandoned). Visible schools are
// never touched. A database without demo orders changes nothing there.
//
// Text the visitor typed into the demo school is not found by name and stays
// in the hidden school; docs/reference/loeschfristen.md names that rest. The
// seed-smoke job searches every text column of three erased demo schools
// for the visitor's name, so a new copy the seed makes fails CI; it also
// covers the signed declarations, which the Go tests cannot build.
//
// The function runs as its owner (SECURITY DEFINER, the pattern of the audit
// retention functions of 1.15.454), so the demo role gets no write grant on
// any of the tables; only phoenix_demo may call it. Unlike those functions it
// writes tables of other owners (Identity & Access, People Directory,
// Enrollment, Audit Platform, Communication, Delivery Platform). ADR 0062
// records this narrow demo-only workflow; it does not transfer ownership of
// those tables.
const demoVisitorErasureUpSQL = `
		ALTER TABLE platform.demo_school_states
			ADD COLUMN IF NOT EXISTS visitor_account_ids BIGINT[] NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS visitor_parent_account_ids BIGINT[] NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS visitor_school_ids BIGINT[] NOT NULL DEFAULT '{}';

		-- Before this migration Fail cleared tenant_id from final failed orders.
		-- Their final school still has the order slug, while every predecessor
		-- has the exact, seed-owned x<school id>-<order slug> marker from
		-- retireAbandonedSchool (both forms may be truncated to 63 characters).
		-- Recover only those exact markers, never a visitor name or every hidden
		-- school of an organization.
		UPDATE platform.demo_school_states AS state
		SET visitor_school_ids = array_remove(
			state.visitor_school_ids || ARRAY[state.tenant_id] || COALESCE((
				SELECT array_agg(school.id ORDER BY school.id)
				FROM platform.schools AS school
				WHERE school.deleted_at IS NOT NULL
					AND (school.id = state.tenant_id
						OR school.subdomain = state.name
						OR school.subdomain = left('x' || school.id::TEXT || '-' || state.name, 63))
			), '{}'::BIGINT[]), NULL)
		WHERE cardinality(state.visitor_school_ids) = 0;

		CREATE OR REPLACE FUNCTION platform.erase_closed_demo_visitors()
		RETURNS INTEGER
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		AS $$
		DECLARE
			hidden_schools BIGINT[];
			legacy_school_ids BIGINT[];
			recovered_visitor_account_ids BIGINT[];
			recovered_visitor_parent_account_ids BIGINT[];
			erased_account_ids BIGINT[];
			visitor RECORD;
			target_name TEXT;
			erased INTEGER := 0;
		BEGIN
			PERFORM pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:visitor-erasure', 0));
			FOR visitor IN
				SELECT state.name, state.tenant_id, state.attempts,
					array_remove(state.visitor_account_ids || ARRAY[state.visitor_account_id], NULL) AS visitor_account_ids,
					array_remove(state.visitor_parent_account_ids || ARRAY[state.visitor_parent_account_id], NULL) AS visitor_parent_account_ids,
					array_remove(state.visitor_school_ids || ARRAY[state.tenant_id], NULL) AS visitor_school_ids
				FROM platform.demo_school_states AS state
				LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
				WHERE (state.status = 'failed' OR (state.status = 'ready' AND school.deleted_at IS NOT NULL))
					AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name)
				ORDER BY state.name
			LOOP
				target_name := visitor.name;
				PERFORM pg_advisory_xact_lock(hashtextextended('demo school seed:' || target_name, 3461));
				PERFORM pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:' || target_name, 0));
				SELECT state.name, state.tenant_id, state.attempts,
					array_remove(state.visitor_account_ids || ARRAY[state.visitor_account_id], NULL) AS visitor_account_ids,
					array_remove(state.visitor_parent_account_ids || ARRAY[state.visitor_parent_account_id], NULL) AS visitor_parent_account_ids,
					array_remove(state.visitor_school_ids || ARRAY[state.tenant_id], NULL) AS visitor_school_ids
				INTO visitor
				FROM platform.demo_school_states AS state
				LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
				WHERE state.name = target_name
					AND (state.status = 'failed' OR (state.status = 'ready' AND school.deleted_at IS NOT NULL))
					AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name)
				FOR UPDATE OF state;
				IF NOT FOUND THEN
					CONTINUE;
				END IF;
				-- Keep the runtime path in addition to the migration backfill. It
				-- covers a row restored from a pre-migration backup before it is
				-- erased, including a final failed school's original subdomain.
				SELECT COALESCE(array_agg(school.id ORDER BY school.id), '{}'::BIGINT[])
				INTO legacy_school_ids
				FROM platform.schools AS school
				WHERE school.deleted_at IS NOT NULL
					AND (school.id = visitor.tenant_id
						OR school.subdomain = visitor.name
						OR school.subdomain = left('x' || school.id::TEXT || '-' || visitor.name, 63));
				visitor.visitor_school_ids := array_remove(visitor.visitor_school_ids || legacy_school_ids, NULL);
				SELECT COALESCE(array_agg(school.id), '{}') INTO hidden_schools
				FROM platform.schools AS school
				WHERE school.id = ANY (visitor.visitor_school_ids)
					AND school.deleted_at IS NOT NULL;
				-- SaveVisitor writes the account ids immediately after each account
				-- is created. If that database write itself failed, recover only the
				-- two deterministic seed identities in the affected schools. The
				-- random order suffix and attempt number make these account addresses
				-- independent of a visitor's display name.
				SELECT COALESCE(array_agg(account.id), '{}'::BIGINT[])
				INTO recovered_visitor_account_ids
				FROM users.persons AS person
				JOIN auth.accounts AS account ON account.id = person.account_id
				WHERE person.tenant_id = ANY (hidden_schools)
					AND account.email = ANY (ARRAY(
						SELECT 'julia.klein@demo-' || regexp_replace(visitor.name, '^.*-', '') ||
							CASE WHEN attempt.number > 1 THEN '-' || attempt.number::TEXT ELSE '' END || '.moto-ogs.de'
						FROM generate_series(1, GREATEST(visitor.attempts, 1)) AS attempt(number)
					));
				SELECT COALESCE(array_agg(account.id), '{}'::BIGINT[])
				INTO recovered_visitor_parent_account_ids
				FROM users.guardian_profiles AS guardian
				JOIN auth.accounts AS account ON account.id = guardian.account_id
				WHERE guardian.tenant_id = ANY (hidden_schools)
					AND account.email = ANY (ARRAY(
						SELECT 'sabine.schneider@demo-' || regexp_replace(visitor.name, '^.*-', '') ||
							CASE WHEN attempt.number > 1 THEN '-' || attempt.number::TEXT ELSE '' END || '.moto-ogs.de'
						FROM generate_series(1, GREATEST(visitor.attempts, 1)) AS attempt(number)
					));
				visitor.visitor_account_ids := array_remove(visitor.visitor_account_ids || recovered_visitor_account_ids, NULL);
				visitor.visitor_parent_account_ids := array_remove(visitor.visitor_parent_account_ids || recovered_visitor_parent_account_ids, NULL);
				IF cardinality(visitor.visitor_account_ids) > 0 OR cardinality(visitor.visitor_parent_account_ids) > 0 THEN
					UPDATE users.persons SET first_name = 'Demo', last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND account_id = ANY (visitor.visitor_account_ids);
					UPDATE users.guardian_profiles SET first_name = 'Demo', last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND account_id = ANY (visitor.visitor_parent_account_ids);
					UPDATE enrollment.requests SET guardian_first_name = 'Demo', guardian_last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND guardian_account_id = ANY (visitor.visitor_parent_account_ids);
					UPDATE audit.guardian_changes SET actor_name_snapshot = 'Demo Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND actor_account_id = ANY (visitor.visitor_parent_account_ids);
					UPDATE users.parent_messages SET sender_name = 'Demo Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND sender_account_id = ANY (visitor.visitor_parent_account_ids);
					DELETE FROM users.parent_announcement_declaration_submissions
					WHERE tenant_id = ANY (hidden_schools)
						AND account_id = ANY (visitor.visitor_parent_account_ids);
					WITH visitor_outbox AS (
						SELECT outbox.id
						FROM platform.email_outbox AS outbox
						WHERE outbox.tenant_id = ANY (hidden_schools)
							AND (
								EXISTS (
									SELECT 1 FROM platform.email_delivery AS delivery
									WHERE delivery.outbox_id = outbox.id
										AND delivery.account_id = ANY (visitor.visitor_parent_account_ids))
								OR LOWER(outbox.recipient->>'address') IN (
									SELECT LOWER(account.email) FROM auth.accounts AS account
									WHERE account.id = ANY (visitor.visitor_parent_account_ids))
							)
					), deleted_deliveries AS (
						DELETE FROM platform.email_delivery AS delivery
						WHERE delivery.tenant_id = ANY (hidden_schools)
							AND (delivery.account_id = ANY (visitor.visitor_parent_account_ids)
								OR delivery.outbox_id IN (SELECT id FROM visitor_outbox))
					)
					DELETE FROM platform.email_outbox AS outbox
					WHERE outbox.id IN (SELECT id FROM visitor_outbox);
					-- Keep a visitor account that belongs to another school, but never
					-- leave its sessions or grants in a retired demo school behind.
					DELETE FROM auth.tokens
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.account_roles
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.account_permissions
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.guardian_student_access
					WHERE account_id = ANY (visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.account_tenants
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.mfa_overrides
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					SELECT COALESCE(array_agg(account.id), '{}') INTO erased_account_ids
					FROM auth.accounts AS account
					WHERE account.id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND NOT EXISTS (
							SELECT 1 FROM auth.account_tenants AS membership
							WHERE membership.account_id = account.id
								AND membership.tenant_id <> ALL (visitor.visitor_school_ids)
						)
						AND NOT EXISTS (
							SELECT 1 FROM users.persons AS person
							WHERE person.account_id = account.id AND person.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.guardian_profiles AS guardian
							WHERE guardian.account_id = account.id AND guardian.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM enrollment.requests AS request
							WHERE request.guardian_account_id = account.id AND request.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_messages AS message
							WHERE message.sender_account_id = account.id AND message.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_message_threads AS thread
							WHERE thread.guardian_account_id = account.id AND thread.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_message_reads AS read
							WHERE read.account_id = account.id AND read.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM audit.guardian_changes AS change
							WHERE change.actor_account_id = account.id AND change.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_announcement_declaration_submissions AS submission
							WHERE submission.account_id = account.id AND submission.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM platform.email_delivery AS delivery
							WHERE delivery.account_id = account.id AND delivery.tenant_id <> ALL (visitor.visitor_school_ids)
						);
					IF cardinality(erased_account_ids) > 0 THEN
						DELETE FROM auth.tokens WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.password_reset_tokens WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.account_roles WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.account_permissions WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.account_tenants WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_credentials WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_email_challenges WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_trusted_devices WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_overrides WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.passkey_credentials WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.passkey_sessions WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.demo_accesses
						WHERE account_id = ANY (erased_account_ids) OR parent_account_id = ANY (erased_account_ids);
						DELETE FROM auth.password_reset_rate_limits
						WHERE email IN (SELECT email FROM auth.accounts WHERE id = ANY (erased_account_ids));
						UPDATE auth.accounts SET
							email = 'demo-besuch-' || id || '@example.invalid',
							username = NULL, active = FALSE, password_hash = NULL, is_password_otp = FALSE,
							last_login = NULL, avatar = NULL, calendar_feed_token = NULL
						WHERE id = ANY (erased_account_ids);
					END IF;
				END IF;
				DELETE FROM platform.demo_school_states WHERE name = visitor.name;
				erased := erased + 1;
			END LOOP;
			RETURN erased;
		END;
		$$;

		CREATE OR REPLACE FUNCTION platform.retire_expired_demo_schools(names TEXT[])
		RETURNS TABLE(hidden INTEGER, erased INTEGER)
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		AS $$
		DECLARE
			target_name TEXT;
			hidden_count INTEGER := 0;
			changed INTEGER;
		BEGIN
			PERFORM pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:visitor-erasure', 0));
			FOR target_name IN
				SELECT name FROM unnest(COALESCE(names, '{}'::TEXT[])) AS target(name) ORDER BY name
			LOOP
				PERFORM pg_advisory_xact_lock(hashtextextended('demo school seed:' || target_name, 3461));
				PERFORM pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:' || target_name, 0));
				IF EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = target_name) THEN
					CONTINUE;
				END IF;
				UPDATE platform.schools AS school SET deleted_at = NOW()
					FROM platform.demo_school_states AS state
					WHERE state.name = target_name
						AND (state.status = 'ready' OR (state.status = 'preparing' AND state.claimed_at IS NULL))
						AND school.id = state.tenant_id AND school.deleted_at IS NULL;
				GET DIAGNOSTICS changed = ROW_COUNT;
				hidden_count := hidden_count + changed;
				UPDATE platform.demo_school_states AS state SET status = 'failed'
					WHERE state.name = target_name AND state.status = 'preparing' AND state.claimed_at IS NULL
						AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name);
			END LOOP;
			hidden := hidden_count;
			erased := platform.erase_closed_demo_visitors();
			RETURN NEXT;
		END;
		$$;

		REVOKE ALL ON FUNCTION platform.erase_closed_demo_visitors() FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION platform.erase_closed_demo_visitors() TO phoenix_demo;
		REVOKE ALL ON FUNCTION platform.retire_expired_demo_schools(TEXT[]) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION platform.retire_expired_demo_schools(TEXT[]) TO phoenix_demo;
`

func demoVisitorErasureUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoVisitorErasureUpSQL); err != nil {
		return fmt.Errorf("create demo visitor erasure: %w", err)
	}
	return nil
}

func demoVisitorErasureDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP FUNCTION IF EXISTS platform.retire_expired_demo_schools(TEXT[]);
		DROP FUNCTION IF EXISTS platform.erase_closed_demo_visitors();
		ALTER TABLE platform.demo_school_states
			DROP COLUMN IF EXISTS visitor_account_ids,
			DROP COLUMN IF EXISTS visitor_parent_account_ids,
			DROP COLUMN IF EXISTS visitor_school_ids;`); err != nil {
		return fmt.Errorf("drop demo visitor erasure: %w", err)
	}
	return nil
}
