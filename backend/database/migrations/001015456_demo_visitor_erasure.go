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
//     credentials of its seed state.
//
// The visitor is matched by the caregiver and parent account IDs, never by a
// name. Their ID histories cover a seed that failed and was repeated: that
// seed left its first school behind (cmd/devtools/demo_scheduler.go,
// ReplaceAbandoned). Visible schools are never touched. The Demo-Träger is
// the organization of the schools the demo has orders for; a database
// without demo orders has none, so the function changes nothing there.
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
// writes tables of other owners (People Directory, Enrollment, Audit
// Platform, Communication, Delivery Platform). ADR 0061 records this narrow
// demo-only workflow; it does not transfer ownership of those tables.
const demoVisitorErasureUpSQL = `
		ALTER TABLE platform.demo_school_states
			ADD COLUMN IF NOT EXISTS visitor_account_ids BIGINT[] NOT NULL DEFAULT '{}',
			ADD COLUMN IF NOT EXISTS visitor_parent_account_ids BIGINT[] NOT NULL DEFAULT '{}';

		CREATE OR REPLACE FUNCTION platform.erase_closed_demo_visitors()
		RETURNS INTEGER
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		AS $$
		DECLARE
			hidden_schools BIGINT[];
			visitor RECORD;
			target_name TEXT;
			erased INTEGER := 0;
		BEGIN
			PERFORM pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:visitor-erasure', 0));
			FOR visitor IN
				SELECT state.name, state.tenant_id,
					array_remove(state.visitor_account_ids || ARRAY[state.visitor_account_id], NULL) AS visitor_account_ids,
					array_remove(state.visitor_parent_account_ids || ARRAY[state.visitor_parent_account_id], NULL) AS visitor_parent_account_ids
				FROM platform.demo_school_states AS state
				LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
				WHERE (state.status = 'failed' OR (state.status = 'ready' AND school.deleted_at IS NOT NULL))
					AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name)
				ORDER BY state.name
			LOOP
				target_name := visitor.name;
				PERFORM pg_advisory_xact_lock(hashtextextended('demo school seed:' || target_name, 3461));
				PERFORM pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:' || target_name, 0));
				SELECT state.name, state.tenant_id,
					array_remove(state.visitor_account_ids || ARRAY[state.visitor_account_id], NULL) AS visitor_account_ids,
					array_remove(state.visitor_parent_account_ids || ARRAY[state.visitor_parent_account_id], NULL) AS visitor_parent_account_ids
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
				IF cardinality(visitor.visitor_account_ids) > 0 OR cardinality(visitor.visitor_parent_account_ids) > 0 THEN
					SELECT COALESCE(array_agg(school.id), '{}') INTO hidden_schools
					FROM platform.schools AS school
					WHERE school.deleted_at IS NOT NULL
						AND school.organization_id = (SELECT organization_id FROM platform.schools WHERE id = visitor.tenant_id);
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
			DROP COLUMN IF EXISTS visitor_parent_account_ids;`); err != nil {
		return fmt.Errorf("drop demo visitor erasure: %w", err)
	}
	return nil
}
