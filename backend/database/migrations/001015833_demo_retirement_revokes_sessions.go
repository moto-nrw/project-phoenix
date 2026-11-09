package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const demoRetirementRevokesSessionsVersion = "1.15.833"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoRetirementRevokesSessionsVersion,
		Description: "Demo-Abbau beendet die Sitzungen der versteckten Demo-Schule im selben Schritt (#6305)",
		DependsOn: []string{
			accountCredentialGenerationBindingVersion, // the last migration before this one
			demoVisitorAccountErasureVersion,          // the retirement this one replaces
		},
	})
	Migrations.MustRegister(demoRetirementRevokesSessionsUp, demoRetirementRevokesSessionsDown)
}

// schoolSessionRevocationSQL is the database side of Identity & Access's
// RevokeTokensByTenantID (#6305): it ends every session a school holds, of
// every account and portal. Every request checks the family its access token
// names (#5194), so a deleted family ends the session at once.
//
// It locks the accounts of those sessions first, the lock refresh, switch
// and logout take before they touch a session, and deletes in a statement of
// its own afterwards: a rotation that committed while the lock was awaited
// has its successor deleted too, one that starts later finds no session.
// The caller's lock_timeout bounds each wait. It writes no audit event: the
// audit evidence of a revocation carries a keyed fingerprint of the family,
// which only the application can compute, and a retired demo school is
// erased with its visitor.
//
// It belongs to the auth schema and runs with the rights of its caller;
// nobody but the owner may execute it.
const schoolSessionRevocationSQL = `
		CREATE FUNCTION auth.revoke_school_sessions(school_id BIGINT)
		RETURNS INTEGER
		LANGUAGE plpgsql
		VOLATILE
		SET search_path = pg_catalog
		AS $$
		DECLARE
			revoked INTEGER;
		BEGIN
			PERFORM 1 FROM auth.accounts AS account
			WHERE account.id IN (SELECT token.account_id FROM auth.tokens AS token WHERE token.tenant_id = school_id)
			ORDER BY account.id
			FOR NO KEY UPDATE;
			DELETE FROM auth.tokens AS token WHERE token.tenant_id = school_id;
			GET DIAGNOSTICS revoked = ROW_COUNT;
			RETURN revoked;
		END;
		$$;
		REVOKE ALL ON FUNCTION auth.revoke_school_sessions(BIGINT) FROM PUBLIC;
		COMMENT ON FUNCTION auth.revoke_school_sessions(BIGINT) IS
			'Identity & Access (#6305): ends every session of the school, accounts locked first. Called by the demo retirement in the unit that hides the school.';
`

// demoRetirementRevokesSessionsSQL is platform.retire_expired_demo_schools
// of 1.15.734 that ends the sessions of each school it hides, in the
// subtransaction that hides it (#6305). Before, they lived on until the
// visitor erasure removed the visitor's own sessions, which a lock can defer
// to a later run, and the sessions of any other account of the school until
// they expired. The refresh refused them, but every request checks only the
// family an access token names, so the school stayed open to its access
// tokens, and a switch could start from it.
//
// A session lock in use defers the order like every other lock of the unit:
// school and sessions stay as they were, one log line notes the deferral,
// and the next run finds the order again. The rest of the function is
// unchanged. CREATE OR REPLACE keeps its grants (phoenix_demo only).
const demoRetirementRevokesSessionsSQL = `
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
			hidden_school BIGINT;
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
							AND school.id = state.tenant_id AND school.deleted_at IS NULL
						RETURNING school.id INTO hidden_school;
					GET DIAGNOSTICS changed = ROW_COUNT;
					-- The school's sessions end with it, in this subtransaction
					-- (#6305): every request checks the family an access token
					-- names, not the school. A session lock in use defers the
					-- order, its hiding included.
					IF changed > 0 THEN
						PERFORM auth.revoke_school_sessions(hidden_school);
					END IF;
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

// demoRetiredSchoolSessionsBackfillSQL ends the sessions of the demo schools
// hidden before this migration. Only a school with a recorded demo origin
// (platform.demo_school_origins, #5544) is touched.
const demoRetiredSchoolSessionsBackfillSQL = `
		SELECT auth.revoke_school_sessions(school.id)
		FROM platform.schools AS school
		JOIN platform.demo_school_origins AS origin ON origin.school_id = school.id
		WHERE school.deleted_at IS NOT NULL
		ORDER BY school.id;
`

func demoRetirementRevokesSessionsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, schoolSessionRevocationSQL); err != nil {
		return fmt.Errorf("create the session revocation of a school: %w", err)
	}
	if _, err := db.ExecContext(ctx, demoRetirementRevokesSessionsSQL); err != nil {
		return fmt.Errorf("end the sessions of a demo school in its retirement: %w", err)
	}
	if _, err := db.ExecContext(ctx, demoRetiredSchoolSessionsBackfillSQL); err != nil {
		return fmt.Errorf("end the sessions of the demo schools already retired: %w", err)
	}
	return nil
}

// demoRetirementOf734SQL is the retirement of 1.15.734, without the deferral
// note that migration also created.
func demoRetirementOf734SQL() string {
	return demoSchoolRetirementWithoutWaitSQL[strings.Index(demoSchoolRetirementWithoutWaitSQL,
		"CREATE OR REPLACE FUNCTION platform.retire_expired_demo_schools"):]
}

// demoRetirementRevokesSessionsDown reinstalls the retirement of 1.15.734 and
// drops the revocation. Sessions it ended stay ended.
func demoRetirementRevokesSessionsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoRetirementOf734SQL()); err != nil {
		return fmt.Errorf("restore the demo school retirement of 1.15.734: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DROP FUNCTION IF EXISTS auth.revoke_school_sessions(BIGINT)`); err != nil {
		return fmt.Errorf("drop the session revocation of a school: %w", err)
	}
	return nil
}
