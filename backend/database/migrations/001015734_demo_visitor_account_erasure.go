package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const demoVisitorAccountErasureVersion = "1.15.734"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoVisitorAccountErasureVersion,
		Description: "Demo-Besuch löschen: Push-Abos, Profil und Avatar-Dateien mit entfernen, Lauf ohne Warten auf Sperren (#5540)",
		DependsOn: []string{
			payrollWageTypesVersion,                        // the last migration before this one
			demoVisitorErasureRealtimeNotificationsVersion, // the function this one replaces
		},
	})
	Migrations.MustRegister(demoVisitorAccountErasureUp, demoVisitorAccountErasureDown)
}

func demoVisitorAccountErasureUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE platform.demo_school_states
		ADD COLUMN IF NOT EXISTS erasure_deferrals INTEGER NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("count the deferred erasures of demo orders: %w", err)
	}
	if _, err := db.ExecContext(ctx, demoVisitorAccountErasureUpSQL); err != nil {
		return fmt.Errorf("erase the push subscriptions, profiles and avatar files of demo visitors: %w", err)
	}
	if _, err := db.ExecContext(ctx, demoSchoolRetirementWithoutWaitSQL); err != nil {
		return fmt.Errorf("retire demo schools without waiting for a lock: %w", err)
	}
	return nil
}

// demoVisitorAccountErasureDown reinstalls the retirement of 1.15.456 and the
// erasure of 1.15.732 and drops the deferral note and counter.
func demoVisitorAccountErasureDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoSchoolRetirementOf456SQL); err != nil {
		return fmt.Errorf("restore the demo school retirement of 1.15.456: %w", err)
	}
	if err := demoVisitorErasureRealtimeNotificationsUp(ctx, db); err != nil {
		return fmt.Errorf("restore the demo visitor erasure of 1.15.732: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DROP FUNCTION IF EXISTS platform.defer_demo_order(TEXT, TEXT, TEXT)`); err != nil {
		return fmt.Errorf("drop the deferral note of demo orders: %w", err)
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE platform.demo_school_states DROP COLUMN IF EXISTS erasure_deferrals`); err != nil {
		return fmt.Errorf("drop the deferral counter of demo orders: %w", err)
	}
	return nil
}

// demoVisitorAccountErasureUpSQL is platform.erase_closed_demo_visitors of
// 1.15.732 with these changes (#5540):
//   - It locks the visitor accounts before it decides which of them hang only
//     in demo schools. Without the lock, a school or Träger membership written
//     while the erasure ran was not seen, and the account was anonymized anyway.
//     A role outside the visitor schools now keeps the account too.
//   - It never waits for a lock. Writers lock rows and accounts in every
//     order (an UPDATE the row first, an upsert the account first, in
//     auth.lock_live_account_reference; role removal the role, then all
//     sessions), so any wait could close a cycle. The function runs with
//     lock_timeout 100ms, far below deadlock_timeout, and each order runs in
//     a subtransaction: on lock_not_available or deadlock_detected it is
//     rolled back, counted in platform.demo_school_states.erasure_deferrals
//     (platform.defer_demo_order, one log line, a warning from 24 in a row)
//     and retried by the next hourly run. What is guaranteed: each single
//     lock wait ends after 100 ms; new orders start only within the budget
//     (at most 50, and none after 250 ms of the erasure's own time; the
//     retirement before it has 200 ms); an order already started is
//     finished. The run time is therefore bounded, but not hard below 1 s.
//     The erasure picks its batch with FOR UPDATE SKIP LOCKED: an order
//     whose state row another transaction holds takes no place and stays
//     open. The rest stays open too.
//   - Open is every order whose school is hidden or that failed, whatever
//     state its seed left it in, until its erasure is done: before, a hidden
//     school of an order still preparing was never erased.
//   - It deletes the push subscriptions of the retired schools: endpoint and
//     user agent of the visitor's devices.
//   - An anonymized account loses what auth.erase_unused_accounts removes,
//     including its push subscriptions everywhere, and its avatar files are
//     queued for the file cleanup of #4756.
//   - An anonymized account is marked erased (personal_data_erased_at), so no
//     school can reference it afterwards.
//
// The rest of the function is unchanged.
const demoVisitorAccountErasureUpSQL = `
		CREATE OR REPLACE FUNCTION platform.erase_closed_demo_visitors()
		RETURNS INTEGER
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		-- The erasure never waits for a lock (#5540): far below
		-- deadlock_timeout (1 s), a wait gives up before PostgreSQL could
		-- pick the other transaction of a cycle as its victim.
		SET lock_timeout = '100ms'
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
			tried INTEGER := 0;
			started TIMESTAMPTZ := clock_timestamp();
		BEGIN
			-- Another erasure is running; it handles the orders. The caller
			-- retire_expired_demo_schools already holds this lock.
			IF NOT pg_try_advisory_xact_lock(hashtextextended('platform.demo_school_states:visitor-erasure', 0)) THEN
				RETURN 0;
			END IF;
			-- The scan of the orders reads tables another transaction may hold
			-- exclusively. Its lock error skips the erasure of this run with one
			-- log line; every order stays for the next run (#5540).
			BEGIN
			FOR visitor IN
				SELECT state.name, state.tenant_id, state.attempts,
					array_remove(state.visitor_account_ids || ARRAY[state.visitor_account_id], NULL) AS visitor_account_ids,
					array_remove(state.visitor_parent_account_ids || ARRAY[state.visitor_parent_account_id], NULL) AS visitor_parent_account_ids,
					array_remove(state.visitor_school_ids || ARRAY[state.tenant_id], NULL) AS visitor_school_ids
				FROM platform.demo_school_states AS state
				LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
				-- Open is every order whose school is hidden or that failed, in
				-- whatever state its seed left it, until its erasure is done
				-- (#5540): a hidden school is never found again otherwise.
				WHERE (state.status = 'failed' OR school.deleted_at IS NOT NULL)
					AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name)
				-- A deferral puts an order behind the orders deferred less often.
				-- A locked state row takes no place in the batch: the order stays
				-- open and the free orders behind it come first.
				ORDER BY state.erasure_deferrals, state.name
				LIMIT 50
				FOR UPDATE OF state SKIP LOCKED
			LOOP
				target_name := visitor.name;
				-- The erasure has a budget of its own: it starts no order after
				-- 250 ms, so open orders the retirement waits for never take its
				-- turn. Each lock wait ends after 100 ms; an order already started
				-- is finished, so the run time is bounded, not cut off. The rest
				-- stays open.
				IF tried >= 50 OR clock_timestamp() - started > INTERVAL '250 milliseconds' THEN
					RAISE LOG 'demo visitor erasure ends this run at its budget: % orders erased, the rest stays open', erased;
					EXIT;
				END IF;
				tried := tried + 1;
				-- One subtransaction per order. When any lock it needs is in use
				-- longer than lock_timeout, or a deadlock is detected anyway, the
				-- order is rolled back untouched, state row included, and the next
				-- hourly run retries it (#5540). Neither the run nor the other
				-- transaction fails.
				BEGIN
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
					AND (state.status = 'failed' OR school.deleted_at IS NOT NULL)
					AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name)
				-- A locked state row defers the order at once: waiting for it,
				-- and again for its counter, would spend the budget of the
				-- orders behind it.
				FOR UPDATE OF state NOWAIT;
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
					DELETE FROM audit.signatures
					WHERE tenant_id = ANY (hidden_schools)
						AND signer_account_id = ANY (visitor.visitor_parent_account_ids);
					-- Every mail of a retired demo school goes, whoever it was
					-- addressed to: a mail to the visitor's caregiver account (the
					-- qualification reminder of #4808) carries the visitor's name as much
					-- as one to their parent account, and a mail to another person of the
					-- school can name the visitor too (an absence request names the
					-- requester). The school is no one's but the visitor's (#5521).
					DELETE FROM platform.email_delivery
					WHERE tenant_id = ANY (hidden_schools);
					DELETE FROM platform.email_outbox
					WHERE tenant_id = ANY (hidden_schools);
					-- The short-lived copies of in-app notifications can name the
					-- visitor in their title or text (#5504); the school is the
					-- visitor's alone, so all of them go (#5540).
					DELETE FROM platform.realtime_notifications
					WHERE tenant_id = ANY (hidden_schools);
					-- A push subscription holds the endpoint and user agent of a
					-- device that opened the school; the school is the visitor's
					-- alone, so all of them go (#5540).
					DELETE FROM iot.push_subscriptions
					WHERE tenant_id = ANY (hidden_schools);
					-- Keep a visitor account that belongs to another school, but never
					-- leave its sessions or grants in a retired demo school behind.
					DELETE FROM auth.tokens
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.account_roles
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
					-- Hold the visitor accounts until the check below is done (#5540):
					-- a school or Träger membership another transaction is writing
					-- holds its account, so the erasure gives up after lock_timeout
					-- and the next run's check sees the membership.
					PERFORM 1 FROM auth.accounts AS account
					WHERE account.id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
					ORDER BY account.id
					FOR UPDATE;
					SELECT COALESCE(array_agg(account.id), '{}') INTO erased_account_ids
					FROM auth.accounts AS account
					WHERE account.id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND NOT EXISTS (
							SELECT 1 FROM auth.account_tenants AS membership
							WHERE membership.account_id = account.id
								AND membership.tenant_id <> ALL (visitor.visitor_school_ids)
						)
						-- A role in another school is an assignment there too.
						AND NOT EXISTS (
							SELECT 1 FROM auth.account_roles AS role
							WHERE role.account_id = account.id
								AND role.tenant_id <> ALL (visitor.visitor_school_ids)
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
							UNION ALL
							-- A Träger member is no demo-only account: it keeps its address,
							-- its Träger access and its Träger mail (#5521).
							SELECT 1 FROM platform.organization_accounts AS traeger_member
							WHERE traeger_member.account_id = account.id
							UNION ALL
							SELECT 1 FROM auth.organization_account_roles AS traeger_role
							WHERE traeger_role.account_id = account.id
						);
					IF cardinality(erased_account_ids) > 0 THEN
						-- The account is anonymized, not deleted, so nothing that would
						-- go with it by cascade goes by itself. It loses what the
						-- general account erasure removes (auth.erase_unused_accounts),
						-- and its avatar files go to the durable file cleanup before
						-- their references disappear (#4756, #5540).
						INSERT INTO auth.account_avatar_file_cleanup (avatar)
						SELECT files.avatar FROM (
							SELECT account.avatar FROM auth.accounts AS account WHERE account.id = ANY (erased_account_ids)
							UNION
							SELECT profile.avatar FROM users.profiles AS profile WHERE profile.account_id = ANY (erased_account_ids)
						) AS files
						WHERE files.avatar LIKE '/uploads/avatars/%'
						ON CONFLICT (avatar) DO NOTHING;
						DELETE FROM auth.tokens WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.password_reset_tokens WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.account_roles WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.account_tenants WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_credentials WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_email_challenges WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_trusted_devices WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_overrides WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.passkey_credentials WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.passkey_sessions WHERE account_id = ANY (erased_account_ids);
						DELETE FROM users.profiles WHERE account_id = ANY (erased_account_ids);
						DELETE FROM users.notification_preferences WHERE account_id = ANY (erased_account_ids);
						DELETE FROM users.parent_request_rss_feeds WHERE account_id = ANY (erased_account_ids);
						DELETE FROM users.parent_message_count_preferences WHERE account_id = ANY (erased_account_ids);
						DELETE FROM iot.push_subscriptions WHERE account_id = ANY (erased_account_ids);
						DELETE FROM iot.pwa_standalone_usage WHERE account_id = ANY (erased_account_ids);
						DELETE FROM config.home_layouts WHERE account_id = ANY (erased_account_ids);
						DELETE FROM config.school_setup_dismissals WHERE account_id = ANY (erased_account_ids);
						DELETE FROM config.staff_onboardings WHERE account_id = ANY (erased_account_ids);
						DELETE FROM documents.folder_accounts WHERE account_id = ANY (erased_account_ids);
						DELETE FROM enrollment.request_reads WHERE account_id = ANY (erased_account_ids);
						DELETE FROM platform.announcement_views WHERE user_id = ANY (erased_account_ids);
						DELETE FROM schedule.timetable_conflict_acks WHERE account_id = ANY (erased_account_ids);
						DELETE FROM users.parent_announcement_reads WHERE account_id = ANY (erased_account_ids);
						DELETE FROM users.staff_notice_acks WHERE account_id = ANY (erased_account_ids);
						DELETE FROM audit.data_imports WHERE imported_by = ANY (erased_account_ids);
						DELETE FROM auth.demo_accesses
						WHERE account_id = ANY (erased_account_ids) OR parent_account_id = ANY (erased_account_ids);
						-- Mail without a school or Träger (password reset, MFA code) is
						-- addressed to the account's address, which is about to be replaced.
						DELETE FROM platform.email_outbox
						WHERE tenant_id IS NULL
							AND organization_id IS NULL
							AND LOWER(recipient->>'address') IN (
								SELECT LOWER(email) FROM auth.accounts WHERE id = ANY (erased_account_ids));
						DELETE FROM auth.password_reset_rate_limits
						WHERE email IN (SELECT email FROM auth.accounts WHERE id = ANY (erased_account_ids));
						UPDATE auth.accounts SET
							email = 'demo-besuch-' || id || '@example.invalid',
							username = NULL, active = FALSE, password_hash = NULL, is_password_otp = FALSE,
							last_login = NULL, avatar = NULL, calendar_feed_token = NULL,
							mfa_attempts = 0, mfa_locked_until = NULL,
							-- Erased like every erased account: no school can take it on
							-- afterwards (auth.lock_live_account_reference).
							personal_data_erased_at = statement_timestamp(), updated_at = statement_timestamp()
						WHERE id = ANY (erased_account_ids);
					END IF;
				END IF;
				DELETE FROM platform.demo_school_states WHERE name = visitor.name;
				erased := erased + 1;
				EXCEPTION WHEN lock_not_available OR deadlock_detected THEN
					PERFORM platform.defer_demo_order(target_name, 'erasure', SQLSTATE);
				END;
			END LOOP;
			-- One line for the orders this run leaves open: deferred, beyond the
			-- budget, or skipped while another transaction holds their row.
			SELECT count(*) INTO tried
			FROM platform.demo_school_states AS state
			LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
			WHERE (state.status = 'failed' OR school.deleted_at IS NOT NULL)
				AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name);
			IF tried > 0 THEN
				RAISE LOG 'demo visitor erasure leaves % orders open for the next run (% erased)', tried, erased;
			END IF;
			EXCEPTION WHEN lock_not_available OR deadlock_detected THEN
				RAISE LOG 'demo visitor erasure skipped this run (SQLSTATE %): a table it reads is locked', SQLSTATE;
				RETURN 0;
			END;
			RETURN erased;
		END;
		$$;
`

// demoSchoolRetirementWithoutWaitSQL is platform.retire_expired_demo_schools
// of 1.15.456, the entry of the hourly demo run (RetireDemoSchools), with two
// invariants (#5540):
//
//   - Deferred work is never lost. The run hides the named orders and every
//     ready visitor order whose school is still open although no demo access
//     names it any more: an order is born with its access in one
//     transaction, so such an order is out of use. What a run defers is
//     therefore found again in the database by the next run, whatever the
//     demo process remembers. The standing school has no order and carries
//     no visitor name. A waiting order whose closing was deferred is not
//     picked up while it waits (the seed smoke queues orders without an
//     access); it is seeded and then found ready without an access.
//   - No lock error leaves the run. Another run holding the run lock skips
//     this one; each order is hidden in a subtransaction under lock_timeout
//     and deferred when a lock it needs is in use; a locked table the scan
//     reads skips the run. Each case writes one log line.
//
// CREATE OR REPLACE keeps the grants of 1.15.456 (phoenix_demo only).
const demoSchoolRetirementWithoutWaitSQL = `
		-- One deferral of one order in one run: counted in its state row and
		-- written as one log line, a warning from 24 in a row (#5540). The
		-- erasure deletes the state row, counter included. Counting never
		-- fails the run.
		CREATE FUNCTION platform.defer_demo_order(order_name TEXT, step TEXT, reason TEXT)
		RETURNS VOID
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		SET lock_timeout = '100ms'
		AS $$
		DECLARE
			deferrals INTEGER;
		BEGIN
			BEGIN
				-- A locked state row is not waited for: the count is skipped.
				UPDATE platform.demo_school_states SET erasure_deferrals = erasure_deferrals + 1
				WHERE name IN (SELECT locked.name FROM platform.demo_school_states AS locked
					WHERE locked.name = order_name FOR UPDATE SKIP LOCKED)
				RETURNING erasure_deferrals INTO deferrals;
			EXCEPTION WHEN lock_not_available OR deadlock_detected THEN
				deferrals := NULL;
			END;
			IF deferrals >= 24 THEN
				RAISE WARNING 'demo % of order % deferred % times in a row (SQLSTATE %): a lock it needs stays in use',
					step, order_name, deferrals, reason;
			ELSE
				RAISE LOG 'demo % of order % deferred (SQLSTATE %, % in a row): a lock it needs is in use',
					step, order_name, reason, COALESCE(deferrals::TEXT, 'unknown');
			END IF;
		END;
		$$;
		REVOKE ALL ON FUNCTION platform.defer_demo_order(TEXT, TEXT, TEXT) FROM PUBLIC;

		CREATE OR REPLACE FUNCTION platform.retire_expired_demo_schools(names TEXT[])
		RETURNS TABLE(hidden INTEGER, erased INTEGER)
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		SET lock_timeout = '100ms'
		AS $$
		DECLARE
			target_name TEXT;
			targets TEXT[];
			hidden_count INTEGER := 0;
			changed INTEGER;
			tried INTEGER := 0;
		BEGIN
			hidden := 0;
			erased := 0;
			IF NOT pg_try_advisory_xact_lock(hashtextextended('platform.demo_school_states:visitor-erasure', 0)) THEN
				RAISE LOG 'demo school retirement skipped: another run holds the run lock';
				RETURN NEXT;
				RETURN;
			END IF;
			BEGIN
				-- Orders deferred less often come first, so orders that stay
				-- locked never starve the others of the budget.
				SELECT COALESCE(array_agg(target.name ORDER BY COALESCE(ordered.erasure_deferrals, 0), target.name), '{}'::TEXT[]) INTO targets
				FROM (
					SELECT name FROM unnest(COALESCE(names, '{}'::TEXT[])) AS named(name)
					UNION
					SELECT state.name
					FROM platform.demo_school_states AS state
					JOIN platform.schools AS school ON school.id = state.tenant_id
					WHERE state.status = 'ready'
						AND state.first_name <> ''
						AND school.deleted_at IS NULL
						AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name)
				) AS target
				LEFT JOIN platform.demo_school_states AS ordered ON ordered.name = target.name;
			EXCEPTION WHEN lock_not_available OR deadlock_detected THEN
				RAISE LOG 'demo school retirement skipped this run (SQLSTATE %): a table it reads is locked', SQLSTATE;
				RETURN NEXT;
				RETURN;
			END;
			FOREACH target_name IN ARRAY targets
			LOOP
				-- The retirement has a budget of its own: it starts no order after
				-- 200 ms, so the erasure keeps its turn. Each lock wait ends after
				-- 100 ms; an order already started is finished, so the run time is
				-- bounded, not cut off. The rest is found again.
				IF tried >= 50 OR clock_timestamp() - statement_timestamp() > INTERVAL '200 milliseconds' THEN
					RAISE LOG 'demo school retirement ends this run at its budget: % schools hidden, the rest stays open', hidden_count;
					EXIT;
				END IF;
				tried := tried + 1;
				BEGIN
					-- Only an order still open to visitors or still waiting is
					-- retired here. A hidden or failed order is the erasure's, which
					-- takes the same locks: one order, one deferral a run.
					PERFORM 1 FROM platform.demo_school_states AS state
					LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
					WHERE state.name = target_name
						AND (state.status = 'ready' OR (state.status = 'preparing' AND state.claimed_at IS NULL))
						AND (school.id IS NULL OR school.deleted_at IS NULL);
					IF NOT FOUND THEN
						CONTINUE;
					END IF;
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
					UPDATE platform.demo_school_states AS state SET status = 'failed'
						WHERE state.name = target_name AND state.status = 'preparing' AND state.claimed_at IS NULL
							AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name);
					hidden_count := hidden_count + changed;
				EXCEPTION WHEN lock_not_available OR deadlock_detected THEN
					PERFORM platform.defer_demo_order(target_name, 'retirement', SQLSTATE);
				END;
			END LOOP;
			hidden := hidden_count;
			erased := platform.erase_closed_demo_visitors();
			RETURN NEXT;
		END;
		$$;
`

// demoSchoolRetirementOf456SQL is platform.retire_expired_demo_schools as
// 1.15.456 created it, frozen here for Down.
const demoSchoolRetirementOf456SQL = `
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
`
