// Hermetic integration tests for the WP-F2 prerequisite GET /instances
// endpoint. Mirrors gaps_test.go: handler is mounted without the JWT and
// TenantTx middleware; tenant context is injected via a thin wrapper.
package timetablehttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// stubOfferingSources serves the offering-source support from fixed options.
// Its empty-roster explainer applies Enrollment's rule to them: the phase of
// the first selected option explains the empty occurrence, and before that
// phase's service start the occurrence waits for it.
type stubOfferingSources struct {
	options []timetable.OfferingSourceOption
	err     error
}

func (s *stubOfferingSources) ListOfferingSourceOptions(
	_ context.Context,
	_ *int64,
) ([]timetable.OfferingSourceOption, error) {
	return s.options, s.err
}

func (s *stubOfferingSources) CombinedOfferingSourceCounts(
	_ context.Context,
	_ []int64,
	_ *int64,
) (timetable.OfferingSourceCounts, error) {
	return timetable.OfferingSourceCounts{}, nil
}

func (s *stubOfferingSources) EmptyRosterExplainer(
	_ context.Context,
	_ *int64,
) (timetable.EmptyOfferingRosterExplainer, error) {
	if s.err != nil {
		return nil, s.err
	}
	return func(selected []int64, date calendar.Date) *timetable.EmptyOfferingRoster {
		if len(selected) == 0 {
			return nil
		}
		explanation := &timetable.EmptyOfferingRoster{Kind: timetable.EmptyOfferingRosterSourceEmpty}
		for _, option := range s.options {
			if !slices.Contains(selected, option.ID) {
				continue
			}
			explanation.PhaseName = option.PhaseName
			explanation.ServiceStartDate = option.PhaseServiceStart
			if !option.PhaseServiceStart.IsZero() && date.Before(option.PhaseServiceStart) {
				explanation.Kind = timetable.EmptyOfferingRosterBeforeServiceStart
			}
			return explanation
		}
		return explanation
	}, nil
}

func (s *stubOfferingSources) TemplateRosterMaintenance(
	_ context.Context,
	_ []timetable.TemplateRosterMaintenanceQuery,
) (map[int64]timetable.TemplateRosterMaintenance, error) {
	return nil, nil
}

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

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("List-Room-%d", suffix))

	cleanup := func() {
	}

	data := testTimetableData(db)
	res := NewResource(Dependencies{
		Templates:         data,
		TimetableData:     data.TimetableData(),
		ConflictDetection: data.ConflictDetection(),
	})

	return &listSetup{res: res, db: db, ctx: ctx, roomID: room.ID, cleanupFn: cleanup}
}

func listRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
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

func listFutureDate(offsetDays int) (string, calendar.Date) {
	d := calendar.NewDate(2026, 8, 24).AddDays(offsetDays)
	return d.String(), d
}

func TestListInstances_Empty(t *testing.T) {
	t.Parallel()

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

func TestResolveEmptyRosterReason_ExplainsOfferingDerivedEmptyOccurrence(t *testing.T) {
	t.Parallel()

	sourceID := time.Now().UnixNano()
	serviceStart := calendar.NewDate(2026, 8, 13)
	resource := NewResource(Dependencies{OfferingSourceOptions: &stubOfferingSources{
		options: []timetable.OfferingSourceOption{{
			ID:                sourceID,
			PhaseName:         "Schuljahr 2026/27",
			PhaseServiceStart: serviceStart,
		}},
	}})
	periodID := sourceID + 1
	meta := templateMeta{sourceCareOfferingIDs: []int64{sourceID}}

	tests := []struct {
		name     string
		date     calendar.Date
		wantKind string
	}{
		{name: "before service start", date: calendar.NewDate(2026, 8, 10), wantKind: timetable.EmptyOfferingRosterBeforeServiceStart},
		{name: "after start with empty offering source", date: calendar.NewDate(2026, 8, 14), wantKind: timetable.EmptyOfferingRosterSourceEmpty},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := timetable.ScheduledInstance{Date: tt.date, CalendarPeriodID: &periodID}
			reason := resource.resolveEmptyRosterReason(
				context.Background(), instance, meta, nil,
				make(map[int64]timetable.EmptyOfferingRosterExplainer),
			)
			require.NotNil(t, reason)
			assert.Equal(t, tt.wantKind, reason.Kind)
			assert.Equal(t, "Schuljahr 2026/27", reason.PhaseName)
			assert.Equal(t, "2026-08-13", reason.ServiceStartDate)
		})
	}

	populated := resource.resolveEmptyRosterReason(
		context.Background(),
		timetable.ScheduledInstance{Date: calendar.NewDate(2026, 8, 10), CalendarPeriodID: &periodID},
		meta,
		[]timetable.ScheduledParticipant{{StudentID: sourceID + 2}},
		make(map[int64]timetable.EmptyOfferingRosterExplainer),
	)
	assert.Nil(t, populated, "a populated occurrence must not carry an empty-roster explanation")
}

func TestListInstances_ReportsOfferingEmptyRosterReason(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	period := createTemplateTestPeriod(t, s.db, "Tpl-Empty-Reason-Period")
	roomID := s.roomID
	sourceID := time.Now().UnixNano()
	repoFactory := mustTimetableTestRepositories(s.db)
	group, err := repoFactory.Timetable.CreateGroup(s.ctx, timetable.GroupInput{
		Name:                  fmt.Sprintf("Tpl-Empty-Reason-%d", time.Now().UnixNano()),
		MaxParticipants:       25,
		IsOpen:                true,
		IsTemplate:            true,
		CategoryID:            s.category.ID,
		CreatedBy:             &s.staffA,
		PlannedRoomID:         &roomID,
		Type:                  timetable.GroupTypeCare,
		CalendarPeriodID:      &period.ID,
		TargetGroupType:       timetable.TargetGroupTypeOffering,
		SourceCareOfferingIDs: []int64{sourceID},
	})
	require.NoError(t, err)

	date := calendar.NewDate(2026, 8, 10)
	testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title:            group.Name,
		ActivityGroupID:  &group.ID,
		CalendarPeriodID: &period.ID,
	})
	s.res.OfferingSourceOptions = &stubOfferingSources{options: []timetable.OfferingSourceOption{{
		ID:                sourceID,
		PhaseName:         "Schuljahr 2026/27",
		PhaseServiceStart: calendar.NewDate(2026, 8, 13),
	}}}

	router := conversionRouter(s.ctx, s.res)
	w := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/instances?from=%s&to=%s", date, date), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	got := decodeTemplateData[weeklyInstancesResponse](t, w)
	require.Len(t, got.Instances, 1)
	require.NotNil(t, got.Instances[0].CalendarPeriodID)
	assert.Equal(t, period.ID, *got.Instances[0].CalendarPeriodID)
	require.NotNil(t, got.Instances[0].EmptyRosterReason)
	assert.Equal(t, timetable.EmptyOfferingRosterBeforeServiceStart, got.Instances[0].EmptyRosterReason.Kind)
	assert.Equal(t, "2026-08-13", got.Instances[0].EmptyRosterReason.ServiceStartDate)
}

func TestListInstances_HappyPath(t *testing.T) {
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "12:00", EndHHMM: "12:50", Title: "Mensa-List-Test",
	})

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
	assert.Equal(t, timetable.InstanceStatusPlanned, item.Status)
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
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)
	template := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("List-Live-Template-%d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroupWithIDsForTenant(t, s.db, tenant.FromContext(s.ctx), template.ID, s.roomID)
	testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &template.ID,
		ActiveGroupID:   &activeGroup.ID,
		Status:          timetable.InstanceStatusCompleted,
		StartHHMM:       "12:00",
		EndHHMM:         "13:00",
		Title:           "Completed historical bridge",
	})

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	assert.Equal(t, timetable.InstanceStatusCompleted, got.Instances[0].Status)
	assert.False(t, got.Instances[0].IsLive)
}

func TestListInstances_StaffAndStudentCounts(t *testing.T) {
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:00", EndHHMM: "14:00", Title: "Lernzeit-List-Test",
	})

	suffix := time.Now().UnixNano()
	staff1 := testpkg.CreateTestStaff(t, s.db, "Mueller", fmt.Sprintf("ListA-%d", suffix))
	staff2 := testpkg.CreateTestStaff(t, s.db, "Klein", fmt.Sprintf("ListB-%d", suffix))
	student1 := testpkg.CreateTestStudent(t, s.db, "Anna", fmt.Sprintf("Pupil-%d-A", suffix), "1a")
	student2 := testpkg.CreateTestStudent(t, s.db, "Ben", fmt.Sprintf("Pupil-%d-B", suffix), "1a")

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff1.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
	sickRow := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff2.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	sickAbsenceID := sickRow.ID
	sickRow.SickAbsenceID = &sickAbsenceID
	require.NoError(t, mustTimetableTestRepositories(s.db).InstanceStaff.Update(s.ctx, sickRow))
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student1.ID, timetable.SlotAttendanceExpected)
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student2.ID, timetable.SlotAttendancePresent)

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
	assert.True(t, staffByID[staff2.ID].IsSickAbsence)
	assert.ElementsMatch(t, []int64{student1.ID, student2.ID}, item.StudentIDs)
	require.Len(t, item.Students, 2)
	studentByID := map[int64]instanceStudentSummary{}
	for _, row := range item.Students {
		studentByID[row.StudentID] = row
	}
	require.Contains(t, studentByID, student1.ID)
	require.Contains(t, studentByID, student2.ID)
	assert.Equal(t, timetable.SlotAttendanceExpected, studentByID[student1.ID].Status)
	assert.Equal(t, timetable.SlotAttendancePresent, studentByID[student2.ID].Status)
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
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		Status:    timetable.InstanceStatusCompleted,
		StartHHMM: "12:00", EndHHMM: "13:00", Title: "Completed-Freeze-Test",
	})

	suffix := time.Now().UnixNano()
	student1 := testpkg.CreateTestStudent(t, s.db, "Frozen", fmt.Sprintf("Marker-%d-A", suffix), "2b")
	student2 := testpkg.CreateTestStudent(t, s.db, "Was", fmt.Sprintf("There-%d-B", suffix), "2b")
	student3 := testpkg.CreateTestStudent(t, s.db, "Reset", fmt.Sprintf("Expected-%d-C", suffix), "2b")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student1.ID, timetable.SlotAttendanceExpected,
		testpkg.InstanceStudentOpts{NotScheduled: true})
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student2.ID, timetable.SlotAttendancePresent)
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student3.ID, timetable.SlotAttendanceExpected)

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
	careDayByStudent := map[int64]careplan.CareDayStatus{}
	for _, row := range item.Students {
		careDayByStudent[row.StudentID] = row.CareDayStatus
	}
	assert.Equal(t, careplan.CareDayNotScheduled, careDayByStudent[student1.ID])
	assert.Equal(t, careplan.CareDayUnknown, careDayByStudent[student2.ID],
		"a row with a real attendance status tells its own story")
	assert.Equal(t, careplan.CareDayUnknown, careDayByStudent[student3.ID],
		"an unmarked expected row must not be relabelled as never booked")
}

// stubListCareDays reports "not_scheduled" for the listed children on every
// date and says nothing about anybody else.
type stubListCareDays struct{ notScheduled map[int64]bool }

func (s stubListCareDays) ResolveForDate(
	_ context.Context, studentIDs []int64, date calendar.Date,
) (map[int64]careplan.CareDayStatus, error) {
	out := map[int64]careplan.CareDayStatus{}
	for _, id := range studentIDs {
		if s.notScheduled[id] {
			out[id] = careplan.CareDayNotScheduled
		}
	}
	return out, nil
}

func (s stubListCareDays) ResolveForRange(
	_ context.Context, studentIDs []int64, from, to calendar.Date,
) (map[int64]map[calendar.Date]careplan.CareDayStatus, error) {
	out := map[int64]map[calendar.Date]careplan.CareDayStatus{}
	for _, id := range studentIDs {
		byDate := map[calendar.Date]careplan.CareDayStatus{}
		for date := from; !date.After(to); date = date.AddDays(1) {
			if s.notScheduled[id] {
				byDate[date] = careplan.CareDayNotScheduled
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
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "StatusDay-Unbooked-Test",
	})

	suffix := time.Now().UnixNano()
	sickUnbooked := testpkg.CreateTestStudent(t, s.db, "Krank", fmt.Sprintf("Unbooked-%d-A", suffix), "3c")
	manualUnbooked := testpkg.CreateTestStudent(t, s.db, "Manuell", fmt.Sprintf("Unbooked-%d-B", suffix), "3c")
	sickBooked := testpkg.CreateTestStudent(t, s.db, "Krank", fmt.Sprintf("Booked-%d-C", suffix), "3c")

	statusDay := testpkg.CreateTestStudentStatusDay(t, s.db, sickUnbooked.ID, fromDate, absencerecords.StudentStatusDaySick)
	bookedStatusDay := testpkg.CreateTestStudentStatusDay(t, s.db, sickBooked.ID, fromDate, absencerecords.StudentStatusDaySick)
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, sickUnbooked.ID, timetable.SlotAttendanceAbsent,
		testpkg.InstanceStudentOpts{StudentStatusDayID: &statusDay.ID})
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, manualUnbooked.ID, timetable.SlotAttendanceAbsent)
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, sickBooked.ID, timetable.SlotAttendanceAbsent,
		testpkg.InstanceStudentOpts{StudentStatusDayID: &bookedStatusDay.ID})

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

	careDayByStudent := map[int64]careplan.CareDayStatus{}
	for _, row := range item.Students {
		careDayByStudent[row.StudentID] = row.CareDayStatus
	}
	assert.Equal(t, careplan.CareDayNotScheduled, careDayByStudent[sickUnbooked.ID],
		"a status-day absence on a day the plan never booked is a false absence")
	assert.Equal(t, careplan.CareDayUnknown, careDayByStudent[manualUnbooked.ID],
		"a manual absence is a human decision and outranks the plan")
	assert.Equal(t, careplan.CareDayUnknown, careDayByStudent[sickBooked.ID],
		"the child was booked, so their absence is real")
}

func TestListInstances_IncludesWindowConflictWarnings(t *testing.T) {
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()

	// Window detection (#2139): two overlapping instances sharing a child
	// carry MIRRORED student warnings with one shared fingerprint; a third,
	// merely adjacent instance in the same room stays warning-free (shared
	// rooms and pure adjacency are sanctioned).
	from, fromDate := listFutureDate(7)
	to, _ := listFutureDate(8)

	suffix := time.Now().UnixNano()
	student := testpkg.CreateTestStudent(t, s.db, "Conflict-Kid", fmt.Sprintf("List-%d", suffix), "2b")
	instA := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Window-Conflict-A",
	})
	instB := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:30", EndHHMM: "15:30", Title: "Window-Conflict-B",
	})
	testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:30", EndHHMM: "16:30", Title: "Window-Adjacent",
	})
	testpkg.CreateTestInstanceStudent(t, s.db, instA.ID, student.ID, "")
	testpkg.CreateTestInstanceStudent(t, s.db, instB.ID, student.ID, "")

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 3)
	byTitle := map[string]enrichedInstance{}
	for _, item := range got.Instances {
		byTitle[item.Title] = item
	}

	warningsA := byTitle["Window-Conflict-A"].ConflictWarnings
	warningsB := byTitle["Window-Conflict-B"].ConflictWarnings
	require.Len(t, warningsA, 1)
	require.Len(t, warningsB, 1)
	assert.Equal(t, timetable.ConflictKindStudent, warningsA[0].Kind)
	assert.Equal(t, student.ID, warningsA[0].ResourceID)
	assert.True(t, warningsA[0].CanOverride)
	assert.Equal(t, instB.ID, warningsA[0].ConflictingInstanceID)
	assert.Equal(t, instA.ID, warningsB[0].ConflictingInstanceID)
	assert.NotEmpty(t, warningsA[0].Fingerprint)
	assert.Equal(t, warningsA[0].Fingerprint, warningsB[0].Fingerprint,
		"mirrored warnings must share one fingerprint")
	assert.Equal(t, "14:30", warningsA[0].OverlapStart)
	assert.Equal(t, "15:00", warningsA[0].OverlapEnd)

	assert.Empty(t, byTitle["Window-Adjacent"].ConflictWarnings,
		"adjacency and a shared room alone must not warn")
}

func TestListInstances_DateValidation(t *testing.T) {
	t.Parallel()

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

func TestEnforcePlannedEndDefaultsTrue(t *testing.T) {
	t.Parallel()

	res := NewResource(Dependencies{})
	got, err := res.enforcePlannedEnd(context.Background())
	require.NoError(t, err)
	assert.True(t, got)
}

func TestEnforcePlannedEndPropagatesResolveError(t *testing.T) {
	t.Parallel()

	res := NewResource(Dependencies{
		SettingsService: &configtest.Mock{
			ResolveBoolFn: func(context.Context, string) (bool, error) {
				return false, errors.New("settings down")
			},
		},
	})
	_, err := res.enforcePlannedEnd(context.Background())
	require.ErrorIs(t, err, timetable.ErrLifecycleSettings)
}

func TestListInstances_IsLive(t *testing.T) {
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(0)
	to, _ := listFutureDate(7)

	suffix := time.Now().UnixNano()
	// An open activity group with its own category and creator (max 20).
	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("Live-Group-%d", suffix))

	activeGroup := testpkg.CreateTestActiveGroup(t, s.db, group.ID, s.roomID)
	testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM:     "10:00",
		EndHHMM:       "11:00",
		Title:         "Live-Test",
		Status:        timetable.InstanceStatusActive,
		ActiveGroupID: &activeGroup.ID,
	})

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	assert.True(t, got.Instances[0].IsLive, "instance with active_group_id should be live")
	assert.Equal(t, timetable.InstanceStatusActive, got.Instances[0].Status)
	assert.NotEmpty(t, got.Instances[0].CompleteAvailableAt)
}

func TestListInstances_SortedByDateAndStartTime(t *testing.T) {
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	day2 := fromDate.AddDays(1)

	// Insert in non-chronological order to exercise the repo's ORDER BY.
	testpkg.CreateTestActivityInstance(t, s.db, day2, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "12:00", EndHHMM: "13:00", Title: "Day2-12",
	})
	testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "12:00", EndHHMM: "13:00", Title: "Day1-12",
	})
	testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Day1-14",
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
	t.Parallel()

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

	suffix := time.Now().UnixNano()
	staff1 := testpkg.CreateTestStaff(t, s.db, "Assigned", fmt.Sprintf("CapA-%d", suffix))
	staff2 := testpkg.CreateTestStaff(t, s.db, "Absent", fmt.Sprintf("CapB-%d", suffix))
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff1.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff2.ID, testpkg.InstanceStaffOpts{IsAbsent: true})

	// 5 children (3 expected + 2 present) at a ratio of 2 -> ceil(5/2) = 3
	// required staff, against 1 actually assigned (staff2 is absent) -> understaffed.
	personIDs := []int64{staff1.ID, staff2.ID}
	for i := 0; i < 5; i++ {
		status := timetable.SlotAttendanceExpected
		if i%2 == 0 {
			status = timetable.SlotAttendancePresent
		}
		student := testpkg.CreateTestStudent(t, s.db, "Cap", fmt.Sprintf("Cap-%d-%d", suffix, i), "1a")
		testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, status)
		personIDs = append(personIDs, student.ID)
	}

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
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	suffix := time.Now().UnixNano()
	seriesNote := "Raum erst ab 14 Uhr offen"
	// An open template group (own category and creator, max 20) carrying the
	// durable Wochennotiz.
	group := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("SeriesNote-Template-%d", suffix))
	_, err := s.db.NewUpdate().
		TableExpr("activities.groups").
		Set("is_template = TRUE").
		Set("notes = ?", seriesNote).
		Where("id = ?", group.ID).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Exec(s.ctx)
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
		TableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("notes = ?", dayNote).
		Where(`"activity_instance".id = ?`, inst.ID).
		Where(`"activity_instance".tenant_id = ?`, testpkg.Tenant(t)).
		Exec(s.ctx)
	require.NoError(t, err)

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
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)

	testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "12:00", EndHHMM: "12:50", Title: "Spontan",
	})

	router := listRouter(s.ctx, s.res)
	w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s", from, to))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeList(t, w)
	require.Len(t, got.Instances, 1)
	assert.Nil(t, got.Instances[0].SeriesNotes)
}
