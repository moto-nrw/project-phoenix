package test

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageTestDBDSNRewrite(t *testing.T) {
	parsed, err := parsePostgresDSN("postgres://user:pass@localhost:5433/phoenix_test?sslmode=disable&application_name=tests")
	require.NoError(t, err)

	assert.Equal(t, "phoenix_test", databaseNameFromURL(parsed))

	rewritten := withDatabaseName(parsed, "phx_test_pkg_abc123").String()
	assert.Equal(t,
		"postgres://user:pass@localhost:5433/phx_test_pkg_abc123?sslmode=disable&application_name=tests",
		rewritten)
}

func TestParsePostgresDSNRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{name: "empty", dsn: ""},
		{name: "wrong scheme", dsn: "mysql://user:pass@localhost/db"},
		{name: "missing database", dsn: "postgres://user:pass@localhost:5433/"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePostgresDSN(tc.dsn)
			assert.Error(t, err)
		})
	}
}

func TestPackageCloneDatabaseNameIsValidPostgresIdentifier(t *testing.T) {
	name, err := packageCloneDatabaseName()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(name, testDBClonePrefix))
	assert.LessOrEqual(t, len(name), 63)
	assert.NotContains(t, name, "-")
	assert.NotContains(t, name, "/")
}

func TestQuoteIdentifierEscapesDoubleQuotes(t *testing.T) {
	assert.Equal(t, `"plain"`, quoteIdentifier("plain"))
	assert.Equal(t, `"has""quote"`, quoteIdentifier(`has"quote`))
}

func TestSetupTestDBUsesPackageClone(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	var currentDB string
	err := db.NewRaw(`SELECT current_database()`).Scan(context.Background(), &currentDB)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(currentDB, testDBClonePrefix), "current database = %s", currentDB)

	var tenantExists bool
	err = db.NewRaw(`SELECT EXISTS (SELECT 1 FROM platform.schools WHERE id = 1)`).Scan(context.Background(), &tenantExists)
	require.NoError(t, err)
	assert.True(t, tenantExists, "default tenant fixture should exist in the package clone")
}

func TestNewTenantScopeCreatesTenantAndContext(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	scope := NewTenantScope(t, db)
	require.NotZero(t, scope.TenantID)
	assert.Equal(t, scope.TenantID, tenantIDFromContextForTest(t, scope.Context()))

	var exists bool
	err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM platform.schools WHERE id = ?)`, scope.TenantID).
		Scan(context.Background(), &exists)
	require.NoError(t, err)
	assert.True(t, exists)
}

func tenantIDFromContextForTest(t *testing.T, ctx context.Context) int64 {
	t.Helper()

	tenantID := tenant.FromContext(ctx)
	require.NotZero(t, tenantID)
	return tenantID
}
