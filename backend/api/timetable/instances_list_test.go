// Hermetic integration tests for the WP-F2 prerequisite GET /instances
// endpoint. Mirrors gaps_test.go: handler is mounted without the JWT and
// TenantTx middleware; tenant context is injected via a thin wrapper.
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
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type listSetup struct {
	res       *Resource
	db        *bun.DB
	ctx       context.Context
	roomID    int64
	cleanupFn func()
}

func buildListSetup(t *testing.T) *listSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("List-Room-%d", suffix))

	cleanup := func() {
		testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)
	}

	res := NewResource(Dependencies{
		TimetableData: testTimetableData(db),
		DB:            db,
	})

	return &listSetup{res: res, db: db, ctx: ctx, roomID: room.ID, cleanupFn: cleanup}
}

func listRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/instances", res.listInstances)
	return r
}

func doList(t *testing.T, router chi.Router, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeList(t *testing.T, w *httptest.ResponseRecorder) weeklyInstancesResponse {
	t.Helper()
	var env struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "body=%s", w.Body.String())
	require.Equal(t, "success", env.Status, "body=%s", w.Body.String())
	var out weeklyInstancesResponse
	require.NoError(t, json.Unmarshal(env.Data, &out))
	return out
}

func listFutureDate(offsetDays int) (string, timezone.Date) {
	d := timezone.TodayDate().AddDays(offsetDays)
	return d.String(), d
}

func TestListInstances_Empty(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()
	router := listRouter(s.ctx, s.res)

	from, _ := listFutureDate(1)
	to, _ := listFutureDate(7)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	assert.Equal(t, from, got.From)
	assert.Equal(t, to, got.To)
	assert.Empty(t, got.Instances)
}

func TestListInstances_HappyPath(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "12:00", EndHHMM: "12:50", Title: "Mensa-List-Test",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	item := got.Instances[0]
	assert.Equal(t, inst.ID, item.ID)
	assert.Equal(t, "Mensa-List-Test", item.Title)
	assert.Equal(t, "12:00", item.StartTime)
	assert.Equal(t, "12:50", item.EndTime)
	assert.Equal(t, schedule.InstanceStatusPlanned, item.Status)
	assert.False(t, item.IsLive, "no active group bridged → not live")
	assert.False(t, item.IsSpontaneous)
	// Spontaneous instances without an activity_group fall back to "activity"
	// type so the frontend has a deterministic colour key.
	assert.NotEmpty(t, item.ActivityType)
	assert.Equal(t, s.roomID, item.RoomID)
	assert.NotEmpty(t, item.RoomName, "room name should resolve via RoomRepo")
	assert.Empty(t, item.Staff)
	assert.Empty(t, item.Students)
	assert.Empty(t, item.ConflictWarnings)
	assert.Equal(t, 0, item.StaffCount)
	assert.Equal(t, 0, item.ExpectedStudentsCount)
	assert.Equal(t, 0, item.PresentStudentsCount)
}

func TestListInstances_CompletedBridgeIsNotLive(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)
	template := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("List-Live-Template-%d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroupWithIDsForTenant(t, s.db, tenant.FromContext(s.ctx), template.ID, s.roomID)
	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &template.ID,
		ActiveGroupID:   &activeGroup.ID,
		Status:          schedule.InstanceStatusCompleted,
		StartHHMM:       "12:00",
		EndHHMM:         "13:00",
		Title:           "Completed historical bridge",
	})
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID)
		testpkg.CleanupTableRecords(t, s.db, "active.groups", activeGroup.ID)
		testpkg.CleanupActivityFixtures(t, s.db, template.ID)
	})

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	assert.Equal(t, schedule.InstanceStatusCompleted, got.Instances[0].Status)
	assert.False(t, got.Instances[0].IsLive)
}

func TestListInstances_StaffAndStudentCounts(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:00", EndHHMM: "14:00", Title: "Lernzeit-List-Test",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	suffix := time.Now().UnixNano()
	staff1 := testpkg.CreateTestStaff(t, s.db, "Mueller", fmt.Sprintf("ListA-%d", suffix))
	staff2 := testpkg.CreateTestStaff(t, s.db, "Klein", fmt.Sprintf("ListB-%d", suffix))
	student1 := testpkg.CreateTestStudent(t, s.db, "Anna", fmt.Sprintf("Pupil-%d-A", suffix), "1a")
	student2 := testpkg.CreateTestStudent(t, s.db, "Ben", fmt.Sprintf("Pupil-%d-B", suffix), "1a")

	row1 := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff1.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
	row2 := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff2.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	is1 := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student1.ID, schedule.AttendanceStatusExpected)
	is2 := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student2.ID, schedule.AttendanceStatusPresent)

	t.Cleanup(func() {
		testpkg.CleanupInstanceStaffFixtures(t, s.db, row1.ID, row2.ID)
		testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", is1.ID, is2.ID)
		testpkg.CleanupActivityFixtures(t, s.db, student1.ID, staff1.ID, 0, 0, 0)
		testpkg.CleanupActivityFixtures(t, s.db, student2.ID, staff2.ID, 0, 0, 0)
	})

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	item := got.Instances[0]
	assert.Equal(t, 2, item.StaffCount, "both staff rows are counted")
	assert.Equal(t, 1, item.AbsentStaffCount, "is_absent staff are flagged in the count")
	assert.Equal(t, 1, item.ExpectedStudentsCount, "AttendanceStatusExpected counted as expected")
	assert.Equal(t, 1, item.PresentStudentsCount, "AttendanceStatusPresent counted as present")
	require.Len(t, item.Staff, 2)
	// Order is non-deterministic; assert presence by id.
	staffByID := map[int64]instanceStaffSummary{}
	for _, s := range item.Staff {
		staffByID[s.StaffID] = s
	}
	require.Contains(t, staffByID, staff1.ID)
	require.Contains(t, staffByID, staff2.ID)
	assert.True(t, staffByID[staff1.ID].IsPrimary)
	assert.False(t, staffByID[staff1.ID].IsAbsent)
	assert.True(t, staffByID[staff2.ID].IsAbsent)
	assert.ElementsMatch(t, []int64{student1.ID, student2.ID}, item.StudentIDs)
	require.Len(t, item.Students, 2)
	studentByID := map[int64]instanceStudentSummary{}
	for _, row := range item.Students {
		studentByID[row.StudentID] = row
	}
	require.Contains(t, studentByID, student1.ID)
	require.Contains(t, studentByID, student2.ID)
	assert.Equal(t, schedule.AttendanceStatusExpected, studentByID[student1.ID].Status)
	assert.Equal(t, schedule.AttendanceStatusPresent, studentByID[student2.ID].Status)
}

// Completion flips every genuinely expected row to 'absent' and stamps the
// not_scheduled marker on the children it spares; that stored marker is the
// frozen "war an dem Tag nicht eingeplant" verdict (#1747). The list endpoint
// must classify it from that column alone — here the care-day lookup reads
// "unknown" (nil service), which on a planned instance would count as
// expected. If the current care plan leaked into the classification, a later
// plan edit could retroactively change a completed instance's expected count
// and required staffing. An unmarked 'expected' row is NOT the marker: only
// the two completion paths write it, so anything else (a reset through the
// attendance PATCH) has to keep counting as a real expectation.
func TestListInstances_CompletedExpectedRowStaysNotScheduled(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		Status:    schedule.InstanceStatusCompleted,
		StartHHMM: "12:00", EndHHMM: "13:00", Title: "Completed-Freeze-Test",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	suffix := time.Now().UnixNano()
	student1 := testpkg.CreateTestStudent(t, s.db, "Frozen", fmt.Sprintf("Marker-%d-A", suffix), "2b")
	student2 := testpkg.CreateTestStudent(t, s.db, "Was", fmt.Sprintf("There-%d-B", suffix), "2b")
	student3 := testpkg.CreateTestStudent(t, s.db, "Reset", fmt.Sprintf("Expected-%d-C", suffix), "2b")
	is1 := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student1.ID, schedule.AttendanceStatusExpected,
		testpkg.InstanceStudentOpts{NotScheduled: true})
	is2 := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student2.ID, schedule.AttendanceStatusPresent)
	is3 := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student3.ID, schedule.AttendanceStatusExpected)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", is1.ID, is2.ID, is3.ID)
		testpkg.CleanupActivityFixtures(t, s.db, student1.ID, 0, 0, 0, 0)
		testpkg.CleanupActivityFixtures(t, s.db, student2.ID, 0, 0, 0, 0)
		testpkg.CleanupActivityFixtures(t, s.db, student3.ID, 0, 0, 0, 0)
	})

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	item := got.Instances[0]
	assert.Equal(t, 1, item.ExpectedStudentsCount,
		"an unmarked 'expected' row is a real expectation, not a non-booking")
	assert.Equal(t, 1, item.NotScheduledCount,
		"the frozen marker is reported as not scheduled")
	assert.Equal(t, 1, item.PresentStudentsCount)

	// The per-child rows must carry the same verdict the counts used, or the
	// planner lists a child under "Erwartet" that its own header count leaves
	// out — and offers "abmelden" for a day that was never care (#1747 review).
	careDayByStudent := map[int64]scheduleSvc.CareDayStatus{}
	for _, row := range item.Students {
		careDayByStudent[row.StudentID] = row.CareDayStatus
	}
	assert.Equal(t, scheduleSvc.CareDayNotScheduled, careDayByStudent[student1.ID])
	assert.Equal(t, scheduleSvc.CareDayUnknown, careDayByStudent[student2.ID],
		"a row with a real attendance status tells its own story")
	assert.Equal(t, scheduleSvc.CareDayUnknown, careDayByStudent[student3.ID],
		"an unmarked expected row must not be relabelled as never booked")
}

// stubListCareDays reports "not_scheduled" for the listed children on every
// date and says nothing about anybody else.
type stubListCareDays struct{ notScheduled map[int64]bool }

func (s stubListCareDays) ResolveForDate(
	_ context.Context, studentIDs []int64, date timezone.Date,
) (map[int64]scheduleSvc.CareDayStatus, error) {
	out := map[int64]scheduleSvc.CareDayStatus{}
	for _, id := range studentIDs {
		if s.notScheduled[id] {
			out[id] = scheduleSvc.CareDayNotScheduled
		}
	}
	return out, nil
}

func (s stubListCareDays) ResolveForRange(
	_ context.Context, studentIDs []int64, from, to timezone.Date,
) (map[int64]map[timezone.Date]scheduleSvc.CareDayStatus, error) {
	out := map[int64]map[timezone.Date]scheduleSvc.CareDayStatus{}
	for _, id := range studentIDs {
		byDate := map[timezone.Date]scheduleSvc.CareDayStatus{}
		for date := from; !date.After(to); date = date.AddDays(1) {
			if s.notScheduled[id] {
				byDate[date] = scheduleSvc.CareDayNotScheduled
			}
		}
		out[id] = byDate
	}
	return out, nil
}

// A sick / excused / class-trip report flips every still-expected slot of the
// day to 'absent' — including the slots of children the care plan never booked
// that weekday, because ApplyStatusDay runs long before anything asks the plan.
// Until the block ends and MarkNotScheduled undoes it, that row is an absence
// from care that was never owed. The parent calendar and the session-end bridge
// both already treat it as one; the planner list has to agree, or the same row
// reads as an ordinary absence here and as a non-booking everywhere else
// (#1747 review).
//
// A manual absence on the same unbooked day is the counter-case: somebody
// decided it (student_status_day_id is NULL), so it stays an absence.
func TestListInstances_StatusDayAbsenceOnUnbookedDayReadsAsNotScheduled(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "StatusDay-Unbooked-Test",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	suffix := time.Now().UnixNano()
	sickUnbooked := testpkg.CreateTestStudent(t, s.db, "Krank", fmt.Sprintf("Unbooked-%d-A", suffix), "3c")
	manualUnbooked := testpkg.CreateTestStudent(t, s.db, "Manuell", fmt.Sprintf("Unbooked-%d-B", suffix), "3c")
	sickBooked := testpkg.CreateTestStudent(t, s.db, "Krank", fmt.Sprintf("Booked-%d-C", suffix), "3c")

	statusDay := testpkg.CreateTestStudentStatusDay(t, s.db, sickUnbooked.ID, fromDate, activeModels.StudentStatusDaySick)
	bookedStatusDay := testpkg.CreateTestStudentStatusDay(t, s.db, sickBooked.ID, fromDate, activeModels.StudentStatusDaySick)
	is1 := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, sickUnbooked.ID, schedule.AttendanceStatusAbsent,
		testpkg.InstanceStudentOpts{StudentStatusDayID: &statusDay.ID})
	is2 := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, manualUnbooked.ID, schedule.AttendanceStatusAbsent)
	is3 := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, sickBooked.ID, schedule.AttendanceStatusAbsent,
		testpkg.InstanceStudentOpts{StudentStatusDayID: &bookedStatusDay.ID})
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", is1.ID, is2.ID, is3.ID)
		testpkg.CleanupStudentStatusDays(t, s.db, statusDay.ID, bookedStatusDay.ID)
		testpkg.CleanupActivityFixtures(t, s.db, sickUnbooked.ID, 0, 0, 0, 0)
		testpkg.CleanupActivityFixtures(t, s.db, manualUnbooked.ID, 0, 0, 0, 0)
		testpkg.CleanupActivityFixtures(t, s.db, sickBooked.ID, 0, 0, 0, 0)
	})

	s.res.CareDayService = stubListCareDays{notScheduled: map[int64]bool{
		sickUnbooked.ID:   true,
		manualUnbooked.ID: true,
	}}

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	item := got.Instances[0]
	assert.Equal(t, 1, item.NotScheduledCount,
		"only the status-day absence on an unbooked day is a non-booking")
	assert.Equal(t, 0, item.ExpectedStudentsCount)
	assert.Equal(t, 0, item.PresentStudentsCount)

	careDayByStudent := map[int64]scheduleSvc.CareDayStatus{}
	for _, row := range item.Students {
		careDayByStudent[row.StudentID] = row.CareDayStatus
	}
	assert.Equal(t, scheduleSvc.CareDayNotScheduled, careDayByStudent[sickUnbooked.ID],
		"a status-day absence on a day the plan never booked is a false absence")
	assert.Equal(t, scheduleSvc.CareDayUnknown, careDayByStudent[manualUnbooked.ID],
		"a manual absence is a human decision and outranks the plan")
	assert.Equal(t, scheduleSvc.CareDayUnknown, careDayByStudent[sickBooked.ID],
		"the child was booked, so their absence is real")
}

func TestListInstances_IncludesPlannedConflictWarnings(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()

	fromDate := timezone.TodayDate()
	from := fromDate.String()
	toDate := fromDate.AddDays(7)
	to := toDate.String()

	suffix := time.Now().UnixNano()
	category := testpkg.CreateTestActivityCategory(t, s.db, fmt.Sprintf("Conflict-Category-%d", suffix))
	staff := testpkg.CreateTestStaff(t, s.db, "Conflict", fmt.Sprintf("Creator-%d", suffix))
	group := &activitiesModels.Group{
		Name:            fmt.Sprintf("Conflict-Group-%d", suffix),
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
		CreatedBy:       &staff.ID,
	}
	group.SetTenantID(1)
	_, err := s.db.NewInsert().
		Model(group).
		ModelTableExpr(`activities.groups AS "group"`).
		Exec(s.ctx)
	require.NoError(t, err)

	activeGroup := testpkg.CreateTestActiveGroup(t, s.db, group.ID, s.roomID)
	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00", EndHHMM: "16:00", Title: "Room-Conflict-List-Test",
	})

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID)
		testpkg.CleanupTableRecords(t, s.db, "active.groups", activeGroup.ID)
		testpkg.CleanupTableRecords(t, s.db, "activities.groups", group.ID)
		testpkg.CleanupTableRecords(t, s.db, "activities.categories", category.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.staff", staff.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.persons", staff.PersonID)
	})

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	warnings := got.Instances[0].ConflictWarnings
	require.Len(t, warnings, 1)
	assert.Equal(t, "room", warnings[0].Kind)
	assert.Equal(t, s.roomID, warnings[0].ResourceID)
	assert.True(t, warnings[0].CanOverride)
}

func TestListInstances_DateValidation(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()
	router := listRouter(s.ctx, s.res)

	cases := []struct {
		name string
		path string
	}{
		{"missing from", "/instances?to=2026-09-26"},
		{"missing to", "/instances?from=2026-09-22"},
		{"invalid from format", "/instances?from=22-09-2026&to=2026-09-26"},
		{"invalid to format", "/instances?from=2026-09-22&to=09-26-2026"},
		{"to before from", "/instances?from=2026-09-26&to=2026-09-22"},
		{"range exceeds 56 days", "/instances?from=2026-01-01&to=2026-04-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doList(t, router, tc.path)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
}

func TestListInstances_IsLive(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(0)
	to, _ := listFutureDate(7)

	suffix := time.Now().UnixNano()
	category := testpkg.CreateTestActivityCategory(t, s.db, fmt.Sprintf("Live-Category-%d", suffix))
	staff := testpkg.CreateTestStaff(t, s.db, "Live", fmt.Sprintf("Creator-%d", suffix))
	group := &activitiesModels.Group{
		Name:            fmt.Sprintf("Live-Group-%d", suffix),
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
		CreatedBy:       &staff.ID,
	}
	group.SetTenantID(1)
	_, err := s.db.NewInsert().
		Model(group).
		ModelTableExpr(`activities.groups AS "group"`).
		Exec(s.ctx)
	require.NoError(t, err)

	activeGroup := testpkg.CreateTestActiveGroup(t, s.db, group.ID, s.roomID)
	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM:     "10:00",
		EndHHMM:       "11:00",
		Title:         "Live-Test",
		Status:        schedule.InstanceStatusActive,
		ActiveGroupID: &activeGroup.ID,
	})
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID)
		testpkg.CleanupTableRecords(t, s.db, "active.groups", activeGroup.ID)
		testpkg.CleanupTableRecords(t, s.db, "activities.groups", group.ID)
		testpkg.CleanupTableRecords(t, s.db, "activities.categories", category.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.staff", staff.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.persons", staff.PersonID)
	})

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	assert.True(t, got.Instances[0].IsLive, "instance with active_group_id should be live")
	assert.Equal(t, schedule.InstanceStatusActive, got.Instances[0].Status)
}

func TestListInstances_SortedByDateAndStartTime(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	day2 := fromDate.AddDays(1)

	// Insert in non-chronological order to exercise the repo's ORDER BY.
	inst3 := testpkg.CreateTestActivityInstance(t, s.db, day2, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "12:00", EndHHMM: "13:00", Title: "Day2-12",
	})
	inst1 := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "12:00", EndHHMM: "13:00", Title: "Day1-12",
	})
	inst2 := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Day1-14",
	})
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst1.ID, inst2.ID, inst3.ID)
	})

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 3)
	assert.Equal(t, "Day1-12", got.Instances[0].Title)
	assert.Equal(t, "Day1-14", got.Instances[1].Title)
	assert.Equal(t, "Day2-12", got.Instances[2].Title)
}

func TestListInstances_CapacityFields(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()

	// Tenant-override ratio of 2 children per staff member, exercised via the
	// same HasTenantOverride -> ResolveInt chain the real settings service
	// uses (settings-system.md), not just the registry default.
	s.res.SettingsService = &configtest.Mock{
		HasTenantOverrideFn: func(context.Context, string) (bool, error) { return true, nil },
		ResolveIntFn:        func(context.Context, string) (int, error) { return 2, nil },
	}

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:00", EndHHMM: "14:00", Title: "Capacity-List-Test",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	suffix := time.Now().UnixNano()
	staff1 := testpkg.CreateTestStaff(t, s.db, "Assigned", fmt.Sprintf("CapA-%d", suffix))
	staff2 := testpkg.CreateTestStaff(t, s.db, "Absent", fmt.Sprintf("CapB-%d", suffix))
	row1 := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff1.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
	row2 := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff2.ID, testpkg.InstanceStaffOpts{IsAbsent: true})

	// 5 children (3 expected + 2 present) at a ratio of 2 -> ceil(5/2) = 3
	// required staff, against 1 actually assigned (staff2 is absent) -> understaffed.
	personIDs := []int64{staff1.ID, staff2.ID}
	instanceStudentIDs := make([]int64, 0, 5)
	for i := 0; i < 5; i++ {
		status := schedule.AttendanceStatusExpected
		if i%2 == 0 {
			status = schedule.AttendanceStatusPresent
		}
		student := testpkg.CreateTestStudent(t, s.db, "Cap", fmt.Sprintf("Cap-%d-%d", suffix, i), "1a")
		is := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, status)
		personIDs = append(personIDs, student.ID)
		instanceStudentIDs = append(instanceStudentIDs, is.ID)
	}

	t.Cleanup(func() {
		testpkg.CleanupInstanceStaffFixtures(t, s.db, row1.ID, row2.ID)
		testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", instanceStudentIDs...)
		testpkg.CleanupActivityFixtures(t, s.db, personIDs...)
	})

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	item := got.Instances[0]
	assert.Equal(t, 3, item.RequiredStaffCount, "ceil(5 children / ratio 2) = 3")
	assert.Equal(t, 1, item.AssignedStaffCount, "1 non-absent staff assigned")
}

// TestListInstances_SeriesNotesJoinedFromTemplate verifies the durable
// Wochennotiz on a template is joined onto every instance at read time
// (series_notes), independently of the per-occurrence Tagesnotiz (notes).
func TestListInstances_SeriesNotesJoinedFromTemplate(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	suffix := time.Now().UnixNano()
	category := testpkg.CreateTestActivityCategory(t, s.db, fmt.Sprintf("SeriesNote-Cat-%d", suffix))
	staff := testpkg.CreateTestStaff(t, s.db, "SeriesNote", fmt.Sprintf("Creator-%d", suffix))
	seriesNote := "Raum erst ab 14 Uhr offen"
	group := &activitiesModels.Group{
		Name:            fmt.Sprintf("SeriesNote-Template-%d", suffix),
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
		CreatedBy:       &staff.ID,
		IsTemplate:      true,
		Notes:           &seriesNote,
	}
	group.SetTenantID(1)
	_, err := s.db.NewInsert().Model(group).ModelTableExpr(`activities.groups AS "group"`).Exec(s.ctx)
	require.NoError(t, err)

	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &group.ID,
		StartHHMM:       "14:00",
		EndHHMM:         "15:00",
		Title:           "Betreuungsblock",
	})

	// Give the occurrence its own one-off Tagesnotiz to prove independence.
	dayNote := "Heute ohne Herrn Müller"
	_, err = s.db.NewUpdate().
		Model((*schedule.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("notes = ?", dayNote).
		Where(`"activity_instance".id = ?`, inst.ID).
		Where(`"activity_instance".tenant_id = ?`, 1).
		Exec(s.ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID)
		testpkg.CleanupTableRecords(t, s.db, "activities.groups", group.ID)
		testpkg.CleanupTableRecords(t, s.db, "activities.categories", category.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.staff", staff.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.persons", staff.PersonID)
	})

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	item := got.Instances[0]
	require.NotNil(t, item.SeriesNotes, "series note must be joined from the template")
	assert.Equal(t, seriesNote, *item.SeriesNotes)
	require.NotNil(t, item.Notes, "per-occurrence Tagesnotiz must remain independent")
	assert.Equal(t, dayNote, *item.Notes)
}

// TestListInstances_NoSeriesNotesWhenTemplateHasNone confirms an instance whose
// template carries no Wochennotiz omits series_notes.
func TestListInstances_NoSeriesNotesWhenTemplateHasNone(t *testing.T) {
	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "12:00", EndHHMM: "12:50", Title: "Spontan",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	assert.Nil(t, got.Instances[0].SeriesNotes)
}
