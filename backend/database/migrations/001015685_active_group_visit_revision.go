package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const activeGroupVisitRevisionVersion = "1.15.685"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activeGroupVisitRevisionVersion,
		Description: "Revision of the open visit count of a session for the kiosk counter (#5128)",
		DependsOn: []string{
			ActiveGroupsVersion,               // active.groups receives the revision
			ActiveVisitsVersion,               // active.visits changes the count
			parentBillingDocumentInboxVersion, // the last migration before this one (#4671)
		},
	})
	Migrations.MustRegister(activeGroupVisitRevisionUp, activeGroupVisitRevisionDown)
}

// activeGroupVisitRevisionUpSQL gives every session (active.groups) a
// revision of its open visit count (#5128). The kiosk orders counts by it
// instead of by the start of its requests.
//
//   - Every statement on active.visits that opens, closes, moves or deletes
//     an open visit raises the revision of each session whose count it
//     changed by one, in the same transaction. A statement trigger covers
//     every writer, including bulk session ends, cleanups and erasures.
//   - The trigger locks the sessions in ascending id order (FOR NO KEY
//     UPDATE, so visit inserts' FK key-share locks do not conflict) before it
//     raises them. Writers of one session therefore commit in revision order,
//     and a writer that reads count and revision after its change sees both
//     of its own transaction.
//   - The revision change does not touch updated_at: a session's own data
//     did not change.
const activeGroupVisitRevisionUpSQL = `
ALTER TABLE active.groups ADD COLUMN IF NOT EXISTS visit_revision BIGINT NOT NULL DEFAULT 0;

CREATE OR REPLACE FUNCTION active.bump_group_visit_revision() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
	changed BIGINT[];
BEGIN
	IF TG_OP = 'INSERT' THEN
		SELECT array_agg(DISTINCT n.active_group_id) INTO changed
		FROM visit_rows_new n WHERE n.exit_time IS NULL;
	ELSIF TG_OP = 'DELETE' THEN
		SELECT array_agg(DISTINCT o.active_group_id) INTO changed
		FROM visit_rows_old o WHERE o.exit_time IS NULL;
	ELSE
		SELECT array_agg(DISTINCT session_id) INTO changed FROM (
			SELECT o.active_group_id AS session_id
			FROM visit_rows_old o JOIN visit_rows_new n ON n.id = o.id
			WHERE o.exit_time IS NULL
				AND (n.exit_time IS NOT NULL OR n.active_group_id IS DISTINCT FROM o.active_group_id)
			UNION
			SELECT n.active_group_id
			FROM visit_rows_old o JOIN visit_rows_new n ON n.id = o.id
			WHERE n.exit_time IS NULL
				AND (o.exit_time IS NOT NULL OR n.active_group_id IS DISTINCT FROM o.active_group_id)
		) moved;
	END IF;
	IF changed IS NULL THEN
		RETURN NULL;
	END IF;
	PERFORM 1 FROM active.groups WHERE id = ANY (changed) ORDER BY id FOR NO KEY UPDATE;
	UPDATE active.groups SET visit_revision = visit_revision + 1 WHERE id = ANY (changed);
	RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS bump_group_visit_revision_insert ON active.visits;
CREATE TRIGGER bump_group_visit_revision_insert AFTER INSERT ON active.visits
	REFERENCING NEW TABLE AS visit_rows_new
	FOR EACH STATEMENT EXECUTE FUNCTION active.bump_group_visit_revision();
DROP TRIGGER IF EXISTS bump_group_visit_revision_update ON active.visits;
CREATE TRIGGER bump_group_visit_revision_update AFTER UPDATE ON active.visits
	REFERENCING OLD TABLE AS visit_rows_old NEW TABLE AS visit_rows_new
	FOR EACH STATEMENT EXECUTE FUNCTION active.bump_group_visit_revision();
DROP TRIGGER IF EXISTS bump_group_visit_revision_delete ON active.visits;
CREATE TRIGGER bump_group_visit_revision_delete AFTER DELETE ON active.visits
	REFERENCING OLD TABLE AS visit_rows_old
	FOR EACH STATEMENT EXECUTE FUNCTION active.bump_group_visit_revision();

DROP TRIGGER IF EXISTS update_active_groups_updated_at ON active.groups;
CREATE TRIGGER update_active_groups_updated_at
	BEFORE UPDATE ON active.groups
	FOR EACH ROW
	WHEN (OLD.visit_revision IS NOT DISTINCT FROM NEW.visit_revision)
	EXECUTE FUNCTION update_modified_column();
`

const activeGroupVisitRevisionDownSQL = `
DROP TRIGGER IF EXISTS update_active_groups_updated_at ON active.groups;
CREATE TRIGGER update_active_groups_updated_at
	BEFORE UPDATE ON active.groups
	FOR EACH ROW
	EXECUTE FUNCTION update_modified_column();
DROP TRIGGER IF EXISTS bump_group_visit_revision_insert ON active.visits;
DROP TRIGGER IF EXISTS bump_group_visit_revision_update ON active.visits;
DROP TRIGGER IF EXISTS bump_group_visit_revision_delete ON active.visits;
DROP FUNCTION IF EXISTS active.bump_group_visit_revision();
ALTER TABLE active.groups DROP COLUMN IF EXISTS visit_revision;
`

func activeGroupVisitRevisionUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, activeGroupVisitRevisionUpSQL); err != nil {
		return fmt.Errorf("add active group visit revision: %w", err)
	}
	return nil
}

func activeGroupVisitRevisionDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, activeGroupVisitRevisionDownSQL); err != nil {
		return fmt.Errorf("revert active group visit revision: %w", err)
	}
	return nil
}
