package test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/testdb"
	"github.com/stretchr/testify/require"
)

func TestIsolatedDatabaseSlotsBoundWaitAndRelease(t *testing.T) {
	t.Parallel()
	require.Equal(t, 2, maxConcurrentIsolatedTestDatabases, "the production clone cap stays two (#6784)")
	t.Run("free_slot_ignores_expired_context", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
		defer cancel()
		require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded, "the wait budget is already exhausted")
		limiter := newIsolatedTestDatabaseLimiter(1)
		release, err := limiter.acquire(ctx, "free holder")
		require.NoError(t, err, "an immediately available slot needs no wait budget")
		defer release()
		require.Equal(t, 1, limiter.owners["free holder"])
		_, err = limiter.acquire(ctx, "blocked")
		require.ErrorIs(t, err, context.DeadlineExceeded, "the same expired context must bound real waiting")
		require.Contains(t, err.Error(), `owner="free holder"`)
		release()
		next, err := limiter.acquire(ctx, "next free holder")
		require.NoError(t, err)
		next()
		require.Empty(t, limiter.slots)
		require.Empty(t, limiter.owners)
		require.Zero(t, limiter.queued)
	})
	synctest.Test(t, func(t *testing.T) {
		limiter := newIsolatedTestDatabaseLimiter(1)
		release, err := limiter.acquire(context.Background(), "holder")
		require.NoError(t, err)
		defer release()
		ctx, cancel := context.WithCancel(context.Background())
		finished := make(chan error, 1)
		go func() { _, err := limiter.acquire(ctx, "cancelled"); finished <- err }()
		synctest.Wait() // the waiter is blocked on the occupied slot, not merely started
		require.Equal(t, 1, limiter.queued)
		cancel()
		err = <-finished
		require.ErrorIs(t, err, context.Canceled)
		for _, diagnostic := range []string{`requester="cancelled"`, `owner="holder"`, "waited=", "queue=1"} {
			require.Contains(t, err.Error(), diagnostic)
		}
		ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err = limiter.acquire(ctx, "deadline")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Contains(t, err.Error(), "waited=20ms", "nonzero wait is independently fixed by the virtual clock")
		require.Contains(t, err.Error(), `owner="holder"`)
		// The holder took the slot before both waits: it held it for both.
		require.Contains(t, err.Error(), "held=20ms", "the message names how long the owner held the slot")
		for _, diagnostic := range []string{"package_elapsed=", "slot_deadline_in=0s"} {
			require.Contains(t, err.Error(), diagnostic)
		}
		release()
		release()
		next, err := limiter.acquire(context.Background(), "next")
		require.NoError(t, err)
		next()
		require.Empty(t, limiter.slots)
		require.Empty(t, limiter.owners)
		require.Zero(t, limiter.queued)
	})
}

func TestIsolatedDatabaseSlotDeadlineLeavesOpenAndDropBudgets(t *testing.T) {
	t.Parallel()
	deadline := time.Now().Add(10 * time.Minute)
	ctx, cancel, err := isolatedTestDatabaseSlotContext(isolatedFixedDeadlineTest{TB: t, deadline: deadline})
	require.NoError(t, err)
	defer cancel()
	actual, ok := ctx.Deadline()
	require.True(t, ok)
	require.Equal(t, deadline.Add(-155*time.Second), actual)
	require.Equal(t, 2*time.Minute, isolatedTestDatabaseOpenTimeout)
	require.Equal(t, 30*time.Second, isolatedTestDatabaseDropTimeout)
	_, _, err = isolatedTestDatabaseSlotContext(isolatedNoDeadlineTest{t})
	require.ErrorContains(t, err, "requires a finite test deadline")
	t.Run("late_package_deadline", func(t *testing.T) {
		// Still inside the package timeout, but its 155-second wait reserve
		// has expired. No sleeping or dependence on the actual package clock.
		tb := isolatedFixedDeadlineTest{TB: t, deadline: time.Now().Add(154 * time.Second)}
		limiter := newIsolatedTestDatabaseLimiter(1)
		release, err := limiter.acquireForTest(tb, "late holder")
		require.NoError(t, err)
		defer release()
		_, err = limiter.acquireForTest(tb, "blocked late caller")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Contains(t, err.Error(), `owner="late holder"`)
		release()
		next, err := limiter.acquireForTest(tb, "next late holder")
		require.NoError(t, err)
		next()
	})
	t.Run("no_package_deadline", func(t *testing.T) {
		tb := isolatedNoDeadlineTest{t}
		limiter := newIsolatedTestDatabaseLimiter(1)
		release, err := limiter.acquireForTest(tb, "debugger holder")
		require.NoError(t, err)
		defer release()
		_, err = limiter.acquireForTest(tb, "blocked debugger caller")
		require.ErrorContains(t, err, "requires a finite test deadline")
		release()
		next, err := limiter.acquireForTest(tb, "next debugger holder")
		require.NoError(t, err)
		next()
	})
}

type isolatedNoDeadlineTest struct{ testing.TB }

func (isolatedNoDeadlineTest) Deadline() (time.Time, bool) { return time.Time{}, false }

type isolatedFixedDeadlineTest struct {
	testing.TB
	deadline time.Time
}

func (t isolatedFixedDeadlineTest) Deadline() (time.Time, bool) { return t.deadline, true }

// Compile the actual dependency-free limiter in a tiny test module. No ORM,
// package fixture or extra clone is needed to reproduce Go's scheduler cycle.
func TestIsolatedDatabaseSlotsBreakParallelSubtestSaturation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source, err := os.ReadFile("database_slots.go")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "database_slots.go"), source, 0o600))
	module := "module isolatedslotfixture\n\ngo " + strings.TrimPrefix(runtime.Version(), "go") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(module), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "saturation_test.go"), []byte(isolatedSlotSaturationFixture), 0o600))
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	output, exitCode, err := testdb.RunGoTest(ctx, dir, 2, 5*time.Second)
	require.NoError(t, err, "%s", output)
	require.Equal(t, 1, exitCode, "%s", output)
	require.NotContains(t, string(output), "panic: test timed out", "%s", output)
	for _, marker := range []string{`owner="TestParent"`, "waited=", "queue=", "context deadline exceeded", "PARENT_CHILD_FINISHED"} {
		require.Contains(t, string(output), marker, "%s", output)
	}
	t.Logf("bounded -parallel 2 scheduler fixture: %d output bytes, zero database clones", len(output))
}

const isolatedSlotSaturationFixture = `package test

import (
	"testing"
	"time"
)

// Scale only the supplied fixture deadline, not the limiter or its reserve.
// The slot expires one second into a five-second Go test subprocess.
type fixtureTest struct{ *testing.T }

func (t fixtureTest) Deadline() (time.Time, bool) {
	deadline, ok := t.T.Deadline()
	return deadline.Add(isolatedTestDatabaseDeadlineReserve - 4*time.Second), ok
}

var fixtureSlots = newIsolatedTestDatabaseLimiter(1)
var waiterStarted = make(chan struct{}, 2)

func claim(t *testing.T) {
	t.Helper()
	release, err := fixtureSlots.acquireForTest(fixtureTest{t}, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
}
func TestParent(t *testing.T) {
	claim(t) // Reserve the parent clone before the parallel barrier.
	t.Parallel()
	t.Run("parallel child", func(t *testing.T) {
		t.Parallel()
		<-waiterStarted
		<-waiterStarted
		t.Log("PARENT_CHILD_FINISHED")
	})
}
func TestWaiterA(t *testing.T) {
	t.Parallel()
	waiterStarted <- struct{}{}
	claim(t)
}
func TestWaiterB(t *testing.T) {
	t.Parallel()
	waiterStarted <- struct{}{}
	claim(t)
}
`
