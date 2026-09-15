package active_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/require"
)

type deletionEventWriter struct {
	write func(context.Context, *audit.DataDeletion) error
}

func (w deletionEventWriter) Create(ctx context.Context, entry *audit.DataDeletion) error {
	return w.write(ctx, entry)
}

func TestDeletionAuditPreservesSubjectEvidenceAndFailure(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, staff := range []bool{false, true} {
		entry := &active.DeletionEvent{
			TenantID: 7, RecordsDeleted: 3, DeletionType: "visit_retention",
			DeletionReason: "retention", DeletedBy: "system",
			DeletedAt: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC),
			Metadata:  map[string]interface{}{"retention_days": 30},
		}
		if staff {
			entry.StaffID = &entry.TenantID
			entry.DeletionType = "time_tracking_retention"
		} else {
			entry.StudentID = &entry.TenantID
		}
		writeErr := errors.New("audit unavailable")
		calls := 0
		writer := services.NewDeletionAudit(deletionEventWriter{write: func(gotCtx context.Context, got *audit.DataDeletion) error {
			calls++
			require.Equal(t, ctx, gotCtx)
			require.Equal(t, entry.TenantID, got.TenantID)
			require.Equal(t, entry.StudentID, got.StudentID)
			require.Equal(t, entry.StaffID, got.StaffID)
			require.Equal(t, entry.DeletionType, got.DeletionType)
			require.Equal(t, entry.RecordsDeleted, got.RecordsDeleted)
			require.Equal(t, entry.DeletionReason, got.DeletionReason)
			require.Equal(t, entry.DeletedBy, got.DeletedBy)
			require.Equal(t, entry.DeletedAt, got.DeletedAt)
			require.Equal(t, entry.Metadata, got.Metadata)
			return writeErr
		}})
		require.ErrorIs(t, writer.Create(ctx, entry), writeErr)
		require.Equal(t, 1, calls)
	}
	require.Nil(t, services.NewDeletionAudit(nil))
}
