package test

// The unit tests for DSN parsing, identifier quoting, and clone naming moved
// with their implementation to internal/testdb (testdb_test.go). What stays
// here is the behavioral contract of SetupTestDB against the lifecycle.

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/testdb"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupTestDBUsesPackageClone(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)

	var currentDB string
	err := db.NewRaw(`SELECT current_database()`).Scan(context.Background(), &currentDB)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(currentDB, testdb.ClonePrefix), "current database = %s", currentDB)
	assert.Contains(t, currentDB, testdb.RunID(), "clone must be stamped with this run's ID")
	require.NotNil(t, packageClone, "package clone handle must be pinned for the process lifetime")
	assert.Equal(t, packageClone.Name, currentDB, "tests must run against the pinned package clone")

	var tenantExists bool
	err = db.NewRaw(`SELECT EXISTS (SELECT 1 FROM platform.schools WHERE id = 1)`).Scan(context.Background(), &tenantExists)
	require.NoError(t, err)
	assert.True(t, tenantExists, "default tenant fixture should exist in the package clone")
}

func TestSetupTestDBAllowsParallelTests(t *testing.T) {
	t.Parallel()

	// The per-test path must be free of t.Setenv (which panics under
	// t.Parallel) — this test IS the regression guard: it runs two parallel
	// subtests through the full SetupTestDB path.
	for _, name := range []string{"a", "b"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			db := SetupTestDB(t)

			var one int
			require.NoError(t, db.NewRaw(`SELECT 1`).Scan(context.Background(), &one))
			assert.Equal(t, 1, one)
		})
	}
}

func TestNewTenantScopeCreatesTenantAndContext(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)

	scope := NewTenantScope(t, db)
	require.NotZero(t, scope.TenantID)
	assert.Equal(t, scope.TenantID, tenantIDFromContextForTest(t, scope.Context()))
	assert.Equal(t, scope.TenantID, audit.TenantIDFromContext(scope.Context()))

	var exists bool
	err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM platform.schools WHERE id = ?)`, scope.TenantID).
		Scan(context.Background(), &exists)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestTenantContextsIncludeAuditTenant(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Tenant(t), audit.TenantIDFromContext(Ctx(t)))
	assert.Equal(t, OwnTenant(t), audit.TenantIDFromContext(OwnCtx(t)))
}

func tenantIDFromContextForTest(t *testing.T, ctx context.Context) int64 {
	t.Helper()

	tenantID := tenant.FromContext(ctx)
	require.NotZero(t, tenantID)
	return tenantID
}
