package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	supervisiondashboardCompose "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	calendarpkg "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The retained interfaces are wide; the mocks embed them and override only
// what a test calls — an unexpected call panics on the nil embedded interface.
type supervisionMockActive struct {
	studentpresence.Presence
	getActiveGroupsByIDsFn     func(ids []int64) (map[int64]*studentpresence.SessionDetail, error)
	getRoomsByIDsFn            func(ids []int64) ([]*studentpresence.SessionRoomSummary, error)
	getUnclaimedActiveGroupsFn func() ([]*studentpresence.SessionDetail, error)
	getActiveGroupVisitsFn     func(activeGroupID int64) ([]*studentpresence.VisitDisplay, error)
	getAttendanceStatusesFn    func(studentIDs []int64) (map[int64]*studentpresence.DailyAttendanceStatus, error)
}

type supervisionMockRoomSessions struct {
	SupervisionOpenRoomSessions
	list  func() ([]int64, error)
	staff func(int64) ([]int64, error)
}

func (m *supervisionMockRoomSessions) GetStaffActiveGroupIDs(_ context.Context, staffID int64) ([]int64, error) {
	return m.staff(staffID)
}

func (m *supervisionMockRoomSessions) ListRunningSessionIDs(context.Context) ([]int64, error) {
	return m.list()
}

func (m *supervisionMockActive) GetActiveGroupsByIDs(_ context.Context, ids []int64) (map[int64]*studentpresence.SessionDetail, error) {
	return m.getActiveGroupsByIDsFn(ids)
}

func (m *supervisionMockActive) GetRoomsByIDs(_ context.Context, ids []int64) ([]*studentpresence.SessionRoomSummary, error) {
	return m.getRoomsByIDsFn(ids)
}

func (m *supervisionMockActive) GetUnclaimedActiveGroups(_ context.Context) ([]*studentpresence.SessionDetail, error) {
	return m.getUnclaimedActiveGroupsFn()
}

func (m *supervisionMockActive) GetActiveGroupVisitsWithDisplay(_ context.Context, activeGroupID int64) ([]*studentpresence.VisitDisplay, error) {
	return m.getActiveGroupVisitsFn(activeGroupID)
}

func (m *supervisionMockActive) GetStudentsAttendanceStatuses(_ context.Context, studentIDs []int64) (map[int64]*studentpresence.DailyAttendanceStatus, error) {
	return m.getAttendanceStatusesFn(studentIDs)
}

type supervisionMockCaller struct {
	SupervisionCallerContext
	getMySupervisedGroupsFn func() ([]*studentpresence.SessionDetail, error)
	myGroupsFn              func() ([]SupervisionCallerGroup, error)
}

func (m *supervisionMockCaller) GetMySupervisedGroups(_ context.Context) ([]*studentpresence.SessionDetail, error) {
	return m.getMySupervisedGroupsFn()
}

func (m *supervisionMockCaller) MyGroups(_ context.Context) ([]SupervisionCallerGroup, error) {
	return m.myGroupsFn()
}

// supervisionMockEducation answers the School Structure group-room read with
// a fixed group map after its hook accepts the ids; the map type comes from
// the read's own signature, which the root's tests may not import.
type supervisionMockEducation[M any] struct {
	rooms  func(ids []int64) error
	result M
}

func (m *supervisionMockEducation[M]) GetGroupsWithRoomsByIDs(_ context.Context, ids []int64) (M, error) {
	if err := m.rooms(ids); err != nil {
		var none M
		return none, err
	}
	return m.result, nil
}

func supervisionEducationFor[M any](result M, rooms func(ids []int64) error) *supervisionMockEducation[M] {
	return &supervisionMockEducation[M]{rooms: rooms, result: result}
}

// supervisionGroupRoomsOf names the group map type of the education source
// newSupervisionGroups takes, through a zero value of that source.
func supervisionGroupRoomsOf[Q interface {
	GetGroupsWithRoomsByIDs(context.Context, []int64) (M, error)
}, M any](Q) (zero M) {
	return zero
}

func supervisionEducationSourceOf[Q, C, R any](func(Q, C) R) (zero Q) { return zero }

type supervisionMockOperations struct {
	timetable.OperationQuery
	plannedNowFn     func(calendarpkg.Date, time.Time, timetable.PlannedNowOptions) ([]timetable.OperationPlannedInstance, error)
	activeSessionsFn func(calendarpkg.Date) ([]timetable.OperationActiveSession, error)
	sessionBlocksFn  func(int64, bool, calendarpkg.Date, map[int64][]int64) ([]timetable.OperationSessionBlock, error)
}

func (m *supervisionMockOperations) SessionBlocks(_ context.Context, accountID int64, isAdmin bool, day calendarpkg.Date, supervisors map[int64][]int64) ([]timetable.OperationSessionBlock, error) {
	return m.sessionBlocksFn(accountID, isAdmin, day, supervisors)
}

func (m *supervisionMockOperations) PlannedNow(_ context.Context, _ int64, _ bool, day calendarpkg.Date, now time.Time, opts timetable.PlannedNowOptions) ([]timetable.OperationPlannedInstance, error) {
	return m.plannedNowFn(day, now, opts)
}

func (m *supervisionMockOperations) ActiveSessions(_ context.Context, day calendarpkg.Date) ([]timetable.OperationActiveSession, error) {
	return m.activeSessionsFn(day)
}

type supervisionMockPickups struct {
	careplan.BulkPickupDays
	getBulkEffectivePickupTimesForDateFn func([]int64, calendarpkg.Date) (map[int64]*careplan.EffectivePickupTime, error)
}

func (m *supervisionMockPickups) GetBulkEffectivePickupDaysForDate(_ context.Context, studentIDs []int64, date calendarpkg.Date) (map[int64]*careplan.EffectivePickupTime, error) {
	return m.getBulkEffectivePickupTimesForDateFn(studentIDs, date)
}

type supervisionMockArrivals struct {
	careplan.BulkArrivalTimes
	getBulkEffectiveArrivalTimesForDateFn func([]int64, calendarpkg.Date) (map[int64]*careplan.EffectiveArrivalTime, error)
}

func (m *supervisionMockArrivals) GetBulkEffectiveArrivalTimesForDate(_ context.Context, studentIDs []int64, date calendarpkg.Date) (map[int64]*careplan.EffectiveArrivalTime, error) {
	return m.getBulkEffectiveArrivalTimesForDateFn(studentIDs, date)
}

// supervisionRootPorts stands in for the access, settings, yard and staff
// ports; the composition only needs them present.
type supervisionRootPorts struct {
	supervisiondashboard.Access
	supervisiondashboard.Settings
	supervisiondashboard.Yard
	supervisiondashboard.Staff
}

func supervisionFullDependencies() supervisiondashboard.Dependencies {
	active, caller := &supervisionMockActive{}, &supervisionMockCaller{}
	groupRooms := supervisionGroupRoomsOf(supervisionEducationSourceOf(newSupervisionGroups))
	return supervisiondashboard.Dependencies{
		Access:   supervisionRootPorts{},
		Sessions: newSupervisionSessions(active, nil, caller),
		Groups:   newSupervisionGroups(supervisionEducationFor(groupRooms, nil), caller),
		Yard:     supervisionRootPorts{},
		Schedule: newSupervisionSchedule(&supervisionMockOperations{}),
		Presence: newSupervisionPresence(active, nil),
		Settings: supervisionRootPorts{},
		Planning: newSupervisionPlanning(&supervisionMockPickups{}, &supervisionMockArrivals{}),
		Staff:    supervisionRootPorts{},
		Calendar: supervisionCalendar{},
	}
}

func supervisionInt64Ptr(value int64) *int64 { return &value }

func TestSupervisionDashboardRejectsMissingSourcesAtComposition(t *testing.T) {
	t.Parallel()

	query, err := newSupervisionDashboard(supervisiondashboard.Dependencies{})
	assert.ErrorIs(t, err, errSupervisionIncompleteSources, "missing wiring fails composition, not the first request")
	assert.Nil(t, query)

	query, err = newSupervisionDashboard(supervisionFullDependencies())
	require.NoError(t, err)
	assert.NotNil(t, query)

	active, caller := &supervisionMockActive{}, &supervisionMockCaller{}
	education := supervisionEducationFor(supervisionGroupRoomsOf(supervisionEducationSourceOf(newSupervisionGroups)), nil)
	assert.Nil(t, newSupervisionSessions(nil, nil, caller), "a port without its owner source binds none")
	assert.Nil(t, newSupervisionSessions(active, nil, nil))
	assert.Nil(t, newSupervisionGroups(nil, caller))
	assert.Nil(t, newSupervisionGroups(education, nil))
	assert.Nil(t, newSupervisionSchedule(nil))
	assert.Nil(t, newSupervisionPresence(nil, nil))
	assert.Nil(t, newSupervisionPlanning(nil, &supervisionMockArrivals{}))
	assert.Nil(t, newSupervisionPlanning(&supervisionMockPickups{}, nil))
}

func TestSupervisionRunningSessionsResolveRelationsAndRooms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	color := "#83CD2D"

	active := &supervisionMockActive{
		getRoomsByIDsFn: func(ids []int64) ([]*studentpresence.SessionRoomSummary, error) {
			assert.Equal(t, []int64{22}, ids, "rooms are bulk-loaded once per missing relation")
			return []*studentpresence.SessionRoomSummary{{ID: 22, Name: "Adler"}}, nil
		},
		getActiveGroupsByIDsFn: func(ids []int64) (map[int64]*studentpresence.SessionDetail, error) {
			assert.Equal(t, []int64{11, 12, 13}, ids, "ended sessions are not re-read")
			return map[int64]*studentpresence.SessionDetail{
				11: {ID: 11, RoomID: 21, Activity: &studentpresence.SessionActivitySummary{Name: "Malen"}, Room: &studentpresence.SessionRoomSummary{ID: 21, Name: "Zebra", Color: &color}},
				12: {ID: 12, RoomID: 22},
				13: {ID: 13, RoomID: 22},
			}, nil
		},
	}
	roomSessions := &supervisionMockRoomSessions{
		list: func() ([]int64, error) { return []int64{11, 12, 13}, nil },
		staff: func(staffID int64) ([]int64, error) {
			assert.Equal(t, int64(91), staffID)
			return []int64{12}, nil
		},
	}
	s := supervisionSessions{active: active, groups: roomSessions, collate: supervisiondashboardCompose.CompareRoomNames}

	records, err := s.Running(ctx)
	require.NoError(t, err)
	assert.Equal(t, []supervisiondashboard.Session{
		{ID: 12, RoomID: supervisionInt64Ptr(22), RoomName: "Adler"},
		{ID: 13, RoomID: supervisionInt64Ptr(22), RoomName: "Adler"},
		{ID: 11, Name: "Malen", RoomID: supervisionInt64Ptr(21), RoomName: "Zebra", RoomColor: &color},
	}, records, "sessions sort by room name in German dictionary order")

	owned, err := s.SupervisedByStaff(ctx, 91)
	require.NoError(t, err)
	assert.Equal(t, map[int64]struct{}{12: {}}, owned)

	active.getRoomsByIDsFn = func([]int64) ([]*studentpresence.SessionRoomSummary, error) { return nil, errors.New("boom") }
	_, err = s.Running(ctx)
	require.ErrorContains(t, err, "bulk load rooms")

	active.getActiveGroupsByIDsFn = func([]int64) (map[int64]*studentpresence.SessionDetail, error) { return nil, errors.New("boom") }
	_, err = s.Running(ctx)
	require.ErrorContains(t, err, "load active group relations")

	roomSessions.list = func() ([]int64, error) { return nil, errors.New("boom") }
	_, err = s.Running(ctx)
	require.EqualError(t, err, "boom")
}

func TestSupervisionSupervisedSessionsAndUnclaimed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userContext := &supervisionMockCaller{getMySupervisedGroupsFn: func() ([]*studentpresence.SessionDetail, error) {
		return []*studentpresence.SessionDetail{{ID: 11, RoomID: 21}, {ID: 12}}, nil
	}}
	active := &supervisionMockActive{
		getRoomsByIDsFn: func(ids []int64) ([]*studentpresence.SessionRoomSummary, error) {
			assert.Equal(t, []int64{21}, ids)
			return []*studentpresence.SessionRoomSummary{{ID: 21, Name: "Zebra"}}, nil
		},
		getUnclaimedActiveGroupsFn: func() ([]*studentpresence.SessionDetail, error) {
			return []*studentpresence.SessionDetail{
				{ID: 11, Room: &studentpresence.SessionRoomSummary{ID: 21, Name: "Adler"}},
				{ID: 12},
			}, nil
		},
	}
	s := supervisionSessions{active: active, userContext: userContext, collate: supervisiondashboardCompose.CompareRoomNames}

	records, err := s.Supervised(ctx)
	require.NoError(t, err)
	assert.Equal(t, []supervisiondashboard.Session{{ID: 12}, {ID: 11, RoomID: supervisionInt64Ptr(21), RoomName: "Zebra"}}, records,
		"a session without a room sorts ahead of named rooms")

	unclaimed, err := s.Unclaimed(ctx)
	require.NoError(t, err)
	assert.Equal(t, []supervisiondashboard.UnclaimedGroup{{ID: 11, RoomName: "Adler"}, {ID: 12}}, unclaimed)

	userContext.getMySupervisedGroupsFn = func() ([]*studentpresence.SessionDetail, error) { return nil, errors.New("boom") }
	_, err = s.Supervised(ctx)
	require.EqualError(t, err, "boom")
}

func TestSupervisionMyGroupsResolvesRoomsOnlyForGroupsWithRooms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	roomID := int64(31)
	userContext := &supervisionMockCaller{myGroupsFn: func() ([]SupervisionCallerGroup, error) {
		return []SupervisionCallerGroup{
			{ID: 41, Name: "Bären", RoomID: &roomID},
			{ID: 42, Name: "Füchse"},
		}, nil
	}}
	loaded := decodeAs(t, supervisionGroupRoomsOf(supervisionEducationSourceOf(newSupervisionGroups)), `{
		"41": {"id": 41, "room": {"id": 31, "name": "Igel"}},
		"42": null
	}`)
	education := supervisionEducationFor(loaded, func(ids []int64) error {
		assert.Equal(t, []int64{41}, ids)
		return nil
	})
	g := supervisionGroups{education: education, userContext: userContext}

	got, err := g.MyGroups(ctx)
	require.NoError(t, err)
	assert.Equal(t, []supervisiondashboard.EducationalGroup{{ID: 41, Name: "Bären", RoomName: "Igel"}, {ID: 42, Name: "Füchse"}}, got)

	education.rooms = func([]int64) error { return errors.New("boom") }
	_, err = g.MyGroups(ctx)
	require.ErrorContains(t, err, "load education group rooms")

	userContext.myGroupsFn = func() ([]SupervisionCallerGroup, error) { return nil, nil }
	education.rooms = func([]int64) error {
		t.Fatal("no groups with rooms, no room lookup")
		return nil
	}
	got, err = g.MyGroups(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSupervisionPresenceMapsVisitsAndAttendance(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	checkedIn := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	checkedOut := checkedIn.Add(time.Hour)
	sick := true
	photo := "/uploads/student-photos/p.jpg"
	active := &supervisionMockActive{
		getActiveGroupVisitsFn: func(activeGroupID int64) ([]*studentpresence.VisitDisplay, error) {
			assert.Equal(t, int64(55), activeGroupID)
			return []*studentpresence.VisitDisplay{
				{StudentID: 1, ActiveGroupID: 55, EntryTime: checkedIn, FirstName: "Erika", LastName: "Muster", SchoolClass: "4a", OGSGroupName: "Bären", Sick: &sick, SickSince: &checkedIn, PhotoPath: &photo},
				{StudentID: 2, ActiveGroupID: 55, EntryTime: checkedIn, ExitTime: &checkedOut},
				nil,
			}, nil
		},
		getAttendanceStatusesFn: func([]int64) (map[int64]*studentpresence.DailyAttendanceStatus, error) {
			return map[int64]*studentpresence.DailyAttendanceStatus{1: {CheckInTime: &checkedIn, CheckOutTime: &checkedOut}, 2: nil}, nil
		},
	}
	p := supervisionPresence{active: active}

	visits, err := p.GroupVisits(ctx, 55)
	require.NoError(t, err)
	assert.Equal(t, []supervisiondashboard.VisitRecord{
		{StudentID: 1, ActiveGroupID: 55, EntryTime: checkedIn, FirstName: "Erika", LastName: "Muster", SchoolClass: "4a", GroupName: "Bären", Sick: true, SickSince: &checkedIn, PhotoPath: &photo},
		{StudentID: 2, ActiveGroupID: 55, EntryTime: checkedIn, ExitTime: &checkedOut},
	}, visits)

	attendance, err := p.AttendanceTimes(ctx, []int64{1, 2})
	require.NoError(t, err)
	assert.Equal(t, map[int64]supervisiondashboard.Attendance{1: {CheckInTime: &checkedIn, CheckOutTime: &checkedOut}}, attendance)
}

func TestSupervisionPlanningMapsEffectiveTimes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	date := calendarpkg.NewDate(2026, 8, 19)
	pickup := calendarpkg.NormalizeWallClock(time.Date(2026, time.August, 19, 15, 30, 0, 0, time.UTC))
	arrival := calendarpkg.NormalizeWallClock(time.Date(2026, time.August, 19, 11, 45, 0, 0, time.UTC))
	p := supervisionPlanning{
		pickups: &supervisionMockPickups{getBulkEffectivePickupTimesForDateFn: func(studentIDs []int64, gotDate calendarpkg.Date) (map[int64]*careplan.EffectivePickupTime, error) {
			assert.Equal(t, []int64{42}, studentIDs)
			assert.Equal(t, date, gotDate)
			return map[int64]*careplan.EffectivePickupTime{
				42: {Date: date, PickupTime: &pickup, WeekdayName: "Mittwoch", IsException: true, Notes: "Oma", DayNotes: []careplan.NoteData{{ID: 3, Content: "Klingeln"}}, DayDepartureMode: "pickup"},
				43: nil,
			}, nil
		}},
		arrivals: &supervisionMockArrivals{getBulkEffectiveArrivalTimesForDateFn: func([]int64, calendarpkg.Date) (map[int64]*careplan.EffectiveArrivalTime, error) {
			return map[int64]*careplan.EffectiveArrivalTime{
				42: {Date: date, ArrivalTime: &arrival, WeekdayName: "Mittwoch", Notes: "Bus", DayNotes: []careplan.ArrivalNoteData{{ID: 4, Content: "Verspätung"}}},
			}, nil
		}},
	}

	pickups, err := p.Pickups(ctx, []int64{42}, "2026-08-19")
	require.NoError(t, err)
	clock := "15:30"
	assert.Equal(t, map[int64]supervisiondashboard.Pickup{42: {
		Date: "2026-08-19", WeekdayName: "Mittwoch", PickupTime: &clock, IsException: true, Notes: "Oma",
		DayNotes:         []supervisiondashboard.DayNote{{ID: 3, Content: "Klingeln"}},
		DayDepartureMode: "pickup",
	}}, pickups, "nil plans are dropped, not projected as empty; a day Abholart marks the row (#3946)")

	arrivals, err := p.Arrivals(ctx, []int64{42}, "2026-08-19")
	require.NoError(t, err)
	arrivalClock := "11:45"
	assert.Equal(t, map[int64]supervisiondashboard.Arrival{42: {
		Date: "2026-08-19", WeekdayName: "Mittwoch", ArrivalTime: &arrivalClock, Notes: "Bus",
		DayNotes: []supervisiondashboard.DayNote{{ID: 4, Content: "Verspätung"}},
	}}, arrivals)

	_, err = p.Pickups(ctx, nil, "yesterday")
	require.Error(t, err, "an unparsable day never reaches the owner")
}

func TestSupervisionScheduleForwardsQueryAndPreservesWireShape(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, time.August, 19, 12, 0, 0, 0, calendarpkg.Berlin)
	pickup := "15:00"
	note := "Nachmittag"
	activeGroupID := int64(88)
	roomName := "Zebra"
	retained := timetable.OperationPlannedInstance{
		ID: 5, Title: "Malen", Date: "2026-08-19", StartTime: "14:00", EndTime: "15:00", RoomID: 21, RoomName: &roomName, Status: "planned",
		IsOverdue: true, MinutesUntilStart: 3, ExpectedStudentsCount: 4, PresentStudentsCount: 2, NotScheduledCount: 1,
		AssignedStaffIDs: []int64{7, 9}, IsAssigned: true, IsPrimary: true, IsSubstitute: false, IsAbsent: false,
		RosterPreview: []timetable.OperationRosterRow{{
			StudentID: 1, StudentName: "Erika Muster", SchoolClass: "4a", GroupName: "Bären", Planned: true, CurrentlyPresent: true,
			VisitID: supervisionInt64Ptr(77), Status: "present", Substatus: &note, Note: &note, CheckedInAt: &pickup, VisitEntryTime: &pickup, PickupTime: &pickup,
			Warnings:          []timetable.OperationRosterWarning{{Kind: "arrival", Message: "zu früh", ExpectedArrival: &pickup, ExpectedGroupID: supervisionInt64Ptr(3)}},
			ParallelPresentIn: &timetable.OperationParallelPresence{InstanceID: 6, Title: "Basteln", StartTime: "14:00", EndTime: "15:00"},
			CareDayStatus:     "scheduled",
		}},
		PickupTimesLoaded: true, PickupTimesRedacted: false,
		Warnings: []timetable.InstanceConflictWarning{{Kind: "staff", ResourceID: 7, Message: "doppelt", CanOverride: true, Fingerprint: "abc", ConflictingInstanceID: 6, ConflictingTitle: "Basteln", OverlapStart: "14:00", OverlapEnd: "14:30"}},
		CanStart: true, StartAvailableAt: "13:45", StartExpiresAt: "15:00", ActiveGroupID: &activeGroupID, CancelReason: &note,
		PlanningTrackName: &note, PlanningTrackColor: &note, GroupName: &note,
		StaffNames: []timetable.OperationStaffName{{StaffID: 7, DisplayName: "Erika", IsSubstitute: true}},
	}
	minimal := timetable.OperationPlannedInstance{ID: 6}
	operations := &supervisionMockOperations{
		plannedNowFn: func(day calendarpkg.Date, gotNow time.Time, opts timetable.PlannedNowOptions) ([]timetable.OperationPlannedInstance, error) {
			assert.Equal(t, calendarpkg.NewDate(2026, 8, 19), day)
			assert.Equal(t, now, gotNow)
			assert.Equal(t, timetable.PlannedNowOptions{HorizonMinutes: 480, Limit: 5, IncludeRoster: true}, opts)
			return []timetable.OperationPlannedInstance{retained, minimal}, nil
		},
		activeSessionsFn: func(day calendarpkg.Date) ([]timetable.OperationActiveSession, error) {
			assert.Equal(t, calendarpkg.NewDate(2026, 8, 19), day)
			return []timetable.OperationActiveSession{{ActiveGroupID: 88, InstanceID: 5, Title: "Malen", StartTime: "14:00", EndTime: "15:00"}}, nil
		},
		sessionBlocksFn: func(accountID int64, isAdmin bool, day calendarpkg.Date, supervisors map[int64][]int64) ([]timetable.OperationSessionBlock, error) {
			assert.Equal(t, int64(70), accountID)
			assert.True(t, isAdmin)
			assert.Equal(t, calendarpkg.NewDate(2026, 8, 19), day)
			assert.Equal(t, map[int64][]int64{88: {7}}, supervisors)
			return []timetable.OperationSessionBlock{{ActiveGroupID: 88, InstanceID: 5, Title: "Malen", StartTime: "14:00", EndTime: "15:00", IsAssigned: true, CanOperate: true}}, nil
		},
	}
	s := supervisionSchedule{operations: operations}

	planned, err := s.PlannedNow(ctx, supervisiondashboard.PlannedNowQuery{Date: "2026-08-19", Now: now, HorizonMinutes: 480, Limit: 5, IncludeRoster: true})
	require.NoError(t, err)
	require.Len(t, planned, 2)
	assertSupervisionSameJSON(t, retained, planned[0])
	assertSupervisionSameJSON(t, minimal, planned[1])

	sessions, err := s.ActiveSessions(ctx, "2026-08-19")
	require.NoError(t, err)
	assertSupervisionSameJSON(t, []timetable.OperationActiveSession{{ActiveGroupID: 88, InstanceID: 5, Title: "Malen", StartTime: "14:00", EndTime: "15:00"}}, sessions)

	blocks, err := s.SessionBlocks(ctx, supervisiondashboard.SessionBlocksQuery{AccountID: 70, TokenAdmin: true, Date: "2026-08-19", Supervisors: map[int64][]int64{88: {7}}})
	require.NoError(t, err)
	assertSupervisionSameJSON(t, []timetable.OperationSessionBlock{{ActiveGroupID: 88, InstanceID: 5, Title: "Malen", StartTime: "14:00", EndTime: "15:00", IsAssigned: true, CanOperate: true}}, blocks)

	_, err = s.PlannedNow(ctx, supervisiondashboard.PlannedNowQuery{Date: "today"})
	require.Error(t, err)
	_, err = s.SessionBlocks(ctx, supervisiondashboard.SessionBlocksQuery{Date: "today"})
	require.Error(t, err)
}

func TestSupervisionCalendarRendersBerlinDaysAndClocks(t *testing.T) {
	t.Parallel()

	c := supervisionCalendar{}
	fixed := time.Date(2026, time.August, 21, 6, 5, 0, 0, time.UTC)
	assert.Equal(t, supervisiondashboard.Date("2026-08-21"), c.DayOf(fixed))
	assert.Equal(t, "08:05", c.Clock(fixed), "instants render in Berlin summer time")
	assert.Equal(t, time.Saturday, c.Weekday("2026-08-29"))
	assert.Equal(t, supervisiondashboard.Date("2026-08-31"), c.DayOf(time.Date(2026, time.August, 30, 18, 30, 0, 0, time.FixedZone("America/New_York", -4*60*60))))
}
