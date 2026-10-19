package api

import (
	"context"
	"errors"
	"testing"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	date := calendar.NewDate(2026, 3, 29)
	zero := calendar.Date("")
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	readErr := errors.New("partial query")
	row := &audit.TimeTrackingAuditLogEntry{OccurredAt: now, Source: "session_edit", EntryID: 7, ActorIsSystem: true, Reason: "reason", Detail: []byte(`{"fields":[]}`)}
	row.StaffID, row.ActorStaffID = &row.EntryID, &row.EntryID
	for _, day := range []*calendar.Date{nil, &zero, &date} {
		reader := owners.NewTimeTrackingAuditReader(auditFeedRecords{read: func(gotCtx context.Context, filter audit.TimeTrackingAuditLogFilter) ([]*audit.TimeTrackingAuditLogEntry, error) {
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
		got, err := reader.ListEntries(ctx, workforceCompose.TimeTrackingAuditFilter{From: day, To: day, StaffID: 7, ActorStaffID: 8, Sources: []string{"session_edit"}, Limit: 51, Cursor: &workforceCompose.TimeTrackingAuditCursor{OccurredAt: now, Source: "session_edit", EntryID: 9}})
		require.ErrorIs(t, err, readErr)
		require.Len(t, got, 2)
		require.Nil(t, got[0])
		require.Equal(t, &workforceCompose.TimeTrackingAuditEntry{OccurredAt: row.OccurredAt, Source: row.Source, EntryID: row.EntryID, StaffID: row.StaffID, ActorStaffID: row.ActorStaffID, ActorIsSystem: row.ActorIsSystem, Reason: row.Reason, Detail: row.Detail}, got[1])
		sources := reader.ValidSources()
		require.Equal(t, audit.ValidAuditLogSources, sources)
		sources[0] = "changed"
		require.Equal(t, audit.ValidAuditLogSources, reader.ValidSources())
	}
}

type editAuditRecords struct {
	audit.WorkSessionEditRepository
	write func(context.Context, []*audit.WorkSessionEdit) error
	read  func(context.Context, int64) ([]*audit.WorkSessionEdit, error)
}

func (r editAuditRecords) CreateBatch(ctx context.Context, rows []*audit.WorkSessionEdit) error {
	return r.write(ctx, rows)
}
func (r editAuditRecords) GetBySessionID(ctx context.Context, id int64) ([]*audit.WorkSessionEdit, error) {
	return r.read(ctx, id)
}

func TestWorkSessionAuditPreservesPartialWriteEvidence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	value := "12:00"
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	edit := &workforceCompose.WorkSessionEdit{TenantID: 9, SessionID: 7, StaffID: 8, EditedBy: 0, FieldName: "check_out_time", OldValue: &value, NewValue: &value, Notes: &value, CreatedAt: now}
	writeErr := errors.New("second row failed")
	writer := owners.NewWorkSessionAudit(editAuditRecords{write: func(gotCtx context.Context, rows []*audit.WorkSessionEdit) error {
		require.Equal(t, ctx, gotCtx)
		require.Len(t, rows, 2)
		require.Nil(t, rows[1])
		row := rows[0]
		require.Equal(t, edit.TenantID, row.TenantID)
		require.Equal(t, edit.SessionID, row.SessionID)
		require.Equal(t, edit.StaffID, row.StaffID)
		require.Equal(t, edit.EditedBy, row.EditedBy)
		require.Equal(t, edit.FieldName, row.FieldName)
		require.Equal(t, edit.OldValue, row.OldValue)
		require.Equal(t, edit.NewValue, row.NewValue)
		require.Equal(t, edit.Notes, row.Notes)
		require.Equal(t, edit.CreatedAt, row.CreatedAt)
		require.NoError(t, row.Validate())
		row.ID = 11
		return writeErr
	}})
	require.ErrorIs(t, writer.CreateBatch(ctx, []*workforceCompose.WorkSessionEdit{edit, nil}), writeErr)
	require.EqualValues(t, 11, edit.ID)
}

func TestWorkSessionAuditPreservesReadShapeAndError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	readErr := errors.New("partial read")
	for _, rows := range [][]*audit.WorkSessionEdit{nil, {}, {nil, {ID: 7, SessionID: 8, FieldName: "date"}}} {
		reader := owners.NewWorkSessionAudit(editAuditRecords{read: func(gotCtx context.Context, id int64) ([]*audit.WorkSessionEdit, error) {
			require.Equal(t, ctx, gotCtx)
			require.EqualValues(t, 8, id)
			return rows, readErr
		}})
		got, err := reader.GetBySessionID(ctx, 8)
		require.ErrorIs(t, err, readErr)
		require.Equal(t, rows == nil, got == nil)
		require.Len(t, got, len(rows))
		for i, row := range rows {
			if row == nil {
				require.Nil(t, got[i])
				continue
			}
			require.Equal(t, row.ID, got[i].ID)
			require.Equal(t, row.SessionID, got[i].SessionID)
			require.Equal(t, row.FieldName, got[i].FieldName)
		}
	}
}
