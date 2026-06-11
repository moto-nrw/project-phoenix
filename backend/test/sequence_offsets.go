package test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var sequenceOffsetsOnce sync.Once

// applySequenceOffsets moves every table's PK sequence into its own disjoint
// numeric range (Nth sequence by schema+name starts at N*10M), so IDs from
// different tables can never collide numerically.
//
// Why: the generic cleanup helpers (CleanupActivityFixtures and friends) try
// each passed ID against many tables with `id = ?` arms. With all sequences
// starting near 1, a staff PK from one test package regularly equals an
// activity-group or supervisor PK from another package running concurrently
// against the shared test DB, and the cleanup deletes the foreign row
// mid-test. Disjoint ranges kill the whole collision class without touching
// any call site.
//
// Properties:
//   - Idempotent and monotonic: setval(GREATEST(last_value, offset)) never
//     moves a sequence backward, so concurrent test binaries all converge.
//   - Explicit-ID system fixtures (schools.id=1, rooms.id=1, staff.id=1)
//     are unaffected; offsets only change nextval.
//   - Max offset stays far below the int4 ceiling (2.1B) up to ~200 tables.
//   - Ordinals are positional: adding a migration with a new table can shift
//     assignments on a long-lived local DB. CI databases are fresh per run;
//     locally, `APP_ENV=test go run main.go migrate reset` re-baselines.
func applySequenceOffsets(tb testing.TB, db *bun.DB) {
	tb.Helper()
	sequenceOffsetsOnce.Do(func() {
		_, err := db.ExecContext(context.Background(), `
DO $$
DECLARE
  seq record;
  cur bigint;
  i bigint := 0;
BEGIN
  FOR seq IN
    SELECT n.nspname, c.relname
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE c.relkind = 'S'
      AND n.nspname NOT IN ('pg_catalog', 'information_schema')
    ORDER BY n.nspname, c.relname
  LOOP
    i := i + 1;
    EXECUTE format('SELECT last_value FROM %I.%I', seq.nspname, seq.relname) INTO cur;
    PERFORM setval(format('%I.%I', seq.nspname, seq.relname), GREATEST(cur, i * 10000000));
  END LOOP;
END $$;
`)
		require.NoError(tb, err, "Failed to apply test sequence offsets")
	})
}
