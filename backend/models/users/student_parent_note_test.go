package users_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestStudentParentNote_AccessorsAndHooks(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	note := &users.StudentParentNote{StudentID: 5, GuardianAccountID: 7, Body: "Hallo"}
	note.SetTenantID(1)

	// Accessors.
	assert.Equal(t, "users.student_parent_notes", note.TableName())
	assert.NotNil(t, note.GetID()) // zero before insert, but the getter must run
	assert.True(t, note.GetCreatedAt().IsZero())
	assert.True(t, note.GetUpdatedAt().IsZero())
	assert.Equal(t, int64(1), note.GetTenantID())

	// BeforeAppendModel must set the table expr for each mutating query
	// type and no-op for anything else.
	require.NoError(t, note.BeforeAppendModel(db.NewInsert().Model(note)))
	require.NoError(t, note.BeforeAppendModel(db.NewUpdate().Model(note)))
	require.NoError(t, note.BeforeAppendModel(db.NewDelete().Model(note)))
	require.NoError(t, note.BeforeAppendModel("not-a-query"))
}
