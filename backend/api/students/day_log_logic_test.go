package students

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dayLogAttendanceRow(checkIn time.Time, checkOut *time.Time) *active.Attendance {
	return &active.Attendance{StudentID: 1, Date: timezone.TodayDate(), CheckInTime: checkIn, CheckOutTime: checkOut}
}

func dayLogStatusRow(status, source string, reportedAt time.Time) *active.StudentStatusDay {
	return &active.StudentStatusDay{StudentID: 1, Date: timezone.TodayDate(), Status: status, Source: source, ReportedAt: reportedAt}
}

func TestClassifyDayLogStudent_PresentWinsAndCarriesHint(t *testing.T) {
	t.Parallel()

	now := time.Now()
	checkOut := now.Add(6 * time.Hour)
	row := dayLogStudent{}
	classifyDayLogStudent(&row,
		[]*active.Attendance{dayLogAttendanceRow(now, &checkOut)},
		[]*active.StudentStatusDay{dayLogStatusRow(active.StudentStatusDaySick, "parent", now)},
		scheduleService.CareDayScheduled,
	)
	assert.Equal(t, dayLogStatusPresent, row.Status)
	assert.Equal(t, "Krankmeldung liegt vor", row.Hint)
	require.NotNil(t, row.CheckInTime)
	require.NotNil(t, row.CheckOutTime)
}

func TestClassifyDayLogStudent_SickBeatsExcused(t *testing.T) {
	t.Parallel()

	now := time.Now()
	row := dayLogStudent{}
	classifyDayLogStudent(&row, nil, []*active.StudentStatusDay{
		dayLogStatusRow(active.StudentStatusDayExcused, "manual", now.Add(time.Hour)),
		dayLogStatusRow(active.StudentStatusDaySick, "parent", now),
	}, "")
	assert.Equal(t, dayLogStatusSick, row.Status)
	assert.Equal(t, "parent", row.Source)
	require.NotNil(t, row.ReportedAt)
}

func TestClassifyDayLogStudent_CancelledCareDayIsSignedOff(t *testing.T) {
	t.Parallel()

	row := dayLogStudent{}
	classifyDayLogStudent(&row, nil, nil, scheduleService.CareDayCancelled)
	assert.Equal(t, dayLogStatusExcused, row.Status)
	assert.Equal(t, dayLogSourceCancelledCareDay, row.Source)
	assert.Equal(t, "Abgemeldet", dayLogStatusLabel(row.Status, row.Source))
}

func TestClassifyDayLogStudent_NotScheduledAndAbsent(t *testing.T) {
	t.Parallel()

	row := dayLogStudent{}
	classifyDayLogStudent(&row, nil, nil, scheduleService.CareDayNotScheduled)
	assert.Equal(t, dayLogStatusNotScheduled, row.Status)
	assert.Equal(t, "Nicht eingeplant", dayLogStatusLabel(row.Status, ""))

	row = dayLogStudent{}
	classifyDayLogStudent(&row, nil, nil, scheduleService.CareDayUnknown)
	assert.Equal(t, dayLogStatusAbsent, row.Status)
	assert.Equal(t, "Abwesend", dayLogStatusLabel(row.Status, ""))
}

func TestParseDayLogDateRejectsHistoryWithoutDatedGroupAssignments(t *testing.T) {
	t.Parallel()

	today := timezone.TodayDate()
	request := httptest.NewRequest("GET", "/day-log?date="+today.AddDays(-1).String(), nil)

	_, err := parseDayLogDate(request, today)
	require.ErrorContains(t, err, "dated group assignments")

	request = httptest.NewRequest("GET", "/day-log?date="+today.String(), nil)
	date, err := parseDayLogDate(request, today)
	require.NoError(t, err)
	assert.Equal(t, today, date)
}

// A request that starts just before Berlin midnight must keep evaluating the
// day it validated: rolling over mid-request would drop the care plan and turn
// every scheduled child into an unexcused absence.
func TestDayLogClock_KeepsRequestDayAcrossMidnightRollover(t *testing.T) {
	t.Parallel()

	beforeMidnight := time.Date(2026, time.July, 26, 23, 59, 59, 0, timezone.Berlin)
	rs := &Resource{ResourceConfig: ResourceConfig{Now: func() time.Time { return beforeMidnight }}}

	clock := rs.dayLogClock()
	date, err := parseDayLogDate(httptest.NewRequest("GET", "/day-log", nil), clock.today)
	require.NoError(t, err)
	assert.Equal(t, timezone.NewDate(2026, time.July, 26), date)

	// The care-plan branch runs after the rollover; the frozen clock still
	// resolves the requested day as live.
	stub := &stubCareDayService{verdicts: map[int64]scheduleService.CareDayStatus{
		10: scheduleService.CareDayNotScheduled,
	}}
	rs.CareDayService = stub
	data := &dayLogData{
		statusByStudent: map[int64][]*active.StudentStatusDay{},
		careDays:        map[int64]scheduleService.CareDayStatus{},
		arrivalTimes:    map[int64]*scheduleService.EffectiveArrivalTime{},
	}
	require.NoError(t, rs.loadDayLogSignOffs(context.Background(), data, []int64{10}, date, clock))
	assert.Equal(t, []int64{10}, stub.askedFor)
	assert.Equal(t, scheduleService.CareDayNotScheduled, data.careDays[10])
}

func TestDayLogArrivalIsStillPending(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 26, 8, 0, 0, 0, timezone.Berlin)
	arrival := timezone.WallClock(now.Add(2 * time.Hour))
	row := dayLogStudent{Status: dayLogStatusAbsent}

	clock := dayLogClock{now: now, today: timezone.DateFromTime(now)}

	assert.True(t, dayLogArrivalIsStillPending(row, scheduleService.CareDayScheduled,
		&scheduleService.EffectiveArrivalTime{ArrivalTime: &arrival}, clock.today, clock))
	assert.False(t, dayLogArrivalIsStillPending(row, scheduleService.CareDayUnknown,
		&scheduleService.EffectiveArrivalTime{ArrivalTime: &arrival}, clock.today, clock))
	assert.False(t, dayLogArrivalIsStillPending(row, scheduleService.CareDayScheduled,
		&scheduleService.EffectiveArrivalTime{ArrivalTime: &arrival}, clock.today.AddDays(-1), clock))
}

func TestBuildDayLogResponse_OmitsStudentBeforeScheduledArrival(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 26, 8, 0, 0, 0, timezone.Berlin)
	date := timezone.DateFromTime(now)
	arrival := timezone.WallClock(now.Add(2 * time.Hour))
	group := &educationModel.Group{Name: "Gruppe A"}
	group.ID = 1
	student := &usersModel.Student{}
	student.ID = 10
	student.GroupID = &group.ID

	response := buildDayLogResponse(date, []*educationModel.Group{group}, &dayLogData{
		studentsByGroup:     map[int64][]*usersModel.Student{group.ID: {student}},
		persons:             map[int64]*usersModel.Person{},
		attendanceByStudent: map[int64][]*active.Attendance{},
		statusByStudent:     map[int64][]*active.StudentStatusDay{},
		careDays:            map[int64]scheduleService.CareDayStatus{student.ID: scheduleService.CareDayScheduled},
		arrivalTimes:        map[int64]*scheduleService.EffectiveArrivalTime{student.ID: {ArrivalTime: &arrival}},
		clock:               dayLogClock{now: now, today: date},
	})

	require.Len(t, response.Groups, 1)
	assert.Empty(t, response.Groups[0].Students)
	assert.Equal(t, 0, response.Groups[0].Counters.Absent)
	assert.Equal(t, 0, response.Groups[0].Counters.Total)
}

// An immediately activated child may already check in, so they stay on the
// roster — but before their enrollment starts there is nothing to be absent
// from. A derived verdict must not be reported or counted; a real check-in
// must.
func TestBuildDayLogResponse_SkipsDerivedVerdictBeforeEnrollmentStart(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, timezone.Berlin)
	date := timezone.DateFromTime(now)
	startsLater := date.AddDays(7)
	group := &educationModel.Group{Name: "Gruppe A"}
	group.ID = 1

	notYetStarted := &usersModel.Student{Status: usersModel.StudentStatusActive, EnrolledFrom: &startsLater}
	notYetStarted.ID = 10
	notYetStarted.GroupID = &group.ID
	checkedIn := &usersModel.Student{Status: usersModel.StudentStatusActive, EnrolledFrom: &startsLater}
	checkedIn.ID = 11
	checkedIn.GroupID = &group.ID

	response := buildDayLogResponse(date, []*educationModel.Group{group}, &dayLogData{
		studentsByGroup: map[int64][]*usersModel.Student{group.ID: {notYetStarted, checkedIn}},
		persons:         map[int64]*usersModel.Person{},
		attendanceByStudent: map[int64][]*active.Attendance{
			checkedIn.ID: {{StudentID: checkedIn.ID, Date: date, CheckInTime: now}},
		},
		statusByStudent: map[int64][]*active.StudentStatusDay{},
		careDays: map[int64]scheduleService.CareDayStatus{
			notYetStarted.ID: scheduleService.CareDayNotScheduled,
		},
		arrivalTimes: map[int64]*scheduleService.EffectiveArrivalTime{},
		clock:        dayLogClock{now: now, today: date},
	})

	require.Len(t, response.Groups, 1)
	require.Len(t, response.Groups[0].Students, 1, "only the child with a recorded fact is reported")
	assert.Equal(t, dayLogStatusPresent, response.Groups[0].Students[0].Status)
	assert.Equal(t, 0, response.Groups[0].Counters.NotScheduled)
	assert.Equal(t, 0, response.Groups[0].Counters.Absent)
	assert.Equal(t, 1, response.Groups[0].Counters.Total)
}

// The roster's enrollment eligibility must be judged against the SAME day the
// request validated. A request that starts at 23:59:59 Berlin and reaches the
// roster query after midnight would otherwise drop a child activated
// immediately for the requested day (enrolled_from still in the future).
func TestLoadDayLogData_PassesFrozenDayToRosterEligibility(t *testing.T) {
	t.Parallel()

	beforeMidnight := time.Date(2026, time.July, 26, 23, 59, 59, 0, timezone.Berlin)
	rs := &Resource{ResourceConfig: ResourceConfig{Now: func() time.Time { return beforeMidnight }}}
	clock := rs.dayLogClock()

	var gotDate, gotToday timezone.Date
	// Returning an error stops loadDayLogData right after the roster query, so
	// the assertion needs no downstream service doubles.
	rosterQueried := errors.New("roster queried")
	rs.PersonService = &userstest.PersonServiceMock{
		GetEligibleGroupStudentsFn: func(_ context.Context, _ []int64, date, today timezone.Date) ([]*usersModel.Student, error) {
			gotDate, gotToday = date, today
			return nil, rosterQueried
		},
	}

	group := &educationModel.Group{Name: "Gruppe A"}
	group.ID = 1
	_, err := rs.loadDayLogData(context.Background(), []*educationModel.Group{group}, clock.today, clock)

	require.ErrorIs(t, err, rosterQueried)
	assert.Equal(t, timezone.NewDate(2026, time.July, 26), gotDate)
	assert.Equal(t, clock.today, gotToday, "eligibility must use the request's frozen day, not the process clock")
}

func TestLoadDayLogSignOffs_UsesCarePlanOnlyForToday(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 26, 8, 0, 0, 0, timezone.Berlin)
	stub := &stubCareDayService{verdicts: map[int64]scheduleService.CareDayStatus{
		10: scheduleService.CareDayNotScheduled,
	}}
	rs := &Resource{ResourceConfig: ResourceConfig{CareDayService: stub}}
	pastData := &dayLogData{
		statusByStudent: map[int64][]*active.StudentStatusDay{},
		careDays:        map[int64]scheduleService.CareDayStatus{},
		arrivalTimes:    map[int64]*scheduleService.EffectiveArrivalTime{},
	}

	clock := dayLogClock{now: now, today: timezone.DateFromTime(now)}

	require.NoError(t, rs.loadDayLogSignOffs(context.Background(), pastData, []int64{10}, clock.today.AddDays(-1), clock))
	assert.Empty(t, stub.askedFor)
	assert.Empty(t, pastData.careDays)

	todayData := &dayLogData{
		statusByStudent: map[int64][]*active.StudentStatusDay{},
		careDays:        map[int64]scheduleService.CareDayStatus{},
		arrivalTimes:    map[int64]*scheduleService.EffectiveArrivalTime{},
	}
	require.NoError(t, rs.loadDayLogSignOffs(context.Background(), todayData, []int64{10}, clock.today, clock))
	assert.Equal(t, []int64{10}, stub.askedFor)
	assert.Equal(t, scheduleService.CareDayNotScheduled, todayData.careDays[10])
}

// The listexport renderers treat a row with GroupTitle as a section MARKER and
// never render its Values — data rows must not carry a GroupTitle (the first
// PDF render shipped empty because every row did).
func TestBuildDayLogExportDocument_UsesMarkerRowConvention(t *testing.T) {
	t.Parallel()

	multi := dayLogResponse{Groups: []dayLogGroup{
		{Name: "Gruppe A", Counters: dayLogCounters{Present: 1, NotScheduled: 2}, Students: []dayLogStudent{{FirstName: "Mia", LastName: "Bauer", Status: dayLogStatusPresent, Label: "Anwesend"}}},
		{Name: "Gruppe B", Students: []dayLogStudent{{FirstName: "Paul", LastName: "Wolf", Status: dayLogStatusAbsent, Label: "Abwesend"}}},
	}}
	doc := buildDayLogExportDocument(multi, timezone.TodayDate())
	require.Len(t, doc.Rows, 4)
	assert.Equal(t, "Gruppe A", doc.Rows[0].GroupTitle)
	assert.Empty(t, doc.Rows[0].Values, "marker rows carry no values")
	assert.Empty(t, doc.Rows[1].GroupTitle, "data rows carry no group title")
	assert.Equal(t, "Bauer, Mia", doc.Rows[1].Values["name"])
	assert.Equal(t, "Gruppe B", doc.Rows[2].GroupTitle)
	assert.Equal(t, "Wolf, Paul", doc.Rows[3].Values["name"])
	assert.Equal(t, "Gruppe A: 1 Anwesend · 0 Krank · 0 Entschuldigt · 0 Klassenfahrt · 2 Nicht eingeplant · 0 Abwesend", doc.Filters[0])

	single := dayLogResponse{Groups: multi.Groups[:1]}
	doc = buildDayLogExportDocument(single, timezone.TodayDate())
	require.Len(t, doc.Rows, 1)
	assert.Empty(t, doc.Rows[0].GroupTitle, "single-group export stays ungrouped")
	assert.Equal(t, "Bauer, Mia", doc.Rows[0].Values["name"])
}

func TestMergeDayLogAttendance_OpenSessionKeepsDepartureOpen(t *testing.T) {
	t.Parallel()

	morning := time.Now().Add(-6 * time.Hour)
	noon := morning.Add(4 * time.Hour)
	noonOut := morning.Add(3 * time.Hour)

	checkIn, checkOut := mergeDayLogAttendance([]*active.Attendance{
		dayLogAttendanceRow(noon, nil), // re-entry, still checked in
		dayLogAttendanceRow(morning, &noonOut),
	})
	assert.Equal(t, morning, checkIn, "earliest arrival wins")
	assert.Nil(t, checkOut, "open session keeps the day open")

	checkIn, checkOut = mergeDayLogAttendance([]*active.Attendance{
		dayLogAttendanceRow(morning, &noonOut),
		dayLogAttendanceRow(noon, &noon),
	})
	assert.Equal(t, morning, checkIn)
	require.NotNil(t, checkOut)
	assert.Equal(t, noon, *checkOut, "latest departure wins")
}
