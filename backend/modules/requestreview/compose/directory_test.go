package compose

import (
	"context"
	"testing"

	peoplecompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}

func TestDirectoryDecoratesOnlyRequestedChildrenInTheTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	people, err := peoplecompose.New(peoplecompose.Dependencies{DB: db, Observe: func(peoplecompose.Observation) {}})
	require.NoError(t, err)
	directory, err := NewStudentDirectory(db, people, func(DirectoryObservation) {})
	require.NoError(t, err)
	group := testpkg.CreateTestEducationGroup(t, db, "ReviewDirectory")
	grouped := testpkg.CreateTestStudent(t, db, "Grouped", "Child", "1a")
	ungrouped := testpkg.CreateTestStudent(t, db, "Ungrouped", "Child", "1a")
	testpkg.AssignStudentToGroup(t, db, grouped.ID, group.ID)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Foreign", "Child", "2a")

	names, err := directory.GroupNames(testpkg.Ctx(t), []int64{grouped.ID, ungrouped.ID, foreign.ID, grouped.ID})
	require.NoError(t, err)
	require.Equal(t, map[int64]string{grouped.ID: group.Name}, names)
	names, err = directory.GroupNames(testpkg.Ctx(t), []int64{ungrouped.ID})
	require.NoError(t, err)
	require.Empty(t, names)
	names, err = directory.GroupNames(testpkg.Ctx(t), nil)
	require.NoError(t, err)
	require.Empty(t, names)
}

func TestDirectoryIsOptionalButConfiguredOwnerFailuresAreNot(t *testing.T) {
	t.Parallel()
	directory, err := NewStudentDirectory(nil, nil, nil)
	require.NoError(t, err)
	require.Nil(t, directory)

	db := testpkg.SetupTestDB(t)
	people, err := peoplecompose.New(peoplecompose.Dependencies{DB: db, Observe: func(peoplecompose.Observation) {}})
	require.NoError(t, err)
	_, err = NewStudentDirectory(nil, people, func(DirectoryObservation) {})
	require.Error(t, err)
	directory, err = NewStudentDirectory(db, people, func(DirectoryObservation) {})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Cancelled", "Read", "1a")
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()
	_, err = directory.GroupNames(ctx, []int64{student.ID})
	require.ErrorIs(t, err, context.Canceled)
}
