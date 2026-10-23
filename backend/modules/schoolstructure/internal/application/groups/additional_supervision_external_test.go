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
// apart; every CreateTestExternalCaregiverRecord person is called „Extern Kursleitung“.
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

// TestAdditionalSupervisionOffersExternalCaregivers pins #3823, #4573 and
// ADR 0090: every currently valid external caregiver can join a running
// supervision, with an account, an open invitation or neither (a placeholder
// with „Kein Zugang“), and stays labelled external there. Group
// handover targets keep offering caregivers only, and an external with an
// account in a caregiver role never becomes one. Targets and supervisors
// carry the external caregiver record's organization.
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

	invited := testpkg.CreateTestExternalCaregiverRecord(t, db, "Trommeln").Staff
	renameTestPerson(t, db, invited.PersonID, "Ina", "Eingeladen")
	inviteTestPerson(t, db, invited.PersonID, ownerAccountID)
	uninvited := testpkg.CreateTestExternalCaregiverRecord(t, db, "Gitarre").Staff
	renameTestPerson(t, db, uninvited.PersonID, "Uwe", "Ohneeinladung")
	withAccount, _ := activeTeacher(t, db, "Anna", "Konto")
	_, err = db.NewRaw(`INSERT INTO users.external_caregivers (tenant_id, staff_id, activity_expertise) VALUES (?, ?, ?)`,
		testpkg.Tenant(t), withAccount.StaffID, "Theater").Exec(context.Background())
	require.NoError(t, err)

	expiredFixture := testpkg.CreateTestExternalCaregiverRecord(t, db, "Abgelaufen")
	expired, err := repos.ExternalCaregiverRecords.FindByID(ctx, expiredFixture.ID)
	require.NoError(t, err)
	expiredUntil := calendar.DateFromTime(now).AddDays(-1)
	expired.EndDate = &expiredUntil
	require.NoError(t, repos.ExternalCaregiverRecords.Update(ctx, expired))
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
		{ID: uninvited.ID, FullName: "Uwe Ohneeinladung", IsExternal: true, ExternalAccountStatus: "none", ExternalOrganization: "Test Organization"},
	}, overview.RunningSupervisions[0].AvailableTargets,
		"the placeholder is offered, the expired external caregiver record is not")
	for _, target := range overview.Targets {
		require.NotEqual(t, invited.ID, target.ID, "a person without an account never takes over a group")
		require.NotEqual(t, withAccount.StaffID, target.ID, "an external caregiver never takes over a group")
	}

	created, err := testpkg.TenantQuery2(t, ctx, db, module.Assign, caller, additionalSupervisionAssignment(running.ID, invited.ID))
	require.NoError(t, err)
	require.Equal(t, StaffRef{ID: invited.ID, FullName: "Ina Eingeladen"}, created.Target)
	require.Equal(t, "additional_supervisor", testpkg.GroupSupervisorRowByID(t, db, created.ID).Role)

	placeholder, err := testpkg.TenantQuery2(t, ctx, db, module.Assign, caller, additionalSupervisionAssignment(running.ID, uninvited.ID))
	require.NoError(t, err, "a placeholder without an account can be added")
	require.Equal(t, "additional_supervisor", testpkg.GroupSupervisorRowByID(t, db, placeholder.ID).Role)

	_, err = testpkg.TenantQuery2(t, ctx, db, module.Assign, caller, additionalSupervisionAssignment(running.ID, expiredFixture.Staff.ID))
	require.ErrorIs(t, err, ErrNotFound, "an expired external caregiver record is no longer offered")

	overview, err = testpkg.TenantQuery2(t, ctx, db, module.Overview, caller, OverviewQuery{ActiveGroupID: running.ID, IncludeTargets: true})
	require.NoError(t, err)
	supervisors := overview.RunningSupervisions[0].Supervisors
	require.Contains(t, supervisors, StaffRef{ID: invited.ID, FullName: "Ina Eingeladen", IsExternal: true, ExternalAccountStatus: "invitation_pending", ExternalOrganization: "Test Organization"})
	require.Contains(t, supervisors, StaffRef{ID: uninvited.ID, FullName: "Uwe Ohneeinladung", IsExternal: true, ExternalAccountStatus: "none", ExternalOrganization: "Test Organization"})
}
