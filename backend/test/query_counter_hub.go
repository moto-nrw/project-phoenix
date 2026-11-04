package test

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/uptrace/bun"
)

// testQueryHooks is the single bun.QueryHook a test pool carries for every
// hook a test attaches later: QueryCounters and lock-wait evidence (#5220).
// bun keeps its hooks in a plain slice that queries read without a lock, so
// AddQueryHook on a pool other goroutines already use is a data race. The
// dispatcher is attached while the pool is still private to its opener;
// afterwards hooks join and leave through its own copy-on-write list, and
// bun's slice is never written again.
type testQueryHooks struct {
	mu    sync.Mutex // serializes writers of hooks
	hooks atomic.Pointer[[]bun.QueryHook]
}

// testQueryHooksSeen carries the hooks BeforeQuery called, so AfterQuery
// reaches exactly those even if the list changed while the statement ran.
type testQueryHooksSeen struct{ dispatcher *testQueryHooks }

var testQueryHookDispatchers sync.Map // *bun.DB -> *testQueryHooks

// attachTestQueryHooks must run before db is handed to any other goroutine.
func attachTestQueryHooks(db *bun.DB) {
	dispatcher := &testQueryHooks{}
	testQueryHookDispatchers.Store(db, dispatcher)
	db.AddQueryHook(dispatcher) //nolint:staticcheck // SA1019: the dispatcher must reach the shared *bun.DB other components will hold; WithQueryHook returns a clone.
}

// detachTestQueryHooks forgets a closed pool.
func detachTestQueryHooks(db *bun.DB) { testQueryHookDispatchers.Delete(db) }

// registerTestQueryHook adds hook to db's dispatcher until tb ends; stop runs
// first at cleanup.
func registerTestQueryHook(tb testing.TB, db *bun.DB, hook bun.QueryHook, stop func()) {
	tb.Helper()
	value, ok := testQueryHookDispatchers.Load(db)
	if !ok {
		tb.Fatalf("test query hook: this *bun.DB was not opened by testpkg (SetupTestDB, SetupIsolatedTestDB, " +
			"SetupClosableTestDB, SetupServeTestDB); count a private pool with db.WithQueryHook(testpkg.NewQueryCounter())")
	}
	dispatcher := value.(*testQueryHooks)
	dispatcher.update(func(hooks []bun.QueryHook) []bun.QueryHook { return append(hooks, hook) })
	tb.Cleanup(func() {
		if stop != nil {
			stop()
		}
		dispatcher.update(func(hooks []bun.QueryHook) []bun.QueryHook {
			return slices.DeleteFunc(hooks, func(other bun.QueryHook) bool { return other == hook })
		})
	})
}

func (d *testQueryHooks) update(change func([]bun.QueryHook) []bun.QueryHook) {
	d.mu.Lock()
	defer d.mu.Unlock()
	next := change(slices.Clone(d.current()))
	d.hooks.Store(&next)
}

// current is the registered hooks; callers must not modify the slice.
func (d *testQueryHooks) current() []bun.QueryHook {
	if p := d.hooks.Load(); p != nil {
		return *p
	}
	return nil
}

func (d *testQueryHooks) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	hooks := d.current()
	if len(hooks) == 0 {
		return ctx
	}
	for _, hook := range hooks {
		ctx = hook.BeforeQuery(ctx, event)
	}
	return context.WithValue(ctx, testQueryHooksSeen{dispatcher: d}, hooks)
}

func (d *testQueryHooks) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	hooks, _ := ctx.Value(testQueryHooksSeen{dispatcher: d}).([]bun.QueryHook)
	for i := len(hooks) - 1; i >= 0; i-- {
		hooks[i].AfterQuery(ctx, event)
	}
}
