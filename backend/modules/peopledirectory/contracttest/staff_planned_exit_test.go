package contracttest_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A planned exit (#4809) runs the production offboarding owners with the real
// Identity & Access step: blocked at the end of the last working day, sign-in
// at this school closes anyway while the other school keeps the account; the
// hourly retry completes the offboarding once the supervision ended.
func TestPlannedExit_BlockedClosesThisSchoolThenCompletes(t *testing.T) {
	t.Parallel()
	sc := newOffboardingScenario(t, testpkg.SetupTestDB(t))

	credential := offboardingCredential("Planned", "Exit1")
	emailAddr := fmt.Sprintf("planned-exit-%d@test.local", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, sc.db, emailAddr, credential)
	person := testpkg.CreateTestPersonWithAccountID(t, sc.db, "Planned", "Exit", account.ID)
	staff := testpkg.CreateTestStaffForPerson(t, sc.db, person.ID)
	planner := testpkg.CreateTestStaff(t, sc.db, "Planning", "Lead")
	plannerAccount := testpkg.CreateTestAccount(t, sc.db, fmt.Sprintf("planned-exit-lead-%d@test.local", time.Now().UnixNano()))
	testpkg.MapAccountToTenant(t, sc.db, account.ID, testpkg.Tenant(t))
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, sc.db, otherTenant)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, otherTenant)

	activityGroup := testpkg.CreateTestActivityGroup(t, sc.db, "PlannedExitSupervision")
	room := testpkg.CreateTestRoom(t, sc.db, fmt.Sprintf("PlannedExitRoom-%d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroup(t, sc.db, activityGroup.ID, room.ID)
	supervisor := testpkg.CreateTestGroupSupervisor(t, sc.db, staff.ID, activeGroup.ID, "supervisor")

	// A day long past: the exit is due on every run, whatever the clock. The
	// suite may not name School Membership, so the plan goes in by SQL.
	_, err := sc.db.NewRaw(`UPDATE users.staff_school_memberships SET planned_exit_day = ?, planned_exit_by_account_id = ?,
		planned_exit_by_staff_id = ?, planned_exit_by_name = 'Leitung', planned_exit_at = NOW() WHERE tenant_id = ? AND id = ?`,
		calendar.NewDate(2000, 1, 3), plannerAccount.ID, planner.ID, testpkg.Tenant(t), staff.ID).Exec(context.Background())
	require.NoError(t, err)

	var notices []staffoffboarding.ExitNotice
	workflow, err := api.NewStaffOffboardingWorkflow(sc.deps)
	require.NoError(t, err)
	run, err := staffoffboarding.NewPlanner(workflow, staffoffboarding.PlannerDependencies{
		AppendAudit: func(ctx context.Context, actor staffoffboarding.Actor, change staffoffboarding.ExitPlanChange) error {
			return sc.deps.Audit.Append(ctx, api.StaffExitPlanRecord(actor, change))
		},
		Notify: func(_ context.Context, notice staffoffboarding.ExitNotice) (bool, error) {
			notices = append(notices, notice)
			return true, nil
		},
		Now: time.Now,
	})
	require.NoError(t, err)
	runDue := func() {
		t.Helper()
		_, err := testpkg.TenantQuery0(t, sc.ctx, sc.db, func(ctx context.Context) (struct{}, error) {
			return struct{}{}, run.RunDue(ctx)
		})
		require.NoError(t, err)
	}

	runDue()
	assert.Nil(t, membershipDeletedAt(t, sc, staff.ID), "the supervision blocks the offboarding")
	require.Len(t, notices, 1)
	assert.Equal(t, staffoffboarding.NoticeBlocked, notices[0].Kind)
	active, err := sc.authSvc.VerifyAccountTenantMembership(context.Background(), account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	assert.False(t, active, "sign-in at this school closes after the last working day although the offboarding is blocked")
	stillOther, err := testpkg.ActiveAccountTenantExists(sc.ctx, sc.db, account.ID, otherTenant)
	require.NoError(t, err)
	assert.True(t, stillOther, "the other school keeps the account")
	_, _, err = sc.authSvc.LoginWithAudit(context.Background(), emailAddr, credential, "", "", "")
	require.NoError(t, err, "login at the other school keeps working")

	// The supervision ended long ago; the fixture started it today.
	_, err = sc.db.NewRaw(`UPDATE active.group_supervisors SET start_date = ?, end_date = ? WHERE tenant_id = ? AND id = ?`,
		calendar.NewDate(2000, 1, 1), calendar.NewDate(2000, 1, 2), testpkg.Tenant(t), supervisor.ID).Exec(context.Background())
	require.NoError(t, err)
	runDue()
	assert.Nil(t, membershipDeletedAt(t, sc, staff.ID), "the retry waits for the hour")

	_, err = sc.db.NewRaw(`UPDATE users.staff_school_memberships SET exit_next_attempt_at = NOW() - INTERVAL '1 minute' WHERE tenant_id = ? AND id = ?`,
		testpkg.Tenant(t), staff.ID).Exec(context.Background())
	require.NoError(t, err)
	runDue()
	assert.NotNil(t, membershipDeletedAt(t, sc, staff.ID), "the hourly retry completes the offboarding")
	assert.Len(t, notices, 1, "the Leitung was told once")

	var reason, deletedBy string
	require.NoError(t, sc.db.NewSelect().TableExpr(`audit.data_deletions`).ColumnExpr(`deletion_reason, deleted_by`).
		Where(`tenant_id = ? AND staff_id = ?`, testpkg.Tenant(t), staff.ID).Scan(context.Background(), &reason, &deletedBy))
	assert.Equal(t, "planned staff exit after last working day 2000-01-03", reason)
	assert.Equal(t, "Leitung", deletedBy, "the offboarding acts in the planner's name")
	remaining, err := sc.db.NewSelect().TableExpr(`users.staff_school_memberships`).Where(`tenant_id = ? AND id = ? AND planned_exit_day IS NOT NULL`, testpkg.Tenant(t), staff.ID).Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, remaining, "the retirement takes the planned exit along")
}

// Planning, moving and withdrawing land in the staff member's
// Änderungshistorie through the production audit binding.
func TestPlannedExit_PlanningIsAudited(t *testing.T) {
	t.Parallel()
	sc := newOffboardingScenario(t, testpkg.SetupTestDB(t))
	staff := testpkg.CreateTestStaff(t, sc.db, "Audited", "Exit")
	actor := testpkg.CreateTestStaff(t, sc.db, "Audit", "Lead")
	workflow, err := api.NewStaffOffboardingWorkflow(sc.deps)
	require.NoError(t, err)
	planner, err := staffoffboarding.NewPlanner(workflow, staffoffboarding.PlannerDependencies{
		AppendAudit: func(ctx context.Context, actor staffoffboarding.Actor, change staffoffboarding.ExitPlanChange) error {
			return sc.deps.Audit.Append(ctx, api.StaffExitPlanRecord(actor, change))
		},
		Now: time.Now,
	})
	require.NoError(t, err)
	ctx := context.WithValue(sc.ctx, offboardingTestActorKey{}, staffoffboarding.Actor{StaffID: actor.ID, Username: "Leitung"})
	first, second := "2099-01-29", "2099-02-26"
	_, err = planner.Plan(ctx, staff.ID, first)
	require.NoError(t, err)
	_, err = planner.Plan(ctx, staff.ID, second)
	require.NoError(t, err)
	require.NoError(t, planner.Withdraw(ctx, staff.ID))

	type change struct {
		OldValue string `bun:"old_value"`
		NewValue string `bun:"new_value"`
		Note     string `bun:"note"`
		By       int64  `bun:"changed_by"`
	}
	var changes []change
	require.NoError(t, sc.db.NewSelect().TableExpr(`audit.staff_master_data_changes`).
		ColumnExpr(`old_value, new_value, note, changed_by`).
		Where(`tenant_id = ? AND staff_id = ? AND section = 'austritt'`, testpkg.Tenant(t), staff.ID).
		OrderExpr(`id ASC`).Scan(context.Background(), &changes))
	assert.Equal(t, []change{
		{NewValue: first, Note: staffoffboarding.ExitPlanned, By: actor.ID},
		{OldValue: first, NewValue: second, Note: staffoffboarding.ExitMoved, By: actor.ID},
		{OldValue: second, Note: staffoffboarding.ExitWithdrawn, By: actor.ID},
	}, changes)
}

func membershipDeletedAt(t *testing.T, sc *offboardingScenario, staffID int64) *time.Time {
	t.Helper()
	var deletedAt *time.Time
	require.NoError(t, sc.db.NewSelect().TableExpr(`users.staff_school_memberships`).ColumnExpr(`deleted_at`).
		Where(`id = ?`, staffID).Scan(context.Background(), &deletedAt))
	return deletedAt
}
