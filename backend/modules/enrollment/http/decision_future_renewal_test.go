package enrollmenthttp_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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

	existing := testpkg.CreateTestStudent(t, env.db, "Mara", "Februar", "2a")
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

	_, reviewerAccountID := createReviewerStaffWithDistinctAccount(t, env)
	requestID, childID := submitOneChildWithCustomData(t, env, "februar-renewal@example.com", "Mara", "Februar", map[string]any{
		"pickup_times":  map[string]any{"mon": "15:10"},
		"arrival_times": map[string]any{"mon": "07:45"},
		"allowed_modes": map[string]any{"mon": []any{"pickup"}},
		"health":        "Nussallergie",
	})
	matchChildToExistingStudent(t, env, childID, existing.ID)

	return &futureRenewal{env: env, studentID: existing.ID, requestID: requestID, childID: childID, reviewerAccountID: reviewerAccountID, start: start}
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
	child.CustomData["pickup_times"] = map[string]any{"mon": "16:00"}
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
	child.CustomData["pickup_times"] = map[string]any{"mon": "25:99"}
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), child))

	assert.Zero(t, r.applyDue(t, r.start))
	r.assertRunningPlan(t, "after a failed apply")

	child.CustomData["pickup_times"] = map[string]any{"mon": "15:10"}
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
	child.CustomData["pickup_times"] = nil
	delete(child.CustomData, "arrival_times")
	child.CustomData["allowed_modes"] = nil
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
	delete(child.CustomData, "arrival_times")
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), child))
	r.approve(t)

	assert.Equal(t, 1, r.applyDue(t, r.start))
	assert.Equal(t, map[int]string{timetable.WeekdayMonday: "15:10"}, r.pickups(t))
	assert.Equal(t, []int{timetable.WeekdayMonday, timetable.WeekdayThursday}, r.careDays(t),
		"the submission did not ask for care days, so the running ones stay")
}
