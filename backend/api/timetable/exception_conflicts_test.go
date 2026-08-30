// Integration tests for the WP-B13 exception-conflict endpoint. Mirrors the
// test harness used by gaps_test.go and student_day_test.go — tenant context
// and permissions are injected via middleware shim; JWT is skipped.
package timetable

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// -----------------------------------------------------------------------------
// Setup harness
// -----------------------------------------------------------------------------

type conflictsSetup struct {
	res       *Resource
	db        *bun.DB
	ctx       context.Context
	roomID    int64
	staffID   int64 // for arrival_schedule.created_by
	cleanupFn func()
}

func buildConflictsSetup(t *testing.T) *conflictsSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Ec-Room-%d", suffix))
	creator := testpkg.CreateTestStaff(t, db, "ExcCreator", fmt.Sprintf("Staff-%d", suffix))

	cleanup := func() {
	}

	clock := func() time.Time { return timezone.NewDate(2026, 8, 24).BerlinMidnight().Add(12 * time.Hour) }
	res := NewResource(Dependencies{
		TimetableData: testTimetableData(db, clock),
		Now:           clock,
		DB:            db,
	})

	return &conflictsSetup{
		res:       res,
		db:        db,
		ctx:       ctx,
		roomID:    room.ID,
		staffID:   creator.ID,
		cleanupFn: cleanup,
	}
}

func conflictsRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/exception-conflicts", res.getExceptionConflicts)
	return r
}

func doConflicts(t *testing.T, router chi.Router, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeConflicts(t *testing.T, w *httptest.ResponseRecorder) ConflictsResponse {
	t.Helper()
	var env struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "body=%s", w.Body.String())
	require.Equal(t, "success", env.Status, "body=%s", w.Body.String())
	var out ConflictsResponse
	require.NoError(t, json.Unmarshal(env.Data, &out))
	return out
}

// nextMonday returns a Berlin-local Monday >= tomorrow, as both a YYYY-MM-DD
// string and a timezone.Date. Test fixtures that exercise weekday rules need
// a concrete weekday, and Monday is ISO 1 — the simplest to reason about.
// The base date is deterministic across the week so the test never crosses a
// Saturday/Sunday boundary where arrival_schedules don't apply.
func nextMonday() (string, timezone.Date) {
	d := timezone.NewDate(2026, 8, 24).AddDays(1)
	for d.Weekday() != time.Monday {
		d = d.AddDays(1)
	}
	return d.String(), d
}

// nextTuesday mirrors nextMonday for tests that exercise a different weekday
// relative to the materialised instance and the arrival schedule.
func nextTuesday() (string, timezone.Date) {
	d := timezone.NewDate(2026, 8, 24).AddDays(1)
	for d.Weekday() != time.Tuesday {
		d = d.AddDays(1)
	}
	return d.String(), d
}

// createTestActivityException is a local fixture — no helper exists in
// test/fixtures.go yet and WP-B13 is the first consumer, so keeping it scoped
// to this test file avoids churn. Mirrors the existing pattern for local
// fixtures (see e.g. the raw timeframe insert in student_day_test.go).
func createTestActivityException(
	t *testing.T,
	db *bun.DB,
	activityGroupID int64,
	date timezone.Date,
	excType string,
	startHHMM, endHHMM string,
	reason string,
) *schedule.ActivityException {
	t.Helper()
	row := &schedule.ActivityException{
		ActivityGroupID: activityGroupID,
		ExceptionDate:   date,
		ExceptionType:   excType,
	}
	row.SetTenantID(testpkg.Tenant(t))
	if startHHMM != "" {
		st := parseHHMM(t, startHHMM)
		row.StartTime = &st
	}
	if endHHMM != "" {
		et := parseHHMM(t, endHHMM)
		row.EndTime = &et
	}
	if reason != "" {
		row.Reason = &reason
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.activity_exceptions`).
		Exec(ctx)
	require.NoError(t, err, "failed to insert activity_exception fixture")

	return row
}

// createTestActivitySchedule inserts an activities.schedules row. Optional
// timeframeID lets tests either link a real timeframe (so the template
// start-time lookup can find it) or leave it nil to exercise the skip path.
func createTestActivitySchedule(
	t *testing.T,
	db *bun.DB,
	activityGroupID int64,
	weekday int,
	timeframeID *int64,
) *activities.Schedule {
	t.Helper()
	row := &activities.Schedule{
		ActivityGroupID: activityGroupID,
		Weekday:         weekday,
		TimeframeID:     timeframeID,
	}
	row.SetTenantID(testpkg.Tenant(t))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`activities.schedules AS "schedule"`).
		Exec(ctx)
	require.NoError(t, err, "failed to insert activities.schedules fixture")

	return row
}

// createTestTimeframeRow inserts a schedule.timeframes row at the given
// timezone-free wall-clock. Mirrors the production flow where admins submit
// HH:MM strings and the backend stores them as SQL TIME.
func createTestTimeframeRow(t *testing.T, db *bun.DB, startHHMM, endHHMM string) *schedule.Timeframe {
	t.Helper()
	start := parseHHMM(t, startHHMM)
	end := parseHHMM(t, endHHMM)
	row := &schedule.Timeframe{
		StartTime:   start,
		EndTime:     &end,
		IsActive:    true,
		Description: fmt.Sprintf("tf-%s-%s-%d", startHHMM, endHHMM, time.Now().UnixNano()),
	}
	row.SetTenantID(testpkg.Tenant(t))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.timeframes AS "timeframe"`).
		Exec(ctx)
	require.NoError(t, err, "failed to insert timeframe fixture")
	return row
}

func parseHHMM(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	return time.Date(2000, 1, 1, parsed.Hour(), parsed.Minute(), 0, 0, time.UTC)
}

// -----------------------------------------------------------------------------
// Happy paths
// -----------------------------------------------------------------------------

func TestExceptionConflicts_CancelledInstance_EmitsWarningPerExpectedStudent(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, date := nextMonday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("Cancel-%d", time.Now().UnixNano()))

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Lernzeit-3a", ActivityGroupID: &group.ID,
	})

	student := testpkg.CreateTestStudent(t, s.db, "Anna", fmt.Sprintf("C-%d", time.Now().UnixNano()), "3a")

	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, "")

	testpkg.CreateTestArrivalSchedule(t, s.db, student.ID, schedule.WeekdayMonday, s.staffID, "13:00")

	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionCancelled, "", "", "Feueralarm-Uebung")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeConflicts(t, w)
	require.Len(t, got.Conflicts, 1)
	c := got.Conflicts[0]
	assert.Equal(t, ConflictKindCancelledArrivals, c.Kind)
	assert.Equal(t, dateStr, c.Date)
	assert.Equal(t, group.ID, c.ActivityGroupID)
	assert.Equal(t, inst.ID, c.InstanceID)
	assert.Equal(t, "Lernzeit-3a", c.ActivityTitle)
	assert.Equal(t, student.ID, c.StudentID)
	assert.Equal(t, "13:00", c.ExpectedArrival)
	assert.Equal(t, SlotSourceSchedule, c.ArrivalSource)
	assert.Equal(t, "Feueralarm-Uebung", c.CancellationReason)
	assert.Empty(t, c.OriginalStartTime)
	assert.Empty(t, c.ModifiedStartTime)
}

func TestExceptionConflicts_CancelledInstance_ArrivalExceptionOverridesSchedule(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, date := nextMonday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("CancelOv-%d", time.Now().UnixNano()))

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", ActivityGroupID: &group.ID,
	})

	student := testpkg.CreateTestStudent(t, s.db, "Ben", fmt.Sprintf("C-%d", time.Now().UnixNano()), "3a")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, "")

	testpkg.CreateTestArrivalSchedule(t, s.db, student.ID, schedule.WeekdayMonday, s.staffID, "13:00")
	// Arrival exception for the date — 12:30, which must win over the schedule.
	testpkg.CreateTestArrivalException(t, s.db, student.ID, date, s.staffID, "12:30", "Arzttermin vormittags")

	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionCancelled, "", "", "Klassen-Kuchenverkauf")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	require.Len(t, got.Conflicts, 1)
	assert.Equal(t, SlotSourceException, got.Conflicts[0].ArrivalSource)
	assert.Equal(t, "12:30", got.Conflicts[0].ExpectedArrival)
}

func TestExceptionConflicts_CancelledInstance_NoArrivalInfo_OmitsTime(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	// Tuesday exception but the arrival schedule only covers Monday — so
	// the student has arrival_source=none on the Tuesday.
	dateStr, date := nextTuesday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("CancelNone-%d", time.Now().UnixNano()))

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", ActivityGroupID: &group.ID,
	})

	student := testpkg.CreateTestStudent(t, s.db, "Cara", fmt.Sprintf("C-%d", time.Now().UnixNano()), "3a")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, "")

	// Monday schedule only — nothing for the Tuesday exception date.
	testpkg.CreateTestArrivalSchedule(t, s.db, student.ID, schedule.WeekdayMonday, s.staffID, "13:00")

	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionCancelled, "", "", "Ferien")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	require.Len(t, got.Conflicts, 1)
	assert.Equal(t, SlotSourceNone, got.Conflicts[0].ArrivalSource)
	assert.Empty(t, got.Conflicts[0].ExpectedArrival)
}

func TestExceptionConflicts_CancelledInstance_SkipsNonExpectedAttendance(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, date := nextMonday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("CancelSkip-%d", time.Now().UnixNano()))

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", ActivityGroupID: &group.ID,
	})

	suffix := time.Now().UnixNano()
	stuExpected := testpkg.CreateTestStudent(t, s.db, "Expo", fmt.Sprintf("S-%d", suffix), "3a")
	stuPresent := testpkg.CreateTestStudent(t, s.db, "Pres", fmt.Sprintf("S-%d", suffix+1), "3a")
	stuAbsent := testpkg.CreateTestStudent(t, s.db, "Abso", fmt.Sprintf("S-%d", suffix+2), "3a")

	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, stuExpected.ID, schedule.AttendanceStatusExpected)
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, stuPresent.ID, schedule.AttendanceStatusPresent)
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, stuAbsent.ID, schedule.AttendanceStatusAbsent)

	for _, stu := range []*struct{ ID int64 }{{stuExpected.ID}, {stuPresent.ID}, {stuAbsent.ID}} {
		testpkg.CreateTestArrivalSchedule(t, s.db, stu.ID, schedule.WeekdayMonday, s.staffID, "13:00")
	}

	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionCancelled, "", "", "")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	require.Len(t, got.Conflicts, 1, "only the expected student should produce a warning")
	assert.Equal(t, stuExpected.ID, got.Conflicts[0].StudentID)
}

func TestExceptionConflicts_ModifiedEarlier_EmitsWarning(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, date := nextMonday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("ModEarly-%d", time.Now().UnixNano()))

	tf := createTestTimeframeRow(t, s.db, "14:00", "15:00")
	createTestActivitySchedule(t, s.db, group.ID, schedule.WeekdayMonday, &tf.ID)

	// Instance materialised AFTER the exception, so inst.start_time matches
	// the modified 13:30 (still works when instance pre-dates the exception
	// because the handler uses the exception's start_time, not the
	// instance's).
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:30", EndHHMM: "15:00", Title: "Lernzeit-Mod", ActivityGroupID: &group.ID,
	})

	student := testpkg.CreateTestStudent(t, s.db, "Dana", fmt.Sprintf("M-%d", time.Now().UnixNano()), "3a")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, "")

	// Student arrives at 13:45 — that is AFTER the new 13:30 start: conflict.
	testpkg.CreateTestArrivalSchedule(t, s.db, student.ID, schedule.WeekdayMonday, s.staffID, "13:45")

	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionModified, "13:30", "15:00", "vorverlegt")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	got := decodeConflicts(t, w)
	require.Len(t, got.Conflicts, 1)
	c := got.Conflicts[0]
	assert.Equal(t, ConflictKindModifiedMismatch, c.Kind)
	assert.Equal(t, "13:45", c.ExpectedArrival)
	assert.Equal(t, SlotSourceSchedule, c.ArrivalSource)
	assert.Equal(t, "13:30", c.ModifiedStartTime)
	assert.Equal(t, "14:00", c.OriginalStartTime)
	assert.Empty(t, c.CancellationReason)
}

func TestExceptionConflicts_ModifiedLater_NoWarning(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, date := nextMonday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("ModLate-%d", time.Now().UnixNano()))

	tf := createTestTimeframeRow(t, s.db, "14:00", "15:00")
	createTestActivitySchedule(t, s.db, group.ID, schedule.WeekdayMonday, &tf.ID)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:30", EndHHMM: "15:00", ActivityGroupID: &group.ID,
	})

	student := testpkg.CreateTestStudent(t, s.db, "Emil", fmt.Sprintf("M-%d", time.Now().UnixNano()), "3a")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, "")

	testpkg.CreateTestArrivalSchedule(t, s.db, student.ID, schedule.WeekdayMonday, s.staffID, "13:45")

	// Activity pushed back 14:00 → 14:30 — the 13:45 arrival still precedes.
	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionModified, "14:30", "15:00", "verschoben")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	assert.Empty(t, got.Conflicts)
}

func TestExceptionConflicts_ModifiedRoomOnly_NoWarning(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, date := nextMonday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("ModRoom-%d", time.Now().UnixNano()))

	altRoom := testpkg.CreateTestRoom(t, s.db, fmt.Sprintf("Alt-%d", time.Now().UnixNano()))

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", ActivityGroupID: &group.ID,
	})

	student := testpkg.CreateTestStudent(t, s.db, "Finn", fmt.Sprintf("M-%d", time.Now().UnixNano()), "3a")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, "")

	testpkg.CreateTestArrivalSchedule(t, s.db, student.ID, schedule.WeekdayMonday, s.staffID, "14:30")

	// room-only modified — start_time stays NULL on the exception row.
	roomID := altRoom.ID
	row := &schedule.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   date,
		ExceptionType:   schedule.ActivityExceptionModified,
		RoomID:          &roomID,
	}
	row.SetTenantID(testpkg.Tenant(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.activity_exceptions`).
		Exec(ctx)
	require.NoError(t, err)

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	assert.Empty(t, got.Conflicts)
}

func TestExceptionConflicts_ModifiedEarlier_ArrivalSourceNone_NoWarning(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	// Tuesday: no arrival schedule for the student → source=none.
	dateStr, date := nextTuesday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("ModNone-%d", time.Now().UnixNano()))

	tf := createTestTimeframeRow(t, s.db, "14:00", "15:00")
	createTestActivitySchedule(t, s.db, group.ID, schedule.WeekdayTuesday, &tf.ID)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:30", EndHHMM: "15:00", ActivityGroupID: &group.ID,
	})

	student := testpkg.CreateTestStudent(t, s.db, "Gara", fmt.Sprintf("M-%d", time.Now().UnixNano()), "3a")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, "")

	// No arrival data for this student on Tuesday → source=none.

	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionModified, "13:30", "15:00", "")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	assert.Empty(t, got.Conflicts, "modified + arrival source=none must not emit")
}

func TestExceptionConflicts_ArrivalExceptionAbsence_SkipsBothKinds(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, date := nextMonday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("AbsSkip-%d", time.Now().UnixNano()))

	tf := createTestTimeframeRow(t, s.db, "14:00", "15:00")
	createTestActivitySchedule(t, s.db, group.ID, schedule.WeekdayMonday, &tf.ID)

	// Two instances: one cancelled, one modified — both on the same day, same
	// group is not allowed (unique per template+date), so split across two
	// templates.
	group2 := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("AbsSkip2-%d", time.Now().UnixNano()))
	tf2 := createTestTimeframeRow(t, s.db, "14:00", "15:00")
	createTestActivitySchedule(t, s.db, group2.ID, schedule.WeekdayMonday, &tf2.ID)

	instCancelled := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", ActivityGroupID: &group.ID,
	})
	instModified := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:30", EndHHMM: "15:00", ActivityGroupID: &group2.ID,
	})

	student := testpkg.CreateTestStudent(t, s.db, "Hera", fmt.Sprintf("Abs-%d", time.Now().UnixNano()), "3a")
	testpkg.CreateTestInstanceStudent(t, s.db, instCancelled.ID, student.ID, "")
	testpkg.CreateTestInstanceStudent(t, s.db, instModified.ID, student.ID, "")

	testpkg.CreateTestArrivalSchedule(t, s.db, student.ID, schedule.WeekdayMonday, s.staffID, "13:00")
	// Absence exception — expected_arrival nil, source=exception + IsZero time.
	testpkg.CreateTestArrivalException(t, s.db, student.ID, date, s.staffID, "", "Krank")

	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionCancelled, "", "", "Ausgefallen")
	createTestActivityException(t, s.db, group2.ID, date, schedule.ActivityExceptionModified, "13:30", "15:00", "vorverlegt")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	assert.Empty(t, got.Conflicts,
		"explicit absence via arrival_exception must suppress both conflict kinds")
}

func TestExceptionConflicts_ModifiedNoTemplateSchedule_EmitsWithoutOriginal(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, date := nextMonday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("ModNoTpl-%d", time.Now().UnixNano()))

	// Intentionally no activities.schedules row — resolveOriginalStart
	// returns empty string with a warn log.

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:30", EndHHMM: "15:00", ActivityGroupID: &group.ID,
	})

	student := testpkg.CreateTestStudent(t, s.db, "Ida", fmt.Sprintf("MNT-%d", time.Now().UnixNano()), "3a")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, "")

	testpkg.CreateTestArrivalSchedule(t, s.db, student.ID, schedule.WeekdayMonday, s.staffID, "13:45")

	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionModified, "13:30", "15:00", "")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	require.Len(t, got.Conflicts, 1)
	assert.Equal(t, "13:30", got.Conflicts[0].ModifiedStartTime)
	assert.Empty(t, got.Conflicts[0].OriginalStartTime,
		"missing template schedule → original_start_time must be absent")
}

func TestExceptionConflicts_ModifiedAmbiguousTemplate_EmitsWithoutOriginal(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, date := nextMonday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("ModAmb-%d", time.Now().UnixNano()))

	tfA := createTestTimeframeRow(t, s.db, "09:00", "10:00")
	tfB := createTestTimeframeRow(t, s.db, "14:00", "15:00")
	createTestActivitySchedule(t, s.db, group.ID, schedule.WeekdayMonday, &tfA.ID)
	createTestActivitySchedule(t, s.db, group.ID, schedule.WeekdayMonday, &tfB.ID)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:30", EndHHMM: "15:00", ActivityGroupID: &group.ID,
	})

	student := testpkg.CreateTestStudent(t, s.db, "Jake", fmt.Sprintf("MA-%d", time.Now().UnixNano()), "3a")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, "")

	testpkg.CreateTestArrivalSchedule(t, s.db, student.ID, schedule.WeekdayMonday, s.staffID, "13:45")

	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionModified, "13:30", "15:00", "")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	require.Len(t, got.Conflicts, 1)
	assert.Equal(t, "13:30", got.Conflicts[0].ModifiedStartTime)
	assert.Empty(t, got.Conflicts[0].OriginalStartTime,
		"ambiguous template weekday → original_start_time must be absent")
}

// -----------------------------------------------------------------------------
// Tenant isolation + empty responses
// -----------------------------------------------------------------------------

func TestExceptionConflicts_Empty_NoExceptions(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, _ := nextMonday()
	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	assert.Equal(t, dateStr, got.From)
	assert.Equal(t, dateStr, got.To)
	assert.NotNil(t, got.Conflicts)
	assert.Empty(t, got.Conflicts)
}

func TestExceptionConflicts_ExceptionWithoutInstance_SkipsSilently(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, date := nextMonday()

	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("Orphan-%d", time.Now().UnixNano()))

	// Exception exists but no materialised instance.
	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionCancelled, "", "", "Geplant")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	got := decodeConflicts(t, w)
	assert.Empty(t, got.Conflicts)
}

func TestExceptionConflicts_TenantIsolation(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	dateStr, date := nextMonday()

	// Tenant 1 has the data.
	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("Tiso-%d", time.Now().UnixNano()))

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", ActivityGroupID: &group.ID,
	})

	student := testpkg.CreateTestStudent(t, s.db, "Kim", fmt.Sprintf("ISO-%d", time.Now().UnixNano()), "3a")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, "")

	testpkg.CreateTestArrivalSchedule(t, s.db, student.ID, schedule.WeekdayMonday, s.staffID, "13:00")
	createTestActivityException(t, s.db, group.ID, date, schedule.ActivityExceptionCancelled, "", "", "")

	// Querying tenant 2 must see nothing.
	tenant2Ctx := testpkg.TenantContext(2)
	router := conflictsRouter(tenant2Ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	assert.Empty(t, got.Conflicts, "tenant 1 data must not leak to tenant 2")
}

func TestExceptionConflicts_SortOrder(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()

	// Two groups on the same date, two students each. Deterministic sort by
	// date ASC, activity_group_id ASC, student_id ASC.
	dateStr, date := nextMonday()

	groupA := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("SortA-%d", time.Now().UnixNano()))
	groupB := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("SortB-%d", time.Now().UnixNano()+1))

	instA := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", ActivityGroupID: &groupA.ID,
	})
	instB := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", ActivityGroupID: &groupB.ID,
	})

	suffix := time.Now().UnixNano()
	stu1 := testpkg.CreateTestStudent(t, s.db, "Alpha", fmt.Sprintf("Sort-%d", suffix), "3a")
	stu2 := testpkg.CreateTestStudent(t, s.db, "Beta", fmt.Sprintf("Sort-%d", suffix+1), "3a")

	// Attach both students to both instances.
	rows := []*schedule.InstanceStudent{
		testpkg.CreateTestInstanceStudent(t, s.db, instA.ID, stu1.ID, ""),
		testpkg.CreateTestInstanceStudent(t, s.db, instA.ID, stu2.ID, ""),
		testpkg.CreateTestInstanceStudent(t, s.db, instB.ID, stu1.ID, ""),
		testpkg.CreateTestInstanceStudent(t, s.db, instB.ID, stu2.ID, ""),
	}
	t.Cleanup(func() {
		ids := make([]int64, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
		}
	})

	testpkg.CreateTestArrivalSchedule(t, s.db, stu1.ID, schedule.WeekdayMonday, s.staffID, "13:00")
	testpkg.CreateTestArrivalSchedule(t, s.db, stu2.ID, schedule.WeekdayMonday, s.staffID, "13:00")

	createTestActivityException(t, s.db, groupA.ID, date, schedule.ActivityExceptionCancelled, "", "", "")
	createTestActivityException(t, s.db, groupB.ID, date, schedule.ActivityExceptionCancelled, "", "", "")

	router := conflictsRouter(s.ctx, s.res)
	w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeConflicts(t, w)
	require.Len(t, got.Conflicts, 4)

	// Verify pairwise ordering against the documented rule: date ASC,
	// activity_group_id ASC, student_id ASC. Because groupA/groupB IDs are
	// DB-assigned, we check the invariant directly rather than comparing to
	// a pre-sorted expectations slice.
	for i := range got.Conflicts {
		if i > 0 {
			prev, cur := got.Conflicts[i-1], got.Conflicts[i]
			if prev.Date != cur.Date {
				assert.Less(t, prev.Date, cur.Date, "date order broken at index %d", i)
				continue
			}
			if prev.ActivityGroupID != cur.ActivityGroupID {
				assert.Less(t, prev.ActivityGroupID, cur.ActivityGroupID, "group order broken at index %d", i)
				continue
			}
			assert.Less(t, prev.StudentID, cur.StudentID, "student order broken at index %d", i)
		}
	}
}

// -----------------------------------------------------------------------------
// Validation errors
// -----------------------------------------------------------------------------

func TestExceptionConflicts_ValidationErrors(t *testing.T) {
	t.Parallel()

	s := buildConflictsSetup(t)
	defer s.cleanupFn()
	router := conflictsRouter(s.ctx, s.res)

	t.Run("date missing → 400", func(t *testing.T) {
		w := doConflicts(t, router, "/exception-conflicts")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("date malformed → 400", func(t *testing.T) {
		w := doConflicts(t, router, "/exception-conflicts?date=not-a-date")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("date_to malformed → 400", func(t *testing.T) {
		dateStr, _ := nextMonday()
		w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s&date_to=foo", dateStr))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("date > date_to → 400", func(t *testing.T) {
		_, date := nextMonday()
		after := date.AddDays(3)
		w := doConflicts(t, router, fmt.Sprintf(
			"/exception-conflicts?date=%s&date_to=%s",
			after.Format("2006-01-02"),
			date.Format("2006-01-02"),
		))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("range > 14 days → 400", func(t *testing.T) {
		_, date := nextMonday()
		tooFar := date.AddDays(20)
		w := doConflicts(t, router, fmt.Sprintf(
			"/exception-conflicts?date=%s&date_to=%s",
			date.Format("2006-01-02"),
			tooFar.Format("2006-01-02"),
		))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("date in the past → 400", func(t *testing.T) {
		past := timezone.NewDate(2026, 8, 24).AddDays(-2).String()
		w := doConflicts(t, router, fmt.Sprintf("/exception-conflicts?date=%s", past))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
