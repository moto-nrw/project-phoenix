package groups_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var fixedNow = time.Date(2026, time.August, 29, 10, 0, 0, 0, calendar.Berlin)

func TestSubstitutionResponseIDsSerializeAsStrings(t *testing.T) {
	t.Parallel()

	const id int64 = 9007199254740993

	for name, response := range map[string]any{
		"group handover": GroupHandover{
			ID: id, Group: GroupRef{ID: id}, Target: StaffRef{ID: id},
		},
		"running supervision": RunningSupervision{
			ID: id, Supervisors: []StaffRef{{ID: id}}, AvailableTargets: []StaffRef{{ID: id}},
		},
		"additional supervision": AssignmentResult{
			ID: id, Group: &GroupRef{ID: id}, ActiveGroupID: id, Target: StaffRef{ID: id},
		},
	} {
		t.Run(name, func(t *testing.T) {
			payload, err := json.Marshal(response)
			require.NoError(t, err)
			require.NotContains(t, string(payload), `:9007199254740993`)
			require.Contains(t, string(payload), `:"9007199254740993"`)
		})
	}
}

func TestAdditionalSupervisionExternalInterface(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	// Supervisor names come from the People Directory composition (#2661),
	// so the module is built on the composed repositories the graph uses.
	repos, err := api.NewSchoolStructurePeopleSuiteFactory(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	activeService := testpkg.GroupSupervisorCreator{Repository: reposPresence.GroupSupervisor}
	now := fixedNow
	module := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		ActiveGroups: reposPresence.ActiveGroup, ActiveSupervisors: reposPresence.GroupSupervisor,
		ActiveSupervisorCreator: activeService,
		Now:                     func() time.Time { return now },
	})

	activity := testpkg.CreateTestActivityGroup(t, db, "Lesen")
	room := testpkg.CreateTestRoom(t, db, "Bibliothek")
	running := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	owner, ownerAccountID := activeTeacher(t, db, "Robin", "Owner")
	target, _ := activeTeacher(t, db, "Toni", "Target")
	testpkg.CreateTestGroupSupervisor(t, db, owner.StaffID, running.ID, "supervisor")
	ctx := testpkg.Ctx(t)
	caller := substitutionCaller(t, ownerAccountID, false)

	overview, err := testpkg.TenantQuery2(t, ctx, db, module.Overview, caller, OverviewQuery{
		ActiveGroupID: running.ID, IncludeTargets: true,
	})
	require.NoError(t, err)
	require.Equal(t, []RunningSupervision{{
		ID: running.ID, Type: TargetAdditionalSupervision,
		Name: "Lesen", RoomName: room.Name,
		Supervisors:              []StaffRef{{ID: owner.StaffID, FullName: "Robin Owner"}},
		AvailableTargets:         []StaffRef{{ID: target.StaffID, FullName: "Toni Target"}},
		IsCurrentUserSupervising: true, CanAssign: true,
	}}, overview.RunningSupervisions)

	created, err := testpkg.TenantQuery2(t, ctx, db, module.Assign, caller, Assignment{
		Type: TargetAdditionalSupervision,
		AdditionalSupervision: &AdditionalSupervisionAssignment{
			ActiveGroupID: running.ID, TargetStaffID: target.StaffID,
		},
	})
	require.NoError(t, err)
	require.Equal(t, TargetAdditionalSupervision, created.Type)
	require.Equal(t, running.ID, created.ActiveGroupID)
	require.Equal(t, target.StaffID, created.Target.ID)

	row := testpkg.GroupSupervisorRowByID(t, db, created.ID)
	require.Equal(t, "additional_supervisor", row.Role)
	require.Equal(t, calendar.DateFromTime(now), row.StartDate)
	require.Nil(t, row.EndDate)

	overview, err = testpkg.TenantQuery2(t, ctx, db, module.Overview, caller, OverviewQuery{
		ActiveGroupID: running.ID, IncludeTargets: true,
	})
	require.NoError(t, err)
	require.Len(t, overview.RunningSupervisions, 1)
	require.ElementsMatch(t, []StaffRef{
		{ID: owner.StaffID, FullName: "Robin Owner"},
		{ID: target.StaffID, FullName: "Toni Target"},
	}, overview.RunningSupervisions[0].Supervisors)
	require.Empty(t, overview.RunningSupervisions[0].AvailableTargets)

	_, err = testpkg.TenantQuery2(t, ctx, db, module.Assign, caller, Assignment{
		Type: TargetAdditionalSupervision,
		AdditionalSupervision: &AdditionalSupervisionAssignment{
			ActiveGroupID: running.ID, TargetStaffID: target.StaffID,
		},
	})
	require.ErrorIs(t, err, ErrAlreadyAssigned)
	_, err = testpkg.TenantQuery2(t, ctx, db, module.Assign, caller, Assignment{
		Type: TargetAdditionalSupervision,
		AdditionalSupervision: &AdditionalSupervisionAssignment{
			ActiveGroupID: running.ID, TargetStaffID: owner.StaffID,
		},
	})
	require.ErrorIs(t, err, ErrSelfAssignment)

	var auditCount int
	require.NoError(t, db.NewSelect().TableExpr(`audit.substitution_changes AS "change"`).
		ColumnExpr("COUNT(*)").
		Where(`"change".substitution_id = ?`, created.ID).
		Where(`"change".target_type = ?`, TargetAdditionalSupervision).
		Scan(ctx, &auditCount))
	require.Equal(t, 1, auditCount)
}

func TestAdditionalSupervisionAuthorizationMatrix(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	activeService := testpkg.GroupSupervisorCreator{Repository: reposPresence.GroupSupervisor}

	activity := testpkg.CreateTestActivityGroup(t, db, "Werken")
	room := testpkg.CreateTestRoom(t, db, "Werkraum")
	owned := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	other := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	owner, ownerAccountID := activeTeacher(t, db, "Olivia", "Owner")
	otherOwner, _ := activeTeacher(t, db, "Oscar", "Other")
	_, viewerAccountID := activeTeacher(t, db, "Vera", "Viewer")
	target, _ := activeTeacher(t, db, "Toni", "Target")
	unverified := testpkg.CreateTestStaff(t, db, "Unverified", "Staff")
	testpkg.CreateTestGroupSupervisor(t, db, owner.StaffID, owned.ID, "supervisor")
	testpkg.CreateTestGroupSupervisor(t, db, otherOwner.StaffID, other.ID, "supervisor")
	ctx := testpkg.Ctx(t)

	newModule := func(broad bool) SubstitutionModule {
		return api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
			ActiveGroups: reposPresence.ActiveGroup, ActiveSupervisors: reposPresence.GroupSupervisor,
			ActiveSupervisorCreator: activeService,

			CanSeeAll: func(context.Context, bool, bool, bool) (bool, error) { return broad, nil },
		})
	}

	personal := newModule(false)
	ownerCaller := substitutionCaller(t, ownerAccountID, false)
	overview, err := testpkg.TenantQuery2(t, ctx, db, personal.Overview, ownerCaller, OverviewQuery{})
	require.NoError(t, err)
	require.Equal(t, []int64{owned.ID}, runningSupervisionIDs(overview))
	_, err = personal.Assign(ctx, ownerCaller, additionalSupervisionAssignment(other.ID, target.StaffID))
	require.ErrorIs(t, err, ErrNotFound)

	broad := newModule(true)
	overview, err = testpkg.TenantQuery2(t, ctx, db, broad.Overview, ownerCaller, OverviewQuery{})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{owned.ID, other.ID}, runningSupervisionIDs(overview))
	require.True(t, findRunningSupervision(t, overview, owned.ID).CanAssign)
	require.False(t, findRunningSupervision(t, overview, other.ID).CanAssign)
	_, err = broad.Assign(ctx, ownerCaller, additionalSupervisionAssignment(other.ID, target.StaffID))
	require.ErrorIs(t, err, ErrForbidden)

	viewerCaller := substitutionCaller(t, viewerAccountID, false)
	overview, err = testpkg.TenantQuery2(t, ctx, db, broad.Overview, viewerCaller, OverviewQuery{})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{owned.ID, other.ID}, runningSupervisionIDs(overview))
	for _, supervision := range overview.RunningSupervisions {
		require.False(t, supervision.IsCurrentUserSupervising)
		require.False(t, supervision.CanAssign)
	}
	_, err = broad.Assign(ctx, viewerCaller, additionalSupervisionAssignment(owned.ID, target.StaffID))
	require.ErrorIs(t, err, ErrForbidden)

	admin := testpkg.CreateTestAccount(t, db, "additional-supervision-admin")
	adminCaller := substitutionCaller(t, admin.ID, true)
	overview, err = testpkg.TenantQuery2(t, ctx, db, personal.Overview, adminCaller, OverviewQuery{})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{owned.ID, other.ID}, runningSupervisionIDs(overview))
	created, err := personal.Assign(ctx, adminCaller, additionalSupervisionAssignment(other.ID, target.StaffID))
	require.NoError(t, err)
	require.Equal(t, other.ID, created.ActiveGroupID)

	_, err = personal.Assign(ctx, adminCaller, additionalSupervisionAssignment(owned.ID, unverified.ID))
	require.ErrorIs(t, err, ErrNotFound)

	endedAt := time.Now()
	_, err = db.NewUpdate().TableExpr(`active.groups AS "group"`).
		Set("end_time = ?", endedAt).
		Where(`"group".id = ?`, owned.ID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = personal.Assign(ctx, adminCaller, additionalSupervisionAssignment(owned.ID, target.StaffID))
	require.ErrorIs(t, err, ErrNotRunning)

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	otherTenantGroup := testpkg.CreateTestActiveGroupForTenant(t, db, otherTenant)
	_, err = personal.Assign(ctx, adminCaller, additionalSupervisionAssignment(otherTenantGroup.ID, target.StaffID))
	require.ErrorIs(t, err, ErrNotFound)
}

func TestAdditionalSupervisionAuditFailureRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	activeService := testpkg.GroupSupervisorCreator{Repository: reposPresence.GroupSupervisor}
	module := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		ActiveGroups: reposPresence.ActiveGroup, ActiveSupervisors: reposPresence.GroupSupervisor,
		ActiveSupervisorCreator: activeService,
		Audit:                   failingAudit{},
	})
	activity := testpkg.CreateTestActivityGroup(t, db, "Rollback activity")
	room := testpkg.CreateTestRoom(t, db, "Rollback room")
	running := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	owner, ownerAccountID := activeTeacher(t, db, "Rita", "Owner")
	target, _ := activeTeacher(t, db, "Tara", "Target")
	testpkg.CreateTestGroupSupervisor(t, db, owner.StaffID, running.ID, "supervisor")

	_, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), db, module.Assign, substitutionCaller(t, ownerAccountID, false), additionalSupervisionAssignment(running.ID, target.StaffID))
	require.Error(t, err)
	rows, listErr := testpkg.TenantQuery2(t, testpkg.Ctx(t), db, reposPresence.GroupSupervisor.FindByActiveGroupID, running.ID, true)
	require.NoError(t, listErr)
	require.Len(t, rows, 1)
	require.Equal(t, owner.StaffID, rows[0].StaffID)
}

func TestAdditionalSupervisionTreatsFutureEndDateAsActive(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	module := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		ActiveGroups: reposPresence.ActiveGroup, ActiveSupervisors: reposPresence.GroupSupervisor,
		ActiveSupervisorCreator: testpkg.GroupSupervisorCreator{Repository: reposPresence.GroupSupervisor},
	})
	activity := testpkg.CreateTestActivityGroup(t, db, "Future end activity")
	room := testpkg.CreateTestRoom(t, db, "Future end room")
	running := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	owner, accountID := activeTeacher(t, db, "Future", "Owner")
	target, _ := activeTeacher(t, db, "Future", "Target")
	ownerRow := testpkg.CreateTestGroupSupervisor(t, db, owner.StaffID, running.ID, "supervisor")
	targetRow := testpkg.CreateTestGroupSupervisor(t, db, target.StaffID, running.ID, "additional_supervisor")
	futureEnd := calendar.NewDate(2099, time.January, 1)
	for _, id := range []int64{ownerRow.ID, targetRow.ID} {
		_, err := db.NewUpdate().TableExpr(`active.group_supervisors`).Set("end_date = ?", futureEnd).Where("id = ?", id).Exec(testpkg.Ctx(t))
		require.NoError(t, err)
	}
	overview, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), db, module.Overview, substitutionCaller(t, accountID, false), OverviewQuery{ActiveGroupID: running.ID, IncludeTargets: true})
	require.NoError(t, err)
	require.Len(t, overview.RunningSupervisions, 1)
	require.Len(t, overview.RunningSupervisions[0].Supervisors, 2)
	require.Empty(t, overview.RunningSupervisions[0].AvailableTargets)
	_, err = testpkg.TenantQuery2(t, testpkg.Ctx(t), db, module.Assign, substitutionCaller(t, accountID, false), additionalSupervisionAssignment(running.ID, target.StaffID))
	require.ErrorIs(t, err, ErrAlreadyAssigned)
}

func TestAdditionalSupervisionSignalsOnlyAfterCommit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	activeService := testpkg.GroupSupervisorCreator{Repository: reposPresence.GroupSupervisor}
	broadcaster := testpkg.NewRecordingBroadcaster()
	module := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		ActiveGroups: reposPresence.ActiveGroup, ActiveSupervisors: reposPresence.GroupSupervisor,
		ActiveSupervisorCreator: activeService,
		Broadcaster:             api.NewGroupEvents(broadcaster, nil),
	})
	activity := testpkg.CreateTestActivityGroup(t, db, "Signals activity")
	room := testpkg.CreateTestRoom(t, db, "Signals room")
	running := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	owner, ownerAccountID := activeTeacher(t, db, "Sina", "Owner")
	target, _ := activeTeacher(t, db, "Tina", "Target")
	testpkg.CreateTestGroupSupervisor(t, db, owner.StaffID, running.ID, "supervisor")

	ctx, commit := testpkg.WithAfterCommitHooks(testpkg.Ctx(t))
	_, err := testpkg.TenantQuery2(t, ctx, db, module.Assign, substitutionCaller(t, ownerAccountID, false), additionalSupervisionAssignment(running.ID, target.StaffID))
	require.NoError(t, err)
	require.Empty(t, broadcaster.Events())
	commit()
	require.ElementsMatch(t, []string{"active_supervision_changed", "group_access_changed"}, []string{
		string(broadcaster.Events()[0].Type), string(broadcaster.Events()[1].Type),
	})
}

func TestAdditionalSupervisionRejectsConcurrentSessionEnd(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	activeService := testpkg.GroupSupervisorCreator{Repository: reposPresence.GroupSupervisor}

	activity := testpkg.CreateTestActivityGroup(t, db, "Race activity")
	room := testpkg.CreateTestRoom(t, db, "Race room")
	running := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	owner, ownerAccountID := activeTeacher(t, db, "Raya", "Owner")
	target, _ := activeTeacher(t, db, "Theo", "Target")
	testpkg.CreateTestGroupSupervisor(t, db, owner.StaffID, running.ID, "supervisor")
	entered := make(chan struct{})
	groups := &testpkg.SignalingGroupRepository{SessionRecords: reposPresence.ActiveGroup, Entered: entered}

	module := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		ActiveGroups: groups, ActiveSupervisors: reposPresence.GroupSupervisor,
		ActiveSupervisorCreator: activeService,
	})

	holder, err := db.BeginTx(testpkg.Ctx(t), nil)
	require.NoError(t, err)
	var lockedID int64
	err = holder.NewSelect().TableExpr(`active.groups AS "group"`).
		ColumnExpr(`"group".id`).Where(`"group".id = ?`, running.ID).
		For("UPDATE").Scan(testpkg.Ctx(t), &lockedID)
	require.NoError(t, err)
	require.Equal(t, running.ID, lockedID)
	_, err = holder.NewUpdate().TableExpr(`active.groups AS "group"`).
		Set("end_time = ?", time.Now()).
		Where(`"group".id = ?`, running.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	result := make(chan error, 1)
	assignCtx := testpkg.Ctx(t)
	caller := substitutionCaller(t, ownerAccountID, false)
	go func() {
		_, assignErr := testpkg.TenantQuery2(t, assignCtx, db, module.Assign, caller, additionalSupervisionAssignment(running.ID, target.StaffID))
		result <- assignErr
	}()
	<-entered
	select {
	case assignErr := <-result:
		t.Fatalf("assignment returned before the concurrent end committed: %v", assignErr)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, holder.Commit())
	select {
	case assignErr := <-result:
		require.ErrorIs(t, assignErr, ErrNotRunning)
	case <-time.After(time.Second):
		t.Fatal("assignment did not resume after the concurrent end committed")
	}
}

func TestGroupHandoverExternalInterface(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	service := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		Now: func() time.Time { return fixedNow },
	})
	group := testpkg.CreateTestEducationGroup(t, db, "Robins Gruppe")
	owner, ownerAccountID := activeTeacher(t, db, "Robin", "Owner")
	target, _ := activeTeacher(t, db, "Toni", "Target")
	require.NotZero(t, owner.ID)
	require.Equal(t, testpkg.Tenant(t), owner.TenantID)
	require.Equal(t, testpkg.Tenant(t), group.TenantID)
	testpkg.CreateTestGroupTeacher(t, db, group.ID, owner.ID)
	ctx := testpkg.Ctx(t)
	caller := substitutionCaller(t, ownerAccountID, false)
	created, err := testpkg.TenantQuery2(t, ctx, db, service.Assign, caller, Assignment{
		Type:          TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{GroupID: group.ID, TargetStaffID: target.StaffID},
	})
	require.NoError(t, err)
	require.Equal(t, TargetGroupHandover, created.Type)
	require.Equal(t, fixedNow.Format(time.DateOnly), created.Period.StartDate)
	require.Equal(t, "Toni Target", created.Target.FullName)

	overview, err := testpkg.TenantQuery2(t, ctx, db, service.Overview, caller, OverviewQuery{GroupID: group.ID, IncludeTargets: true})
	require.NoError(t, err)
	require.Equal(t, []GroupRef{{ID: group.ID, Name: group.Name}}, overview.Groups)
	require.Len(t, overview.GroupHandovers, 1)
	require.True(t, overview.GroupHandovers[0].CanEnd)
	require.Contains(t, overview.Targets, StaffRef{ID: target.StaffID, FullName: "Toni Target"})
	payload, err := json.Marshal(overview)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "account_id")
	require.NotContains(t, string(payload), "email")
	require.NotContains(t, string(payload), "first_name")

	var auditCount int
	require.NoError(t, db.NewSelect().TableExpr(`audit.substitution_changes AS "change"`).
		ColumnExpr("COUNT(*)").
		Where(`"change".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"change".target_type = ?`, TargetGroupHandover).
		Where(`"change".substitution_id = ?`, created.ID).
		Scan(ctx, &auditCount))
	require.Equal(t, 1, auditCount)

	_, err = testpkg.TenantQuery2(t, ctx, db, service.Assign, caller, Assignment{
		Type:          TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{GroupID: group.ID, TargetStaffID: target.StaffID},
	})
	require.ErrorIs(t, err, ErrAlreadyAssigned)
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, service.End, caller, EndRequest{Type: TargetGroupHandover, ID: created.ID}))
	require.NoError(t, db.NewSelect().TableExpr(`audit.substitution_changes AS "change"`).
		ColumnExpr("COUNT(*)").
		Where(`"change".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"change".target_type = ?`, TargetGroupHandover).
		Where(`"change".substitution_id = ?`, created.ID).
		Scan(ctx, &auditCount))
	require.Equal(t, 2, auditCount)
}

func TestGroupHandoverPermissionsAndPeriod(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	service := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		Now: func() time.Time { return fixedNow },
	})
	owned := testpkg.CreateTestEducationGroup(t, db, "Own")
	foreign := testpkg.CreateTestEducationGroup(t, db, "Other")
	owner, accountID := activeTeacher(t, db, "Alex", "Owner")
	target, targetAccountID := activeTeacher(t, db, "Chris", "Target")
	testpkg.CreateTestGroupTeacher(t, db, owned.ID, owner.ID)
	ctx := testpkg.Ctx(t)
	caller := substitutionCaller(t, accountID, false)
	unauthorized := caller
	unauthorized.Roles = nil
	_, err := testpkg.TenantQuery2(t, ctx, db, service.Overview, unauthorized, OverviewQuery{})
	require.ErrorIs(t, err, ErrForbidden)

	// A role the school defines itself (#3469) reaches its own groups through
	// the permission its routes read, without the standard role names.
	schoolRole := caller
	schoolRole.Roles = []string{"betreuungskraft"}
	schoolRole.HasPermission = func(permission string) bool { return permission == "substitutions:read" }
	overview, err := testpkg.TenantQuery2(t, ctx, db, service.Overview, schoolRole, OverviewQuery{})
	require.NoError(t, err)
	require.NotNil(t, overview)
	schoolRole.HasPermission = func(string) bool { return false }
	_, err = testpkg.TenantQuery2(t, ctx, db, service.Overview, schoolRole, OverviewQuery{})
	require.ErrorIs(t, err, ErrForbidden)

	_, err = testpkg.TenantQuery2(t, ctx, db, service.Assign, caller, Assignment{Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{GroupID: owned.ID, TargetStaffID: owner.StaffID}})
	require.ErrorIs(t, err, ErrInvalidTarget)

	_, err = testpkg.TenantQuery2(t, ctx, db, service.Assign, caller, Assignment{Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{GroupID: foreign.ID, TargetStaffID: target.StaffID}})
	require.ErrorIs(t, err, ErrNotFound)
	tomorrow := calendar.DateFromTime(fixedNow).AddDays(1)
	_, err = testpkg.TenantQuery2(t, ctx, db, service.Assign, caller, Assignment{Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{GroupID: owned.ID, TargetStaffID: target.StaffID, StartDate: &tomorrow, EndDate: &tomorrow}})
	require.ErrorIs(t, err, ErrInvalidPeriod)
	received, err := testpkg.TenantQuery2(t, ctx, db, service.Assign, caller, Assignment{Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{GroupID: owned.ID, TargetStaffID: target.StaffID}})
	require.NoError(t, err)
	receivedOverview, err := testpkg.TenantQuery2(t, ctx, db, service.Overview, substitutionCaller(t, targetAccountID, false), OverviewQuery{GroupID: owned.ID})
	require.NoError(t, err)
	require.Len(t, receivedOverview.GroupHandovers, 1)
	require.False(t, receivedOverview.GroupHandovers[0].CanEnd)
	require.ErrorIs(t, testpkg.TenantCommand2(t, ctx, db, service.End, substitutionCaller(t, targetAccountID, false), EndRequest{
		Type: TargetGroupHandover, ID: received.ID,
	}), ErrNotFound)
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, service.End, caller, EndRequest{Type: TargetGroupHandover, ID: received.ID}))
	dualRoleCaller := caller
	dualRoleCaller.Admin = true
	todayHandover, err := testpkg.TenantQuery2(t, ctx, db, service.Assign, dualRoleCaller, Assignment{
		Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{
			GroupID: owned.ID, TargetStaffID: target.StaffID,
		},
	})
	require.NoError(t, err)
	require.Equal(t, calendar.DateFromTime(fixedNow).String(), todayHandover.Period.StartDate)
	require.Equal(t, todayHandover.Period.StartDate, todayHandover.Period.EndDate)
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, service.End, dualRoleCaller, EndRequest{
		Type: TargetGroupHandover, ID: todayHandover.ID,
	}))

	admin := testpkg.CreateTestAccount(t, db, "substitution-admin")
	end := tomorrow.AddDays(3)
	adminCaller := substitutionCaller(t, admin.ID, true)
	_, err = testpkg.TenantQuery2(t, ctx, db, service.Assign, adminCaller, Assignment{Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{
			GroupID: foreign.ID, TargetStaffID: target.StaffID, StartDate: &tomorrow,
		}})
	require.ErrorIs(t, err, ErrInvalidPeriod)
	created, err := testpkg.TenantQuery2(t, ctx, db, service.Assign, adminCaller, Assignment{Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{GroupID: foreign.ID, TargetStaffID: target.StaffID, StartDate: &tomorrow, EndDate: &end}})
	require.NoError(t, err)
	require.Equal(t, end.String(), created.Period.EndDate)
	futureOwn, err := testpkg.TenantQuery2(t, ctx, db, service.Assign, adminCaller, Assignment{Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{GroupID: owned.ID, TargetStaffID: target.StaffID, StartDate: &tomorrow, EndDate: &end}})
	require.NoError(t, err)
	ownerOverview, err := testpkg.TenantQuery2(t, ctx, db, service.Overview, caller, OverviewQuery{})
	require.NoError(t, err)
	require.Empty(t, ownerOverview.GroupHandovers)
	require.ErrorIs(t, testpkg.TenantCommand2(t, ctx, db, service.End, caller, EndRequest{Type: TargetGroupHandover, ID: futureOwn.ID}), ErrNotRunning)

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	otherGroup := testpkg.CreateTestEducationGroupForTenant(t, db, otherTenant, "Other tenant")
	_, err = testpkg.TenantQuery2(t, ctx, db, service.Assign, adminCaller, Assignment{Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{GroupID: otherGroup.ID, TargetStaffID: target.StaffID, StartDate: &tomorrow, EndDate: &tomorrow}})
	require.ErrorIs(t, err, ErrNotFound)

	regularStaffID := owner.StaffID
	legacy := testpkg.CreateTestGroupSubstitution(t, db, foreign.ID, &regularStaffID, target.StaffID, tomorrow, tomorrow)
	adminOverview, err := testpkg.TenantQuery2(t, ctx, db, service.Overview, adminCaller, OverviewQuery{On: &tomorrow, IncludeTargets: true})
	require.NoError(t, err)
	require.ElementsMatch(t, []GroupRef{
		{ID: owned.ID, Name: owned.Name},
		{ID: foreign.ID, Name: foreign.Name},
	}, adminOverview.Groups)
	for _, handover := range adminOverview.GroupHandovers {
		require.NotEqual(t, legacy.ID, handover.ID)
	}
	require.ErrorIs(t, testpkg.TenantCommand2(t, ctx, db, service.End, adminCaller, EndRequest{Type: TargetGroupHandover, ID: legacy.ID}), ErrNotFound)
	_, err = testpkg.TenantQuery1(t, ctx, db, repos.GroupSubstitution.FindByID, any(legacy.ID))
	require.NoError(t, err)
}

func TestGroupHandoverAllStaffVisibilityDoesNotGrantActions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	service := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		Now:       func() time.Time { return fixedNow },
		CanSeeAll: func(context.Context, bool, bool, bool) (bool, error) { return true, nil },
	})
	group := testpkg.CreateTestEducationGroup(t, db, "Visible foreign group")
	owner, ownerAccountID := activeTeacher(t, db, "Olivia", "Owner")
	_, observerAccountID := activeTeacher(t, db, "Vera", "Viewer")
	target, _ := activeTeacher(t, db, "Toni", "Target")
	testpkg.CreateTestGroupTeacher(t, db, group.ID, owner.ID)
	ctx := testpkg.Ctx(t)

	created, err := testpkg.TenantQuery2(t, ctx, db, service.Assign, substitutionCaller(t, ownerAccountID, false), Assignment{
		Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{
			GroupID: group.ID, TargetStaffID: target.StaffID,
		},
	})
	require.NoError(t, err)

	observerCaller := substitutionCaller(t, observerAccountID, false)
	overview, err := testpkg.TenantQuery2(t, ctx, db, service.Overview, observerCaller, OverviewQuery{GroupID: group.ID, IncludeTargets: true})
	require.NoError(t, err)
	require.Empty(t, overview.Groups, "school-wide visibility must not grant group handover actions")
	require.Len(t, overview.GroupHandovers, 1)
	require.False(t, overview.GroupHandovers[0].CanEnd)

	_, err = testpkg.TenantQuery2(t, ctx, db, service.Assign, observerCaller, Assignment{
		Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{
			GroupID: group.ID, TargetStaffID: target.StaffID,
		},
	})
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, testpkg.TenantCommand2(t, ctx, db, service.End, observerCaller, EndRequest{
		Type: TargetGroupHandover, ID: created.ID,
	}), ErrNotFound)

	schoolCaller := observerCaller
	schoolCaller.Scope = "school"
	_, err = testpkg.TenantQuery2(t, ctx, db, service.Overview, schoolCaller, OverviewQuery{GroupID: group.ID})
	require.ErrorIs(t, err, ErrForbidden)
}

func TestGroupHandoverAuditFailureRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	service := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		Audit: failingAudit{}, Now: func() time.Time { return fixedNow },
	})
	group := testpkg.CreateTestEducationGroup(t, db, "Rollback")
	target, _ := activeTeacher(t, db, "Sam", "Target")
	admin := testpkg.CreateTestAccount(t, db, "rollback-admin")
	today := calendar.DateFromTime(fixedNow)
	_, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), db, service.Assign, substitutionCaller(t, admin.ID, true), Assignment{Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{GroupID: group.ID, TargetStaffID: target.StaffID, StartDate: &today, EndDate: &today}})
	require.Error(t, err)
	rows, listErr := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.GroupSubstitution.FindByGroup, group.ID)
	require.NoError(t, listErr)
	require.Empty(t, rows)

	workingService := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		Now: func() time.Time { return fixedNow },
	})
	created, err := workingService.Assign(testpkg.Ctx(t), substitutionCaller(t, admin.ID, true), Assignment{
		Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{
			GroupID: group.ID, TargetStaffID: target.StaffID, StartDate: &today, EndDate: &today,
		},
	})
	require.NoError(t, err)
	require.Error(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), db, service.End, substitutionCaller(t, admin.ID, true), EndRequest{
		Type: TargetGroupHandover, ID: created.ID,
	}))
	_, err = testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.GroupSubstitution.FindByID, any(created.ID))
	require.NoError(t, err)
}

func TestGroupHandoverSignalsOnlyAfterCommit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	broadcaster := testpkg.NewRecordingBroadcaster()
	service := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		Broadcaster: api.NewGroupEvents(broadcaster, nil),
		Now:         func() time.Time { return fixedNow },
	})
	group := testpkg.CreateTestEducationGroup(t, db, "Signals")
	target, _ := activeTeacher(t, db, "Siggi", "Signal")
	admin := testpkg.CreateTestAccount(t, db, "signal-admin")
	today := calendar.DateFromTime(fixedNow)
	caller := substitutionCaller(t, admin.ID, true)

	assignCtx, commitAssign := testpkg.WithAfterCommitHooks(testpkg.Ctx(t))
	created, err := testpkg.TenantQuery2(t, assignCtx, db, service.Assign, caller, Assignment{
		Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{
			GroupID: group.ID, TargetStaffID: target.StaffID, StartDate: &today, EndDate: &today,
		},
	})
	require.NoError(t, err)
	require.Empty(t, broadcaster.Events())
	commitAssign()
	require.Len(t, broadcaster.Events(), 1)
	require.Equal(t, "group_access_changed", string(broadcaster.Events()[0].Type))

	endCtx, commitEnd := testpkg.WithAfterCommitHooks(testpkg.Ctx(t))
	require.NoError(t, testpkg.TenantCommand2(t, endCtx, db, service.End, caller, EndRequest{
		Type: TargetGroupHandover, ID: created.ID,
	}))
	require.Len(t, broadcaster.Events(), 1)
	commitEnd()
	require.Len(t, broadcaster.Events(), 2)
	require.Equal(t, "group_access_changed", string(broadcaster.Events()[1].Type))
}

func TestGroupHandoverRechecksOwnershipAfterGroupLock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposStructure := owners.NewSchoolStructureSuiteRepositories(testpkg.NewDatabase(t, db))
	group := testpkg.CreateTestEducationGroup(t, db, "OwnershipRace")
	owner, accountID := activeTeacher(t, db, "Owner", "Race")
	target, _ := activeTeacher(t, db, "Target", "Race")
	testpkg.CreateTestGroupTeacher(t, db, group.ID, owner.ID)
	groups := &ownershipRevokingGroups{
		GroupStore: reposStructure.Group, links: reposStructure.GroupTeacher, teacherID: owner.ID,
	}
	service := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		Groups: groups, Now: func() time.Time { return fixedNow },
	})

	_, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), db, service.Assign, substitutionCaller(t, accountID, false), Assignment{
		Type: TargetGroupHandover,
		GroupHandover: &GroupHandoverAssignment{
			GroupID: group.ID, TargetStaffID: target.StaffID,
		},
	})
	require.ErrorIs(t, err, ErrNotFound)
	rows, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.GroupSubstitution.FindByGroup, group.ID)
	require.NoError(t, err)
	require.Empty(t, rows)
	relations, err := reposStructure.GroupTeacher.FindByGroup(testpkg.Ctx(t), group.ID)
	require.NoError(t, err)
	require.Len(t, relations, 1, "the simulated concurrent removal must roll back with the rejected assignment")
}

// groupTeacherLinks is the slice of the group teacher store the ownership
// race reads and changes.
type groupTeacherLinks interface {
	FindByGroup(ctx context.Context, groupID int64) ([]*api.SchoolStructureSuiteGroupTeacher, error)
	Delete(ctx context.Context, id any) error
}

type ownershipRevokingGroups struct {
	GroupStore
	links     groupTeacherLinks
	teacherID int64
}

func (g *ownershipRevokingGroups) FindByIDForUpdate(ctx context.Context, id any) (*schoolStructureCompose.Group, error) {
	group, err := g.GroupStore.FindByIDForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	relations, err := g.links.FindByGroup(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	for _, relation := range relations {
		if relation.TeacherID == g.teacherID {
			if err := g.links.Delete(ctx, relation.ID); err != nil {
				return nil, err
			}
			break
		}
	}
	return group, nil
}

type failingAudit struct{}

func (failingAudit) RecordSubstitutionChange(context.Context, schoolStructureCompose.SubstitutionChange) error {
	return errors.New("audit unavailable")
}

func activeTeacher(t *testing.T, db *bun.DB, firstName, lastName string) (*testpkg.Teacher, int64) {
	t.Helper()
	staff, account := testpkg.CreateTestCalendarStaff(t, db, firstName, lastName)
	teacher := &testpkg.Teacher{StaffID: staff.ID, Staff: staff}
	teacher.TenantID = testpkg.Tenant(t)
	_, err := db.NewInsert().Model(teacher).ModelTableExpr("users.teachers").Exec(context.Background())
	require.NoError(t, err)
	return teacher, account.ID
}

func substitutionCaller(t *testing.T, accountID int64, admin bool) SubstitutionCaller {
	t.Helper()
	return SubstitutionCaller{
		AccountID: accountID, TenantID: testpkg.Tenant(t), Roles: []string{"user"}, Admin: admin,
	}
}

func additionalSupervisionAssignment(activeGroupID, targetStaffID int64) Assignment {
	return Assignment{
		Type: TargetAdditionalSupervision,
		AdditionalSupervision: &AdditionalSupervisionAssignment{
			ActiveGroupID: activeGroupID,
			TargetStaffID: targetStaffID,
		},
	}
}

func runningSupervisionIDs(overview *OverviewResult) []int64 {
	ids := make([]int64, 0, len(overview.RunningSupervisions))
	for _, supervision := range overview.RunningSupervisions {
		ids = append(ids, supervision.ID)
	}
	return ids
}

func findRunningSupervision(t *testing.T, overview *OverviewResult, activeGroupID int64) RunningSupervision {
	t.Helper()
	for _, supervision := range overview.RunningSupervisions {
		if supervision.ID == activeGroupID {
			return supervision
		}
	}
	t.Fatalf("running supervision %d not found", activeGroupID)
	return RunningSupervision{}
}
