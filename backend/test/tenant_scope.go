package test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// Per-test tenants (#2419).
//
// Every fixture in this package takes a `tb testing.TB`, so a test's tenant
// can be derived from the test itself instead of being hardcoded. A test opts
// in by calling Tenant or Ctx before it creates fixtures; from then on every
// fixture it creates lands in its own tenant, and tenant-wide assertions see
// only its own rows. That is what makes t.Parallel() safe inside a package
// clone: parallel tests no longer mutate one shared tenant.
//
// A package opts in once, from TestMain, via PerTestTenants. Packages that
// have not opted in keep using the bootstrap tenant seeded by db_clone.go, so
// the migration runs package by package instead of all at once. The fallback
// disappears with the last migrated package.

// legacyBootstrapTenantID is the fixed tenant seeded into every package clone
// for tests that have not been migrated to a per-test tenant yet.
const legacyBootstrapTenantID int64 = 1

// tenantEntry lazily creates one tenant per run of a top-level test. The once
// makes a concurrent fixture call block until the tenant row exists rather
// than read a half-initialized ID.
//
// owner is the testing.T of the run that claimed the entry; ownerDone flips
// when that run has finished. go test -count=N reruns a test under the same
// name, and the rerun must not inherit the finished run's tenant: its rows
// are still in the package clone.
type tenantEntry struct {
	once      sync.Once
	id        int64
	mu        sync.Mutex
	owner     testing.TB
	ownerDone atomic.Bool
}

// claim records tb as the entry's owner when tb is the test the entry is
// keyed by, and marks the entry finished once that test is done.
func (e *tenantEntry) claim(tb testing.TB, key string) {
	if tb.Name() != key {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.owner != nil {
		return
	}
	e.owner = tb
	tb.Cleanup(func() { e.ownerDone.Store(true) })
}

// staleFor reports whether the entry belongs to a finished earlier run. The
// finished run's own cleanups still see their tenant.
func (e *tenantEntry) staleFor(tb testing.TB) bool {
	if !e.ownerDone.Load() {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.owner != tb
}

func (e *tenantEntry) ensure(tb testing.TB) int64 {
	e.once.Do(func() {
		e.id = uniqueJWTSafeTenantID()
		EnsureTestTenant(tb, SetupTestDB(tb), e.id)
	})
	return e.id
}

// perTestTenants is set once from TestMain, before any test runs, and only
// read afterwards. A package-level switch rather than a per-test opt-in: it
// cannot be order-dependent, and a fixture created before the test first
// mentions its tenant still lands in the right place.
var perTestTenants bool

// PerTestTenants switches this test binary to per-test tenants: every test
// gets its own tenant and every fixture it creates follows automatically.
// Call it from TestMain, before m.Run.
func PerTestTenants() { perTestTenants = true }

// testTenants maps a top-level test name to the tenant of its current run. Subtests share their
// parent's tenant: a parent commonly creates the fixtures its subtests read.
var testTenants sync.Map // string -> *tenantEntry

// Tenant returns the tenant this test owns, creating it on first call. Call it
// before creating fixtures; every fixture the same test creates afterwards
// belongs to this tenant.
func Tenant(tb testing.TB) int64 {
	tb.Helper()
	key := topLevelTestName(tb)
	for {
		loaded, _ := testTenants.LoadOrStore(key, &tenantEntry{})
		entry := loaded.(*tenantEntry)
		if entry.staleFor(tb) {
			testTenants.CompareAndDelete(key, entry)
			continue
		}
		entry.claim(tb, key)
		return entry.ensure(tb)
	}
}

// Ctx returns a context scoped to this test's own tenant. It replaces
// TenantContext(1) in migrated tests.
func Ctx(tb testing.TB) context.Context {
	tb.Helper()
	tenantID := Tenant(tb)
	return tenant.WithTenantID(testRuntimeContext(tb), tenantID)
}

// fixtureTenantID returns the tenant a fixture created by tb belongs to: the
// test's own tenant once it opted in, otherwise the bootstrap tenant.
func fixtureTenantID(tb testing.TB) int64 {
	if perTestTenants {
		return Tenant(tb)
	}
	if entry, ok := testTenants.Load(topLevelTestName(tb)); ok {
		return entry.(*tenantEntry).ensure(tb)
	}
	return legacyBootstrapTenantID
}

// OwnTenant gives THIS test its own tenant, even when it is a subtest, and
// returns the ID. Everything the same tb creates afterwards lands there, and
// its own subtests share it.
//
// The default — subtests share the parent's tenant — is right whenever the
// parent builds the fixtures its subtests read. It is wrong for the other
// common shape: a table of subtests that each create the same kind of row and
// then assert something tenant-wide about it. Those used to stay apart
// because each subtest deleted its rows again; without the per-row teardowns
// (#2419) the second subtest sees the first one's row and hits a unique
// index, an overlap check, or a count that is one too high. Calling OwnTenant
// first is the fix, not putting the deletes back.
func OwnTenant(tb testing.TB) int64 {
	tb.Helper()
	ownTenantScopes.Store(tb.Name(), struct{}{})
	return Tenant(tb)
}

// OwnCtx is OwnTenant plus the context scoped to it.
func OwnCtx(tb testing.TB) context.Context {
	tb.Helper()
	tenantID := OwnTenant(tb)
	return tenant.WithTenantID(testRuntimeContext(tb), tenantID)
}

// ownTenantScopes holds the full names of the subtests that claimed a tenant
// of their own via OwnTenant.
var ownTenantScopes sync.Map // string -> struct{}

// topLevelTestName resolves a test to the name that owns its tenant: the
// longest prefix of tb.Name() that claimed one via OwnTenant, and otherwise
// the top-level test — so a parent and its subtests share one tenant.
func topLevelTestName(tb testing.TB) string {
	name := tb.Name()
	for {
		if _, ok := ownTenantScopes.Load(name); ok {
			return name
		}
		cut := strings.LastIndex(name, "/")
		if cut < 0 {
			return name
		}
		name = name[:cut]
	}
}

// RebaseTenantID maps the bootstrap tenant onto the tenant this test owns and
// leaves every other tenant ID untouched. It lets helpers that build JWT
// claims (api/testutil) follow a test into its own tenant without threading a
// tenant argument through hundreds of call sites.
func RebaseTenantID(tb testing.TB, tenantID int64) int64 {
	if tenantID == legacyBootstrapTenantID {
		return fixtureTenantID(tb)
	}
	return tenantID
}
