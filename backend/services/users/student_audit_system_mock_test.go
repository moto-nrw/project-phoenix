package users

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
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
	t.Parallel()

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

func TestStudentAuditService_RecordChangesForActor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		claims     jwt.AppClaims
		editedBy   int64
		wantEditor string
	}{
		{
			name:       "full name",
			claims:     jwt.AppClaims{ID: 23, FirstName: "Mara", LastName: "Muster"},
			editedBy:   23,
			wantEditor: "Mara Muster",
		},
		{
			name:       "username fallback",
			claims:     jwt.AppClaims{ID: 23, Username: "mara.muster"},
			editedBy:   23,
			wantEditor: "mara.muster",
		},
		{
			name:       "mismatched actor",
			claims:     jwt.AppClaims{ID: 99, FirstName: "Andere", LastName: "Person"},
			editedBy:   23,
			wantEditor: "Unbekannt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &capturingStudentAuditRepo{}
			service := NewStudentAuditService(repo, nil)
			ctx := context.WithValue(context.Background(), jwt.CtxClaims, tc.claims)

			err := service.RecordChangesForActor(
				ctx,
				&userModels.Student{HealthInfo: strPtr("alt")},
				&userModels.Student{HealthInfo: strPtr("neu")},
				tc.editedBy,
			)

			require.NoError(t, err)
			require.Len(t, repo.edits, 1)
			assert.Equal(t, tc.editedBy, repo.edits[0].EditedBy)
			assert.Equal(t, tc.wantEditor, repo.edits[0].EditedByName)
		})
	}
}
