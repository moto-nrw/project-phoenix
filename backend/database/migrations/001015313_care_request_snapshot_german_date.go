package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careRequestSnapshotGermanDateVersion     = "1.15.313"
	careRequestSnapshotGermanDateDescription = "Rewrite frozen pickup-change diff labels from ISO to German dates (#2480)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careRequestSnapshotGermanDateVersion,
		Description: careRequestSnapshotGermanDateDescription,
		DependsOn:   []string{careRequestDecisionSnapshotVersion},
	})
	Migrations.MustRegister(careRequestSnapshotGermanDateUp, careRequestSnapshotGermanDateDown)
}

// The decision snapshot freezes the diff as it read at decision time (ADR
// 0002), labels included. Requests decided before #2480 therefore keep the ISO
// label "2026-08-23 · Abholzeit" forever, while every request decided
// afterwards says "23.08.2026 · Abholzeit" — two date formats in one list,
// which is the very confusion #2362 removed from the neighbouring queue.
//
// Only the date part is rewritten, and only where the label starts with a
// complete ISO date followed by the separator the builder writes. A weekday
// label ("Montag · Abholzeit") cannot match, so the regex needs no kind filter.
func careRequestSnapshotGermanDateUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.313: Rewriting frozen pickup-change labels to German dates...")

	res, err := db.ExecContext(ctx, `
		UPDATE schedule.care_schedule_change_requests r
		   SET decision_snapshot = jsonb_set(
		           r.decision_snapshot,
		           '{diff}',
		           (
		               SELECT jsonb_agg(
		                          CASE
		                            WHEN entry->>'label' ~ '^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01]) · '
		                            THEN jsonb_set(
		                                     entry,
		                                     '{label}',
		                                     to_jsonb(
		                                         to_char(
		                                             to_date(left(entry->>'label', 10), 'YYYY-MM-DD'),
		                                             'DD.MM.YYYY'
		                                         ) || substr(entry->>'label', 11)
		                                     )
		                                 )
		                            ELSE entry
		                          END
		                          ORDER BY idx
		                      )
		                 FROM jsonb_array_elements(r.decision_snapshot->'diff')
		                      WITH ORDINALITY AS t(entry, idx)
		           )
		       )
		 WHERE r.decision_snapshot ? 'diff'
		   AND jsonb_typeof(r.decision_snapshot->'diff') = 'array'
		   AND EXISTS (
		       SELECT 1
		         FROM jsonb_array_elements(r.decision_snapshot->'diff') AS e(entry)
		        WHERE e.entry->>'label' ~ '^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01]) · '
		   );
	`)
	if err != nil {
		return fmt.Errorf("rewrite frozen pickup-change labels: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count rewritten frozen pickup-change labels: %w", err)
	}
	fmt.Printf("Migration 1.15.313: %d snapshot(s) rewritten\n", rows)
	return nil
}

// Deliberate no-op: after the rewrite a German label may come from this
// migration or from the running code, and nothing tells them apart. Converting
// every German label back to ISO would corrupt the ones written after the
// change, so the presentation stays as it is — the frozen values themselves
// (old/new times) were never touched.
func careRequestSnapshotGermanDateDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back 1.15.313: no-op — the label rewrite is not reversible.")
	return nil
}
