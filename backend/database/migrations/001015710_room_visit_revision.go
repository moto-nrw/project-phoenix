package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const roomVisitRevisionVersion = "1.15.710"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     roomVisitRevisionVersion,
		Description: "Room scope for the change events that order the kiosk student count (#5205)",
		DependsOn: []string{
			visitRevisionEventsVersion, // the event table and visit trigger this one extends
		},
	})
	Migrations.MustRegister(roomVisitRevisionUp, roomVisitRevisionDown)
}

// roomVisitRevisionUpSQL gives the change events of 1.15.705 a room scope
// (#5205, ADR 0133). The revision of a room is the sum of the events with its
// room_id that a read sees; the revision of a session stays the sum of the
// events with its active_group_id.
//
//   - The visit trigger stamps each session event with the room the
//     session has when the statement runs, also for an ended session, so a
//     visit write racing a session end or reopen still raises the room.
//   - A row trigger on active.groups records a room event (no session) when
//     a session changes its room (old and new room), ends or reopens, and
//     before an open session with open visits is deleted: the visits of a
//     deleted session cascade after the session row is gone, so the visit
//     trigger cannot name their room. The school erasure deletes the visits
//     before the sessions, so it finds no open visit there and records
//     nothing after it deleted the event table.
//   - Neither trigger locks a row. Both only insert events and read
//     active.groups or active.visits without a lock; room_id has no foreign
//     key, like active_group_id, so no key-share lock reaches a room or a
//     session.
const roomVisitRevisionUpSQL = `
ALTER TABLE active.visit_revision_events ADD COLUMN IF NOT EXISTS room_id BIGINT;
ALTER TABLE active.visit_revision_events ALTER COLUMN active_group_id DROP NOT NULL;
ALTER TABLE active.visit_revision_events DROP CONSTRAINT IF EXISTS visit_revision_events_scope;
ALTER TABLE active.visit_revision_events ADD CONSTRAINT visit_revision_events_scope
	CHECK (active_group_id IS NOT NULL OR room_id IS NOT NULL);
CREATE INDEX IF NOT EXISTS idx_visit_revision_events_room
	ON active.visit_revision_events (tenant_id, room_id) WHERE room_id IS NOT NULL;

CREATE OR REPLACE FUNCTION active.record_visit_revision() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
	IF TG_OP = 'INSERT' THEN
		INSERT INTO active.visit_revision_events (tenant_id, active_group_id, room_id)
		SELECT c.tenant_id, c.active_group_id, g.room_id
		FROM (SELECT DISTINCT n.tenant_id, n.active_group_id FROM visit_rows_new n WHERE n.exit_time IS NULL) AS c
		LEFT JOIN active.groups AS g ON g.tenant_id = c.tenant_id AND g.id = c.active_group_id;
	ELSIF TG_OP = 'DELETE' THEN
		INSERT INTO active.visit_revision_events (tenant_id, active_group_id, room_id)
		SELECT c.tenant_id, c.active_group_id, g.room_id
		FROM (SELECT DISTINCT o.tenant_id, o.active_group_id FROM visit_rows_old o WHERE o.exit_time IS NULL) AS c
		LEFT JOIN active.groups AS g ON g.tenant_id = c.tenant_id AND g.id = c.active_group_id;
	ELSE
		INSERT INTO active.visit_revision_events (tenant_id, active_group_id, room_id)
		SELECT c.tenant_id, c.active_group_id, g.room_id
		FROM (
			SELECT o.tenant_id, o.active_group_id
			FROM visit_rows_old o JOIN visit_rows_new n ON n.id = o.id
			WHERE o.exit_time IS NULL
				AND (n.exit_time IS NOT NULL OR n.active_group_id IS DISTINCT FROM o.active_group_id)
			UNION
			SELECT n.tenant_id, n.active_group_id
			FROM visit_rows_old o JOIN visit_rows_new n ON n.id = o.id
			WHERE n.exit_time IS NULL
				AND (o.exit_time IS NOT NULL OR n.active_group_id IS DISTINCT FROM o.active_group_id)
		) AS c
		LEFT JOIN active.groups AS g ON g.tenant_id = c.tenant_id AND g.id = c.active_group_id;
	END IF;
	RETURN NULL;
END;
$$;
REVOKE ALL ON FUNCTION active.record_visit_revision() FROM PUBLIC;

CREATE OR REPLACE FUNCTION active.record_room_revision() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
	IF TG_OP = 'DELETE' THEN
		IF EXISTS (SELECT 1 FROM active.visits AS v
			WHERE v.tenant_id = OLD.tenant_id AND v.active_group_id = OLD.id AND v.exit_time IS NULL) THEN
			INSERT INTO active.visit_revision_events (tenant_id, room_id) VALUES (OLD.tenant_id, OLD.room_id);
		END IF;
		RETURN OLD;
	END IF;
	INSERT INTO active.visit_revision_events (tenant_id, room_id)
	SELECT DISTINCT NEW.tenant_id, r.room_id FROM (VALUES (OLD.room_id), (NEW.room_id)) AS r(room_id);
	RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION active.record_room_revision() FROM PUBLIC;

DROP TRIGGER IF EXISTS record_room_revision_update ON active.groups;
CREATE TRIGGER record_room_revision_update AFTER UPDATE OF room_id, end_time ON active.groups
	FOR EACH ROW WHEN (OLD.room_id IS DISTINCT FROM NEW.room_id OR (OLD.end_time IS NULL) <> (NEW.end_time IS NULL))
	EXECUTE FUNCTION active.record_room_revision();
DROP TRIGGER IF EXISTS record_room_revision_delete ON active.groups;
CREATE TRIGGER record_room_revision_delete BEFORE DELETE ON active.groups
	FOR EACH ROW WHEN (OLD.end_time IS NULL)
	EXECUTE FUNCTION active.record_room_revision();
`

// roomVisitRevisionDownSQL restores the session-only events of 1.15.705. Room
// events carry no session and go; the session events keep their sums.
const roomVisitRevisionDownSQL = `
DROP TRIGGER IF EXISTS record_room_revision_update ON active.groups;
DROP TRIGGER IF EXISTS record_room_revision_delete ON active.groups;
DROP FUNCTION IF EXISTS active.record_room_revision();
DELETE FROM active.visit_revision_events WHERE active_group_id IS NULL;
ALTER TABLE active.visit_revision_events DROP CONSTRAINT IF EXISTS visit_revision_events_scope;
ALTER TABLE active.visit_revision_events ALTER COLUMN active_group_id SET NOT NULL;
DROP INDEX IF EXISTS active.idx_visit_revision_events_room;
ALTER TABLE active.visit_revision_events DROP COLUMN IF EXISTS room_id;
`

func roomVisitRevisionUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, roomVisitRevisionUpSQL); err != nil {
			return fmt.Errorf("add the room scope to visit revision events: %w", err)
		}
		return nil
	})
}

// roomVisitRevisionDown reinstalls the visit trigger function of 1.15.705
// after it removed the room scope.
func roomVisitRevisionDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, roomVisitRevisionDownSQL); err != nil {
			return fmt.Errorf("remove the room scope from visit revision events: %w", err)
		}
		if _, err := tx.ExecContext(ctx, sessionVisitRevisionFunctionSQL); err != nil {
			return fmt.Errorf("restore the session visit revision trigger: %w", err)
		}
		return nil
	})
}

// sessionVisitRevisionFunctionSQL is a static copy of the 1.15.705 visit
// trigger function, which records session events without a room.
const sessionVisitRevisionFunctionSQL = `
CREATE OR REPLACE FUNCTION active.record_visit_revision() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
	IF TG_OP = 'INSERT' THEN
		INSERT INTO active.visit_revision_events (tenant_id, active_group_id)
		SELECT DISTINCT n.tenant_id, n.active_group_id FROM visit_rows_new n WHERE n.exit_time IS NULL;
	ELSIF TG_OP = 'DELETE' THEN
		INSERT INTO active.visit_revision_events (tenant_id, active_group_id)
		SELECT DISTINCT o.tenant_id, o.active_group_id FROM visit_rows_old o WHERE o.exit_time IS NULL;
	ELSE
		INSERT INTO active.visit_revision_events (tenant_id, active_group_id)
		SELECT o.tenant_id, o.active_group_id
		FROM visit_rows_old o JOIN visit_rows_new n ON n.id = o.id
		WHERE o.exit_time IS NULL
			AND (n.exit_time IS NOT NULL OR n.active_group_id IS DISTINCT FROM o.active_group_id)
		UNION
		SELECT n.tenant_id, n.active_group_id
		FROM visit_rows_old o JOIN visit_rows_new n ON n.id = o.id
		WHERE n.exit_time IS NULL
			AND (o.exit_time IS NOT NULL OR n.active_group_id IS DISTINCT FROM o.active_group_id);
	END IF;
	RETURN NULL;
END;
$$;
REVOKE ALL ON FUNCTION active.record_visit_revision() FROM PUBLIC;
`
