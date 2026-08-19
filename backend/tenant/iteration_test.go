package tenant_test

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLister is a test double for tenant.ActiveSchoolLister. It returns the
// pre-seeded schools (or error) without touching the DB.
type fakeLister struct {
	schools []platform.School
	err     error
	calls   int32
}

func (f *fakeLister) ListActive(_ context.Context) ([]platform.School, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.err != nil {
		return nil, f.err
	}
	return f.schools, nil
}

// --- Guard paths (unit, no DB) ---

func TestForEachActive_NilDB_ReturnsError(t *testing.T) {
	t.Parallel()
	err := tenant.ForEachActive(context.Background(), nil, &fakeLister{}, slog.Default(), "op",
		func(_ context.Context, _ int64) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db is required")
}

func TestForEachActive_NilLister_ReturnsError(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	err := tenant.ForEachActive(context.Background(), db, nil, slog.Default(), "op",
		func(_ context.Context, _ int64) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lister is required")
}

// --- Integration paths (real DB + fake lister) ---

func TestForEachActive_HappyPath_InvokesFnPerTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	// testpkg.SetupTestDB guarantees tenant_id=1 exists via EnsureTestTenant.
	lister := &fakeLister{
		schools: []platform.School{{}, {}},
	}
	// Use the real tenant ID so WithTenantTx can SET LOCAL ROLE against an
	// existing FK target.
	lister.schools[0].ID = 1
	lister.schools[1].ID = 1

	var observed []int64
	err := tenant.ForEachActive(context.Background(), db, lister, slog.Default(), "happy-path",
		func(_ context.Context, tenantID int64) error {
			observed = append(observed, tenantID)
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 1}, observed, "fn must run once per listed tenant")
	assert.Equal(t, int32(1), atomic.LoadInt32(&lister.calls),
		"ListActive must be called exactly once")
}

func TestForEachActive_ListerError_BubblesError(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	listErr := errors.New("admin list blew up")
	lister := &fakeLister{err: listErr}

	err := tenant.ForEachActive(context.Background(), db, lister, slog.Default(), "list-fails",
		func(_ context.Context, _ int64) error {
			t.Fatal("fn must not be invoked when listing fails")
			return nil
		})
	require.Error(t, err)
	assert.ErrorIs(t, err, listErr, "original error must be wrapped, not swallowed")
	assert.Contains(t, err.Error(), "list-fails", "wrap must include the operation name")
}

func TestForEachActive_PerTenantError_Swallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	lister := &fakeLister{
		schools: []platform.School{{}, {}},
	}
	lister.schools[0].ID = 1
	lister.schools[1].ID = 1

	fnErr := errors.New("one tenant fails")
	var invocations int
	err := tenant.ForEachActive(context.Background(), db, lister, slog.Default(), "per-tenant-err",
		func(_ context.Context, _ int64) error {
			invocations++
			// Fail on the first tenant; second tenant must still be invoked.
			if invocations == 1 {
				return fnErr
			}
			return nil
		})
	// Per-tenant errors are logged + swallowed; outer ForEachActive returns nil.
	require.NoError(t, err, "per-tenant errors must be swallowed, not surfaced")
	assert.Equal(t, 2, invocations, "iteration must continue past per-tenant failures")
}

func TestForEachActive_NilLogger_FallsBackToDefault(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	lister := &fakeLister{
		schools: []platform.School{{}},
	}
	lister.schools[0].ID = 1

	// Pass nil logger + a failing fn to exercise the logger nil-fallback path
	// (the error branch that writes to logger).
	err := tenant.ForEachActive(context.Background(), db, lister, nil, "nil-logger",
		func(_ context.Context, _ int64) error {
			return errors.New("boom")
		})
	require.NoError(t, err, "nil logger must not panic — ForEachActive substitutes slog.Default()")
}

func TestForEachActive_EmptyTenantList_Succeeds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	lister := &fakeLister{schools: nil}
	var called bool

	err := tenant.ForEachActive(context.Background(), db, lister, slog.Default(), "empty-list",
		func(_ context.Context, _ int64) error {
			called = true
			return nil
		})
	require.NoError(t, err)
	assert.False(t, called, "fn must not be invoked when there are no active tenants")
}
