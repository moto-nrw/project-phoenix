package users

import (
	"context"
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturingStudentAuditRepo struct {
	edits []*auditModels.StudentFieldEdit
}

func (r *capturingStudentAuditRepo) CreateBatch(_ context.Context, edits []*auditModels.StudentFieldEdit) error {
	r.edits = edits
	return nil
}

func (*capturingStudentAuditRepo) GetByStudentID(context.Context, int64) ([]*auditModels.StudentFieldEdit, error) {
	return nil, nil
}

func (*capturingStudentAuditRepo) CountOlderThanByStudent(context.Context, time.Time) (map[int64]int, error) {
	return nil, nil
}

func (*capturingStudentAuditRepo) DeleteOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func TestStudentAuditService_RecordSystemStatusChange(t *testing.T) {
	repo := &capturingStudentAuditRepo{}
	service := NewStudentAuditService(repo, nil)

	err := service.RecordSystemStatusChange(
		context.Background(),
		42,
		userModels.StudentStatusPending,
		userModels.StudentStatusActive,
	)

	require.NoError(t, err)
	require.Len(t, repo.edits, 1)
	edit := repo.edits[0]
	assert.Equal(t, int64(42), edit.StudentID)
	assert.Equal(t, auditModels.StudentFieldEditSystemActorID, edit.EditedBy)
	assert.Equal(t, auditModels.StudentFieldEditSystemActorName, edit.EditedByName)
	assert.Equal(t, auditModels.StudentFieldStatus, edit.FieldName)
	assert.Equal(t, "Ausstehend", *edit.OldValue)
	assert.Equal(t, "Aktiv", *edit.NewValue)
	require.NoError(t, edit.Validate())
}
