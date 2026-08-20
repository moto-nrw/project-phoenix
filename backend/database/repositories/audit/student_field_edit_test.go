package audit_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentFieldEditRepository_CreateBatchAndGetByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Audit", "History", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-field-edit-%d@example.test", time.Now().UnixNano()))

	repo := repositories.NewFactory(db).StudentFieldEdit
	ctx := testpkg.TenantContext(student.TenantID)
	older := &audit.StudentFieldEdit{
		StudentID:    student.ID,
		EditedBy:     account.ID,
		EditedByName: "Ältere Bearbeitung",
		FieldName:    audit.StudentFieldSupervisorNotes,
		OldValue:     testpkg.StrPtr("vorher"),
		NewValue:     testpkg.StrPtr("nachher"),
		CreatedAt:    time.Now().UTC().Add(-time.Minute),
	}
	newestAt := time.Now().UTC()
	newerFirst := &audit.StudentFieldEdit{
		StudentID:    student.ID,
		EditedBy:     account.ID,
		EditedByName: "Neuere Bearbeitung 1",
		FieldName:    audit.StudentFieldStatus,
		OldValue:     testpkg.StrPtr("Ausstehend"),
		NewValue:     testpkg.StrPtr("Aktiv"),
		CreatedAt:    newestAt,
	}
	newerSecond := &audit.StudentFieldEdit{
		StudentID:    student.ID,
		EditedBy:     account.ID,
		EditedByName: "Neuere Bearbeitung 2",
		FieldName:    audit.StudentFieldHealthInfo,
		OldValue:     testpkg.StrPtr("alt"),
		NewValue:     testpkg.StrPtr("neu"),
		CreatedAt:    newestAt,
	}

	require.NoError(t, repo.CreateBatch(ctx, []*audit.StudentFieldEdit{older, newerFirst, newerSecond}))
	require.NotZero(t, older.ID)
	require.NotZero(t, newerFirst.ID)
	require.NotZero(t, newerSecond.ID)
	require.Greater(t, newerSecond.ID, newerFirst.ID)

	edits, err := repo.GetByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, edits, 3)
	assert.Equal(t, newerSecond.ID, edits[0].ID)
	assert.Equal(t, newerFirst.ID, edits[1].ID)
	assert.Equal(t, older.ID, edits[2].ID)
	assert.Equal(t, student.TenantID, edits[0].TenantID)
}

func TestStudentFieldEditRepository_CreateBatchValidation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentFieldEdit
	ctx := testpkg.TenantContext(testpkg.UniqueTestTenantID(t))

	require.NoError(t, repo.CreateBatch(ctx, nil))

	err := repo.CreateBatch(ctx, []*audit.StudentFieldEdit{nil})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "edit cannot be nil")

	err = repo.CreateBatch(ctx, []*audit.StudentFieldEdit{{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "student ID is required")
}
