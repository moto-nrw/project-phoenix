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
// 14 days after their last use and hidden the schools left without one.
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
// The names are matched in every hidden school of the Demo-Träger, not only
// in the school of the order: a seed that failed and was repeated left its
// first school behind without a link to the order
// (cmd/devtools/demo_scheduler.go, ReplaceAbandoned). Visible schools are
// never touched. The Demo-Träger is
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
// Platform, Communication, Delivery Platform). That is a deliberate exception
// for the demo environment, which holds no school data of its own; #4130
// decides whether it stays.
const demoVisitorErasureUpSQL = `
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
			visitor_name TEXT;
			erased INTEGER := 0;
		BEGIN
			SELECT COALESCE(array_agg(school.id), '{}') INTO hidden_schools
			FROM platform.schools AS school
			WHERE school.deleted_at IS NOT NULL
				AND school.organization_id IN (
					SELECT demo_school.organization_id
					FROM platform.demo_school_states AS state
					JOIN platform.schools AS demo_school ON demo_school.id = state.tenant_id);

			FOR visitor IN
				SELECT state.name, state.first_name, state.last_name
				FROM platform.demo_school_states AS state
				LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
				WHERE (state.status = 'failed' OR (state.status = 'ready' AND school.deleted_at IS NOT NULL))
					AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name)
				FOR UPDATE OF state
			LOOP
				IF visitor.first_name <> '' AND visitor.last_name <> '' THEN
					visitor_name := visitor.first_name || ' ' || visitor.last_name;
					UPDATE users.persons SET first_name = 'Demo', last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND first_name = visitor.first_name AND last_name = visitor.last_name;
					UPDATE users.guardian_profiles SET first_name = 'Demo', last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND first_name = visitor.first_name AND last_name = visitor.last_name;
					UPDATE enrollment.requests SET guardian_first_name = 'Demo', guardian_last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND guardian_first_name = visitor.first_name AND guardian_last_name = visitor.last_name;
					UPDATE audit.guardian_changes SET actor_name_snapshot = 'Demo Besuch'
					WHERE tenant_id = ANY (hidden_schools) AND actor_name_snapshot = visitor_name;
					UPDATE users.parent_messages SET sender_name = 'Demo Besuch'
					WHERE tenant_id = ANY (hidden_schools) AND sender_name = visitor_name;
					DELETE FROM users.parent_announcement_declaration_submissions
					WHERE tenant_id = ANY (hidden_schools) AND signer_name = visitor_name;
					DELETE FROM platform.email_outbox
					WHERE tenant_id = ANY (hidden_schools)
						AND strpos(payload::text, visitor.first_name) > 0
						AND strpos(payload::text, visitor.last_name) > 0;
				END IF;
				DELETE FROM platform.demo_school_states WHERE name = visitor.name;
				erased := erased + 1;
			END LOOP;
			RETURN erased;
		END;
		$$;

		REVOKE ALL ON FUNCTION platform.erase_closed_demo_visitors() FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION platform.erase_closed_demo_visitors() TO phoenix_demo;
`

func demoVisitorErasureUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoVisitorErasureUpSQL); err != nil {
		return fmt.Errorf("create demo visitor erasure: %w", err)
	}
	return nil
}

func demoVisitorErasureDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP FUNCTION IF EXISTS platform.erase_closed_demo_visitors()`); err != nil {
		return fmt.Errorf("drop demo visitor erasure: %w", err)
	}
	return nil
}
