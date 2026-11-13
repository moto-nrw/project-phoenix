package test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The row lock guard on deliberately inserted bypasses (#6702): every lock
// site is found, and only a checked tenant.OrderLock call before it admits
// it. Text presence, a discarded refusal or an admission after the wait do
// not count.
func TestRowLockScannerRefusesBypasses(t *testing.T) {
	t.Parallel()
	const header = `package fixture

import (
	"context"

	"github.com/moto-ogs/project-phoenix/tenant"
)
`
	for _, tc := range []struct {
		name     string
		body     string
		admitted bool
		what     string
	}{
		{"bun lock without admission", `func (s *Store) F(ctx context.Context) error {
	return s.q.For("UPDATE").Scan(ctx)
}`, false, `For("UPDATE")`},
		{"admitted bun lock", `func (s *Store) F(ctx context.Context) error {
	if err := tenant.OrderLock(ctx, tenant.LockLeaf, "t"); err != nil {
		return err
	}
	return s.q.For("UPDATE").Scan(ctx)
}`, true, `For("UPDATE")`},
		{"admission after the lock", `func (s *Store) F(ctx context.Context) error {
	err := s.q.For("SHARE").Scan(ctx)
	if err := tenant.OrderLock(ctx, tenant.LockLeaf, "t"); err != nil {
		return err
	}
	return err
}`, false, `For("SHARE")`},
		{"discarded refusal", `func (s *Store) F(ctx context.Context) error {
	_ = tenant.OrderLock(ctx, tenant.LockLeaf, "t")
	return s.q.For("UPDATE").Scan(ctx)
}`, false, `For("UPDATE")`},
		{"bare admission statement", `func (s *Store) F(ctx context.Context) error {
	tenant.OrderLock(ctx, tenant.LockLeaf, "t")
	return s.q.For("UPDATE").Scan(ctx)
}`, false, `For("UPDATE")`},
		{"admission named only in a comment", `func (s *Store) F(ctx context.Context) error {
	// tenant.OrderLock(ctx, tenant.LockLeaf, "t") admits this lock.
	return s.q.For("UPDATE").Scan(ctx)
}`, false, `For("UPDATE")`},
		{"raw SQL row lock", "func (s *Store) F(ctx context.Context) error {\n\treturn s.db.NewRaw(`SELECT id FROM x WHERE id = ? FOR NO KEY UPDATE SKIP LOCKED`, 1).Scan(ctx)\n}", false, "SQL FOR NO KEY UPDATE"},
		{"lower-case SQL row lock", "func (s *Store) F(ctx context.Context) error {\n\treturn s.db.NewRaw(`select id from x for key share`).Scan(ctx)\n}", false, "SQL for key share"},
		{"table lock", `func (s *Store) F(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "LOCK TABLE x IN SHARE ROW EXCLUSIVE MODE")
	return err
}`, false, "SQL LOCK TABLE"},
		{"advisory lock", `func (s *Store) F(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "SELECT pg_advisory_xact_lock_shared(1, 2)")
	return err
}`, false, "SQL pg_advisory_xact_lock"},
		{"runtime advisory lock", `func F(ctx context.Context) error {
	return tenant.AcquireLock(ctx, "k", false)
}`, false, "tenant.AcquireLock"},
		{"lock in a package constant", "const lockSQL = `SELECT 1 FOR UPDATE`\n\nfunc (s *Store) F(ctx context.Context) error {\n\treturn s.db.NewRaw(lockSQL).Scan(ctx)\n}", false, "SQL constant lockSQL"},
		{"same-file admitter", `func admit(ctx context.Context) error {
	return tenant.OrderLock(ctx, tenant.LockLeaf, "t")
}

func (s *Store) F(ctx context.Context) error {
	if err := admit(ctx); err != nil {
		return err
	}
	return s.q.For("UPDATE").Scan(ctx)
}`, true, `For("UPDATE")`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sites, err := scanRowLockSites("fixture.go", []byte(header+"\n"+tc.body+"\n"))
			require.NoError(t, err)
			var found []rowLockSite
			for _, site := range sites {
				if strings.Contains(site.what, tc.what) {
					found = append(found, site)
				}
			}
			require.Len(t, found, 1, "the lock site %q is found: %v", tc.what, sites)
			assert.Equal(t, tc.admitted, found[0].admitted)
		})
	}
}

// Error texts that mention a lock are no lock site.
func TestRowLockScannerIgnoresProse(t *testing.T) {
	t.Parallel()
	sites, err := scanRowLockSites("fixture.go", []byte(`package fixture

import "errors"

var errFind = errors.New("find person for update")
`))
	require.NoError(t, err)
	assert.Empty(t, sites)
}

// An admitter of a persistence package that may not import the tenant
// runtime admits only before its last call of the lock statement.
func TestRowLockAdmitterMustAdmitBeforeItsCall(t *testing.T) {
	t.Parallel()
	const source = `package fixture

import (
	"context"

	"github.com/moto-ogs/project-phoenix/tenant"
)

func (a store) Before(ctx context.Context) error {
	if err := tenant.OrderLock(ctx, tenant.LockLeaf, "t"); err != nil {
		return err
	}
	return a.inner.Lock(ctx)
}

func (a store) After(ctx context.Context) error {
	if err := a.inner.Lock(ctx); err != nil {
		return err
	}
	return tenant.OrderLock(ctx, tenant.LockLeaf, "t")
}

func (a store) Without(ctx context.Context) error {
	return a.inner.Lock(ctx)
}
`
	for function, want := range map[string]bool{"Before": true, "After": false, "Without": false, "Missing": false} {
		ok, err := admitsBeforeCall("fixture.go", []byte(source), function, "Lock")
		require.NoError(t, err)
		assert.Equal(t, want, ok, function)
	}
}
