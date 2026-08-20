package test

import (
	"context"

	"github.com/uptrace/bun"
)

// applySequenceOffsets moves every table's PK sequence into its own disjoint
// numeric range (Nth sequence by schema+name starts at N*10M), so IDs from
// different tables can never collide numerically. It runs once per package
// clone from the process-once bootstrap in db_clone.go.
//
// Why: the generic cleanup helpers (CleanupActivityFixtures and friends) try
// each passed ID against many tables with `id = ?` arms. With all sequences
// starting near 1, a staff PK regularly equals an activity-group or
// supervisor PK from another test, and the cleanup deletes the foreign row
// mid-test. Disjoint ranges kill the whole collision class without touching
// any call site. The offsets stay until the cleanup helpers are gone.
//
// That is a mechanical condition, not a judgement call: this whole file is
// dead the moment cleanupCallBaseline in test/hermetic_verification_test.go
// reaches zero, because the ID-collision class it defends against exists
// only inside those helpers' cross-table `id = ?` arms. The #2419 teardown
// sweep took 5120 calls down to 587 in 11 packages — the remainder is
// teardown the clone genuinely does not cover (tenant-less rows, state reset
// between subtests, the delete that is the test itself), so the offsets stay
// for now (ADR 0004).
//
// Properties:
//   - Idempotent and monotonic: setval(GREATEST(last_value, offset)) never
//     moves a sequence backward.
//   - Explicit-ID system fixtures (schools.id=1, rooms.id=1, staff.id=1)
//     are unaffected; offsets only change nextval.
//   - Max offset stays far below the int4 ceiling (2.1B) up to ~200 tables.
//   - Ordinals are positional, but every package clone is created fresh from
//     the template per run, so assignments cannot drift between runs.
func applySequenceOffsets(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
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
	return err
}
