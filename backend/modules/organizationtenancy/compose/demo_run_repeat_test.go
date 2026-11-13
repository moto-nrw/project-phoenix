package compose

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// A demo run may reach no order at all: the retirement and the erasure
// start no order after their budget, the first one included (#6698). These
// helpers repeat such runs within a stated bound, so no test depends on how
// fast the database answers, and every assertion about what a run hides,
// erases and defers stays as it is.

// stalledRuns is how many runs that hide and erase nothing repeatRun
// repeats. The retirement (200 ms) and the erasure (250 ms) start no order
// after their budget, the first one included: when the database stalls that
// long before a run reaches its first order, the run ends without one and
// the order stays open for the next run (#6698). That is the product
// behaviour, so no test may expect one run to reach an order. Ten stalls in
// a row would mean the database stalled for seconds on every run, a failure
// of its own. A product fault that leaves an order open also stops after
// them, and the caller's assertion reports it.
const stalledRuns = 10

// repeatRun repeats run until done reports true for the totals hidden and
// erased so far and the deferrals of the last run, and returns the totals.
// A run that hides and erases nothing counts as stalled; after stalledRuns
// of them repeatRun returns what it has. done sees every run, so a caller
// can also check each run on its way, and ends at the first run that meets
// it: no run more than needed can do work the caller expects of none.
func repeatRun(run func() (hidden, erased int, deferrals map[string]int), done func(hidden, erased int, deferrals map[string]int) bool) (hidden, erased int) {
	for stalls := 0; stalls < stalledRuns; {
		hiddenMore, erasedMore, deferrals := run()
		hidden += hiddenMore
		erased += erasedMore
		if done(hidden, erased, deferrals) {
			return hidden, erased
		}
		if hiddenMore+erasedMore == 0 {
			stalls++
		}
	}
	return hidden, erased
}

// retireUntil repeats the demo run like countedRun, each run bounded in its
// lock waits, until done reports true (see repeatRun).
func retireUntil(t *testing.T, db *bun.DB, queue *organizationtenancy.DemoSchoolQueue, slugs []string, done func(hidden, erased int, deferrals map[string]int) bool) (hidden, erased int) {
	t.Helper()
	return repeatRun(func() (int, int, map[string]int) { return countedRun(t, db, queue, slugs) }, done)
}

// retireAtLeast repeats the demo run until it has hidden wantHidden schools
// and erased wantErased orders and returns how many it hid and erased. It
// stops at the first run that reaches both, so a caller that compares the
// totals with its wants still sees a run that hides or erases too much.
func retireAtLeast(t *testing.T, db *bun.DB, queue *organizationtenancy.DemoSchoolQueue, slugs []string, wantHidden, wantErased int) (hidden, erased int) {
	t.Helper()
	return retireUntil(t, db, queue, slugs, func(hidden, erased int, _ map[string]int) bool {
		return hidden >= wantHidden && erased >= wantErased
	})
}

// retireUntilDeferred repeats the demo run until it has deferred slug to
// want deferrals and returns what that run hid and erased. A run deferring
// slug is a run that reached it: a stalled one defers nothing.
func retireUntilDeferred(t *testing.T, db *bun.DB, queue *organizationtenancy.DemoSchoolQueue, slugs []string, slug string, want int) (hidden, erased int) {
	t.Helper()
	return retireUntil(t, db, queue, slugs, func(_, _ int, deferrals map[string]int) bool {
		return deferrals[slug] >= want
	})
}

// runOutcome is what a demo run started in the background returned.
type runOutcome struct {
	erased int
	err    error
}

// waitingRun starts the demo run in the background and returns it once
// waiting reports that it waits for the lock the test holds. A run that
// ends before, having erased nothing, stalled before its first order (see
// stalledRuns) and is started again, at most stalledRuns times; a run that
// erased the order without waiting fails the test with message. Use it with
// waitForLocks, so the run cannot give up on the lock and defer the order.
func (o demoOrder) waitingRun(t *testing.T, message string, waiting func() bool) <-chan runOutcome {
	t.Helper()
	for range stalledRuns {
		done := make(chan runOutcome, 1)
		go func() {
			_, erased, err := o.queue.RetireDemoSchools(t.Context(), nil)
			done <- runOutcome{erased: erased, err: err}
		}()
		deadline := time.After(5 * time.Second)
		poll := time.NewTicker(time.Millisecond)
		stalled := false
		for !stalled {
			select {
			case result := <-done:
				require.NoError(t, result.err)
				require.Zero(t, result.erased, "%s: the run erased the order without waiting", message)
				stalled = true
			case <-deadline:
				poll.Stop()
				require.FailNow(t, message)
			case <-poll.C:
				if waiting() {
					poll.Stop()
					return done
				}
			}
		}
		poll.Stop()
	}
	require.FailNow(t, message, "%d runs ended before their first order", stalledRuns)
	return nil
}

// stallRuns makes the next calls runs of the demo run in this test's clone
// stall before their first order, as a loaded database does (#6698): the
// retirement and the erasure each try the run lock right after they start,
// and each of the next calls tries sleeps 300 ms first, longer than either
// budget. The functions find that try through their search_path, which puts
// the schema stall before pg_catalog; their code stays as migrated.
func stallRuns(t *testing.T, db *bun.DB, calls int) {
	t.Helper()
	for _, statement := range []string{
		`CREATE SCHEMA IF NOT EXISTS stall`,
		`DROP SEQUENCE IF EXISTS stall.calls`,
		`CREATE SEQUENCE stall.calls`,
		fmt.Sprintf(`CREATE OR REPLACE FUNCTION stall.pg_try_advisory_xact_lock(key BIGINT) RETURNS BOOLEAN
			LANGUAGE plpgsql AS $$
			BEGIN
				IF pg_catalog.nextval('stall.calls') <= %d THEN
					PERFORM pg_catalog.pg_sleep(0.3);
				END IF;
				RETURN pg_catalog.pg_try_advisory_xact_lock(key);
			END $$`, calls),
		`ALTER FUNCTION platform.retire_expired_demo_schools(text[]) SET search_path = stall, pg_catalog`,
		`ALTER FUNCTION platform.erase_closed_demo_visitors() SET search_path = stall, pg_catalog`,
	} {
		_, err := db.NewRaw(statement).Exec(t.Context())
		require.NoError(t, err, statement)
	}
}

// A run that stalls before its first order hides and erases nothing, and
// the helpers repeat it instead of failing the test (#6698).
func TestDemoRunHelpersRepeatARunThatStalledBeforeItsFirstOrder(t *testing.T) {
	t.Parallel()
	order := newDemoOrder(t)
	stallRuns(t, order.db, 2)
	hidden, erased, took := runDemo(t, order.queue, []string{order.slug})
	assert.Zero(t, hidden, "the retirement starts no order after its budget (took %s)", took)
	assert.Zero(t, erased, "the erasure starts no order after its budget (took %s)", took)

	stallRuns(t, order.db, 2)
	hidden, erased = retireAtLeast(t, order.db, order.queue, []string{order.slug}, 1, 1)
	assert.Equal(t, 1, hidden)
	assert.Equal(t, 1, erased)
	order.requireFullyErased(t)
}

// waitingRun starts a run again that stalled before it could wait for the
// held lock, and returns the one that waits (#6698).
func TestDemoRunHelpersWaitForARunThatStalledBeforeItsFirstOrder(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	ctx := t.Context()
	holder, err := order.db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	require.NoError(t, keyShare(order.accounts.caregiver)(ctx, holder))
	order.waitForLocks(t)
	stallRuns(t, order.db, 2)

	done := order.waitingRun(t, "the erasure waits for the held account", func() bool {
		return lockWaits(t, order.db, "retire_expired_demo_schools")
	})
	require.NoError(t, holder.Commit())
	select {
	case result := <-done:
		require.NoError(t, result.err)
		assert.Equal(t, 1, result.erased, "the run that waited erases the order once the lock is free")
	case <-time.After(5 * time.Second):
		t.Fatal("the run did not finish after the holder committed")
	}
	var calls int64
	require.NoError(t, order.db.NewRaw(`SELECT last_value FROM stall.calls`).Scan(ctx, &calls))
	assert.GreaterOrEqual(t, calls, int64(4), "the first run stalled in both budgets and a second run started")
	order.requireFullyErased(t)
}
