package users_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestStudentDataChangeRequest_AccessorsAndTableHooks(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	row := &usersModels.StudentDataChangeRequest{}
	row.ID = 42
	row.CreatedAt = now
	row.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(42), row.GetID())
	assert.Equal(t, now, row.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), row.GetUpdatedAt())

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
		{status: usersModels.DataChangeStatusAutoApplied, want: true},
		{status: usersModels.DataChangeStatusApproved, want: true},
		{status: usersModels.DataChangeStatusRejected, want: true},
		{status: usersModels.DataChangeStatusPending, want: false},
		{status: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			row := &usersModels.StudentDataChangeRequest{Status: tt.status}
			assert.Equal(t, tt.want, row.IsTerminal())
		})
	}
}
