package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const visitRevisionSessionIndexChangesVersion = "1.15.718"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     visitRevisionSessionIndexChangesVersion,
		Description: "Session index of the visit revision events carries the changes (#5273)",
		DependsOn: []string{
			holidayRegistrationProofsVersion, // the last migration before this one
			roomVisitRevisionVersion,         // the room read this index serves
		},
	})
	Migrations.MustRegister(visitRevisionSessionIndexChangesUp, visitRevisionSessionIndexChangesDown)
}

// visitRevisionSessionIndexChangesUpSQL lets the session index of 1.15.705
// carry the changes of each row (#5273). The room revision of 1.15.710 sums
// the revision of every session a room ever held; with the changes in the
// index, PostgreSQL reads each of those sums from the index alone wherever
// vacuum has marked the table pages all-visible, by one
// probe per held session or one pass over the school's index entries,
// without visiting the table. The key stays (tenant_id, active_group_id) and
// the index stays without a predicate, so the session revision and the school
// erasure use it as before, and every event insert still maintains one index
// for its session scope. The swap runs in one transaction: no reader sees the
// table without its session index, and event inserts wait while the index
// builds over the compacted rows.
const visitRevisionSessionIndexChangesUpSQL = `
CREATE INDEX idx_visit_revision_events_session_changes
	ON active.visit_revision_events (tenant_id, active_group_id) INCLUDE (changes);
DROP INDEX IF EXISTS active.idx_visit_revision_events_session;
ALTER INDEX active.idx_visit_revision_events_session_changes RENAME TO idx_visit_revision_events_session;
`

// visitRevisionSessionIndexChangesDownSQL restores the key-only index of
// 1.15.705.
const visitRevisionSessionIndexChangesDownSQL = `
DROP INDEX IF EXISTS active.idx_visit_revision_events_session;
CREATE INDEX idx_visit_revision_events_session
	ON active.visit_revision_events (tenant_id, active_group_id);
`

func visitRevisionSessionIndexChangesUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, visitRevisionSessionIndexChangesUpSQL); err != nil {
			return fmt.Errorf("add the changes to the visit revision session index: %w", err)
		}
		return nil
	})
}

func visitRevisionSessionIndexChangesDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, visitRevisionSessionIndexChangesDownSQL); err != nil {
			return fmt.Errorf("remove the changes from the visit revision session index: %w", err)
		}
		return nil
	})
}
