package groups_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestAdditionalSupervisionOffersExternalCaregivers pins #3823: an external
// caregiver without a moto account can join a running supervision, while the
// group handover targets keep offering account holders only, and other staff
// without an account stay out of both.
func TestAdditionalSupervisionOffersExternalCaregivers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos, err := testutil.NewSchoolStructurePeopleSuiteFactory(db)
	require.NoError(t, err)
	now := fixedNow
	module := testutil.NewSubstitutionSuiteModule(repos, db, SubstitutionDependencies{
		ActiveGroups: repos.ActiveGroup, ActiveSupervisors: repos.GroupSupervisor,
		ActiveSupervisorCreator: testpkg.GroupSupervisorCreator{Repository: repos.GroupSupervisor},
		Now:                     func() time.Time { return now },
	})

	activity := testpkg.CreateTestActivityGroup(t, db, "Trommel-AG")
	room := testpkg.CreateTestRoom(t, db, "Musikraum")
	running := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	owner, ownerAccountID := activeTeacher(t, db, "Robin", "Owner")
	colleague, _ := activeTeacher(t, db, "Toni", "Kollege")
	external := testpkg.CreateTestGuest(t, db, "Trommeln").Staff
	testpkg.CreateTestStaff(t, db, "Ohne", "Konto")
	testpkg.CreateTestGroupSupervisor(t, db, owner.StaffID, running.ID, "supervisor")
	ctx := testpkg.Ctx(t)
	caller := substitutionCaller(t, ownerAccountID, false)

	overview, err := module.Overview(ctx, caller, OverviewQuery{ActiveGroupID: running.ID, IncludeTargets: true})
	require.NoError(t, err)
	require.Len(t, overview.RunningSupervisions, 1)
	require.Equal(t, []StaffRef{
		{ID: colleague.StaffID, FullName: "Toni Kollege"},
		{ID: external.ID, FullName: "Guest Instructor", IsExternal: true},
	}, overview.RunningSupervisions[0].AvailableTargets)
	for _, target := range overview.Targets {
		require.NotEqual(t, external.ID, target.ID, "a person without an account never takes over a group")
	}

	created, err := module.Assign(ctx, caller, Assignment{
		Type: TargetAdditionalSupervision,
		AdditionalSupervision: &AdditionalSupervisionAssignment{
			ActiveGroupID: running.ID, TargetStaffID: external.ID,
		},
	})
	require.NoError(t, err)
	require.Equal(t, StaffRef{ID: external.ID, FullName: "Guest Instructor"}, created.Target)
	require.Equal(t, "additional_supervisor", testpkg.GroupSupervisorRowByID(t, db, created.ID).Role)
}
