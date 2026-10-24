package test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/internal/testdb"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// DemoRoleDatabase logs in as the restricted role on this test's existing
// database. SET ROLE on a superuser session would not test denied escalation.
func DemoRoleDatabase(t *testing.T, db *bun.DB) *bun.DB {
	t.Helper()
	var name string
	require.NoError(t, db.NewRaw("SELECT current_database()").Scan(t.Context(), &name))
	parsed, err := url.Parse(os.Getenv("TEST_DB_DSN"))
	require.NoError(t, err)
	parsed.Path = "/" + name
	parsed.User = url.UserPassword("phoenix_demo", testdb.DemoRolePassword)
	pool := bun.NewDB(sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(parsed.String()))), pgdialect.New())
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, pool.PingContext(t.Context()))
	return pool
}

// NewDemoDatabase binds the restricted role's native transaction runtime.
func NewDemoDatabase(t *testing.T, db *bun.DB) Database {
	t.Helper()
	value, err := transactionruntime.NewDemoDatabase(db)
	require.NoError(t, err)
	return value
}

// AssertRestrictedDemoUnit proves global expiry keeps the demo login and that
// the normal administrative runtime still cannot escalate that login.
func AssertRestrictedDemoUnit(t *testing.T, db *bun.DB) {
	t.Helper()
	demo := NewDemoDatabase(t, db)
	ctx := tenant.WithUnitOfWork(t.Context(), demo.UnitOfWork())
	require.NoError(t, tenant.WithinAdmin(ctx, func(txCtx context.Context) error {
		handle, err := tenant.CurrentTransaction[bun.IDB](txCtx)
		if err != nil {
			return err
		}
		var role string
		if err := handle.NewRaw("SELECT current_user").Scan(txCtx, &role); err != nil {
			return err
		}
		require.Equal(t, "phoenix_demo", role)
		return nil
	}))
	regular := NewDatabase(t, db)
	ctx = tenant.WithUnitOfWork(t.Context(), regular.UnitOfWork())
	require.ErrorContains(t, tenant.WithinAdmin(ctx, func(context.Context) error { return nil }), "permission denied to set role")
}
