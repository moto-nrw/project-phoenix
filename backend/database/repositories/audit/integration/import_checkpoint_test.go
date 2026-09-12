package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestImportCheckpoints_CommitRollbackUniquenessAndTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, db, "checkpoint@example.test")
	key := strings.Repeat("a", 64)
	store := auditRepo.NewAppender(func(ctx context.Context) (bun.IDB, int64) {
		if raw, ok := auditModels.TransactionFromContext(ctx); ok {
			return raw.(bun.Tx), auditModels.TenantIDFromContext(ctx)
		}
		return db, auditModels.TenantIDFromContext(ctx)
	})
	appendReceipt := func(ctx context.Context, lastRow int) error {
		t.Helper()
		stamp := time.Now()
		record := &auditModels.DataImport{
			EntityType: "student", Filename: "batch.csv", TotalRows: 200,
			ImportedBy: account.ID, StartedAt: stamp, CompletedAt: &stamp,
		}
		require.NoError(t, record.AttachImportCheckpoint(auditModels.ImportCheckpoint{
			ImportKey: key, LastRow: lastRow, TotalRows: 200, Payload: []byte(`{"created":100}`),
		}))
		return store.Append(ctx, record)
	}
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return appendReceipt(ctx, 100)
	}))
	read := func(ctx context.Context) []auditModels.ImportCheckpoint {
		t.Helper()
		receipts, err := store.ListImportCheckpoints(ctx, key)
		require.NoError(t, err)
		return receipts
	}
	require.Len(t, read(testpkg.Ctx(t)), 1)
	stop := errors.New("fail after audit command")
	err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		require.NoError(t, appendReceipt(ctx, 200))
		require.Len(t, read(ctx), 2)
		return stop
	})
	require.ErrorIs(t, err, stop)
	receipts := read(testpkg.Ctx(t))
	require.Len(t, receipts, 1, "the rolled-back batch must not advance the durable cursor")
	assert.Equal(t, 100, receipts[0].LastRow)
	assert.JSONEq(t, `{"created":100}`, string(receipts[0].Payload))
	err = testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return appendReceipt(ctx, 100)
	})
	require.Error(t, err, "the unique index prevents two receipts for the same committed batch")

	other := testpkg.NewTenantScope(t, db)
	assert.Empty(t, read(auditModels.WithTenantID(testpkg.Ctx(t), other.TenantID)), "explicit scope also protects privileged readers")
	require.NoError(t, testpkg.WithTenantTx(t, other.Context(), db, other.TenantID, func(ctx context.Context, _ bun.Tx) error {
		assert.Empty(t, read(ctx), "RLS hides the other school's checkpoint")
		return nil
	}))
	_, err = store.ListImportCheckpoints(context.Background(), key)
	require.Error(t, err, "missing tenant must fail closed")
}
