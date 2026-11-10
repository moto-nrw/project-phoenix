package enrollmenthttp_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// followingPhases is the phase administration as the root binds it for a
// phase-start edit (#6505): the real recurrence gate, School Membership for
// the pending students and a fixed school day.
func followingPhases(t *testing.T, env *decisionTestEnv, today calendar.Date) enrollmentNative.PhaseAdministration {
	t.Helper()
	pendingWindows, err := owners.ComposePendingWindows(testpkg.NewDatabase(t, env.db))
	require.NoError(t, err)
	return NewTestPhases(TestPhaseDependencies{
		Records:                env.repos.Enrollment(),
		Offerings:              phaseOfferings{rows: newCareOfferingFixtures(env.reposCare.CarePlan)},
		LockTemplateRecurrence: owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, env.db)).LockRecurrenceWrites,
		Resyncer:               &recordingSourcedTemplateResyncer{},
		Students:               owners.NewEnrollmentPhaseStudents(pendingWindows),
		Logger:                 slog.Default(),
		Today:                  func() calendar.Date { return today },
	})
}

// scheduledNewChild approves a new child for the source phase, which starts
// on 2026-09-01, after the decision day: the child stays pending until then.
type scheduledNewChild struct {
	env       *decisionTestEnv
	childID   int64
	studentID int64
	start     calendar.Date
}

func approveScheduledNewChild(t *testing.T, env *decisionTestEnv, lastName string) *scheduledNewChild {
	t.Helper()
	requestID, childID := submitOneChild(t, env, fmt.Sprintf("start-%s@example.com", lastName), "Neu", lastName)
	outcome, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), env.db, env.decision.Decide, enrollmentAPI.DecideInput{
		RequestID: requestID, ChildID: childID, Status: enrollmentNative.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	c := &scheduledNewChild{env: env, childID: childID, studentID: *outcome.Child.CreatedStudentID, start: calendar.Date(env.sourcePhase.ServiceStartDate)}
	c.assertPlan(t, peopleValues.StudentStatusPending, c.start, "after the approval")
	return c
}

func (c *scheduledNewChild) student(t *testing.T) *peopleValues.StudentState {
	t.Helper()
	student, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), c.env.db, c.env.repos.Student.FindByID, any(c.studentID))
	require.NoError(t, err)
	return student
}

// assertPlan checks the status, the shown start and the activation plan.
func (c *scheduledNewChild) assertPlan(t *testing.T, status string, start calendar.Date, when string) {
	t.Helper()
	student := c.student(t)
	assert.Equal(t, status, student.Status, "%s: status", when)
	require.NotNil(t, student.EnrolledFrom, "%s: start", when)
	assert.Equal(t, start, *student.EnrolledFrom, "%s: the child's start", when)
	child, err := enrollmentAPI.ReadOwnerChildForTest(testpkg.Ctx(t), c.env.repos.Enrollment(), c.childID)
	require.NoError(t, err)
	require.NotNil(t, child.ActivateOn, "%s: activation plan", when)
	assert.Equal(t, start, *child.ActivateOn, "%s: the activation plan", when)
}

func (c *scheduledNewChild) movedPhase(days int) *enrollmentNative.Phase {
	phase := enrollmentAPI.OwnerPhaseForTest(c.env.sourcePhase)
	phase.ServiceStartDate = enrollmentNative.Date(c.start.AddDays(days))
	return phase
}

// A confirmed new child whose care starts after today follows a moved phase
// start in both directions: its shown start, its activation plan and the day
// the activation tick picks it up stay the phase start (#6505, EC-4). The
// move rolls back with the edit. A child already activated, or whose start
// has come, keeps its start.
func TestPhaseService_Update_MovesPendingActivation(t *testing.T) {
	t.Parallel()

	for _, days := range []int{-7, 7} {
		t.Run(fmt.Sprintf("shift_%d_days", days), func(t *testing.T) {
			testpkg.OwnTenant(t)
			env := setupDecisionTest(t)
			ctx := testpkg.Ctx(t)
			c := approveScheduledNewChild(t, env, "Verschoben")
			phases := followingPhases(t, env, decisionTestToday)
			moved := c.movedPhase(days)
			newStart := c.start.AddDays(days)

			failure := errors.New("after phase update")
			err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
				if err := phases.UpdatePhase(txCtx, moved); err != nil {
					return err
				}
				return failure
			})
			require.ErrorIs(t, err, failure)
			c.assertPlan(t, peopleValues.StudentStatusPending, c.start, "after a rolled back edit")

			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, phases.UpdatePhase, moved))
			c.assertPlan(t, peopleValues.StudentStatusPending, newStart, "after the edit")
			due, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Student.FindPendingDueForActivation, newStart.AddDays(-1))
			require.NoError(t, err)
			assert.NotContains(t, studentIDs(due), c.studentID, "the tick does not activate before the new start")
			due, err = testpkg.TenantQuery1(t, ctx, env.db, env.repos.Student.FindPendingDueForActivation, newStart)
			require.NoError(t, err)
			assert.Contains(t, studentIDs(due), c.studentID, "the tick activates on the new start")

			// The start has come: the activation counts as executed.
			again := enrollmentAPI.OwnerPhaseForTest(moved)
			again.ServiceStartDate = enrollmentNative.Date(newStart.AddDays(3))
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, followingPhases(t, env, newStart).UpdatePhase, again))
			c.assertPlan(t, peopleValues.StudentStatusPending, newStart, "after an edit on the start day")
		})
	}
}

func TestPhaseService_Update_KeepsActivatedChildStart(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	c := approveScheduledNewChild(t, env, "Aktiviert")
	student := c.student(t)
	student.Status = peopleValues.StudentStatusActive
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Student.Update, student))

	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, followingPhases(t, env, decisionTestToday).UpdatePhase, c.movedPhase(7)))
	c.assertPlan(t, peopleValues.StudentStatusActive, c.start, "an activated child")
}

func TestPhaseService_Update_KeepsWindowFromSupersedingApproval(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	env.sourcePhase.Kind = enrollmentNative.PhaseKindHoliday
	env.sourcePhase.ServiceStartDate = "2100-07-20"
	env.sourcePhase.ServiceEndDate = "2100-07-31"
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().UpdatePhase, enrollmentAPI.OwnerPhaseForTest(env.sourcePhase)))
	holiday := approveScheduledNewChild(t, env, "Ersetzt")
	oldHoliday := enrollmentAPI.OwnerPhaseForTest(env.sourcePhase)

	year := *env.sourcePhase
	year.ID = 0
	year.Name = "superseding-school-year"
	year.Kind = enrollmentNative.PhaseKindSchoolYear
	year.ServiceStartDate = "2100-08-01"
	year.ServiceEndDate = "2101-07-31"
	require.NoError(t, enrollmentAPI.InsertOwnerPhaseForTest(ctx, env.repos.Enrollment(), &year))
	env.sourcePhase = &year
	requestID, childID := submitOneChild(t, env, "superseding@example.com", "Neu", "Ersetzt")
	matchChildToExistingStudent(t, env, childID, holiday.studentID)
	_, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.Decide, enrollmentAPI.DecideInput{
		RequestID: requestID, ChildID: childID, Status: enrollmentNative.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	current := &scheduledNewChild{env: env, childID: childID, studentID: holiday.studentID, start: calendar.Date(year.ServiceStartDate)}
	current.assertPlan(t, peopleValues.StudentStatusPending, current.start, "after the school-year approval")

	// Editing the superseded holiday must not restore its earlier start,
	// whether its new start is later or earlier than the old one.
	phases := followingPhases(t, env, decisionTestToday)
	for _, start := range []enrollmentNative.Date{"2100-07-21", "2100-07-15"} {
		moved := *oldHoliday
		moved.ServiceStartDate = start
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, phases.UpdatePhase, &moved))
		current.assertPlan(t, peopleValues.StudentStatusPending, current.start, "after editing the superseded holiday")
		assert.Equal(t, calendar.Date(year.ServiceEndDate), *current.student(t).EnrolledUntil)
		due, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Student.FindPendingDueForActivation, current.start.AddDays(-1))
		require.NoError(t, err)
		assert.NotContains(t, studentIDs(due), current.studentID, "the superseded phase cannot activate the child early")
	}
}

func studentIDs(students []*peopleValues.StudentState) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.ID)
	}
	return ids
}

// whileHolding runs other on a second connection while hold's transaction is
// still open. Once other waits on one of hold's locks, or has finished, hold
// runs then (when given) and commits. It returns both results.
func whileHolding(
	t *testing.T,
	env *decisionTestEnv,
	hold func(context.Context) error,
	other func(context.Context) error,
	then func(context.Context) error,
) (holdErr, otherErr error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()
	done := make(chan error, 1)
	finishedFirst := false
	holdErr = testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(holdCtx context.Context, tx bun.Tx) error {
		if err := hold(holdCtx); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(holdCtx, &pid))
		go func() {
			done <- testpkg.WithinTenantContext(t, ctx, env.db, testpkg.Tenant(t), other)
		}()
		if finished := waitForBlockedWriter(t, holdCtx, tx, pid, done); finished != nil {
			otherErr, finishedFirst = *finished, true
		}
		if then != nil {
			return then(holdCtx)
		}
		return nil
	})
	if finishedFirst {
		return holdErr, otherErr
	}
	select {
	case otherErr = <-done:
	case <-ctx.Done():
		t.Fatalf("the concurrent write did not finish; holder returned %v", holdErr)
	}
	return holdErr, otherErr
}

// The hourly activation tick reads the due children before it writes. A
// phase edit that moved a start that had already come could commit in
// between, and the tick then activated a child whose shown start lay in the
// future. The edit counts a start that has come as executed, so the tick and
// the child agree on the start (#6505).
func TestPhaseService_Update_ActivationTickKeepsDueStart(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	c := approveScheduledNewChild(t, env, "Faellig")
	phases := followingPhases(t, env, c.start)

	editErr, tickErr := whileHolding(t, env,
		func(ctx context.Context) error { return phases.UpdatePhase(ctx, c.movedPhase(7)) },
		func(ctx context.Context) error {
			due, err := env.repos.Student.FindPendingDueForActivation(ctx, c.start)
			if err != nil {
				return err
			}
			for _, id := range studentIDs(due) {
				if _, err := env.repos.Student.TransitionStatus(ctx, id, peopleValues.StudentStatusPending, peopleValues.StudentStatusActive); err != nil {
					return err
				}
			}
			return nil
		}, nil)
	require.NoError(t, editErr)
	require.NoError(t, tickErr)
	c.assertPlan(t, peopleValues.StudentStatusActive, c.start, "after the tick and the edit")
}

// School Membership locks each pending student before it reads its status
// for a phase edit. A status change committed while the edit waits is kept:
// the edit neither moves the activated child's start nor writes the stale
// pending status back (#6505).
func TestPhaseService_Update_WaitsForConcurrentActivation(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	c := approveScheduledNewChild(t, env, "Gleichzeitig")
	phases := followingPhases(t, env, decisionTestToday)

	activateErr, editErr := whileHolding(t, env,
		func(ctx context.Context) error {
			student, err := env.repos.Student.FindByID(ctx, c.studentID)
			if err != nil {
				return err
			}
			student.Status = peopleValues.StudentStatusActive
			return env.repos.Student.Update(ctx, student)
		},
		func(ctx context.Context) error { return phases.UpdatePhase(ctx, c.movedPhase(7)) }, nil)
	require.NoError(t, activateErr)
	require.NoError(t, editErr)
	c.assertPlan(t, peopleValues.StudentStatusActive, c.start, "after the activation and the edit")
}

// classSwitchRenewal is a future renewal of a running 2a child into grade 3:
// the approval plans the class switch for the phase start (#3917).
func classSwitchRenewal(t *testing.T, r *futureRenewal) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	grade := int16(3)
	child.TargetGradeLevel = &grade
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), child))
	r.approve(t)
	r.assertClassSwitch(t, r.start, "2a", "after the approval")
}

func (r *futureRenewal) assertClassSwitch(t *testing.T, on calendar.Date, class, when string) {
	t.Helper()
	planned, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), r.env.db, r.env.repos.Enrollment().ChildClassSwitch, r.childID)
	require.NoError(t, err)
	require.NotNil(t, planned, "%s: a class switch is planned", when)
	assert.Equal(t, enrollmentNative.Date(on), planned.On, "%s: the switch day", when)
	assert.Equal(t, "3", planned.To, "%s: the target class", when)
	assert.Equal(t, class, r.student(t).SchoolClass, "%s: the running class", when)
}

func (r *futureRenewal) applyDueClassSwitches(t *testing.T, asOf calendar.Date) int {
	t.Helper()
	applied, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), r.env.db, decisionOwner(t, r.env.decision).ApplyDueClassSwitches, asOf)
	require.NoError(t, err)
	return applied
}

// A planned class switch of a running renewal follows a moved phase start
// with the weekly plan, so the new class and the new weekly plan start on
// the same day in both directions (#6505, EC-5).
func TestPhaseService_Update_ReschedulesPendingClassSwitch(t *testing.T) {
	t.Parallel()

	for _, days := range []int{-7, 7} {
		t.Run(fmt.Sprintf("shift_%d_days", days), func(t *testing.T) {
			testpkg.OwnTenant(t)
			r := setupFutureRenewal(t)
			classSwitchRenewal(t, r)
			phases, _, _, _, _ := setupPhaseTest(t)
			phase := enrollmentAPI.OwnerPhaseForTest(r.env.sourcePhase)
			phase.ServiceStartDate = enrollmentNative.Date(r.start.AddDays(days))
			newStart := r.start.AddDays(days)

			require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), r.env.db, phases.UpdatePhase, phase))
			r.assertClassSwitch(t, newStart, "2a", "after the edit")
			weekly, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), r.env.db, r.env.repos.Enrollment().ChildWeeklyPlan, r.childID)
			require.NoError(t, err)
			require.NotNil(t, weekly)
			assert.Equal(t, enrollmentNative.Date(newStart), weekly.On, "class switch and weekly plan share the day")

			assert.Zero(t, r.applyDueClassSwitches(t, newStart.AddDays(-1)), "nothing switches before the new start")
			assert.Equal(t, "2a", r.student(t).SchoolClass)
			assert.Equal(t, 1, r.applyDueClassSwitches(t, newStart))
			assert.Equal(t, "3", r.student(t).SchoolClass, "the class switches on the new start")

			phase.ServiceStartDate = enrollmentNative.Date(newStart.AddDays(1))
			require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), r.env.db, phases.UpdatePhase, phase))
			planned, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), r.env.db, r.env.repos.Enrollment().ChildClassSwitch, r.childID)
			require.NoError(t, err)
			assert.Nil(t, planned, "a phase change never recreates an applied switch")
		})
	}
}

// The class-switch tick takes the approval gates the phase edit holds. A tick
// that waited for an edit moving the start later finds the switch no longer
// due on the old start (#6505).
func TestPhaseService_Update_ClassSwitchTickWaitsForPhaseStartEdit(t *testing.T) {
	t.Parallel()

	r, phases := gatedFutureRenewal(t)
	classSwitchRenewal(t, r)
	editErr, tickErr := whilePhaseStartMoves(t, r, phases, 7, false, func(ctx context.Context) error {
		_, err := decisionOwner(t, r.env.decision).ApplyDueClassSwitches(ctx, r.start)
		return err
	})
	require.NoError(t, editErr)
	require.NoError(t, tickErr)
	r.assertClassSwitch(t, r.start.AddDays(7), "2a", "after the tick on the old start")
}

// A phase edit that moves pending students holds the recurrence gate and
// never waits for the class-writes gate afterwards (ADR 0162). A grade
// transition holds the class gate exclusively and takes the recurrence gate
// next; an edit whose student move waited for the class gate closed a
// deadlock with it (#6505).
func TestPhaseService_Update_NeverWaitsForClassGateAfterRecurrenceGate(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	c := approveScheduledNewChild(t, env, "Jahrgang")
	phases := followingPhases(t, env, decisionTestToday)
	database := testpkg.NewDatabase(t, env.db)
	membership, err := owners.NewSchoolMembership(database)
	require.NoError(t, err)
	recurrence := owners.MustNewTimetableRecurrenceLock(database).LockRecurrenceWrites

	gradeErr, editErr := whileHolding(t, env,
		func(ctx context.Context) error { return membership.LockStudentClassWrites(ctx, true) },
		func(ctx context.Context) error { return phases.UpdatePhase(ctx, c.movedPhase(7)) },
		recurrence)
	require.NoError(t, gradeErr, "the grade transition must not deadlock")
	require.NoError(t, editErr, "the edit runs after the grade transition")
	c.assertPlan(t, peopleValues.StudentStatusPending, c.start.AddDays(7), "after the grade transition and the edit")
}
