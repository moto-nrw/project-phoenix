package groups_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// renameTestPerson gives a fixture person a name the assertions can tell
// apart; every CreateTestGuest person is called „Guest Instructor“.
func renameTestPerson(t *testing.T, db *bun.DB, personID int64, firstName, lastName string) {
	t.Helper()
	_, err := db.NewRaw(`UPDATE users.persons SET first_name = ?, last_name = ? WHERE id = ?`, firstName, lastName, personID).
		Exec(context.Background())
	require.NoError(t, err)
}

// inviteTestPerson stores an open invitation that names the person.
func inviteTestPerson(t *testing.T, db *bun.DB, personID, createdBy int64) {
	t.Helper()
	role := testpkg.GetOrCreateTestRole(t, db, "user")
	invitation := testpkg.CreateTestInvitationToken(t, db, "extern", role.ID, createdBy, time.Now().Add(48*time.Hour))
	_, err := db.NewRaw(`UPDATE auth.invitation_tokens SET person_id = ? WHERE id = ?`, personID, invitation.ID).
		Exec(context.Background())
	require.NoError(t, err)
}

// TestAdditionalSupervisionOffersExternalCaregivers pins #3823 and #4573: an
// external caregiver with an account or an open invitation can join a
// running supervision; one without either is not offered and cannot be
// assigned, but stays labelled external where it already supervises. Group
// handover targets keep offering caregivers only, and an external with an
// account in a caregiver role never becomes one. Targets and supervisors
// carry the guest profile's organization.
func TestAdditionalSupervisionOffersExternalCaregivers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos, err := api.NewSchoolStructurePeopleSuiteFactory(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	now := fixedNow
	module := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		ActiveGroups: reposPresence.ActiveGroup, ActiveSupervisors: reposPresence.GroupSupervisor,
		ActiveSupervisorCreator: testpkg.GroupSupervisorCreator{Repository: reposPresence.GroupSupervisor},
		Now:                     func() time.Time { return now },
	})

	activity := testpkg.CreateTestActivityGroup(t, db, "Trommel-AG")
	room := testpkg.CreateTestRoom(t, db, "Musikraum")
	running := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	ctx := testpkg.Ctx(t)
	owner, ownerAccountID := activeTeacher(t, db, "Robin", "Owner")
	colleague, _ := activeTeacher(t, db, "Toni", "Kollege")

	invited := testpkg.CreateTestGuest(t, db, "Trommeln").Staff
	renameTestPerson(t, db, invited.PersonID, "Ina", "Eingeladen")
	inviteTestPerson(t, db, invited.PersonID, ownerAccountID)
	uninvited := testpkg.CreateTestGuest(t, db, "Gitarre").Staff
	renameTestPerson(t, db, uninvited.PersonID, "Uwe", "Ohneeinladung")
	withAccount, _ := activeTeacher(t, db, "Anna", "Konto")
	_, err = db.NewRaw(`INSERT INTO users.guests (tenant_id, staff_id, activity_expertise) VALUES (?, ?, ?)`,
		testpkg.Tenant(t), withAccount.StaffID, "Theater").Exec(context.Background())
	require.NoError(t, err)

	expiredFixture := testpkg.CreateTestGuest(t, db, "Abgelaufen")
	expired, err := repos.Guest.FindByID(ctx, expiredFixture.ID)
	require.NoError(t, err)
	expiredUntil := calendar.DateFromTime(now).AddDays(-1)
	expired.EndDate = &expiredUntil
	require.NoError(t, repos.Guest.Update(ctx, expired))
	inviteTestPerson(t, db, expiredFixture.Staff.PersonID, ownerAccountID)
	testpkg.CreateTestStaff(t, db, "Ohne", "Konto")
	testpkg.CreateTestGroupSupervisor(t, db, owner.StaffID, running.ID, "supervisor")
	caller := substitutionCaller(t, ownerAccountID, false)

	overview, err := testpkg.TenantQuery2(t, ctx, db, module.Overview, caller, OverviewQuery{ActiveGroupID: running.ID, IncludeTargets: true})
	require.NoError(t, err)
	require.Len(t, overview.RunningSupervisions, 1)
	require.Equal(t, []StaffRef{
		{ID: colleague.StaffID, FullName: "Toni Kollege"},
		{ID: withAccount.StaffID, FullName: "Anna Konto", IsExternal: true, ExternalAccountStatus: "account"},
		{ID: invited.ID, FullName: "Ina Eingeladen", IsExternal: true, ExternalAccountStatus: "invitation_pending", ExternalOrganization: "Test Organization"},
	}, overview.RunningSupervisions[0].AvailableTargets,
		"neither the uninvited external nor the expired guest profile is offered")
	for _, target := range overview.Targets {
		require.NotEqual(t, invited.ID, target.ID, "a person without an account never takes over a group")
		require.NotEqual(t, withAccount.StaffID, target.ID, "an external caregiver never takes over a group")
	}

	_, err = testpkg.TenantQuery2(t, ctx, db, module.Assign, caller, additionalSupervisionAssignment(running.ID, uninvited.ID))
	require.ErrorIs(t, err, ErrNotFound, "nobody vouched for the uninvited external with an address")

	created, err := testpkg.TenantQuery2(t, ctx, db, module.Assign, caller, additionalSupervisionAssignment(running.ID, invited.ID))
	require.NoError(t, err)
	require.Equal(t, StaffRef{ID: invited.ID, FullName: "Ina Eingeladen"}, created.Target)
	require.Equal(t, "additional_supervisor", testpkg.GroupSupervisorRowByID(t, db, created.ID).Role)

	// An external recorded before #4573 who already supervises keeps the
	// label, though nobody may add them anew.
	testpkg.CreateTestGroupSupervisor(t, db, uninvited.ID, running.ID, "additional_supervisor")
	overview, err = testpkg.TenantQuery2(t, ctx, db, module.Overview, caller, OverviewQuery{ActiveGroupID: running.ID, IncludeTargets: true})
	require.NoError(t, err)
	supervisors := overview.RunningSupervisions[0].Supervisors
	require.Contains(t, supervisors, StaffRef{ID: invited.ID, FullName: "Ina Eingeladen", IsExternal: true, ExternalAccountStatus: "invitation_pending", ExternalOrganization: "Test Organization"})
	require.Contains(t, supervisors, StaffRef{ID: uninvited.ID, FullName: "Uwe Ohneeinladung", IsExternal: true, ExternalAccountStatus: "none", ExternalOrganization: "Test Organization"})
}
