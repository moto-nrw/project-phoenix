package enrollmenthttp_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

// futureRenewal is an approved re-enrollment of an already cared-for child
// whose new care period starts after the approval day (#6043): Barnstorf
// approves in December for care from 2027-02-03.
type futureRenewal struct {
	env               *decisionTestEnv
	studentID         int64
	requestID         int64
	childID           int64
	reviewerAccountID int64
	reviewerStaffID   int64
	start             calendar.Date
}

// setupFutureRenewal seeds an active child with a running Monday and
// Wednesday pickup at 14:25, care days on Monday and Thursday and a Monday
// Heimweg "alone", and submits a renewal for a phase starting 2027-02-03
// that picks up on Monday at 15:10 only, comes on Monday only and is
// collected on Monday.
func setupFutureRenewal(t *testing.T) *futureRenewal {
	t.Helper()
	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	start := calendar.NewDate(2027, 2, 3)

	schema, err := NewTestFormSchemas(env.repos.Enrollment()).CreateSchema(ctx, "Wiederanmeldung Februar", []enrollmentNative.FormField{
		{Key: "pickup_times", Label: "Abholzeiten", Type: enrollmentNative.FormFieldWeekdaySchedule, AppliesToCh: true,
			Target: enrollmentNative.TargetSchedulePickup, AllowedTimes: []string{"14:25", "15:10", "16:00"}, SortOrder: 0},
		{Key: "arrival_times", Label: "Betreuungstage", Type: enrollmentNative.FormFieldWeekdaySchedule, AppliesToCh: true,
			Target: enrollmentNative.TargetScheduleArrival, SortOrder: 1},
		{Key: "allowed_modes", Label: "Heimweg", Type: enrollmentNative.FormFieldWeekdayMultiMode, AppliesToCh: true,
			Target: enrollmentNative.TargetStudentAllowedDepartureModes, SortOrder: 2},
		{Key: "health", Label: "Gesundheit", Type: enrollmentNative.FormFieldTextarea, AppliesToCh: true,
			Target: enrollmentNative.TargetStudentHealthInfo, SortOrder: 3},
	}, env.creatorID)
	require.NoError(t, err)
	env.sourcePhase.Kind = enrollmentNative.PhaseKindSchoolYear
	env.sourcePhase.ServiceStartDate = enrollmentNative.Date(start)
	env.sourcePhase.ServiceEndDate = enrollmentNative.Date(calendar.NewDate(2027, 7, 31))
	env.sourcePhase.FormSchemaID = &schema.ID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().UpdatePhase, enrollmentAPI.OwnerPhaseForTest(env.sourcePhase)))

	return setupFutureRenewalChild(t, env, "Februar")
}

func setupFutureRenewalChild(t *testing.T, env *decisionTestEnv, lastName string) *futureRenewal {
	t.Helper()
	ctx := testpkg.Ctx(t)
	start := calendar.NewDate(2027, 2, 3)
	existing := testpkg.CreateTestStudent(t, env.db, "Mara", lastName, "2a")
	stored, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), env.repos.Student.FindByID, any(existing.ID))
	require.NoError(t, err)
	stored.Status = peopleValues.StudentStatusActive
	stored.AllowedDepartureModes = peopleValues.AllowedDepartureModes{
		peopleValues.PickupDayMonday: []peopleValues.DepartureMode{peopleValues.DepartureAlone},
	}
	stored.DepartureDays, stored.BusDays, stored.PickupDays = nil, nil, nil
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), env.repos.Student.Update, stored))
	author := testpkg.CreateTestStaff(t, env.db, "Betreuer", "Februar")
	testpkg.CreateTestPickupSchedule(t, env.db, existing.ID, timetable.WeekdayMonday, author.ID, "14:25")
	testpkg.CreateTestPickupSchedule(t, env.db, existing.ID, timetable.WeekdayWednesday, author.ID, "14:25")
	testpkg.CreateTestArrivalSchedule(t, env.db, existing.ID, timetable.WeekdayMonday, author.ID, "")
	testpkg.CreateTestArrivalSchedule(t, env.db, existing.ID, timetable.WeekdayThursday, author.ID, "")

	reviewerStaff, reviewerAccountID := createReviewerStaffWithDistinctAccount(t, env)
	requestID, childID := submitOneChildWithCustomData(t, env, fmt.Sprintf("future-%d@example.com", existing.ID), "Mara", lastName, map[string]any{
		"pickup_times":  map[string]any{"mon": "15:10"},
		"arrival_times": map[string]any{"mon": "07:45"},
		"allowed_modes": map[string]any{"mon": []any{"pickup"}},
		"health":        "Nussallergie",
	})
	matchChildToExistingStudent(t, env, childID, existing.ID)

	return &futureRenewal{
		env: env, studentID: existing.ID, requestID: requestID, childID: childID,
		reviewerAccountID: reviewerAccountID, reviewerStaffID: reviewerStaff.ID, start: start,
	}
}

func (r *futureRenewal) approve(t *testing.T) {
	t.Helper()
	outcome, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), testpkg.SetupTestDB(t), r.env.decision.Decide, enrollmentAPI.DecideInput{
		RequestID: r.requestID, ChildID: r.childID, Status: enrollmentNative.DecisionApproved, ReviewedBy: r.reviewerAccountID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	assert.Equal(t, r.studentID, *outcome.Child.CreatedStudentID, "a renewal links the existing child instead of creating a new one")
}

// applyDue runs the rollover tick's weekly-plan step for asOf.
func (r *futureRenewal) applyDue(t *testing.T, asOf calendar.Date) int {
	t.Helper()
	applied, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), testpkg.SetupTestDB(t), decisionOwner(t, r.env.decision).ApplyDueWeeklyPlans, asOf)
	require.NoError(t, err)
	return applied
}

// pickups maps weekday → "HH:MM" of the stored weekly pickup rows.
func (r *futureRenewal) pickups(t *testing.T) map[int]string {
	t.Helper()
	rows, err := r.env.reposTimetable.StudentPickupSchedule.FindByStudentID(testpkg.Ctx(t), r.studentID)
	require.NoError(t, err)
	out := make(map[int]string, len(rows))
	for _, row := range rows {
		out[row.Weekday] = row.PickupTime.Format("15:04")
	}
	return out
}

// careDays lists the weekdays of the stored weekly arrival rows.
func (r *futureRenewal) careDays(t *testing.T) []int {
	t.Helper()
	rows, err := r.env.reposTimetable.StudentArrivalSchedule.FindByStudentID(testpkg.Ctx(t), r.studentID)
	require.NoError(t, err)
	out := make([]int, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Weekday)
	}
	sort.Ints(out)
	return out
}

func (r *futureRenewal) student(t *testing.T) *peopleValues.StudentState {
	t.Helper()
	student, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), testpkg.SetupTestDB(t), r.env.repos.Student.FindByID, any(r.studentID))
	require.NoError(t, err)
	return student
}

func (r *futureRenewal) assertRunningPlan(t *testing.T, when string) {
	t.Helper()
	assert.Equal(t, map[int]string{timetable.WeekdayMonday: "14:25", timetable.WeekdayWednesday: "14:25"}, r.pickups(t),
		"%s: the running pickup times stay valid", when)
	assert.Equal(t, []int{timetable.WeekdayMonday, timetable.WeekdayThursday}, r.careDays(t),
		"%s: the running care days stay valid", when)
	assert.Equal(t, peopleValues.DepartureAlone,
		r.student(t).AllowedDepartureModes.DepartureDays().ModeFor(peopleValues.PickupDayMonday),
		"%s: the running Heimweg stays valid", when)
}

// Approving a renewal whose care starts in the future keeps the running
// weekly pickup times, care days and Heimweg until the day before the start
// and applies the approved plan from the start day (#6043). Fields without a
// period, such as health information, apply at approval as before. A
// confirmed edit before the start only changes what the start applies.
func TestDecisionService_Decide_FutureRenewalAppliesWeeklyPlanAtStart(t *testing.T) {
	t.Parallel()

	r := setupFutureRenewal(t)
	r.approve(t)

	r.assertRunningPlan(t, "after the approval")
	student := r.student(t)
	require.NotNil(t, student.HealthInfo)
	assert.Equal(t, "Nussallergie", *student.HealthInfo, "fields without a period apply at approval")

	// The office corrects the approved Monday pickup before the start.
	ctx := testpkg.Ctx(t)
	child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	require.NotNil(t, child.WeeklyPlanOn, "the request child names the day its weekly plan applies")
	assert.Equal(t, r.start, *child.WeeklyPlanOn)
	child.CustomData.Set("pickup_times", map[string]any{"mon": "16:00"})
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), child))
	_, err = changeRequestApplierForTest(t, r.env).SyncApprovedChildData(ctx, enrollmentAPI.SyncApprovedChildDataInput{
		RequestID: r.requestID, ChildID: r.childID, ActorAccountID: r.reviewerAccountID, ReplaceTargetedData: true,
	})
	require.NoError(t, err)
	r.assertRunningPlan(t, "after an edit before the start")

	assert.Zero(t, r.applyDue(t, r.start.AddDays(-1)), "nothing applies before the start")
	r.assertRunningPlan(t, "on 2027-02-02")

	assert.Equal(t, 1, r.applyDue(t, r.start))
	assert.Equal(t, map[int]string{timetable.WeekdayMonday: "16:00"}, r.pickups(t),
		"from the start the edited Monday applies and the dropped Wednesday is gone")
	assert.Equal(t, []int{timetable.WeekdayMonday}, r.careDays(t), "the dropped Thursday care day is gone")
	assert.Equal(t, peopleValues.DeparturePickup,
		r.student(t).AllowedDepartureModes.DepartureDays().ModeFor(peopleValues.PickupDayMonday),
		"the approved Heimweg applies from the start")

	assert.Zero(t, r.applyDue(t, r.start.AddDays(1)), "the plan applies once")
	child, err = enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	assert.Nil(t, child.WeeklyPlanOn, "an applied plan is cleared")
	assert.Equal(t, map[int]string{timetable.WeekdayMonday: "16:00"}, r.pickups(t))
}

func TestPhaseService_Update_ReschedulesPendingWeeklyPlans(t *testing.T) {
	t.Parallel()

	for _, days := range []int{-7, 7} {
		t.Run(fmt.Sprintf("shift_%d_days", days), func(t *testing.T) {
			testpkg.OwnTenant(t)
			r := setupFutureRenewal(t)
			r.approve(t)
			ctx := testpkg.Ctx(t)
			module := r.env.repos.Enrollment()
			phases, _, _, _, _ := setupPhaseTest(t)
			phase := enrollmentAPI.OwnerPhaseForTest(r.env.sourcePhase)
			phase.ServiceStartDate = enrollmentNative.Date(r.start.AddDays(days))

			plan, err := testpkg.TenantQuery1(t, ctx, r.env.db, module.ChildWeeklyPlan, r.childID)
			require.NoError(t, err)
			require.NotNil(t, plan)
			plan.Replace = true
			require.NoError(t, testpkg.TenantCommand2(t, ctx, r.env.db, module.SetChildWeeklyPlan, r.childID, plan))

			failure := errors.New("after phase update")
			err = testpkg.WithTenantTx(t, ctx, r.env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
				if err := phases.UpdatePhase(txCtx, phase); err != nil {
					return err
				}
				return failure
			})
			require.ErrorIs(t, err, failure)
			unchanged, err := testpkg.TenantQuery1(t, ctx, r.env.db, module.ChildWeeklyPlan, r.childID)
			require.NoError(t, err)
			require.Equal(t, plan, unchanged, "rescheduling rolls back with the phase change")
			storedPhase, err := testpkg.TenantQuery1(t, ctx, r.env.db, module.Phase, phase.ID)
			require.NoError(t, err)
			require.Equal(t, enrollmentNative.Date(r.start), storedPhase.ServiceStartDate)

			require.NoError(t, testpkg.TenantCommand1(t, ctx, r.env.db, phases.UpdatePhase, phase))
			rescheduled, err := testpkg.TenantQuery1(t, ctx, r.env.db, module.ChildWeeklyPlan, r.childID)
			require.NoError(t, err)
			plan.On = phase.ServiceStartDate
			require.Equal(t, plan, rescheduled, "only the activation day changes, not the author or replacement mode")
			r.assertRunningPlan(t, "after rescheduling")

			newStart := calendar.Date(phase.ServiceStartDate)
			assert.Zero(t, r.applyDue(t, newStart.AddDays(-1)))
			r.assertRunningPlan(t, "before the new start")
			assert.Equal(t, 1, r.applyDue(t, newStart))
			assert.Equal(t, map[int]string{timetable.WeekdayMonday: "15:10"}, r.pickups(t))
			assert.Equal(t, []int{timetable.WeekdayMonday}, r.careDays(t))
			assert.Equal(t, peopleValues.DeparturePickup,
				r.student(t).AllowedDepartureModes.DepartureDays().ModeFor(peopleValues.PickupDayMonday))

			phase.ServiceStartDate = enrollmentNative.Date(newStart.AddDays(1))
			require.NoError(t, testpkg.TenantCommand1(t, ctx, r.env.db, phases.UpdatePhase, phase))
			applied, err := testpkg.TenantQuery1(t, ctx, r.env.db, module.ChildWeeklyPlan, r.childID)
			require.NoError(t, err)
			assert.Nil(t, applied, "a phase change never recreates an applied plan")
		})
	}
}

// A plan that cannot be applied on the start day rolls back completely: the
// running weekly rows and Heimweg stay, and the plan stays due for the next
// tick instead of leaving the child with a half-replaced week (#6043).
func TestDecisionService_ApplyDueWeeklyPlans_FailureRollsBackCompletely(t *testing.T) {
	t.Parallel()

	r := setupFutureRenewal(t)
	r.approve(t)

	// The stored submission no longer decodes, so the pickup re-insert fails
	// after the running rows were deleted.
	ctx := testpkg.Ctx(t)
	child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	child.CustomData.Set("pickup_times", map[string]any{"mon": "25:99"})
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), child))

	assert.Zero(t, r.applyDue(t, r.start))
	r.assertRunningPlan(t, "after a failed apply")

	child.CustomData.Set("pickup_times", map[string]any{"mon": "15:10"})
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), child))
	assert.Equal(t, 1, r.applyDue(t, r.start.AddDays(1)), "the plan stays due after a failure")
	assert.Equal(t, map[int]string{timetable.WeekdayMonday: "15:10"}, r.pickups(t))
}

// A renewal whose care has already started applies the weekly plan at
// approval, as before, and plans nothing for the tick.
func TestDecisionService_Decide_StartedRenewalAppliesWeeklyPlanAtOnce(t *testing.T) {
	t.Parallel()

	r := setupFutureRenewal(t)
	r.env.sourcePhase.ServiceStartDate = enrollmentNative.Date(decisionTestToday)
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), r.env.db, r.env.repos.Enrollment().UpdatePhase, enrollmentAPI.OwnerPhaseForTest(r.env.sourcePhase)))
	r.approve(t)

	assert.Equal(t, map[int]string{timetable.WeekdayMonday: "15:10"}, r.pickups(t))
	assert.Equal(t, []int{timetable.WeekdayMonday}, r.careDays(t))
	assert.Equal(t, peopleValues.DeparturePickup,
		r.student(t).AllowedDepartureModes.DepartureDays().ModeFor(peopleValues.PickupDayMonday))
	assert.Zero(t, r.applyDue(t, r.start), "an immediate renewal plans nothing")
}

// A confirmed replacing edit before the start that removes weekly fields
// clears them at the start, as the same edit clears them at once for a child
// whose care has started (#6043).
func TestDecisionService_ApplyDueWeeklyPlans_ClearedConfirmedFields(t *testing.T) {
	t.Parallel()

	r := setupFutureRenewal(t)
	r.approve(t)
	ctx := testpkg.Ctx(t)
	child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	child.CustomData.Set("pickup_times", nil)
	child.CustomData.Unset("arrival_times")
	child.CustomData.Set("allowed_modes", nil)
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), child))
	_, err = changeRequestApplierForTest(t, r.env).SyncApprovedChildData(ctx, enrollmentAPI.SyncApprovedChildDataInput{
		RequestID: r.requestID, ChildID: r.childID, ActorAccountID: r.reviewerAccountID, ReplaceTargetedData: true,
	})
	require.NoError(t, err)
	r.assertRunningPlan(t, "after a confirmed removal before the start")

	assert.Equal(t, 1, r.applyDue(t, r.start))
	assert.Empty(t, r.pickups(t), "an explicit null clears the approved pickup times")
	assert.Empty(t, r.careDays(t), "an omitted field clears the approved care days")
	assert.Empty(t, r.student(t).AllowedDepartureModes, "an explicit null clears the approved Heimweg")
	assert.Zero(t, r.applyDue(t, r.start.AddDays(1)))
}

// A submission without a weekly field keeps that running target at the
// start, as the approval of a renewal whose care has started does: only a
// confirmed replacing edit clears a field (#6043).
func TestDecisionService_ApplyDueWeeklyPlans_OmittedFieldKeepsRunningTarget(t *testing.T) {
	t.Parallel()

	r := setupFutureRenewal(t)
	ctx := testpkg.Ctx(t)
	child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	child.CustomData.Unset("arrival_times")
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), child))
	r.approve(t)

	assert.Equal(t, 1, r.applyDue(t, r.start))
	assert.Equal(t, map[int]string{timetable.WeekdayMonday: "15:10"}, r.pickups(t))
	assert.Equal(t, []int{timetable.WeekdayMonday, timetable.WeekdayThursday}, r.careDays(t),
		"the submission did not ask for care days, so the running ones stay")
}

func TestDecisionService_SyncApprovedChildData_AppliesDueWeeklyPlan(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		replaceBefore bool
		replaceNow    bool
	}{
		{name: "approval"},
		{name: "earlier replacing edit", replaceBefore: true},
		{name: "replacing edit when due", replaceNow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			r := setupFutureRenewal(t)
			r.approve(t)
			ctx := testpkg.Ctx(t)
			if tc.replaceBefore || tc.replaceNow {
				child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
				require.NoError(t, err)
				child.CustomData.Unset("arrival_times")
				child.CustomData.Set("allowed_modes", nil)
				require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), child))
			}
			if tc.replaceBefore {
				_, err := changeRequestApplierForTest(t, r.env).SyncApprovedChildData(ctx, enrollmentAPI.SyncApprovedChildDataInput{
					RequestID: r.requestID, ChildID: r.childID, ActorAccountID: r.reviewerAccountID, ReplaceTargetedData: true,
				})
				require.NoError(t, err)
			}
			// Make the stored plan due before the worker has run. The correction
			// either keeps the submitted fields or replaces them explicitly.
			plan, err := testpkg.TenantQuery1(t, ctx, r.env.db, r.env.repos.Enrollment().ChildWeeklyPlan, r.childID)
			require.NoError(t, err)
			require.NotNil(t, plan)
			plan.On = enrollmentNative.Date(decisionTestToday)
			require.NoError(t, testpkg.TenantCommand2(t, ctx, r.env.db, r.env.repos.Enrollment().SetChildWeeklyPlan, r.childID, plan))
			_, err = changeRequestApplierForTest(t, r.env).SyncApprovedChildData(ctx, enrollmentAPI.SyncApprovedChildDataInput{
				RequestID: r.requestID, ChildID: r.childID, ActorAccountID: r.reviewerAccountID, ReplaceTargetedData: tc.replaceNow,
			})
			require.NoError(t, err)
			assert.Equal(t, map[int]string{timetable.WeekdayMonday: "15:10"}, r.pickups(t), "the due plan removes the old Wednesday")
			if tc.replaceBefore || tc.replaceNow {
				assert.Empty(t, r.careDays(t), "the stored replacing edit clears removed care days")
				assert.Empty(t, r.student(t).AllowedDepartureModes, "the stored replacing edit clears the removed Heimweg")
			} else {
				assert.Equal(t, []int{timetable.WeekdayMonday}, r.careDays(t))
			}
			child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
			require.NoError(t, err)
			assert.Nil(t, child.WeeklyPlanOn)
			assert.Zero(t, r.applyDue(t, r.start), "the correction applied the plan once")
			if !tc.replaceBefore && !tc.replaceNow {
				history, auditErr := testpkg.TenantQuery1(t, ctx, r.env.db, r.env.reposAudit.StudentFieldEdit.GetByStudentID, r.studentID)
				require.NoError(t, auditErr)
				departures := 0
				for _, entry := range history {
					if entry.FieldName == peopleValues.StudentFieldDepartureDays {
						departures++
					}
				}
				assert.Equal(t, 1, departures, "an edit that activates a due plan audits its departure transition once")
			}
		})
	}
}

func TestDecisionService_ApplyDueWeeklyPlans_BroadcastsAfterSuccessfulApply(t *testing.T) {
	t.Parallel()
	r := setupFutureRenewal(t)
	r.approve(t)
	bc := testpkg.NewRecordingBroadcaster()
	repos, care, _, schedules, _, _ := testRepositories(t, r.env.db)
	studentEnrollment := testStudentEnrollment(r.env.db)
	owner := newTestDecisions(enrollmentCompose.DecisionDependencies{
		Requests: repos.Enrollment(), Children: repos.Enrollment(), Schemas: repos.Enrollment(),
		People: owners.EnrollmentDecisionPeopleDirectory(
			repos.Person, repos.Staff, repos.Student, repos.StudentGuardian, repos.GuardianProfile, repos.GuardianPhoneNumber, studentEnrollment,
		),
		StudentEnrollment: studentEnrollment,
		Companions:        owners.EnrollmentDecisionCompanions(owners.NewStudentCompanionRepository(care.CarePlan), care.CarePlan.DeleteCompanionEdges),
		PickupSchedules:   schedules.EnrollmentPickupSchedules(), ArrivalSchedules: schedules.EnrollmentArrivalSchedules(),
		Broadcasts: owners.EnrollmentDecisionBroadcasts(bc),
		Today:      func() calendar.Date { return r.start },
	}, nil)
	r.env.decision = newFlowDecisionService(owner)
	ctx := testpkg.Ctx(t)
	child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, repos.Enrollment(), r.childID)
	require.NoError(t, err)
	child.CustomData.Set("pickup_times", map[string]any{"mon": "25:99"})
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, repos.Enrollment(), child))
	assert.Zero(t, r.applyDue(t, r.start))
	assert.False(t, bc.HasEventType(realtime.EventStudentUpdated), "a rolled-back application stays silent")

	child.CustomData.Set("pickup_times", map[string]any{"mon": "15:10"})
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, repos.Enrollment(), child))
	assert.Equal(t, 1, r.applyDue(t, r.start))
	assert.True(t, bc.HasEventType(realtime.EventStudentUpdated), "ordinary plan activation refreshes open views")
	assert.False(t, bc.HasEventType(realtime.EventStudentCompanionsChanged), "no companion link was trimmed")
	bc.Reset()
	assert.Zero(t, r.applyDue(t, r.start.AddDays(1)))
	assert.False(t, bc.HasEventType(realtime.EventStudentUpdated), "a cleared plan emits no further update")
}

// The start names the approving staff member as the author of the weekly
// rows even when the reviewing account is gone by then: the approval stores
// the author with the plan instead of resolving the account at the start
// (#6043).
func TestDecisionService_ApplyDueWeeklyPlans_KeepsAuthorWithoutReviewerAccount(t *testing.T) {
	t.Parallel()

	r := setupFutureRenewal(t)
	r.approve(t)
	ctx := testpkg.Ctx(t)
	_, err := r.env.db.NewUpdate().TableExpr("enrollment.request_children").
		Set("reviewed_by = NULL").
		Where("tenant_id = ? AND id = ?", testpkg.Tenant(t), r.childID).
		Exec(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, r.applyDue(t, r.start))
	rows, err := r.env.reposTimetable.StudentPickupSchedule.FindByStudentID(ctx, r.studentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, r.reviewerStaffID, rows[0].CreatedBy, "the stored approver authors the applied rows")
}

func TestDecisionService_ApplyDueWeeklyPlans_AuditsDeferredDeparture(t *testing.T) {
	t.Parallel()
	r := setupFutureRenewal(t)
	r.approve(t)
	ctx := testpkg.Ctx(t)
	child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	child.CustomData.Set("allowed_modes", map[string]any{"mon": []any{"bus"}})
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), child))
	_, err = changeRequestApplierForTest(t, r.env).SyncApprovedChildData(ctx, enrollmentAPI.SyncApprovedChildDataInput{
		RequestID: r.requestID, ChildID: r.childID, ActorAccountID: r.reviewerAccountID, ReplaceTargetedData: true,
	})
	require.NoError(t, err)
	history, err := testpkg.TenantQuery1(t, ctx, r.env.db, r.env.reposAudit.StudentFieldEdit.GetByStudentID, r.studentID)
	require.NoError(t, err)
	beforeCount := len(history)
	assert.Equal(t, 1, r.applyDue(t, r.start))
	history, err = testpkg.TenantQuery1(t, ctx, r.env.db, r.env.reposAudit.StudentFieldEdit.GetByStudentID, r.studentID)
	require.NoError(t, err)
	var departures int
	for _, entry := range history {
		if entry.FieldName == peopleValues.StudentFieldDepartureDays {
			departures++
			assert.Equal(t, peopleValues.StudentAuditSystemActorID, entry.EditedBy)
			assert.Equal(t, peopleValues.StudentAuditSystemActorName, entry.EditedByName)
			require.NotNil(t, entry.OldValue)
			require.NotNil(t, entry.NewValue)
			assert.Contains(t, *entry.OldValue, "Mo: Allein")
			assert.Contains(t, *entry.NewValue, "Bus")
		}
	}
	assert.Equal(t, 1, departures, "activation records the deferred Heimweg change exactly once")
	assert.Greater(t, len(history), beforeCount)
	assert.Zero(t, r.applyDue(t, r.start.AddDays(1)))
	again, err := testpkg.TenantQuery1(t, ctx, r.env.db, r.env.reposAudit.StudentFieldEdit.GetByStudentID, r.studentID)
	require.NoError(t, err)
	assert.Len(t, again, len(history))
}

func TestDecisionService_ApplyDueWeeklyPlans_ActivatesCompanionsTogether(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"none", "invalid plan", "stranded bridge"} {
		t.Run(failure, func(t *testing.T) {
			testpkg.OwnTenant(t)
			first := setupFutureRenewal(t)
			second := setupFutureRenewalChild(t, first.env, "Geschwister")
			independent := setupFutureRenewalChild(t, first.env, "Unabhängig")
			first.approve(t)
			second.approve(t)
			independent.approve(t)
			ctx := testpkg.Ctx(t)
			members := []*futureRenewal{first, second}
			if failure == "stranded bridge" {
				members = append(members, setupFutureRenewalChild(t, first.env, "Laufpartner"))
			}
			for _, r := range members {
				student := r.student(t)
				student.AllowedDepartureModes = peopleValues.AllowedDepartureModes{"mon": {peopleValues.DepartureAccompanied}}
				student.DepartureDays, student.BusDays, student.PickupDays = nil, nil, nil
				note := "Geschwisterkind"
				student.DepartureCompanionNote = &note
				require.NoError(t, testpkg.TenantCommand1(t, ctx, r.env.db, r.env.repos.Student.Update, student))
			}

			partners := []int64{second.studentID}
			if failure == "stranded bridge" {
				partners = []int64{members[2].studentID}
			}
			for _, subject := range []*futureRenewal{first, second} {
				if failure != "stranded bridge" && subject == second {
					continue
				}
				edge, err := api.NewStudentCompanionEdge(subject.studentID, partners[0], timetable.WeekdayMonday)
				require.NoError(t, err)
				require.NoError(t, api.ReplaceStudentCompanions(ctx, owners.NewStudentCompanionRepository(first.env.reposCare.CarePlan), subject.studentID, []*careplan.CompanionEdge{edge}))
			}
			for _, r := range members {
				student := r.student(t)
				student.DepartureCompanionNote = nil
				require.NoError(t, testpkg.TenantCommand1(t, ctx, r.env.db, r.env.repos.Student.Update, student))
			}
			if failure == "invalid plan" {
				child, readErr := enrollmentAPI.ReadOwnerChildForTest(ctx, second.env.repos.Enrollment(), second.childID)
				require.NoError(t, readErr)
				child.CustomData.Set("pickup_times", map[string]any{"mon": "25:99"})
				require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, second.env.repos.Enrollment(), child))
			}
			want := 3
			if failure != "none" {
				want = 1
			}
			assert.Equal(t, want, first.applyDue(t, first.start))
			for _, r := range []*futureRenewal{first, second} {
				history, auditErr := testpkg.TenantQuery1(t, ctx, r.env.db, r.env.reposAudit.StudentFieldEdit.GetByStudentID, r.studentID)
				require.NoError(t, auditErr)
				departures := 0
				for _, entry := range history {
					if entry.FieldName == peopleValues.StudentFieldDepartureDays {
						departures++
					}
				}
				expectedAudits := 1
				if failure != "none" {
					expectedAudits = 0
				}
				assert.Equal(t, expectedAudits, departures, "the departure audit commits or rolls back with its companion group")
				child, readErr := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
				require.NoError(t, readErr)
				if failure != "none" {
					require.NotNil(t, child.WeeklyPlanOn)
					assert.Equal(t, peopleValues.DepartureAccompanied, r.student(t).AllowedDepartureModes.DepartureDays().ModeFor("mon"))
					assert.Equal(t, map[int]string{1: "14:25", 3: "14:25"}, r.pickups(t))
				} else {
					assert.Nil(t, child.WeeklyPlanOn)
					assert.Equal(t, peopleValues.DeparturePickup, r.student(t).AllowedDepartureModes.DepartureDays().ModeFor("mon"))
				}
			}
			edges, err := owners.NewStudentCompanionRepository(first.env.reposCare.CarePlan).ListForStudent(ctx, first.studentID)
			require.NoError(t, err)
			if failure != "none" {
				assert.Len(t, edges, 1)
			} else {
				assert.Empty(t, edges)
			}
		})
	}
}

// A plan stored without an author, because the reviewer had no staff
// membership when approving, takes the reviewer's staff membership at the
// start (#6043).
func TestDecisionService_ApplyDueWeeklyPlans_ResolvesMissingAuthorFromReviewer(t *testing.T) {
	t.Parallel()

	r := setupFutureRenewal(t)
	r.approve(t)
	ctx := testpkg.Ctx(t)
	_, err := r.env.db.NewUpdate().TableExpr("enrollment.request_children").
		Set("weekly_plan_author = NULL").
		Where("tenant_id = ? AND id = ?", testpkg.Tenant(t), r.childID).
		Exec(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, r.applyDue(t, r.start))
	rows, err := r.env.reposTimetable.StudentPickupSchedule.FindByStudentID(ctx, r.studentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, r.reviewerStaffID, rows[0].CreatedBy)
}

// revokedFutureRenewal approves an account-bound open-phase renewal of a
// child in care whose weekly plan and class switch (2a → 3) wait for the
// start, then removes the parent's relationship to the child (#6053).
func revokedFutureRenewal(t *testing.T) (*futureRenewal, *existingChildEnv, int64) {
	t.Helper()
	r := setupFutureRenewal(t)
	ctx := testpkg.Ctx(t)
	accountID := parentAccountLinkedTo(t, r.env.db, r.studentID, securityruntime.GuardianRolePrimaryGuardian)
	_, err := r.env.db.NewUpdate().TableExpr(`enrollment.requests`).Set("guardian_account_id = ?", accountID).Where("id = ?", r.requestID).Exec(ctx)
	require.NoError(t, err)
	_, err = r.env.db.NewUpdate().TableExpr(`enrollment.phases`).Set("audience = ?", enrollmentNative.PhaseAudienceOpen).Where("id = ?", r.env.sourcePhase.ID).Exec(ctx)
	require.NoError(t, err)
	submitted, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	grade := int16(3)
	submitted.TargetGradeLevel = &grade
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), submitted))
	r.approve(t)
	child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	require.NotNil(t, child.WeeklyPlanOn)
	plannedClass, err := testpkg.TenantQuery1(t, ctx, r.env.db, r.env.repos.Enrollment().ChildClassSwitch, r.childID)
	require.NoError(t, err)
	require.NotNil(t, plannedClass)
	env := &existingChildEnv{decisionTestEnv: r.env}
	env.revokeGuardianLinks(t, r.studentID)
	return r, env, accountID
}

// assertApprovedPlanAppliesAtStart runs the start day's ticks and checks the
// school's approved plan and class switch land without restoring the
// revoked parent's link or portal access.
func (r *futureRenewal) assertApprovedPlanAppliesAtStart(t *testing.T, env *existingChildEnv, accountID int64) {
	t.Helper()
	r.assertRunningPlan(t, "before the start")
	switched, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), r.env.db, decisionOwner(t, r.env.decision).ApplyDueClassSwitches, r.start)
	require.NoError(t, err)
	assert.Equal(t, 1, switched, "the school's approved class switch applies at the start")
	assert.Equal(t, "3", r.student(t).SchoolClass)
	assert.Equal(t, 1, r.applyDue(t, r.start), "the school's approved weekly plan applies at the start")
	assert.Equal(t, map[int]string{timetable.WeekdayMonday: "15:10"}, r.pickups(t))
	assert.Equal(t, []int{timetable.WeekdayMonday}, r.careDays(t))
	assert.Equal(t, peopleValues.DeparturePickup,
		r.student(t).AllowedDepartureModes.DepartureDays().ModeFor(peopleValues.PickupDayMonday))
	assert.Empty(t, env.guardianLinks(t, r.studentID), "applying the approved plan restores no guardian link")
	assert.Zero(t, env.portalAccessRows(t, accountID), "applying the approved plan restores no portal access")
}

// The school approved the weekly plan and class switch while the parent's
// relationship held. Removing the relationship later blocks new parent
// writes, not the care the school already decided: the start applies both,
// also after a confirmed edit, and links nobody (#6053).
func TestDecisionService_RevokedParentKeepsApprovedDeferredPlan(t *testing.T) {
	t.Parallel()
	for _, syncEdit := range []bool{false, true} {
		t.Run(fmt.Sprintf("confirmed_edit_%t", syncEdit), func(t *testing.T) {
			testpkg.OwnTenant(t)
			r, env, accountID := revokedFutureRenewal(t)
			if syncEdit {
				_, err := changeRequestApplierForTest(t, r.env).SyncApprovedChildData(testpkg.Ctx(t), enrollmentAPI.SyncApprovedChildDataInput{RequestID: r.requestID, ChildID: r.childID, ActorAccountID: r.reviewerAccountID, ReplaceTargetedData: true})
				require.NoError(t, err)
				afterEdit, err := enrollmentAPI.ReadOwnerChildForTest(testpkg.Ctx(t), r.env.repos.Enrollment(), r.childID)
				require.NoError(t, err)
				require.NotNil(t, afterEdit.WeeklyPlanOn, "a confirmed edit keeps the school's approved plan")
				assert.Equal(t, r.start, *afterEdit.WeeklyPlanOn)
				assert.Empty(t, env.guardianLinks(t, r.studentID), "the edit restores no guardian link")
			}
			r.assertApprovedPlanAppliesAtStart(t, env, accountID)
		})
	}
}

// A direct staff correction is a trusted school action. After the parent's
// relationship was removed it still corrects the actual child and keeps the
// approved plans, without relinking the parent (#6053).
func TestChangeRequestService_CorrectApprovedChildData_AfterRevocationChangesChild(t *testing.T) {
	t.Parallel()
	r, env, accountID := revokedFutureRenewal(t)
	ctx := testpkg.Ctx(t)
	child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	svc := newChangeRequestServiceWithDecisionForTest(t, r.env)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, r.env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := testpkg.TenantQuery1(t, txCtx, r.env.db, svc.CorrectApprovedChildData, enrollmentAPI.CorrectApprovedChildDataInput{
			RequestID: r.requestID, ChildID: r.childID,
			FirstName: child.FirstName, LastName: "Berichtigt", DateOfBirth: child.DateOfBirth, TargetGradeLevel: child.TargetGradeLevel,
			Reason: "Nachname berichtigt", ActorAccountID: r.reviewerAccountID,
		})
		return err
	}))
	person, err := testpkg.TenantQuery1(t, ctx, r.env.db, r.env.repos.Person.FindByID, any(r.student(t).PersonID))
	require.NoError(t, err)
	assert.Equal(t, "Berichtigt", person.LastName, "the correction reaches the actual child")
	assert.Equal(t, "Mara", person.FirstName)
	after, err := enrollmentAPI.ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	require.NotNil(t, after.WeeklyPlanOn, "a surname correction keeps the approved weekly plan")
	plannedClass, err := testpkg.TenantQuery1(t, ctx, r.env.db, r.env.repos.Enrollment().ChildClassSwitch, r.childID)
	require.NoError(t, err)
	require.NotNil(t, plannedClass, "a surname correction keeps the approved class switch")
	assert.Equal(t, "3", plannedClass.To)
	assert.Equal(t, "2a", r.student(t).SchoolClass, "the running class stays until the start")
	assert.Empty(t, env.guardianLinks(t, r.studentID), "the correction restores no guardian link")
	assert.Zero(t, env.portalAccessRows(t, accountID), "the correction restores no portal access")
	r.assertApprovedPlanAppliesAtStart(t, env, accountID)
}

// A family's change request for the child, approved after the school
// removed the family's relationship, is a new parent-authorized write. It is
// refused instead of reaching the request child the school-approved weekly
// plan applies at the start (#6053).
func TestDecisionService_RevokedParentChangeRequestCannotReplaceApprovedPlan(t *testing.T) {
	t.Parallel()
	r, env, accountID := revokedFutureRenewal(t)
	_, err := changeRequestApplierForTest(t, r.env).SyncApprovedChildData(testpkg.Ctx(t), enrollmentAPI.SyncApprovedChildDataInput{
		RequestID: r.requestID, ChildID: r.childID, ActorAccountID: r.reviewerAccountID,
		ReplaceTargetedData: true, ChildDataChanged: true,
	})
	require.ErrorIs(t, err, enrollmentNative.ErrChangeRequestParentNotPermitted)
	child, err := enrollmentAPI.ReadOwnerChildForTest(testpkg.Ctx(t), r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	require.NotNil(t, child.WeeklyPlanOn, "the refusal keeps the school's approved plan")
	assert.Empty(t, env.guardianLinks(t, r.studentID))
	r.assertApprovedPlanAppliesAtStart(t, env, accountID)
}
