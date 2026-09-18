package services

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The class-list administration asks the People Directory whether a name is
// already a child of the class (#2382). What that lookup has to match is a
// property of this adapter, not of the owner behind it: the whole name and
// the class, case-insensitive and trimmed on all three keys, with alumni
// invisible.
//
// Alumni matter twice over. Before graduation became a soft delete the row
// was gone and could not collide; with the row retained, an unfiltered lookup
// would refuse a new child who happens to share a graduate's name and class
// (#405 review).

func newClassListEntryStudents(t *testing.T, db *bun.DB) classListEntryStudents {
	t.Helper()
	persons, err := repositories.NewPeopleDirectory(db)
	require.NoError(t, err)
	return classListEntryStudents{persons: persons}
}

func TestClassListEntryStudentsMatchTheWholeNameCaseInsensitively(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	students := newClassListEntryStudents(t, db)
	ctx := testpkg.Ctx(t)
	class := "cls-1a"

	student := testpkg.CreateTestStudent(t, db, "John", "Doe", class)

	matches, err := students.ListStudentIDsByNameAndClass(ctx, " JOHN ", "doe", " "+class+" ")
	require.NoError(t, err)
	assert.Equal(t, []int64{student.ID}, matches, "case and surrounding whitespace are ignored on all three keys")

	partial, err := students.ListStudentIDsByNameAndClass(ctx, "Joh", "Doe", class)
	require.NoError(t, err)
	assert.Empty(t, partial, "a partial first name is a different child")

	otherClass, err := students.ListStudentIDsByNameAndClass(ctx, "John", "Doe", class+"-b")
	require.NoError(t, err)
	assert.Empty(t, otherClass, "the same name in another class is a different child")

	empty, err := students.ListStudentIDsByNameAndClass(ctx, "  ", "Doe", class)
	require.NoError(t, err)
	assert.Empty(t, empty, "an incomplete name matches nobody instead of everybody")
}

func TestClassListEntryStudentsIgnoreGraduates(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	persons, err := repositories.NewPeopleDirectory(db)
	require.NoError(t, err)
	students := classListEntryStudents{persons: persons}
	ctx := testpkg.Ctx(t)
	class := "grad-1a"

	graduate := testpkg.CreateTestStudent(t, db, "Namensvetter", "Kid", class)
	graduated, err := persons.GraduateStudents(ctx, []int64{graduate.ID})
	require.NoError(t, err)
	require.Positive(t, graduated, "the fixture student must have graduated")

	matches, err := students.ListStudentIDsByNameAndClass(ctx, "Namensvetter", "Kid", class)
	require.NoError(t, err)
	assert.Empty(t, matches, "a graduate must not block a new child of the same name")

	enrolled, err := students.IsEnrolledStudent(ctx, graduate.ID)
	require.NoError(t, err)
	assert.False(t, enrolled, "a graduate is not a valid resolve target")

	// An enrolled namesake is still found, and is a valid resolve target.
	active := testpkg.CreateTestStudent(t, db, "Namensvetter", "Kid", class)
	matches, err = students.ListStudentIDsByNameAndClass(ctx, "Namensvetter", "Kid", class)
	require.NoError(t, err)
	assert.Equal(t, []int64{active.ID}, matches)

	enrolled, err = students.IsEnrolledStudent(ctx, active.ID)
	require.NoError(t, err)
	assert.True(t, enrolled)
}

func TestClassListEntryStudentsReportAnUnknownIDAsAbsent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	students := newClassListEntryStudents(t, db)
	ctx := testpkg.Ctx(t)

	// Deleting the fixture leaves an ID that is guaranteed to name no student.
	student := testpkg.CreateTestStudent(t, db, "Geloescht", "Kid", "gone-1a")
	persons, err := repositories.NewPeopleDirectory(db)
	require.NoError(t, err)
	_, err = persons.DeleteStudent(ctx, student.ID)
	require.NoError(t, err)

	enrolled, err := students.IsEnrolledStudent(ctx, student.ID)
	require.NoError(t, err)
	assert.False(t, enrolled, "an unknown ID is absent, not an error")
}
