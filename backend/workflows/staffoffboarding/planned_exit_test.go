package staffoffboarding_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	membershipcompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/stretchr/testify/require"
)

// plannerFixture runs the planned exit of #4809 on the offboarding fixture
// with a settable day, clock and supervision state, recording the audit
// changes, the notices and the access steps.
type plannerFixture struct {
	fixture
	today      string
	now        time.Time
	blocked    bool
	notify     bool
	changes    []staffoffboarding.ExitPlanChange
	notices    []staffoffboarding.ExitNotice
	accessRuns int
	deletion   []staffoffboarding.Actor
	// beforeRunPlan changes a listed plan immediately before DueExits locks it.
	beforeRunPlan func(context.Context) error
	// failDeletion makes the offboarding's audit append fail, which rolls
	// the offboarding back like any owner failure.
	failDeletion bool
	exits        *schoolmembership.PlannedExits
}

// testPlanner plans through the workflow's Planner and runs the due exits
// the way the scheduler does: inside one school transaction, every planned
// exit in a savepoint. lastRun keeps the failures the run reported.
type testPlanner struct {
	*staffoffboarding.Planner
	t       *testing.T
	due     *staffoffboarding.DueExits
	lastRun staffoffboarding.DueRun
}

func datePtr(value string) *calendar.Date {
	date, err := calendar.ParseDate(value)
	if err != nil {
		panic(err)
	}
	return &date
}

func plannedExitDate(value string) calendar.Date { return *datePtr(value) }

func (p *testPlanner) RunDue(ctx context.Context) error {
	db := testpkg.SetupTestDB(p.t)
	return testpkg.WithinTenantContext(p.t, ctx, db, tenant.FromContext(ctx), func(txCtx context.Context) (err error) {
		p.lastRun, err = p.due.RunDue(txCtx)
		return err
	})
}

func isolate(ctx context.Context, fn func(context.Context) error) (failed, err error) {
	failed = tenant.WithSavepoint(ctx, fn)
	if errors.Is(failed, tenant.ErrSavepointControl) {
		return nil, failed
	}
	return failed, nil
}

func newPlannerFixture(t *testing.T) (*plannerFixture, *testPlanner) {
	t.Helper()
	f := &plannerFixture{fixture: newFixture(t), today: "2027-10-01", notify: true,
		now: calendar.NewDate(2027, 10, 1).BerlinMidnight().Add(9 * time.Hour)}
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "planned-exit-subject@example.org")
	f.linkAccount(t, account.ID)
	retirement, err := membershipcompose.NewOffboarding(membershipcompose.Dependencies{DB: db, Observe: func(membershipcompose.Observation) {}, Employment: staffEmployment(t, db)})
	require.NoError(t, err)
	f.exits = retirement.PlannedExits
	f.deps.Today = func() string { return f.today }
	lock := f.deps.LockSupervision
	f.deps.LockSupervision = func(ctx context.Context, staffID int64, day string) ([]int64, error) {
		if f.blocked {
			return []int64{1}, nil
		}
		return lock(ctx, staffID, day)
	}
	f.deps.PreviewAccess = func(context.Context, int64) (staffoffboarding.AccessPreview, error) {
		return staffoffboarding.AccessPreview{Revision: "access"}, nil
	}
	f.deps.ExecuteAccess = func(_ context.Context, id int64, revision string) (staffoffboarding.AccessResult, error) {
		require.Equal(t, account.ID, id)
		require.Equal(t, "access", revision)
		f.accessRuns++
		return staffoffboarding.AccessResult{TokensRevoked: 1}, nil
	}
	f.deps.AppendAudit = func(_ context.Context, actor staffoffboarding.Actor, _ staffoffboarding.Result) error {
		if f.failDeletion {
			return errors.New("audit refused")
		}
		f.deletion = append(f.deletion, actor)
		return nil
	}
	workflow, err := staffoffboarding.New(f.deps)
	require.NoError(t, err)
	planner, err := staffoffboarding.NewPlanner(workflow, staffoffboarding.PlannerDependencies{
		AppendAudit: func(_ context.Context, _ staffoffboarding.Actor, change staffoffboarding.ExitPlanChange) error {
			f.changes = append(f.changes, change)
			return nil
		},
		Now: func() time.Time { return f.now },
	})
	require.NoError(t, err)
	due, err := staffoffboarding.NewDueExits(planner, staffoffboarding.DueExitDependencies{
		Notify: func(_ context.Context, notice staffoffboarding.ExitNotice) (bool, error) {
			if f.notify {
				f.notices = append(f.notices, notice)
			}
			return f.notify, nil
		},
		Isolate: func(ctx context.Context, fn func(context.Context) error) (failed, err error) {
			if f.beforeRunPlan != nil {
				before := f.beforeRunPlan
				f.beforeRunPlan = nil
				if err := before(ctx); err != nil {
					return err, nil
				}
			}
			return isolate(ctx, fn)
		},
		Now: func() time.Time { return f.now },
	})
	require.NoError(t, err)
	return f, &testPlanner{Planner: planner, t: t, due: due}
}

func (f *plannerFixture) linkAccount(t *testing.T, accountID int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	_, err := db.NewRaw(`UPDATE users.persons SET account_id = ? WHERE tenant_id = ? AND id = ?`, accountID, testpkg.Tenant(t), f.personID).Exec(context.Background())
	require.NoError(t, err)
}

func (f *plannerFixture) retired(t *testing.T) bool {
	t.Helper()
	_, err := f.membership.FindStaff(testpkg.Ctx(t), f.staffID)
	if err == nil {
		return false
	}
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	return true
}

func TestPlannedExitIsPlannedMovedAndWithdrawnWithAudit(t *testing.T) {
	t.Parallel()
	f, planner := newPlannerFixture(t)
	ctx := testpkg.Ctx(t)

	_, err := planner.Plan(ctx, f.staffID, plannedExitDate("2027-09-30"))
	require.ErrorIs(t, err, staffoffboarding.ErrPlannedExitInPast)

	plan, err := planner.Plan(ctx, f.staffID, plannedExitDate("2027-10-31"))
	require.NoError(t, err)
	require.Equal(t, plannedExitDate("2027-10-31"), plan.LastWorkingDay)
	staff, err := f.membership.FindStaff(ctx, f.staffID)
	require.NoError(t, err)
	require.NotNil(t, staff.PlannedExit, "staff reads carry the planned exit")
	require.Equal(t, plannedExitDate("2027-10-31"), staff.PlannedExit.LastWorkingDay)

	_, err = planner.Plan(ctx, f.staffID, plannedExitDate("2027-10-31"))
	require.NoError(t, err, "re-entering the same day is no change")
	_, err = planner.Plan(ctx, f.staffID, plannedExitDate("2027-11-30"))
	require.NoError(t, err)
	require.NoError(t, planner.Withdraw(ctx, f.staffID))
	require.ErrorIs(t, planner.Withdraw(ctx, f.staffID), schoolmembership.ErrPlannedExitNotFound)

	require.Equal(t, []staffoffboarding.ExitPlanChange{
		{StaffID: f.staffID, Action: staffoffboarding.ExitPlanned, NewDay: datePtr("2027-10-31")},
		{StaffID: f.staffID, Action: staffoffboarding.ExitMoved, OldDay: datePtr("2027-10-31"), NewDay: datePtr("2027-11-30")},
		{StaffID: f.staffID, Action: staffoffboarding.ExitWithdrawn, OldDay: datePtr("2027-11-30")},
	}, f.changes)

	f.today = "2027-12-02"
	require.NoError(t, planner.RunDue(ctx))
	require.False(t, f.retired(t), "a withdrawn exit never runs")
	require.Empty(t, f.deletion)
}

func TestPlannedExitRunsAfterItsDayAsThePlanner(t *testing.T) {
	t.Parallel()
	f, planner := newPlannerFixture(t)
	ctx := testpkg.Ctx(t)
	_, err := planner.Plan(ctx, f.staffID, plannedExitDate("2027-10-10"))
	require.NoError(t, err)

	f.today = "2027-10-03"
	require.NoError(t, planner.RunDue(ctx))
	require.NoError(t, planner.RunDue(ctx))
	require.Len(t, f.notices, 1, "the week-before reminder goes once")
	require.Equal(t, staffoffboarding.NoticeWeekBefore, f.notices[0].Kind)

	f.today = "2027-10-10"
	require.NoError(t, planner.RunDue(ctx))
	require.False(t, f.retired(t), "the member works normally through the last working day")
	require.Len(t, f.notices, 2)
	require.Equal(t, staffoffboarding.NoticeExitDay, f.notices[1].Kind)

	f.today = "2027-10-11"
	require.NoError(t, planner.RunDue(ctx))
	require.True(t, f.retired(t))
	require.Equal(t, 1, f.accessRuns, "the access step of the same offboarding ran")
	require.Len(t, f.deletion, 1)
	require.Equal(t, datePtr("2027-10-10"), f.deletion[0].PlannedExitDay)
	require.Equal(t, "actor", f.deletion[0].Username, "the offboarding acts as the planner")
	require.Equal(t, "planned staff exit after last working day 2027-10-10", staffoffboarding.NewStaffDeletion(f.deletion[0], staffoffboarding.Result{}).Reason)
	_, err = f.exits.FindPlannedExitForMutation(ctx, f.staffID)
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound, "the retirement takes the planned exit along")
	_, err = planner.Plan(ctx, f.staffID, plannedExitDate("2027-10-20"))
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
}

func TestPlannedExitInsideTheWeekSkipsTheWeekReminder(t *testing.T) {
	t.Parallel()
	f, planner := newPlannerFixture(t)
	ctx := testpkg.Ctx(t)
	_, err := planner.Plan(ctx, f.staffID, plannedExitDate("2027-10-04"))
	require.NoError(t, err)
	require.NoError(t, planner.RunDue(ctx))
	require.Empty(t, f.notices, "the planner just entered it")
}

func TestBlockedPlannedExitClosesAccessAndRetriesHourly(t *testing.T) {
	t.Parallel()
	f, planner := newPlannerFixture(t)
	ctx := testpkg.Ctx(t)
	_, err := planner.Plan(ctx, f.staffID, plannedExitDate("2027-10-01"))
	require.NoError(t, err)

	f.today, f.blocked = "2027-10-02", true
	f.now = calendar.NewDate(2027, 10, 2).BerlinMidnight().Add(5 * time.Minute)
	require.NoError(t, planner.RunDue(ctx))
	require.False(t, f.retired(t))
	require.Equal(t, 1, f.accessRuns, "sign-in closes at the end of the day although the offboarding is blocked")
	require.Len(t, f.notices, 1)
	require.Equal(t, staffoffboarding.NoticeBlocked, f.notices[0].Kind)
	plan, err := f.exits.FindPlannedExitForMutation(ctx, f.staffID)
	require.NoError(t, err)
	require.NotNil(t, plan.AccessLockedAt)
	require.NotNil(t, plan.NextAttemptAt)
	require.True(t, plan.NextAttemptAt.Equal(f.now.Add(time.Hour)))

	_, err = planner.Plan(ctx, f.staffID, plannedExitDate("2027-10-20"))
	require.ErrorIs(t, err, staffoffboarding.ErrPlannedExitStarted, "a due exit can no longer move")
	require.ErrorIs(t, planner.Withdraw(ctx, f.staffID), staffoffboarding.ErrPlannedExitStarted)

	f.now = f.now.Add(30 * time.Minute)
	f.blocked = false
	require.NoError(t, planner.RunDue(ctx))
	require.False(t, f.retired(t), "the next attempt waits for the hour")

	f.now = f.now.Add(31 * time.Minute)
	f.blocked = true
	require.NoError(t, planner.RunDue(ctx))
	require.Len(t, f.notices, 1, "the Leitung is told once")
	require.Equal(t, 1, f.accessRuns)

	f.now = f.now.Add(time.Hour)
	f.blocked = false
	require.NoError(t, planner.RunDue(ctx))
	require.True(t, f.retired(t))
	require.Equal(t, 2, f.accessRuns, "the completing run repeats the access step on the closed access")
}

func TestPlannedExitReminderWaitsUntilItIsDelivered(t *testing.T) {
	t.Parallel()
	f, planner := newPlannerFixture(t)
	ctx := testpkg.Ctx(t)
	_, err := planner.Plan(ctx, f.staffID, plannedExitDate("2027-10-01"))
	require.NoError(t, err)
	f.notify = false
	require.NoError(t, planner.RunDue(ctx))
	f.notify = true
	require.NoError(t, planner.RunDue(ctx))
	require.NoError(t, planner.RunDue(ctx))
	require.Len(t, f.notices, 1)
	require.Equal(t, staffoffboarding.NoticeExitDay, f.notices[0].Kind)
}

func TestPlannedExitReminderUsesTheLockedCurrentPlan(t *testing.T) {
	t.Parallel()
	f, planner := newPlannerFixture(t)
	ctx := testpkg.Ctx(t)
	plan, err := planner.Plan(ctx, f.staffID, plannedExitDate("2027-10-10"))
	require.NoError(t, err)

	f.beforeRunPlan = func(ctx context.Context) error {
		_, err := f.exits.SavePlannedExit(ctx, schoolmembership.PlanExit{
			StaffID:            plan.StaffID,
			LastWorkingDay:     plannedExitDate("2027-11-10"),
			PlannedByAccountID: plan.PlannedByAccountID,
			PlannedByStaffID:   plan.PlannedByStaffID,
			PlannedByName:      plan.PlannedByName,
		})
		return err
	}
	f.today = "2027-10-03"

	require.NoError(t, planner.RunDue(ctx))
	require.Empty(t, f.notices, "a moved exit must not receive the stale reminder")
	current, err := f.exits.FindPlannedExitForMutation(ctx, f.staffID)
	require.NoError(t, err)
	require.Equal(t, plannedExitDate("2027-11-10"), current.LastWorkingDay)
	require.Nil(t, current.WeekNoticeSentAt)
}

func TestPlannedExitStaysInsideItsSchool(t *testing.T) {
	t.Parallel()
	f, planner := newPlannerFixture(t)
	_, err := planner.Plan(testpkg.Ctx(t), f.staffID, plannedExitDate("2027-10-05"))
	require.NoError(t, err)

	t.Run("other school", func(t *testing.T) {
		other := testpkg.OwnCtx(t)
		_, err := planner.Plan(other, f.staffID, plannedExitDate("2027-10-05"))
		require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
		require.ErrorIs(t, planner.Withdraw(other, f.staffID), schoolmembership.ErrStaffNotFound)
		exits, err := f.exits.ListPlannedExits(other)
		require.NoError(t, err)
		require.Empty(t, exits)
	})
}

// A failing offboarding is reported and rolled back on its own; sign-in is
// closed anyway and the next attempt waits an hour (#4809).
func TestFailingPlannedExitClosesAccessAndIsReported(t *testing.T) {
	t.Parallel()
	f, planner := newPlannerFixture(t)
	ctx := testpkg.Ctx(t)
	_, err := planner.Plan(ctx, f.staffID, plannedExitDate("2027-10-01"))
	require.NoError(t, err)
	f.failDeletion = true

	f.today = "2027-10-02"
	f.now = calendar.NewDate(2027, 10, 2).BerlinMidnight().Add(5 * time.Minute)
	require.NoError(t, planner.RunDue(ctx))
	require.Contains(t, planner.lastRun.Failed, f.staffID)
	require.False(t, f.retired(t), "the failed offboarding rolled back")
	require.Equal(t, 2, f.accessRuns, "the rolled-back offboarding ran the access step, then the lock-out closed sign-in for good")
	require.Empty(t, f.notices, "only a blocked exit has a notice of its own")
	plan, err := f.exits.FindPlannedExitForMutation(ctx, f.staffID)
	require.NoError(t, err)
	require.NotNil(t, plan.AccessLockedAt)
	require.NotNil(t, plan.NextAttemptAt)
	require.True(t, plan.NextAttemptAt.Equal(f.now.Add(time.Hour)))
}
