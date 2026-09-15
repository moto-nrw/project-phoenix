package active_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/require"
)

type auditFeedRecords struct {
	read func(context.Context, audit.TimeTrackingAuditLogFilter) ([]*audit.TimeTrackingAuditLogEntry, error)
}

func (r auditFeedRecords) ListEntries(ctx context.Context, filter audit.TimeTrackingAuditLogFilter) ([]*audit.TimeTrackingAuditLogEntry, error) {
	return r.read(ctx, filter)
}

func TestTimeTrackingAuditReaderPreservesQueryAndPartialRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	date := timezone.NewDate(2026, 3, 29)
	zero := timezone.Date("")
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	readErr := errors.New("partial query")
	row := &audit.TimeTrackingAuditLogEntry{OccurredAt: now, Source: "session_edit", EntryID: 7, ActorIsSystem: true, Reason: "reason", Detail: []byte(`{"fields":[]}`)}
	row.StaffID, row.ActorStaffID = &row.EntryID, &row.EntryID
	for _, day := range []*timezone.Date{nil, &zero, &date} {
		reader := services.NewTimeTrackingAuditReader(auditFeedRecords{read: func(gotCtx context.Context, filter audit.TimeTrackingAuditLogFilter) ([]*audit.TimeTrackingAuditLogEntry, error) {
			require.Equal(t, ctx, gotCtx)
			if day == nil {
				require.Nil(t, filter.From)
				require.Nil(t, filter.To)
			} else {
				require.NotNil(t, filter.From)
				require.Equal(t, string(*day), string(*filter.From))
				require.Equal(t, filter.From, filter.To)
			}
			require.EqualValues(t, 7, filter.StaffID)
			require.EqualValues(t, 8, filter.ActorStaffID)
			require.Equal(t, []string{"session_edit"}, filter.Sources)
			require.Equal(t, 51, filter.Limit)
			require.Equal(t, &audit.TimeTrackingAuditLogCursor{OccurredAt: now, Source: "session_edit", EntryID: 9}, filter.Cursor)
			return []*audit.TimeTrackingAuditLogEntry{nil, row}, readErr
		}})
		got, err := reader.ListEntries(ctx, active.TimeTrackingAuditFilter{From: day, To: day, StaffID: 7, ActorStaffID: 8, Sources: []string{"session_edit"}, Limit: 51, Cursor: &active.TimeTrackingAuditCursor{OccurredAt: now, Source: "session_edit", EntryID: 9}})
		require.ErrorIs(t, err, readErr)
		require.Len(t, got, 2)
		require.Nil(t, got[0])
		require.Equal(t, &active.TimeTrackingAuditEntry{OccurredAt: row.OccurredAt, Source: row.Source, EntryID: row.EntryID, StaffID: row.StaffID, ActorStaffID: row.ActorStaffID, ActorIsSystem: row.ActorIsSystem, Reason: row.Reason, Detail: row.Detail}, got[1])
		sources := reader.ValidSources()
		require.Equal(t, audit.ValidAuditLogSources, sources)
		sources[0] = "changed"
		require.Equal(t, audit.ValidAuditLogSources, reader.ValidSources())
	}
}
