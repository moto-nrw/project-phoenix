package audit

import (
	"context"
	"testing"
	"time"

	records "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestAuditTenantReadsRejectMissingScopeAndEnforceRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	missing := Runtime(func(context.Context) (bun.IDB, int64) { return db, 0 })
	now := time.Now()
	checks := []func() error{
		func() error {
			_, err := NewDeviationEventRepository(missing).ListByRange(context.Background(), records.Date("2030-01-01"), records.Date("2030-01-02"), nil, nil)
			return err
		},
		func() error {
			_, err := NewFileEventRepository(missing).ListRecent(context.Background(), 10)
			return err
		},
		func() error {
			_, err := NewDataDeletionRepository(missing).ListRecentRetentionSummaries(context.Background(), now.Add(-time.Hour), 10)
			return err
		},
		func() error {
			_, err := NewAttendanceCorrectionRepository(missing).ListByInstanceAndStudent(context.Background(), 1, 1)
			return err
		},
		func() error {
			_, err := NewAttendanceCorrectionRepository(missing).CountByInstanceAndStudents(context.Background(), 1, []int64{1})
			return err
		},
		func() error {
			_, err := NewEnrollmentOfferingAdjustmentRepository(missing).ListDirectForTenant(context.Background(), records.DirectAdjustmentFilter{Limit: 10})
			return err
		},
	}
	for _, check := range checks {
		require.ErrorContains(t, check(), "tenant is required")
	}
	ids := []int64{testpkg.Tenant(t), testpkg.UniqueTestTenantID(t)}
	testpkg.EnsureTestTenant(t, db, ids[1])
	base := testpkg.WithTenantRuntime(t, context.Background(), db)
	for _, id := range ids {
		require.NoError(t, testpkg.WithinTenantTransaction(base, db, id, func(ctx context.Context, tx bun.Tx) error {
			runtime := Runtime(func(context.Context) (bun.IDB, int64) { return tx, id })
			folderID := id
			event := &records.FileEvent{FolderID: &folderID, Action: records.FileEventFolderCreated, ActorName: "Audit scope", Detail: "Tenant isolation"}
			event.TenantID = id
			return NewFileEventRepository(runtime).Create(ctx, event)
		}))
	}
	for _, id := range ids {
		require.NoError(t, testpkg.WithinTenantTransaction(base, db, id, func(ctx context.Context, tx bun.Tx) error {
			runtime := Runtime(func(context.Context) (bun.IDB, int64) { return tx, id })
			rows, err := NewFileEventRepository(runtime).ListRecent(ctx, 10)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			require.Equal(t, id, rows[0].TenantID)
			var count int
			require.NoError(t, tx.NewRaw(`SELECT count(*) FROM audit.file_event_ledger WHERE tenant_id IN (?, ?)`, ids[0], ids[1]).Scan(ctx, &count))
			require.Equal(t, 1, count, "RLS isolates rows even without the owner predicate")
			return nil
		}))
	}
}
