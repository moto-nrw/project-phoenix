package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const demoVisitorErasureCareInvoicesVersion = "1.15.791"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoVisitorErasureCareInvoicesVersion,
		Description: "Demo-Besuch löschen: Monatsrechnungen und Laufspur der Demo-Schule entfernen",
		DependsOn: []string{
			parentBillingCareMonthsVersion,         // the tables it deletes from
			demoVisitorErasureCareContractsVersion, // the function this one replaces
		},
	})
	Migrations.MustRegister(demoVisitorErasureCareInvoicesUp, demoVisitorErasureCareInvoicesDown)
}

// demoVisitorErasureCareInvoicesUpSQL is platform.erase_closed_demo_visitors
// of 1.15.786 that also deletes the monthly invoices of the retired demo
// schools (#4684, ADR 0158 point 10). An invoice keeps its own copy of child,
// recipient and mail address and outlives the contract, so nothing removes
// it with the school; its positions go by cascade. A read in the trail of a
// monthly run names the contracts it showed and goes with them, before the
// contracts do. The run and its other events name no one and stay. The
// erasure holds the monthly billing of each Träger like a release does, so
// a release under way cannot insert an invoice after the cleanup.
//
// The rest of the function is unchanged.
const demoVisitorErasureCareInvoicesUpSQL = `
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

					-- A care contract outlives its school (ADR 0088 section 9) and
					-- names the visitor as the contracting parent, in its PDF and in
					-- the mail that carried it. The school is the visitor's alone, so
					-- every contract of it goes with what hangs on it by cascade.
					-- Send holds this row lock until its mail and event commit. Take
					-- it before cleanup so those copies cannot arrive after it.
					-- A release of a monthly run holds the monthly billing of its
					-- Träger before it reads the contracts (#4684). Take the same
					-- lock first: a release under way defers the order, and a later
					-- release reads the contracts after they are gone, so no invoice
					-- with the visitor's name arrives after this cleanup.
					PERFORM pg_advisory_xact_lock(hashtextextended('parent_billing.care_billing:' || traeger.organization_id::text, 0))
					FROM (SELECT DISTINCT school.organization_id FROM platform.schools AS school
						WHERE school.id = ANY (hidden_schools) AND school.organization_id IS NOT NULL
						ORDER BY school.organization_id) AS traeger;
					PERFORM 1 FROM care_contract.contracts AS contract
					WHERE contract.school_id = ANY (hidden_schools)
					ORDER BY contract.id
					FOR UPDATE;
					-- The monthly invoice of a contract copies child, recipient and
					-- mail address (#4684) and outlives the contract: the invoices
					-- of the school go, their positions by cascade. A read in the
					-- trail of a monthly run names the contracts it showed; it goes
					-- while those contracts still exist. The numbers leave a gap in
					-- the Träger's number range, which ADR 0119 accepts here.
					DELETE FROM parent_billing.care_invoices
					WHERE school_id = ANY (hidden_schools);
					DELETE FROM parent_billing.care_run_events AS event
					USING care_contract.contracts AS contract
					WHERE contract.school_id = ANY (hidden_schools)
						AND event.organization_id = contract.organization_id
						AND event.details->'contract_ids' @> to_jsonb(contract.id);
					DELETE FROM platform.email_outbox AS outbox
					USING care_contract.contracts AS contract
					WHERE contract.school_id = ANY (hidden_schools)
						AND outbox.organization_id = contract.organization_id
						AND outbox.related_entity_type = 'care_contract'
						AND outbox.related_entity_id = contract.id;
					DELETE FROM care_contract.contract_events AS event
					WHERE event.school_id = ANY (hidden_schools)
						OR event.contract_id IN (
							SELECT contract.id FROM care_contract.contracts AS contract
							WHERE contract.school_id = ANY (hidden_schools));
					DELETE FROM care_contract.contracts
					WHERE school_id = ANY (hidden_schools);
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

func demoVisitorErasureCareInvoicesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoVisitorErasureCareInvoicesUpSQL); err != nil {
		return fmt.Errorf("erase the monthly invoices of retired demo schools: %w", err)
	}
	return nil
}

// demoVisitorErasureCareInvoicesDown reinstalls the erasure of 1.15.786.
func demoVisitorErasureCareInvoicesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoVisitorErasureOf786SQL); err != nil {
		return fmt.Errorf("restore the demo visitor erasure of 1.15.786: %w", err)
	}
	return nil
}

// demoVisitorErasureOf786SQL is platform.erase_closed_demo_visitors as
// 1.15.786 created it, frozen here for Down.
const demoVisitorErasureOf786SQL = `
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

					-- A care contract outlives its school (ADR 0088 section 9) and
					-- names the visitor as the contracting parent, in its PDF and in
					-- the mail that carried it. The school is the visitor's alone, so
					-- every contract of it goes with what hangs on it by cascade.
					-- Send holds this row lock until its mail and event commit. Take
					-- it before cleanup so those copies cannot arrive after it.
					PERFORM 1 FROM care_contract.contracts AS contract
					WHERE contract.school_id = ANY (hidden_schools)
					ORDER BY contract.id
					FOR UPDATE;
					DELETE FROM platform.email_outbox AS outbox
					USING care_contract.contracts AS contract
					WHERE contract.school_id = ANY (hidden_schools)
						AND outbox.organization_id = contract.organization_id
						AND outbox.related_entity_type = 'care_contract'
						AND outbox.related_entity_id = contract.id;
					DELETE FROM care_contract.contract_events AS event
					WHERE event.school_id = ANY (hidden_schools)
						OR event.contract_id IN (
							SELECT contract.id FROM care_contract.contracts AS contract
							WHERE contract.school_id = ANY (hidden_schools));
					DELETE FROM care_contract.contracts
					WHERE school_id = ANY (hidden_schools);
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
