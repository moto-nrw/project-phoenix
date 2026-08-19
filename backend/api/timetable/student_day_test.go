// Integration tests for the WP-B11 per-student day/week endpoints. Uses a
// real DB + real repos; the test router mounts the handlers without the
// JWT/TenantTx middleware stack, so tenant context and permissions are
// injected directly into the request context (the same pattern used by
// instance_students_test.go).
package timetable

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// studentDaySetup holds wired fixtures + resource for the B11 handler tests.
type studentDaySetup struct {
	res        *Resource
	db         *bun.DB
	ctx        context.Context
	studentID  int64
	staffID    int64
	roomID     int64
	activityID int64
}

// buildStudentDaySetup creates one student, one staff member (for
// created_by FKs), one room, and a planned activity_instance on
// 2026-04-22 (a Wednesday). Per-test helpers add instance_students /
// arrival / pickup / visit rows on top.
func buildStudentDaySetup(t *testing.T) *studentDaySetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SD-Room-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SD-Act-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "SD", fmt.Sprintf("Staff-%d", suffix))
	student := testpkg.CreateTestStudent(t, db, "SD", fmt.Sprintf("Stu-%d", suffix), "3a")
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, 0, activity.ID, room.ID)
	})

	// Wire the full resource with real repos for the B11 path.
	res := NewResource(Dependencies{
		TimetableData: testTimetableData(db),
		PersonService: usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{StudentRepo: usersRepo.NewStudentRepository(db)}),
		// UserContextService + SettingsService intentionally nil:
		// admin-perm path short-circuits CanReadStudent; the 403 test relies on
		// the fallthrough returning false when userCtx is nil.
		DB: db,
	})

	return &studentDaySetup{
		res:        res,
		db:         db,
		ctx:        ctx,
		studentID:  student.ID,
		staffID:    staff.ID,
		roomID:     room.ID,
		activityID: activity.ID,
	}
}

// adminRouter mounts /student/{id}/{day|week} without middleware, pre-baking
// admin permissions into the request context so CanReadStudent short-circuits
// to allow. A separate non-admin router is used for the forbidden-path tests.
func adminRouter(parentCtx context.Context, res *Resource) chi.Router {
	return studentRouter(parentCtx, res, []string{"admin:*"})
}

func studentRouter(parentCtx context.Context, res *Resource, perms []string) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
			ctx = context.WithValue(ctx, jwt.CtxPermissions, perms)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/student/{id}/day", res.getStudentDay)
	r.Get("/student/{id}/week", res.getStudentWeek)
	return r
}

func doGet(t *testing.T, router chi.Router, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeEnvelope(t *testing.T, w *httptest.ResponseRecorder, target any) {
	t.Helper()
	var env struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "body=%s", w.Body.String())
	require.Equal(t, "success", env.Status, "body=%s", w.Body.String())
	require.NoError(t, json.Unmarshal(env.Data, target))
}

// --- /day -----------------------------------------------------------------

func TestGetStudentDay_HappyPath_WithScheduleAndEnrolledInstance(t *testing.T) {
	s := buildStudentDaySetup(t)

	// Create a planned instance on Wed 2026-04-22.
	inst := testpkg.CreateTestActivityInstance(t, s.db, timezone.NewDate(2026, 4, 22), s.roomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &s.activityID,
		StartHHMM:       "14:00",
		EndHHMM:         "15:00",
		Title:           "Lernzeit-3a",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	row := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, s.studentID, schedule.AttendanceStatusExpected)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", row.ID) })

	// Wednesday is weekday 3 (ISO).
	arrival := testpkg.CreateTestArrivalSchedule(t, s.db, s.studentID, schedule.WeekdayWednesday, s.staffID, "13:00")
	pickup := testpkg.CreateTestPickupSchedule(t, s.db, s.studentID, schedule.WeekdayWednesday, s.staffID, "16:00")
	t.Cleanup(func() {
		testpkg.CleanupScheduleFixturesB11(t, s.db,
			[]int64{arrival.ID}, nil,
			[]int64{pickup.ID}, nil,
			nil, nil)
	})

	router := adminRouter(s.ctx, s.res)
	w := doGet(t, router, fmt.Sprintf("/student/%d/day?date=2026-04-22", s.studentID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var got StudentDayResponse
	decodeEnvelope(t, w, &got)

	assert.Equal(t, s.studentID, got.StudentID)
	assert.Equal(t, "2026-04-22", got.Date)
	assert.Equal(t, 3, got.Weekday)

	// arrival: source=schedule, expected_time=13:00.
	assert.Equal(t, SlotSourceSchedule, got.Arrival.Source)
	require.NotNil(t, got.Arrival.ExpectedTime)
	assert.Equal(t, "13:00", *got.Arrival.ExpectedTime)
	assert.Nil(t, got.Arrival.Reason)

	// pickup: source=schedule, expected_time=16:00.
	assert.Equal(t, SlotSourceSchedule, got.Pickup.Source)
	require.NotNil(t, got.Pickup.ExpectedTime)
	assert.Equal(t, "16:00", *got.Pickup.ExpectedTime)

	// instances list: one enrolled, no is_unplanned.
	require.Len(t, got.Instances, 1)
	i := got.Instances[0]
	assert.Equal(t, inst.ID, i.ID)
	assert.Equal(t, "Lernzeit-3a", i.Title)
	assert.Equal(t, "14:00", i.StartTime)
	assert.Equal(t, "15:00", i.EndTime)
	assert.Equal(t, schedule.InstanceStatusPlanned, i.Status)
	assert.False(t, i.Attendance.IsUnplanned)
	assert.Equal(t, schedule.AttendanceStatusExpected, i.Attendance.Status)
}

func TestGetStudentDay_ExceptionOverridesSchedule(t *testing.T) {
	s := buildStudentDaySetup(t)

	arrSched := testpkg.CreateTestArrivalSchedule(t, s.db, s.studentID, schedule.WeekdayWednesday, s.staffID, "13:00")
	arrExc := testpkg.CreateTestArrivalException(t, s.db, s.studentID,
		timezone.NewDate(2026, 4, 22), s.staffID, "10:30", "Wandertag")
	t.Cleanup(func() {
		testpkg.CleanupScheduleFixturesB11(t, s.db,
			[]int64{arrSched.ID}, []int64{arrExc.ID}, nil, nil, nil, nil)
	})

	router := adminRouter(s.ctx, s.res)
	w := doGet(t, router, fmt.Sprintf("/student/%d/day?date=2026-04-22", s.studentID))
	require.Equal(t, http.StatusOK, w.Code)

	var got StudentDayResponse
	decodeEnvelope(t, w, &got)

	assert.Equal(t, SlotSourceException, got.Arrival.Source)
	require.NotNil(t, got.Arrival.ExpectedTime)
	assert.Equal(t, "10:30", *got.Arrival.ExpectedTime)
	require.NotNil(t, got.Arrival.Reason)
	assert.Equal(t, "Wandertag", *got.Arrival.Reason)
}

func TestGetStudentDay_PickupException_NilTimeMeansAbsence(t *testing.T) {
	s := buildStudentDaySetup(t)

	// A pickup exception with empty HHMM → PickupTime=NULL = absence for
	// the day. Source must still be "exception", ExpectedTime must be nil.
	exc := testpkg.CreateTestPickupException(t, s.db, s.studentID,
		timezone.NewDate(2026, 4, 22), s.staffID, "", "Krank")
	t.Cleanup(func() {
		testpkg.CleanupScheduleFixturesB11(t, s.db,
			nil, nil, nil, []int64{exc.ID}, nil, nil)
	})

	router := adminRouter(s.ctx, s.res)
	w := doGet(t, router, fmt.Sprintf("/student/%d/day?date=2026-04-22", s.studentID))
	require.Equal(t, http.StatusOK, w.Code)

	var got StudentDayResponse
	decodeEnvelope(t, w, &got)

	assert.Equal(t, SlotSourceException, got.Pickup.Source)
	assert.Nil(t, got.Pickup.ExpectedTime, "absence exception must surface nil time")
	require.NotNil(t, got.Pickup.Reason)
	assert.Equal(t, "Krank", *got.Pickup.Reason)
}

func TestGetStudentDay_NoArrivalNoPickup_SourceNone(t *testing.T) {
	s := buildStudentDaySetup(t)

	router := adminRouter(s.ctx, s.res)
	w := doGet(t, router, fmt.Sprintf("/student/%d/day?date=2026-04-22", s.studentID))
	require.Equal(t, http.StatusOK, w.Code)

	var got StudentDayResponse
	decodeEnvelope(t, w, &got)

	assert.Equal(t, SlotSourceNone, got.Arrival.Source)
	assert.Nil(t, got.Arrival.ExpectedTime)
	assert.Equal(t, SlotSourceNone, got.Pickup.Source)
	assert.Nil(t, got.Pickup.ExpectedTime)
	assert.Empty(t, got.Instances)
}

// Regression guard: when a student is BOTH enrolled (instance_students row
// exists) AND has a visit on the same active_group, the response must
// contain exactly ONE entry for that instance — the enrolled one, with
// is_unplanned=false. The dedup lives in buildStudentDay where the visit
// loop skips ids already present in enrolledInstanceIDs; this test locks
// that behavior in so a future rewrite can't introduce a duplicate.
func TestGetStudentDay_EnrolledPlusVisit_NoDuplicate(t *testing.T) {
	s := buildStudentDaySetup(t)

	// Active group + bridge instance (same shape as the unplanned test).
	ag := testpkg.CreateTestActiveGroup(t, s.db, s.activityID, s.roomID)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.groups", ag.ID) })

	agID := ag.ID
	inst := testpkg.CreateTestActivityInstance(t, s.db,
		timezone.NewDate(2026, 4, 22), s.roomID,
		testpkg.ActivityInstanceOpts{
			ActivityGroupID: &s.activityID,
			ActiveGroupID:   &agID,
			Status:          schedule.InstanceStatusActive,
			StartHHMM:       "14:00",
			EndHHMM:         "15:00",
			Title:           "Enrolled-And-Present",
		})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	// Both signals: enrolled row AND a visit on the same active_group.
	row := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, s.studentID, schedule.AttendanceStatusPresent)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", row.ID) })

	visit := testpkg.CreateTestVisit(t, s.db, s.studentID, ag.ID,
		time.Date(2026, 4, 22, 14, 5, 0, 0, time.UTC), nil)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.visits", visit.ID) })

	router := adminRouter(s.ctx, s.res)
	w := doGet(t, router, fmt.Sprintf("/student/%d/day?date=2026-04-22", s.studentID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var got StudentDayResponse
	decodeEnvelope(t, w, &got)

	require.Len(t, got.Instances, 1,
		"enrolled student with a matching visit must appear exactly once, not duplicated")
	entry := got.Instances[0]
	assert.Equal(t, inst.ID, entry.ID)
	assert.False(t, entry.Attendance.IsUnplanned,
		"enrolled row must win over the visit-side path (is_unplanned=false)")
	assert.Equal(t, schedule.AttendanceStatusPresent, entry.Attendance.Status)
}

// Unplanned scenario: active.visit exists for student on an active
// instance's bridge group, but no instance_students row.
func TestGetStudentDay_UnplannedStudent(t *testing.T) {
	s := buildStudentDaySetup(t)

	// Active group + bridge instance.
	ag := testpkg.CreateTestActiveGroup(t, s.db, s.activityID, s.roomID)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.groups", ag.ID) })

	agID := ag.ID
	inst := testpkg.CreateTestActivityInstance(t, s.db,
		timezone.NewDate(2026, 4, 22), s.roomID,
		testpkg.ActivityInstanceOpts{
			ActivityGroupID: &s.activityID,
			ActiveGroupID:   &agID,
			Status:          schedule.InstanceStatusActive,
			StartHHMM:       "14:00",
			EndHHMM:         "15:00",
			Title:           "Unplanned-Session",
		})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	// No instance_students row for our student — they're NOT on the plan.
	// But they checked in, so a visit exists.
	visit := testpkg.CreateTestVisit(t, s.db, s.studentID, ag.ID,
		time.Date(2026, 4, 22, 14, 5, 0, 0, time.UTC), nil)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.visits", visit.ID) })

	router := adminRouter(s.ctx, s.res)
	w := doGet(t, router, fmt.Sprintf("/student/%d/day?date=2026-04-22", s.studentID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var got StudentDayResponse
	decodeEnvelope(t, w, &got)

	require.Len(t, got.Instances, 1, "must surface the walk-in as an instance")
	entry := got.Instances[0]
	assert.Equal(t, inst.ID, entry.ID)
	assert.True(t, entry.Attendance.IsUnplanned, "must flag is_unplanned")
	assert.Equal(t, schedule.AttendanceStatusPresent, entry.Attendance.Status)
	assert.Nil(t, entry.Attendance.Substatus)
	assert.Nil(t, entry.Attendance.Note)
	require.NotNil(t, entry.Attendance.CheckedInAt)
	assert.Contains(t, *entry.Attendance.CheckedInAt, "2026-04-22")
}

func TestGetStudentDay_InvalidDate_Returns400(t *testing.T) {
	s := buildStudentDaySetup(t)
	router := adminRouter(s.ctx, s.res)

	w := doGet(t, router, fmt.Sprintf("/student/%d/day?date=not-a-date", s.studentID))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid date format")
}

func TestGetStudentDay_MissingDate_Returns400(t *testing.T) {
	s := buildStudentDaySetup(t)
	router := adminRouter(s.ctx, s.res)

	w := doGet(t, router, fmt.Sprintf("/student/%d/day", s.studentID))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "date is required")
}

func TestGetStudentDay_InvalidStudentID_Returns400(t *testing.T) {
	s := buildStudentDaySetup(t)
	router := adminRouter(s.ctx, s.res)

	w := doGet(t, router, "/student/abc/day?date=2026-04-22")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid student ID")
}

func TestGetStudentDay_UnknownStudent_Returns404(t *testing.T) {
	s := buildStudentDaySetup(t)
	router := adminRouter(s.ctx, s.res)

	w := doGet(t, router, "/student/999999999/day?date=2026-04-22")
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

// Cross-tenant must 404, not 403: returning 403 for a student in tenant B
// would leak their existence to a caller in tenant A.
func TestGetStudentDay_CrossTenant_Returns404(t *testing.T) {
	s := buildStudentDaySetup(t)

	// Pretend the caller is in tenant 2 — our fixture student lives in
	// tenant 1 and must be invisible.
	otherCtx := testpkg.TenantContext(2)
	router := adminRouter(otherCtx, s.res)

	w := doGet(t, router, fmt.Sprintf("/student/%d/day?date=2026-04-22", s.studentID))
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

// Same-tenant, non-admin, not a group supervisor → 403 (we already know the
// student exists in this tenant, so hiding them would be misleading).
func TestGetStudentDay_SameTenantNoSupervisor_Returns403(t *testing.T) {
	s := buildStudentDaySetup(t)

	// Strip admin perms; CanReadStudent falls through to the group-
	// supervisor branch and fails because userContextService is nil.
	router := studentRouter(s.ctx, s.res, []string{"schedules:read"})

	w := doGet(t, router, fmt.Sprintf("/student/%d/day?date=2026-04-22", s.studentID))
	assert.Equal(t, http.StatusForbidden, w.Code, "body=%s", w.Body.String())
}

// --- /week ----------------------------------------------------------------

func TestGetStudentWeek_HappyPath_ReturnsEntryPerDay(t *testing.T) {
	s := buildStudentDaySetup(t)
	router := adminRouter(s.ctx, s.res)

	w := doGet(t, router, fmt.Sprintf("/student/%d/week?from=2026-04-20&to=2026-04-24", s.studentID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var got StudentWeekResponse
	decodeEnvelope(t, w, &got)

	assert.Equal(t, "2026-04-20", got.From)
	assert.Equal(t, "2026-04-24", got.To)
	require.Len(t, got.Days, 5, "must return one entry per day in the range (inclusive)")
	for i, d := range got.Days {
		assert.Equal(t, s.studentID, d.StudentID)
		// Days in ascending order.
		if i > 0 {
			assert.True(t, d.Date > got.Days[i-1].Date, "days must be ascending")
		}
	}
}

func TestGetStudentWeek_RangeAtLimit_14Days_ReturnsOK(t *testing.T) {
	s := buildStudentDaySetup(t)
	router := adminRouter(s.ctx, s.res)

	w := doGet(t, router, fmt.Sprintf("/student/%d/week?from=2026-04-01&to=2026-04-14", s.studentID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var got StudentWeekResponse
	decodeEnvelope(t, w, &got)
	assert.Len(t, got.Days, 14)
}

func TestGetStudentWeek_RangeExceedsLimit_Returns400(t *testing.T) {
	s := buildStudentDaySetup(t)
	router := adminRouter(s.ctx, s.res)

	w := doGet(t, router, fmt.Sprintf("/student/%d/week?from=2026-04-01&to=2026-04-15", s.studentID))
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.True(t,
		strings.Contains(w.Body.String(), "date range exceeds maximum of 14 days"),
		"body=%s", w.Body.String())
}

// Spring DST: 2026-03-29 is the Berlin spring-forward day (23h long).
// A 15-day request spanning it must still be rejected by the cap — a naive
// to.Sub(from).Hours()/24 would undercount by the missing hour and let it
// through.
func TestGetStudentWeek_SpringDST_15DayRangeStillRejected(t *testing.T) {
	s := buildStudentDaySetup(t)
	router := adminRouter(s.ctx, s.res)

	w := doGet(t, router, fmt.Sprintf("/student/%d/week?from=2026-03-22&to=2026-04-05", s.studentID))
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "date range exceeds maximum of 14 days")
}

// Spring DST: the same span minus one day (14 inclusive days crossing the
// transition) must pass and return one entry per day.
func TestGetStudentWeek_SpringDST_14DayRangeAtLimit_ReturnsOK(t *testing.T) {
	s := buildStudentDaySetup(t)
	router := adminRouter(s.ctx, s.res)

	w := doGet(t, router, fmt.Sprintf("/student/%d/week?from=2026-03-23&to=2026-04-05", s.studentID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var got StudentWeekResponse
	decodeEnvelope(t, w, &got)
	assert.Len(t, got.Days, 14)
}

func TestGetStudentWeek_FromAfterTo_Returns400(t *testing.T) {
	s := buildStudentDaySetup(t)
	router := adminRouter(s.ctx, s.res)

	w := doGet(t, router, fmt.Sprintf("/student/%d/week?from=2026-04-22&to=2026-04-20", s.studentID))
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "'from' must be before or equal to 'to'")
}

func TestGetStudentWeek_MissingParams_Returns400(t *testing.T) {
	s := buildStudentDaySetup(t)
	router := adminRouter(s.ctx, s.res)

	w := doGet(t, router, fmt.Sprintf("/student/%d/week?from=2026-04-22", s.studentID))
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "from and to are required")
}

// queryCounterHook counts every SELECT the handler issues during a /week
// call. Exists purely to regression-guard the N+1 fix: /week over any valid
// range must stay at or below the 7-query ceiling.
type queryCounterHook struct{ n int64 }

func (h *queryCounterHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	h.n++
	return ctx
}
func (h *queryCounterHook) AfterQuery(context.Context, *bun.QueryEvent) {}

// TestGetStudentWeek_QueryBudget_BatchedNotNPlusOne locks in the N+1 fix:
// a 14-day /week must fire at most 7 DB queries (enrolled + instances +
// visits + 2 schedules + 2 exceptions). Previously this was ~98.
func TestGetStudentWeek_QueryBudget_BatchedNotNPlusOne(t *testing.T) {
	s := buildStudentDaySetup(t)

	hook := &queryCounterHook{}
	s.db.AddQueryHook(hook)

	router := adminRouter(s.ctx, s.res)
	w := doGet(t, router, fmt.Sprintf("/student/%d/week?from=2026-04-01&to=2026-04-14", s.studentID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// FindByID (student resolution) + up to 7 preload queries = 8. Allow a
	// little headroom for implicit bun/pg metadata queries; assert we're
	// far below the pre-fix 14*7=98 regime.
	assert.LessOrEqual(t, hook.n, int64(12),
		"14-day /week must stay under the batched ceiling, got %d", hook.n)
	t.Logf("14-day /week fired %d queries", hook.n)
}
