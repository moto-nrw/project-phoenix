package api

import (
	"testing"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestMembershipCompositionKeepsGroupSupervisionBound(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, err := owners.NewMembershipTestRepositories(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	teacher := testpkg.CreateTestTeacher(t, db, "Group", "Supervisor")
	group := testpkg.CreateTestEducationGroup(t, db, "Membership supervision")
	testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
	pairs, err := owners.NewSchoolStructureSuiteRepositories(testpkg.NewDatabase(t, db)).Group.ListStaffIDsByEducationGroupIDs(testpkg.Ctx(t), []int64{group.ID}, calendar.Date("2026-10-01"))
	require.NoError(t, err)
	require.Len(t, pairs, 1)
	require.Equal(t, teacher.StaffID, pairs[0].StaffID)
	require.Equal(t, group.ID, pairs[0].GroupID)
}

func TestGroupCompositionFailsWhileRoomOwnerIsUnbound(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repo := NewEducationGroupRepository(testpkg.NewDatabase(t, db))
	group := testpkg.CreateTestEducationGroup(t, db, "Missing room owner")
	_, err := repo.FindWithRoom(testpkg.Ctx(t), group.ID)
	require.EqualError(t, err, "database error during find with room: education repositories: room directory is not bound")
}
