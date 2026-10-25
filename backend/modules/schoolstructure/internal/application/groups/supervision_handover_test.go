package groups_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// supervisionHandoverFixture is a running room session supervised by owner,
// with one child in the room and a colleague who could join (#4763).
type supervisionHandoverFixture struct {
	db           *bun.DB
	presence     *api.PresenceSuiteRepositories
	module       SubstitutionModule
	broadcaster  *testpkg.RecordingBroadcaster
	running      int64
	visitID      int64
	ownerStaff   int64
	ownerAccount int64
	targetStaff  int64
	targetAcct   int64
	viewerAcct   int64
	ownerRow     int64
}

func newSupervisionHandoverFixture(t *testing.T) supervisionHandoverFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	presence := api.NewActiveTestPresence(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db)).Active
	broadcaster := testpkg.NewRecordingBroadcaster()
	module := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		ActiveGroups: reposPresence.ActiveGroup, ActiveSupervisors: reposPresence.GroupSupervisor,
		ActiveSupervisorCreator: testpkg.GroupSupervisorCreator{Repository: reposPresence.GroupSupervisor},
		SupervisionRelease:      presence,
		Broadcaster:             api.NewGroupEvents(broadcaster, nil),
	})

	activity := testpkg.CreateTestActivityGroup(t, db, "Basteln")
	room := testpkg.CreateTestRoom(t, db, "Bastelraum")
	running := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	owner, ownerAccountID := activeTeacher(t, db, "Hanna", "Owner")
	target, targetAccountID := activeTeacher(t, db, "Theo", "Target")
	_, viewerAccountID := activeTeacher(t, db, "Vera", "Viewer")
	ownerRow := testpkg.CreateTestGroupSupervisor(t, db, owner.StaffID, running.ID, "supervisor")
	student := testpkg.CreateTestStudent(t, db, "Kim", "Kind", "2a")
	visit := testpkg.CreateTestVisit(t, db, student.ID, running.ID, time.Now().Add(-time.Hour), nil)

	return supervisionHandoverFixture{
		db: db, presence: reposPresence, module: module, broadcaster: broadcaster, running: running.ID, visitID: visit.ID,
		ownerStaff: owner.StaffID, ownerAccount: ownerAccountID, targetStaff: target.StaffID,
		targetAcct: targetAccountID, viewerAcct: viewerAccountID, ownerRow: ownerRow.ID,
	}
}

func (f supervisionHandoverFixture) handOver(t *testing.T, ctx context.Context, caller SubstitutionCaller, targetStaffID int64) (*AssignmentResult, error) {
	t.Helper()
	return testpkg.TenantQuery2(t, ctx, f.db, f.module.Assign, caller, Assignment{
		Type: TargetAdditionalSupervision,
		AdditionalSupervision: &AdditionalSupervisionAssignment{
			ActiveGroupID: f.running, TargetStaffID: targetStaffID, HandOver: true,
		},
	})
}

func (f supervisionHandoverFixture) release(t *testing.T, ctx context.Context, caller SubstitutionCaller) error {
	t.Helper()
	return testpkg.TenantCommand2(t, ctx, f.db, f.module.End, caller, EndRequest{Type: TargetAdditionalSupervision, ID: f.running})
}

func (f supervisionHandoverFixture) currentSupervisors(t *testing.T) []int64 {
	t.Helper()
	rows, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), f.db, f.presence.GroupSupervisor.FindByActiveGroupID, f.running, true)
	require.NoError(t, err)
	staffIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		staffIDs = append(staffIDs, row.StaffID)
	}
	slices.Sort(staffIDs)
	return staffIDs
}

// requireSessionUntouched checks that the session still runs and the child
// is still in the room.
func (f supervisionHandoverFixture) requireSessionUntouched(t *testing.T) {
	t.Helper()
	var sessionEnd, visitExit *time.Time
	require.NoError(t, f.db.NewSelect().TableExpr("active.groups").ColumnExpr("end_time").
		Where("id = ?", f.running).Scan(testpkg.Ctx(t), &sessionEnd))
	require.Nil(t, sessionEnd, "the activity keeps running")
	require.NoError(t, f.db.NewSelect().TableExpr("active.visits").ColumnExpr("exit_time").
		Where("id = ?", f.visitID).Scan(testpkg.Ctx(t), &visitExit))
	require.Nil(t, visitExit, "the child stays in the room")
}

func (f supervisionHandoverFixture) auditActions(t *testing.T) []string {
	t.Helper()
	var actions []string
	require.NoError(t, f.db.NewSelect().TableExpr(`audit.substitution_changes AS "change"`).
		ColumnExpr(`"change".action`).
		Where(`"change".group_id = ?`, f.running).
		Where(`"change".target_type = ?`, TargetAdditionalSupervision).
		OrderExpr(`"change".id`).Scan(testpkg.Ctx(t), &actions))
	return actions
}

func TestSupervisionHandOverMovesOnlyTheRunningSupervision(t *testing.T) {
	t.Parallel()
	f := newSupervisionHandoverFixture(t)
	ctx, commit := testpkg.WithAfterCommitHooks(testpkg.Ctx(t))

	result, err := f.handOver(t, ctx, substitutionCaller(t, f.ownerAccount, false), f.targetStaff)
	require.NoError(t, err)
	require.Equal(t, TargetAdditionalSupervision, result.Type)
	require.Equal(t, f.running, result.ActiveGroupID)
	require.Equal(t, f.targetStaff, result.Target.ID)
	require.Empty(t, f.broadcaster.Events(), "signals wait for the commit")
	commit()

	require.Equal(t, []int64{f.targetStaff}, f.currentSupervisors(t))
	require.Equal(t, "supervisor", testpkg.GroupSupervisorRowByID(t, f.db, result.ID).Role,
		"the target takes over the role the owner held")
	owner := testpkg.GroupSupervisorRowByID(t, f.db, f.ownerRow)
	require.NotNil(t, owner.EndDate, "the hand-over ends the owner's supervision instead of deleting it")
	f.requireSessionUntouched(t)
	require.Equal(t, []string{"assigned", "ended"}, f.auditActions(t))

	events := f.broadcaster.Events()
	require.Len(t, events, 2)
	types := []string{string(events[0].Type), string(events[1].Type)}
	require.ElementsMatch(t, []string{"active_supervision_changed", "group_access_changed"}, types)
	for _, event := range events {
		if string(event.Type) == "active_supervision_changed" {
			require.NotNil(t, event.Data.Reason)
			require.Equal(t, "supervision_handed_over", *event.Data.Reason)
		}
	}
}

func TestSupervisionReleaseNeedsAnotherSupervisor(t *testing.T) {
	t.Parallel()
	f := newSupervisionHandoverFixture(t)
	ctx := testpkg.Ctx(t)
	ownerCaller := substitutionCaller(t, f.ownerAccount, false)

	err := f.release(t, ctx, ownerCaller)
	require.ErrorIs(t, err, schoolStructureCompose.ErrConflict)
	operation, ok := errors.AsType[*schoolStructureCompose.OperationError](err)
	require.True(t, ok, "the refusal carries its own code: %T", err)
	require.Equal(t, "substitutions.last_supervisor", operation.Code)
	require.Equal(t, []int64{f.ownerStaff}, f.currentSupervisors(t))
	require.Empty(t, f.auditActions(t))

	testpkg.CreateTestGroupSupervisor(t, f.db, f.targetStaff, f.running, "additional_supervisor")
	require.NoError(t, f.release(t, ctx, ownerCaller))
	require.Equal(t, []int64{f.targetStaff}, f.currentSupervisors(t))
	f.requireSessionUntouched(t)
	require.Equal(t, []string{"ended"}, f.auditActions(t))

	// The remaining supervisor is now the last one.
	err = f.release(t, ctx, substitutionCaller(t, f.targetAcct, false))
	require.ErrorIs(t, err, schoolStructureCompose.ErrConflict)
	require.Equal(t, []int64{f.targetStaff}, f.currentSupervisors(t))
}

func TestSupervisionHandOverAndReleaseNeedOwnSupervision(t *testing.T) {
	t.Parallel()
	f := newSupervisionHandoverFixture(t)
	ctx := testpkg.Ctx(t)
	testpkg.CreateTestGroupSupervisor(t, f.db, f.targetStaff, f.running, "additional_supervisor")
	viewer := substitutionCaller(t, f.viewerAcct, false)
	admin := substitutionCaller(t, testpkg.CreateTestAccount(t, f.db, "handover-admin").ID, true)

	for name, caller := range map[string]SubstitutionCaller{"colleague": viewer, "admin without supervision": admin} {
		t.Run(name, func(t *testing.T) {
			_, err := f.handOver(t, ctx, caller, f.ownerStaff)
			require.ErrorIs(t, err, ErrForbidden)
			require.ErrorIs(t, f.release(t, ctx, caller), ErrForbidden)
		})
	}
	require.ElementsMatch(t, []int64{f.ownerStaff, f.targetStaff}, f.currentSupervisors(t))

	ownerCaller := substitutionCaller(t, f.ownerAccount, false)
	_, err := f.handOver(t, ctx, ownerCaller, f.ownerStaff)
	require.ErrorIs(t, err, ErrSelfAssignment)
	// Only someone „Betreuer hinzufügen“ could add may take over.
	unverified := testpkg.CreateTestStaff(t, f.db, "Unverified", "Staff")
	_, err = f.handOver(t, ctx, ownerCaller, unverified.ID)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.handOver(t, ctx, ownerCaller, f.targetStaff)
	require.ErrorIs(t, err, ErrAlreadyAssigned)

	_, err = f.db.NewUpdate().TableExpr("active.groups").Set("end_time = ?", time.Now()).
		Where("id = ?", f.running).Exec(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, f.release(t, ctx, ownerCaller), ErrNotRunning)
	_, err = f.handOver(t, ctx, ownerCaller, f.targetStaff)
	require.ErrorIs(t, err, ErrNotRunning)
	require.Empty(t, f.auditActions(t))
}

func TestSupervisionReleaseWithoutPresenceBindingIsRefused(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	module := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		ActiveGroups: reposPresence.ActiveGroup, ActiveSupervisors: reposPresence.GroupSupervisor,
		ActiveSupervisorCreator: testpkg.GroupSupervisorCreator{Repository: reposPresence.GroupSupervisor},
	})
	_, accountID := activeTeacher(t, db, "Nora", "Unbound")
	err := testpkg.TenantCommand2(t, testpkg.Ctx(t), db, module.End, substitutionCaller(t, accountID, false),
		EndRequest{Type: TargetAdditionalSupervision, ID: 1})
	require.ErrorIs(t, err, ErrInvalidTarget)
}

func TestSupervisionReleaseSerializesConcurrentReleases(t *testing.T) {
	t.Parallel()
	f := newSupervisionHandoverFixture(t)
	testpkg.CreateTestGroupSupervisor(t, f.db, f.targetStaff, f.running, "additional_supervisor")

	holder, err := f.db.BeginTx(testpkg.Ctx(t), nil)
	require.NoError(t, err)
	var lockedID int64
	require.NoError(t, holder.NewSelect().TableExpr(`active.groups AS "group"`).
		ColumnExpr(`"group".id`).Where(`"group".id = ?`, f.running).
		For("UPDATE").Scan(testpkg.Ctx(t), &lockedID))

	results := make(chan error, 2)
	for _, accountID := range []int64{f.ownerAccount, f.targetAcct} {
		caller := substitutionCaller(t, accountID, false)
		go func() { results <- f.release(t, testpkg.Ctx(t), caller) }()
	}
	select {
	case err := <-results:
		t.Fatalf("a release returned while the session row was locked: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	require.NoError(t, holder.Commit())

	var released, refused int
	for range 2 {
		select {
		case err := <-results:
			if err == nil {
				released++
				continue
			}
			require.ErrorIs(t, err, schoolStructureCompose.ErrConflict)
			refused++
		case <-time.After(5 * time.Second):
			t.Fatal("a release did not resume after the lock was released")
		}
	}
	require.Equal(t, 1, released, "exactly one supervisor may leave")
	require.Equal(t, 1, refused, "the other stays as the last supervisor")
	require.Len(t, f.currentSupervisors(t), 1)
}
