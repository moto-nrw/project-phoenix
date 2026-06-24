package users

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func TestStudentDataChangeRequest_AccessorsAndTableHooks(t *testing.T) {
	db := bun.NewDB(nil, pgdialect.New())
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	row := &StudentDataChangeRequest{}
	row.ID = 42
	row.CreatedAt = now
	row.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(42), row.GetID())
	assert.Equal(t, now, row.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), row.GetUpdatedAt())
	assert.Equal(t, tableUsersStudentDataChangeRequests, row.TableName())

	require.NoError(t, row.BeforeAppendModel(db.NewInsert().Model(row)))
	require.NoError(t, row.BeforeAppendModel(db.NewUpdate().Model(row)))
	require.NoError(t, row.BeforeAppendModel(db.NewDelete().Model(row)))
	require.NoError(t, row.BeforeAppendModel("ignored"))
	require.NoError(t, row.BeforeAppendModel(nil))
}

func TestStudentDataChangeRequest_IsTerminal(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: DataChangeStatusAutoApplied, want: true},
		{status: DataChangeStatusApproved, want: true},
		{status: DataChangeStatusRejected, want: true},
		{status: DataChangeStatusPending, want: false},
		{status: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			row := &StudentDataChangeRequest{Status: tt.status}
			assert.Equal(t, tt.want, row.IsTerminal())
		})
	}
}
