package repositories_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeachingAssignmentAdaptersPreserveLegacyContracts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Assignment", "Contract")
	classAssignment := testpkg.CreateTestClassTeacher(t, db, staff.ID, " 1A ")

	matching, err := repos.ClassTeacher.List(ctx, map[string]any{"school_class": "1a"})
	require.NoError(t, err)
	require.Len(t, matching, 1)
	assert.Equal(t, classAssignment.ID, matching[0].ID)

	_, err = repos.GroupTeacher.List(ctx, map[string]any{"teacher_id": "not an ID"})
	require.Error(t, err, "a malformed legacy filter must not silently become an unfiltered query")

}
