package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const visitRevisionEventsVersion = "1.15.705"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     visitRevisionEventsVersion,
		Description: "Append-only change events that order the kiosk student count of a session (#5128)",
		DependsOn: []string{
			ActiveVisitsVersion,          // active.visits changes the count
			holidayGuestErasureVersion,   // the erasure functions this one extends
			manualRosterConfirmedVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(visitRevisionEventsUp, visitRevisionEventsDown)
}

// visitRevisionEventsUpSQL records the changes of a session's open visit
// count in an append-only table (#5128, ADR 0131). The revision of a session
// is the sum of its events that a read sees; no writer locks a row for it.
//
//   - Every statement on active.visits that opens, closes, moves or deletes
//     an open visit inserts one event per session whose count it changed,
//     in the writer's transaction. A later snapshot sees every event an
//     earlier one saw, so the revision never falls, also for deletes and
//     moves, and it does not depend on the start or answer order of
//     requests.
//   - The trigger only inserts. active_group_id has no foreign key on
//     purpose: its key-share lock would take a lock on the session row
//     that visit closes and moves do not take today. tenant_id references
//     the school like every tenant table; visit inserts already take that
//     key-share lock.
//   - The request role may insert, read and delete, never update: events
//     are not rewritten. The nightly close compacts the events of ended
//     sessions into one row with their sum (changes), which keeps every
//     revision.
const visitRevisionEventsUpSQL = `
CREATE TABLE IF NOT EXISTS active.visit_revision_events (
	id              BIGSERIAL PRIMARY KEY,
	tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
	active_group_id BIGINT NOT NULL,
	changes         INTEGER NOT NULL DEFAULT 1 CHECK (changes > 0),
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_visit_revision_events_session
	ON active.visit_revision_events (tenant_id, active_group_id);

GRANT SELECT, INSERT, DELETE ON active.visit_revision_events TO phoenix_tenant;
REVOKE UPDATE, TRUNCATE ON active.visit_revision_events FROM phoenix_tenant;
GRANT USAGE ON SEQUENCE active.visit_revision_events_id_seq TO phoenix_tenant;
GRANT ALL ON active.visit_revision_events TO phoenix_admin;
GRANT USAGE ON SEQUENCE active.visit_revision_events_id_seq TO phoenix_admin;

CREATE OR REPLACE FUNCTION active.record_visit_revision() RETURNS trigger
LANGUAGE plpgsql AS $$
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

DROP TRIGGER IF EXISTS record_visit_revision_insert ON active.visits;
CREATE TRIGGER record_visit_revision_insert AFTER INSERT ON active.visits
	REFERENCING NEW TABLE AS visit_rows_new
	FOR EACH STATEMENT EXECUTE FUNCTION active.record_visit_revision();
DROP TRIGGER IF EXISTS record_visit_revision_update ON active.visits;
CREATE TRIGGER record_visit_revision_update AFTER UPDATE ON active.visits
	REFERENCING OLD TABLE AS visit_rows_old NEW TABLE AS visit_rows_new
	FOR EACH STATEMENT EXECUTE FUNCTION active.record_visit_revision();
DROP TRIGGER IF EXISTS record_visit_revision_delete ON active.visits;
CREATE TRIGGER record_visit_revision_delete AFTER DELETE ON active.visits
	REFERENCING OLD TABLE AS visit_rows_old
	FOR EACH STATEMENT EXECUTE FUNCTION active.record_visit_revision();
`

const visitRevisionEventsDownSQL = `
DROP TRIGGER IF EXISTS record_visit_revision_insert ON active.visits;
DROP TRIGGER IF EXISTS record_visit_revision_update ON active.visits;
DROP TRIGGER IF EXISTS record_visit_revision_delete ON active.visits;
DROP FUNCTION IF EXISTS active.record_visit_revision();
DROP TABLE IF EXISTS active.visit_revision_events;
`

func visitRevisionEventsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, visitRevisionEventsUpSQL); err != nil {
			return fmt.Errorf("create visit revision events: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "active.visit_revision_events"); err != nil {
			return err
		}
		for _, step := range []func(context.Context, bun.Tx) error{
			installVisitRevisionErasurePlan, installVisitRevisionErasurePreflight, installVisitRevisionErasureResidual,
			installVisitRevisionErasureDelete, installVisitRevisionErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("install the erasure with visit revision events: %w", err)
			}
		}
		return nil
	})
}

// visitRevisionEventsDown reinstalls the 1.15.681 erasure before it drops
// the table, so the installed plan never names a missing table.
func visitRevisionEventsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installHolidayGuestErasurePlan, installHolidayGuestErasurePreflight, installHolidayGuestErasureResidual,
			installHolidayGuestErasureDelete, installHolidayGuestErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("restore the erasure without visit revision events: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, visitRevisionEventsDownSQL); err != nil {
			return fmt.Errorf("drop visit revision events: %w", err)
		}
		return nil
	})
}
