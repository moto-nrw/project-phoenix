package active_test

import (
	"context"
	"errors"
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/stretchr/testify/require"
)

type accessEventWriter struct {
	write func(context.Context, *auditModels.DataAccessLog) error
}

func (w accessEventWriter) Create(ctx context.Context, entry *auditModels.DataAccessLog) error {
	return w.write(ctx, entry)
}

func TestDataAccessAuditPreservesEvidenceAndFailure(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	entry := &active.DataAccessEvent{
		ActorAccountID: 7, ActorRole: "staff", ResourceType: "attendance_history",
		RangeStart: now.Add(-time.Hour), RangeEnd: now, AccessedAt: now,
		Metadata: map[string]interface{}{"format": "csv", "row_count": 3},
	}
	entry.StudentID = &entry.ActorAccountID
	writeErr := errors.New("audit unavailable")
	calls := 0
	writer := services.NewDataAccessAudit(accessEventWriter{write: func(gotCtx context.Context, got *auditModels.DataAccessLog) error {
		calls++
		require.Equal(t, ctx, gotCtx)
		require.Equal(t, entry.ActorAccountID, got.ActorAccountID)
		require.Equal(t, entry.ActorRole, got.ActorRole)
		require.Equal(t, entry.ResourceType, got.ResourceType)
		require.Equal(t, entry.StudentID, got.StudentID)
		require.Equal(t, entry.RangeStart, got.RangeStart)
		require.Equal(t, entry.RangeEnd, got.RangeEnd)
		require.Equal(t, entry.AccessedAt, got.AccessedAt)
		require.Equal(t, entry.Metadata, got.Metadata)
		return writeErr
	}})
	require.ErrorIs(t, writer.Create(ctx, entry), writeErr)
	require.Equal(t, 1, calls)
	require.Nil(t, services.NewDataAccessAudit(nil))
}
