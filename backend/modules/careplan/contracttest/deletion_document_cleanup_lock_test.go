package contracttest_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestDeletedStudentDocumentCleanupPreservesPerStatementLockWaits(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Cleanup", "Locks", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("cleanup-locks-%d@example.test", student.ID))
	records := careDocumentRecords(t, db)
	ctx := testpkg.Ctx(t)
	for i := range 2 {
		_, err := records.CreateCareDocument(ctx, newTestCareDocument(student.ID, account.ID,
			careplan.StudentDocumentCategorySonstiges, "document.pdf", fmt.Sprintf("cleanup-locks-%d-%d.pdf", student.ID, i)))
		require.NoError(t, err)
	}
	documents, err := records.ListPendingCareDocumentCleanup(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, documents, 2)
	locks := make([]bun.Tx, 0, 2)
	for _, document := range documents {
		job, err := records.QueueCareDocumentCleanup(ctx, careplan.CareDocumentCleanup{
			OwnerID: student.ID, FilenameStored: document.FilenameStored, RetryAfter: time.Now(),
		})
		require.NoError(t, err)
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = tx.Rollback() })
		_, err = tx.ExecContext(ctx, `SELECT id FROM users.student_document_file_cleanup WHERE tenant_id = ? AND id = ? FOR UPDATE`, student.TenantID, job.ID)
		require.NoError(t, err)
		locks = append(locks, tx)
	}
	ready, done := make(chan struct{}), make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, ctx, db, student.TenantID, func(txCtx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(txCtx, `SET LOCAL statement_timeout = '1s'`); err != nil {
				return err
			}
			close(ready)
			_, err := records.QueueCareDocumentCleanupForDeletedStudent(txCtx, student.ID, time.Now())
			return err
		})
	}()
	<-ready
	// Each original upsert may wait 650ms. Their combined wait exceeds one
	// statement timeout but not the two independent statement deadlines.
	for _, tx := range locks {
		timer := time.NewTimer(650 * time.Millisecond)
		<-timer.C
		require.NoError(t, tx.Commit())
	}
	select {
	case err := <-done:
		require.NoError(t, err, "separate legal row-lock waits must still succeed")
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup command did not finish after releasing its rows")
	}
}
