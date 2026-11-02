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
// (#5205, ADR 0133). It adds two row kinds next to the session events, which
// stay as 1.15.705 writes them:
//
//   - An entry row (room_id, entered_group_id) records that a session was
//     in a room: when it starts there, moves there or reopens there. Entry
//     rows are never removed while their session exists, so the set of
//     sessions a room ever held only grows.
//   - A room row (room_id only) records a membership change: a session left
//     the room, ended or reopened in it.
//
// The revision of a room is the sum of its entry and room rows plus the
// session revision of every session it ever held, all from one snapshot.
// The read decides which sessions belong to the room, not the writer: a
// visit write racing a room change raises the session, and the session
// counts in both rooms, so no write is attributed to a room it saw too
// early (the finding of #5232). The session revision stays the sum of the
// rows with its active_group_id.
//
// The triggers only insert rows. room_id and entered_group_id have no
// foreign key, like active_group_id, so no key-share lock reaches a room or
// a session, and no trigger fires on deletes: the visits of a deleted
// session cascade into session events, and the nightly compaction folds a
// deleted session's events into room rows of the rooms it entered.
const roomVisitRevisionUpSQL = `
ALTER TABLE active.visit_revision_events ADD COLUMN IF NOT EXISTS room_id BIGINT;
ALTER TABLE active.visit_revision_events ADD COLUMN IF NOT EXISTS entered_group_id BIGINT;
ALTER TABLE active.visit_revision_events ALTER COLUMN active_group_id DROP NOT NULL;
ALTER TABLE active.visit_revision_events DROP CONSTRAINT IF EXISTS visit_revision_events_scope;
ALTER TABLE active.visit_revision_events ADD CONSTRAINT visit_revision_events_scope CHECK (
	(active_group_id IS NOT NULL AND room_id IS NULL AND entered_group_id IS NULL)
	OR (active_group_id IS NULL AND room_id IS NOT NULL));
CREATE INDEX IF NOT EXISTS idx_visit_revision_events_room
	ON active.visit_revision_events (tenant_id, room_id) INCLUDE (entered_group_id, changes)
	WHERE room_id IS NOT NULL;

CREATE OR REPLACE FUNCTION active.record_room_revision() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
	IF TG_OP = 'INSERT' OR OLD.room_id IS DISTINCT FROM NEW.room_id
		OR (OLD.end_time IS NOT NULL AND NEW.end_time IS NULL) THEN
		INSERT INTO active.visit_revision_events (tenant_id, room_id, entered_group_id)
		VALUES (NEW.tenant_id, NEW.room_id, NEW.id);
	END IF;
	IF TG_OP = 'UPDATE' AND OLD.room_id IS DISTINCT FROM NEW.room_id THEN
		INSERT INTO active.visit_revision_events (tenant_id, room_id) VALUES (OLD.tenant_id, OLD.room_id);
	END IF;
	IF TG_OP = 'UPDATE' AND OLD.end_time IS NULL AND NEW.end_time IS NOT NULL THEN
		INSERT INTO active.visit_revision_events (tenant_id, room_id) VALUES (NEW.tenant_id, NEW.room_id);
	END IF;
	RETURN NULL;
END;
$$;
REVOKE ALL ON FUNCTION active.record_room_revision() FROM PUBLIC;

DROP TRIGGER IF EXISTS record_room_revision_insert ON active.groups;
CREATE TRIGGER record_room_revision_insert AFTER INSERT ON active.groups
	FOR EACH ROW EXECUTE FUNCTION active.record_room_revision();
DROP TRIGGER IF EXISTS record_room_revision_update ON active.groups;
CREATE TRIGGER record_room_revision_update AFTER UPDATE OF room_id, end_time ON active.groups
	FOR EACH ROW WHEN (OLD.room_id IS DISTINCT FROM NEW.room_id OR (OLD.end_time IS NULL) <> (NEW.end_time IS NULL))
	EXECUTE FUNCTION active.record_room_revision();

INSERT INTO active.visit_revision_events (tenant_id, room_id, entered_group_id)
SELECT g.tenant_id, g.room_id, g.id FROM active.groups AS g
WHERE NOT EXISTS (SELECT 1 FROM active.visit_revision_events AS e
	WHERE e.tenant_id = g.tenant_id AND e.room_id = g.room_id AND e.entered_group_id = g.id);
`

// roomVisitRevisionDownSQL restores the session-only events of 1.15.705.
// Entry and room rows carry no session and go; the session events keep their
// sums.
const roomVisitRevisionDownSQL = `
DROP TRIGGER IF EXISTS record_room_revision_insert ON active.groups;
DROP TRIGGER IF EXISTS record_room_revision_update ON active.groups;
DROP FUNCTION IF EXISTS active.record_room_revision();
DELETE FROM active.visit_revision_events WHERE active_group_id IS NULL;
ALTER TABLE active.visit_revision_events DROP CONSTRAINT IF EXISTS visit_revision_events_scope;
ALTER TABLE active.visit_revision_events ALTER COLUMN active_group_id SET NOT NULL;
DROP INDEX IF EXISTS active.idx_visit_revision_events_room;
ALTER TABLE active.visit_revision_events DROP COLUMN IF EXISTS entered_group_id;
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

func roomVisitRevisionDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, roomVisitRevisionDownSQL); err != nil {
			return fmt.Errorf("remove the room scope from visit revision events: %w", err)
		}
		return nil
	})
}
