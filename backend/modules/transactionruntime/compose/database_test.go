package compose_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A root holds one Database value and hands its parts to the owner
// compositions. Both bindings run tenant transactions on the pool they wrap.
func TestDatabaseHandsItsPoolAndUnitOfWorkToTheRoots(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)

	for name, build := range map[string]func(*bun.DB) (compose.Database, error){
		"tenant": compose.NewDatabase,
		"demo":   compose.NewDemoDatabase,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			database, err := build(db)
			require.NoError(t, err)
			assert.Same(t, db, database.Pool())

			ctx := tenant.WithTenantID(tenant.WithUnitOfWork(context.Background(), database.UnitOfWork()), tenantID)
			require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				transaction, ok := tenant.TransactionFromContext(txCtx)
				require.True(t, ok, "the unit of work opens its tenant transaction")
				assert.IsType(t, bun.Tx{}, transaction)
				return nil
			}))
		})
	}
}

func TestDatabaseRegistersQueryHooksAndClosesItsPool(t *testing.T) {
	t.Parallel()
	database, err := compose.NewDatabase(testpkg.SetupClosableTestDB(t))
	require.NoError(t, err)

	counter := testpkg.NewQueryCounter()
	database.AddQueryHook(counter)
	var one int
	require.NoError(t, database.Pool().NewSelect().ColumnExpr("1").Scan(context.Background(), &one))
	assert.Len(t, counter.Queries(), 1, "the hook sees the statements of the pool")

	require.NoError(t, database.Close())
	require.Error(t, database.Pool().PingContext(context.Background()), "closing the value closes its pool")
}

// The server root logs the pool's statements under db_debug and reads its
// connection statistics for the metrics endpoint, through the value alone.
func TestDatabaseLogsQueriesAndReportsCapacity(t *testing.T) {
	t.Parallel()
	database, err := compose.NewDatabase(testpkg.SetupClosableTestDB(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })

	var logged bytes.Buffer
	database.LogQueries(slog.New(slog.NewJSONHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	var one int
	require.NoError(t, database.Pool().NewSelect().ColumnExpr("1").Scan(context.Background(), &one))
	assert.Contains(t, logged.String(), `"operation":"SELECT"`, "the logger sees the statements of the pool")

	assert.GreaterOrEqual(t, database.Capacity().OpenConnections, 1, "the snapshot counts the connection the query opened")
}

func TestDatabaseRequiresAPool(t *testing.T) {
	t.Parallel()
	_, err := compose.NewDatabase(nil)
	require.Error(t, err)
	_, err = compose.NewDemoDatabase(nil)
	require.Error(t, err)
	assert.NoError(t, compose.Database{}.Close(), "a value without a pool has nothing to close")
}
