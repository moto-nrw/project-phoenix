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
	edit := &active.WorkSessionEdit{TenantID: 9, SessionID: 7, StaffID: 8, EditedBy: 0, FieldName: "check_out_time", OldValue: &value, NewValue: &value, Notes: &value, CreatedAt: now}
	writeErr := errors.New("second row failed")
	writer := services.NewWorkSessionAudit(editAuditRecords{write: func(gotCtx context.Context, rows []*audit.WorkSessionEdit) error {
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
	require.ErrorIs(t, writer.CreateBatch(ctx, []*active.WorkSessionEdit{edit, nil}), writeErr)
	require.EqualValues(t, 11, edit.ID)
}

func TestWorkSessionAuditPreservesReadShapeAndError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	readErr := errors.New("partial read")
	for _, rows := range [][]*audit.WorkSessionEdit{nil, {}, {nil, {ID: 7, SessionID: 8, FieldName: "date"}}} {
		reader := services.NewWorkSessionAudit(editAuditRecords{read: func(gotCtx context.Context, id int64) ([]*audit.WorkSessionEdit, error) {
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
