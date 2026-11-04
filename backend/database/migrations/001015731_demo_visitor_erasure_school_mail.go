package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const demoVisitorErasureSchoolMailVersion = "1.15.731"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoVisitorErasureSchoolMailVersion,
		Description: "Demo-Besuch löschen: alle Mails der ausgemusterten Demo-Schule mit entfernen (#5521)",
		DependsOn: []string{
			traegerLeadershipRolesVersion,  // the last migration before this one
			retireDirectPermissionsVersion, // the function this one replaces
		},
	})
	Migrations.MustRegister(demoVisitorErasureSchoolMailUp, demoVisitorErasureSchoolMailDown)
}

func demoVisitorErasureSchoolMailUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoVisitorErasureSchoolMailUpSQL); err != nil {
		return fmt.Errorf("erase the mail of retired demo schools with their visitor: %w", err)
	}
	return nil
}

// demoVisitorErasureSchoolMailDown reinstalls the function exactly as 1.15.646
// left it.
func demoVisitorErasureSchoolMailDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := installDirectPermissionDemoErasureUp(ctx, tx); err != nil {
			return fmt.Errorf("restore the demo visitor erasure of 1.15.646: %w", err)
		}
		return nil
	})
}

// demoVisitorErasureSchoolMailUpSQL is platform.erase_closed_demo_visitors of
// 1.15.646 with the visitor's mail erased by school instead of by parent
// account. Since #4808 the scheduler mails the Leitung of a demo school, the
// visitor's caregiver account, about expiring qualifications; the parent-only
// match left those mails and the visitor's name in platform.email_outbox.
// An exclusive visitor account also loses its mail without a school or Träger
// before its address is replaced; a Träger member is no exclusive account.
// The rest of the function is unchanged.
const demoVisitorErasureSchoolMailUpSQL = `
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
`
