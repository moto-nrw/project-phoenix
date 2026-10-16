package legacy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	educationModels "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	calendarpkg "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The retained interfaces are wide; the mocks embed them and override only
// what a test calls — an unexpected call panics on the nil embedded interface.
type mockActiveService struct {
	studentpresence.Presence
	getActiveGroupsByIDsFn     func(ids []int64) (map[int64]*studentpresence.SessionDetail, error)
	getRoomsByIDsFn            func(ids []int64) ([]*studentpresence.SessionRoomSummary, error)
	getUnclaimedActiveGroupsFn func() ([]*studentpresence.SessionDetail, error)
	getActiveGroupVisitsFn     func(activeGroupID int64) ([]*studentpresence.VisitDisplay, error)
	getAttendanceStatusesFn    func(studentIDs []int64) (map[int64]*studentpresence.DailyAttendanceStatus, error)
}

type mockRoomSessions struct {
	OpenRoomSessions
	list  func() ([]int64, error)
	staff func(int64) ([]int64, error)
}

func (m *mockRoomSessions) GetStaffActiveGroupIDs(_ context.Context, staffID int64) ([]int64, error) {
	return m.staff(staffID)
}

func (m *mockRoomSessions) ListRunningSessionIDs(context.Context) ([]int64, error) {
	return m.list()
}

func (m *mockActiveService) GetActiveGroupsByIDs(_ context.Context, ids []int64) (map[int64]*studentpresence.SessionDetail, error) {
	return m.getActiveGroupsByIDsFn(ids)
}

func (m *mockActiveService) GetRoomsByIDs(_ context.Context, ids []int64) ([]*studentpresence.SessionRoomSummary, error) {
	return m.getRoomsByIDsFn(ids)
}

func (m *mockActiveService) GetUnclaimedActiveGroups(_ context.Context) ([]*studentpresence.SessionDetail, error) {
	return m.getUnclaimedActiveGroupsFn()
}

func (m *mockActiveService) GetActiveGroupVisitsWithDisplay(_ context.Context, activeGroupID int64) ([]*studentpresence.VisitDisplay, error) {
	return m.getActiveGroupVisitsFn(activeGroupID)
}

func (m *mockActiveService) GetStudentsAttendanceStatuses(_ context.Context, studentIDs []int64) (map[int64]*studentpresence.DailyAttendanceStatus, error) {
	return m.getAttendanceStatusesFn(studentIDs)
}

type mockUserContextService struct {
	CallerContext
	getMySupervisedGroupsFn func() ([]*studentpresence.SessionDetail, error)
	myGroupsFn              func() ([]CallerGroup, error)
}

func (m *mockUserContextService) GetMySupervisedGroups(_ context.Context) ([]*studentpresence.SessionDetail, error) {
	return m.getMySupervisedGroupsFn()
}

func (m *mockUserContextService) MyGroups(_ context.Context) ([]CallerGroup, error) {
	return m.myGroupsFn()
}

type mockEducationService struct {
	educationModels.GroupRoomsQuery
	getGroupsWithRoomsByIDsFn func(ids []int64) (map[int64]*educationModels.Group, error)
}

func (m *mockEducationService) GetGroupsWithRoomsByIDs(_ context.Context, ids []int64) (map[int64]*educationModels.Group, error) {
	return m.getGroupsWithRoomsByIDsFn(ids)
}

type mockOperationsService struct {
	timetable.OperationQuery
	plannedNowFn     func(calendarpkg.Date, time.Time, timetable.PlannedNowOptions) ([]timetable.OperationPlannedInstance, error)
	activeSessionsFn func(calendarpkg.Date) ([]timetable.OperationActiveSession, error)
	sessionBlocksFn  func(int64, bool, calendarpkg.Date, map[int64][]int64) ([]timetable.OperationSessionBlock, error)
}

func (m *mockOperationsService) SessionBlocks(_ context.Context, accountID int64, isAdmin bool, day calendarpkg.Date, supervisors map[int64][]int64) ([]timetable.OperationSessionBlock, error) {
	return m.sessionBlocksFn(accountID, isAdmin, day, supervisors)
}

func (m *mockOperationsService) PlannedNow(_ context.Context, _ int64, _ bool, day calendarpkg.Date, now time.Time, opts timetable.PlannedNowOptions) ([]timetable.OperationPlannedInstance, error) {
	return m.plannedNowFn(day, now, opts)
}

func (m *mockOperationsService) ActiveSessions(_ context.Context, day calendarpkg.Date) ([]timetable.OperationActiveSession, error) {
	return m.activeSessionsFn(day)
}

type mockPickupService struct {
	careplan.BulkPickupDays
	getBulkEffectivePickupTimesForDateFn func([]int64, calendarpkg.Date) (map[int64]*careplan.EffectivePickupTime, error)
}

func (m *mockPickupService) GetBulkEffectivePickupDaysForDate(_ context.Context, studentIDs []int64, date calendarpkg.Date) (map[int64]*careplan.EffectivePickupTime, error) {
	return m.getBulkEffectivePickupTimesForDateFn(studentIDs, date)
}

type mockArrivalService struct {
	careplan.BulkArrivalTimes
	getBulkEffectiveArrivalTimesForDateFn func([]int64, calendarpkg.Date) (map[int64]*careplan.EffectiveArrivalTime, error)
}

func (m *mockArrivalService) GetBulkEffectiveArrivalTimesForDate(_ context.Context, studentIDs []int64, date calendarpkg.Date) (map[int64]*careplan.EffectiveArrivalTime, error) {
	return m.getBulkEffectiveArrivalTimesForDateFn(studentIDs, date)
}

// rootPorts stands in for the access, settings and yard ports the root
// binds; the composition only needs them present.
type rootPorts struct {
	supervisiondashboard.Access
	supervisiondashboard.Settings
	supervisiondashboard.Yard
}

func fullSources() Sources {
	return Sources{
		Access:      rootPorts{},
		Active:      &mockActiveService{},
		UserContext: &mockUserContextService{},
		Education:   &mockEducationService{},
		Yard:        rootPorts{},
		Operations:  &mockOperationsService{},
		Settings:    rootPorts{},
		Pickups:     &mockPickupService{},
		Arrivals:    &mockArrivalService{},
	}
}

func int64Ptr(value int64) *int64 { return &value }

func TestNewRejectsMissingSourcesAtComposition(t *testing.T) {
	t.Parallel()

	query, err := New(Sources{})
	assert.ErrorIs(t, err, ErrIncompleteSources, "missing wiring fails composition, not the first request")
	assert.Nil(t, query)

	query, err = New(fullSources())
	require.NoError(t, err)
	assert.NotNil(t, query)
}

func TestRunningSessionsResolveRelationsAndRooms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	color := "#83CD2D"

	active := &mockActiveService{
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
	roomSessions := &mockRoomSessions{
		list: func() ([]int64, error) { return []int64{11, 12, 13}, nil },
		staff: func(staffID int64) ([]int64, error) {
			assert.Equal(t, int64(91), staffID)
			return []int64{12}, nil
		},
	}
	s := sessions{active: active, groups: roomSessions}

	records, err := s.Running(ctx)
	require.NoError(t, err)
	assert.Equal(t, []supervisiondashboard.Session{
		{ID: 12, RoomID: int64Ptr(22), RoomName: "Adler"},
		{ID: 13, RoomID: int64Ptr(22), RoomName: "Adler"},
		{ID: 11, Name: "Malen", RoomID: int64Ptr(21), RoomName: "Zebra", RoomColor: &color},
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

func TestSupervisedSessionsAndUnclaimed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userContext := &mockUserContextService{getMySupervisedGroupsFn: func() ([]*studentpresence.SessionDetail, error) {
		return []*studentpresence.SessionDetail{{ID: 11, RoomID: 21}, {ID: 12}}, nil
	}}
	active := &mockActiveService{
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
	s := sessions{active: active, userContext: userContext}

	records, err := s.Supervised(ctx)
	require.NoError(t, err)
	assert.Equal(t, []supervisiondashboard.Session{{ID: 12}, {ID: 11, RoomID: int64Ptr(21), RoomName: "Zebra"}}, records,
		"a session without a room sorts ahead of named rooms")

	unclaimed, err := s.Unclaimed(ctx)
	require.NoError(t, err)
	assert.Equal(t, []supervisiondashboard.UnclaimedGroup{{ID: 11, RoomName: "Adler"}, {ID: 12}}, unclaimed)

	userContext.getMySupervisedGroupsFn = func() ([]*studentpresence.SessionDetail, error) { return nil, errors.New("boom") }
	_, err = s.Supervised(ctx)
	require.EqualError(t, err, "boom")
}

func TestMyGroupsResolvesRoomsOnlyForGroupsWithRooms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	roomID := int64(31)
	userContext := &mockUserContextService{myGroupsFn: func() ([]CallerGroup, error) {
		return []CallerGroup{
			{ID: 41, Name: "Bären", RoomID: &roomID},
			{ID: 42, Name: "Füchse"},
		}, nil
	}}
	education := &mockEducationService{getGroupsWithRoomsByIDsFn: func(ids []int64) (map[int64]*educationModels.Group, error) {
		assert.Equal(t, []int64{41}, ids)
		return map[int64]*educationModels.Group{
			41: {ID: 41, Room: &educationModels.GroupRoom{ID: 31, Name: "Igel"}},
			42: nil,
		}, nil
	}}
	g := groups{education: education, userContext: userContext}

	got, err := g.MyGroups(ctx)
	require.NoError(t, err)
	assert.Equal(t, []supervisiondashboard.EducationalGroup{{ID: 41, Name: "Bären", RoomName: "Igel"}, {ID: 42, Name: "Füchse"}}, got)

	education.getGroupsWithRoomsByIDsFn = func([]int64) (map[int64]*educationModels.Group, error) { return nil, errors.New("boom") }
	_, err = g.MyGroups(ctx)
	require.ErrorContains(t, err, "load education group rooms")

	userContext.myGroupsFn = func() ([]CallerGroup, error) { return nil, nil }
	education.getGroupsWithRoomsByIDsFn = func([]int64) (map[int64]*educationModels.Group, error) {
		t.Fatal("no groups with rooms, no room lookup")
		return nil, nil
	}
	got, err = g.MyGroups(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPresenceMapsVisitsAndAttendance(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	checkedIn := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	checkedOut := checkedIn.Add(time.Hour)
	sick := true
	photo := "/uploads/student-photos/p.jpg"
	active := &mockActiveService{
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
	p := presence{active: active}

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

func TestPlanningMapsEffectiveTimes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	date := calendarpkg.NewDate(2026, 8, 19)
	pickup := calendarpkg.NormalizeWallClock(time.Date(2026, time.August, 19, 15, 30, 0, 0, time.UTC))
	arrival := calendarpkg.NormalizeWallClock(time.Date(2026, time.August, 19, 11, 45, 0, 0, time.UTC))
	p := planning{
		pickups: &mockPickupService{getBulkEffectivePickupTimesForDateFn: func(studentIDs []int64, gotDate calendarpkg.Date) (map[int64]*careplan.EffectivePickupTime, error) {
			assert.Equal(t, []int64{42}, studentIDs)
			assert.Equal(t, date, gotDate)
			return map[int64]*careplan.EffectivePickupTime{
				42: {Date: date, PickupTime: &pickup, WeekdayName: "Mittwoch", IsException: true, Notes: "Oma", DayNotes: []careplan.NoteData{{ID: 3, Content: "Klingeln"}}, DayDepartureMode: "pickup"},
				43: nil,
			}, nil
		}},
		arrivals: &mockArrivalService{getBulkEffectiveArrivalTimesForDateFn: func([]int64, calendarpkg.Date) (map[int64]*careplan.EffectiveArrivalTime, error) {
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

func TestScheduleForwardsQueryAndPreservesWireShape(t *testing.T) {
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
			VisitID: int64Ptr(77), Status: "present", Substatus: &note, Note: &note, CheckedInAt: &pickup, VisitEntryTime: &pickup, PickupTime: &pickup,
			Warnings:          []timetable.OperationRosterWarning{{Kind: "arrival", Message: "zu früh", ExpectedArrival: &pickup, ExpectedGroupID: int64Ptr(3)}},
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
	operations := &mockOperationsService{
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
	s := schedule{operations: operations}

	planned, err := s.PlannedNow(ctx, supervisiondashboard.PlannedNowQuery{Date: "2026-08-19", Now: now, HorizonMinutes: 480, Limit: 5, IncludeRoster: true})
	require.NoError(t, err)
	require.Len(t, planned, 2)
	assertSameJSON(t, retained, planned[0])
	assertSameJSON(t, minimal, planned[1])

	sessions, err := s.ActiveSessions(ctx, "2026-08-19")
	require.NoError(t, err)
	assertSameJSON(t, []timetable.OperationActiveSession{{ActiveGroupID: 88, InstanceID: 5, Title: "Malen", StartTime: "14:00", EndTime: "15:00"}}, sessions)

	blocks, err := s.SessionBlocks(ctx, supervisiondashboard.SessionBlocksQuery{AccountID: 70, TokenAdmin: true, Date: "2026-08-19", Supervisors: map[int64][]int64{88: {7}}})
	require.NoError(t, err)
	assertSameJSON(t, []timetable.OperationSessionBlock{{ActiveGroupID: 88, InstanceID: 5, Title: "Malen", StartTime: "14:00", EndTime: "15:00", IsAssigned: true, CanOperate: true}}, blocks)

	_, err = s.PlannedNow(ctx, supervisiondashboard.PlannedNowQuery{Date: "today"})
	require.Error(t, err)
	_, err = s.SessionBlocks(ctx, supervisiondashboard.SessionBlocksQuery{Date: "today"})
	require.Error(t, err)
}

func TestCalendarRendersBerlinDaysAndClocks(t *testing.T) {
	t.Parallel()

	c := calendar{}
	fixed := time.Date(2026, time.August, 21, 6, 5, 0, 0, time.UTC)
	assert.Equal(t, supervisiondashboard.Date("2026-08-21"), c.DayOf(fixed))
	assert.Equal(t, "08:05", c.Clock(fixed), "instants render in Berlin summer time")
	assert.Equal(t, time.Saturday, c.Weekday("2026-08-29"))
	assert.Equal(t, supervisiondashboard.Date("2026-08-31"), c.DayOf(time.Date(2026, time.August, 30, 18, 30, 0, 0, time.FixedZone("America/New_York", -4*60*60))))
}

// assertSameJSON is the wire-parity golden: the projection's own record must
// serialise byte-identically to the retained row it mirrors.
func assertSameJSON(t *testing.T, retained, projected any) {
	t.Helper()
	want, err := json.Marshal(retained)
	require.NoError(t, err)
	got, err := json.Marshal(projected)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}
