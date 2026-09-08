package supervisiondashboard

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakes implements every consumer-owned port with func fields; a nil field
// answers with an empty value so each test only wires what it asserts.
type fakes struct {
	currentStaffID    func(context.Context) (*int64, error)
	caller            func(context.Context) (Caller, error)
	fullStudentAccess func(context.Context) (bool, error)
	running           func(context.Context) ([]Session, error)
	supervised        func(context.Context) ([]Session, error)
	supervisedByStaff func(context.Context, int64) (map[int64]struct{}, error)
	unclaimed         func(context.Context) ([]UnclaimedGroup, error)
	yard              func(context.Context, int64) (*SchulhofStatus, error)
	myGroups          func(context.Context) ([]EducationalGroup, error)
	plannedNow        func(context.Context, PlannedNowQuery) ([]PlannedInstance, error)
	activeSessions    func(context.Context, Date) ([]ActiveSession, error)
	groupVisits       func(context.Context, int64) ([]VisitRecord, error)
	attendanceTimes   func(context.Context, []int64) (map[int64]Attendance, error)
	tracking          func(context.Context, []int64, []string) (map[int64][]bool, error)
	pickups           func(context.Context, []int64, Date) (map[int64]Pickup, error)
	arrivals          func(context.Context, []int64, Date) (map[int64]Arrival, error)
	prepare           func(context.Context) (context.Context, error)
	photosEnabled     func(context.Context) (bool, error)
	trackingLabels    func(context.Context) ([]string, error)
	spontaneous       func(context.Context) (bool, error)
	now               func() time.Time
}

func (f *fakes) CurrentStaffID(ctx context.Context) (*int64, error) {
	if f.currentStaffID == nil {
		return nil, nil
	}
	return f.currentStaffID(ctx)
}

func (f *fakes) Caller(ctx context.Context) (Caller, error) {
	if f.caller == nil {
		return Caller{}, nil
	}
	return f.caller(ctx)
}

func (f *fakes) FullStudentAccess(ctx context.Context) (bool, error) {
	if f.fullStudentAccess == nil {
		return false, nil
	}
	return f.fullStudentAccess(ctx)
}

func (f *fakes) Running(ctx context.Context) ([]Session, error) {
	if f.running == nil {
		return nil, nil
	}
	return f.running(ctx)
}

func (f *fakes) Supervised(ctx context.Context) ([]Session, error) {
	if f.supervised == nil {
		return nil, nil
	}
	return f.supervised(ctx)
}

func (f *fakes) SupervisedByStaff(ctx context.Context, staffID int64) (map[int64]struct{}, error) {
	if f.supervisedByStaff == nil {
		return nil, nil
	}
	return f.supervisedByStaff(ctx, staffID)
}

func (f *fakes) Unclaimed(ctx context.Context) ([]UnclaimedGroup, error) {
	if f.unclaimed == nil {
		return nil, nil
	}
	return f.unclaimed(ctx)
}

func (f *fakes) Status(ctx context.Context, staffID int64) (*SchulhofStatus, error) {
	if f.yard == nil {
		return &SchulhofStatus{}, nil
	}
	return f.yard(ctx, staffID)
}

func (f *fakes) MyGroups(ctx context.Context) ([]EducationalGroup, error) {
	if f.myGroups == nil {
		return nil, nil
	}
	return f.myGroups(ctx)
}

func (f *fakes) PlannedNow(ctx context.Context, query PlannedNowQuery) ([]PlannedInstance, error) {
	if f.plannedNow == nil {
		return nil, nil
	}
	return f.plannedNow(ctx, query)
}

func (f *fakes) ActiveSessions(ctx context.Context, date Date) ([]ActiveSession, error) {
	if f.activeSessions == nil {
		return nil, nil
	}
	return f.activeSessions(ctx, date)
}

func (f *fakes) GroupVisits(ctx context.Context, activeGroupID int64) ([]VisitRecord, error) {
	if f.groupVisits == nil {
		return nil, nil
	}
	return f.groupVisits(ctx, activeGroupID)
}

func (f *fakes) AttendanceTimes(ctx context.Context, studentIDs []int64) (map[int64]Attendance, error) {
	if f.attendanceTimes == nil {
		return nil, nil
	}
	return f.attendanceTimes(ctx, studentIDs)
}

func (f *fakes) TrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	if f.tracking == nil {
		return nil, nil
	}
	return f.tracking(ctx, studentIDs, labels)
}

func (f *fakes) Pickups(ctx context.Context, studentIDs []int64, date Date) (map[int64]Pickup, error) {
	if f.pickups == nil {
		return nil, nil
	}
	return f.pickups(ctx, studentIDs, date)
}

func (f *fakes) Arrivals(ctx context.Context, studentIDs []int64, date Date) (map[int64]Arrival, error) {
	if f.arrivals == nil {
		return nil, nil
	}
	return f.arrivals(ctx, studentIDs, date)
}

func (f *fakes) Prepare(ctx context.Context) (context.Context, error) {
	if f.prepare == nil {
		return ctx, nil
	}
	return f.prepare(ctx)
}

func (f *fakes) StudentPhotosEnabled(ctx context.Context) (bool, error) {
	if f.photosEnabled == nil {
		return false, nil
	}
	return f.photosEnabled(ctx)
}

func (f *fakes) TrackingIndicatorLabels(ctx context.Context) ([]string, error) {
	if f.trackingLabels == nil {
		return nil, nil
	}
	return f.trackingLabels(ctx)
}

func (f *fakes) SpontaneousActivitiesEnabled(ctx context.Context) (bool, error) {
	if f.spontaneous == nil {
		return false, nil
	}
	return f.spontaneous(ctx)
}

var berlin = mustLoadBerlin()

func mustLoadBerlin() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return location
}

// testCalendar renders Berlin days and clocks the way the retained calendar
// helpers do, without importing them.
type testCalendar struct{}

func (testCalendar) DayOf(at time.Time) Date   { return Date(at.In(berlin).Format("2006-01-02")) }
func (testCalendar) Clock(at time.Time) string { return at.In(berlin).Format("15:04") }

func (testCalendar) Weekday(date Date) time.Weekday {
	parsed, err := time.Parse("2006-01-02", string(date))
	if err != nil {
		panic(err)
	}
	return parsed.Weekday()
}

func newService(f *fakes) *service {
	deps := Dependencies{
		Access: f, Sessions: f, Yard: f, Groups: f, Schedule: f, Presence: f, Planning: f, Settings: f,
		Calendar: testCalendar{},
		Now:      f.now,
	}
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Date(2026, time.August, 19, 12, 0, 0, 0, berlin) }
	}
	return New(deps).(*service)
}

func int64Ptr(value int64) *int64 { return &value }

func TestDashboardFailsFast(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, err := New(Dependencies{}).Dashboard(ctx, 0)
	require.ErrorIs(t, err, ErrIncompleteDependencies)

	_, err = newService(&fakes{prepare: func(ctx context.Context) (context.Context, error) {
		return ctx, errors.New("settings down")
	}}).Dashboard(ctx, 0)
	require.ErrorContains(t, err, "resolve dashboard settings")

	_, err = newService(&fakes{currentStaffID: func(context.Context) (*int64, error) {
		return nil, errors.New("staff lookup failed")
	}}).Dashboard(ctx, 0)
	require.ErrorContains(t, err, "load current staff")

	_, err = newService(&fakes{caller: func(context.Context) (Caller, error) {
		return Caller{}, errors.New("resolve operational overview scope: boom")
	}}).Dashboard(ctx, 0)
	require.ErrorContains(t, err, "operational overview scope")
}

func TestServiceDefaultsTheClock(t *testing.T) {
	t.Parallel()

	svc := New(Dependencies{}).(*service)
	assert.NotNil(t, svc.deps.Now)
	assert.False(t, svc.deps.complete())
}

func TestResolveGroupsBroadScope(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	color := "#83CD2D"
	f := &fakes{
		running: func(context.Context) ([]Session, error) {
			return []Session{
				{ID: 12, RoomID: int64Ptr(22), RoomName: "Adler"},
				{ID: 13, RoomID: int64Ptr(22), RoomName: "Adler"},
				{ID: 11, Name: "Malen", RoomID: int64Ptr(21), RoomName: "Zebra", RoomColor: &color},
			}, nil
		},
		supervisedByStaff: func(_ context.Context, staffID int64) (map[int64]struct{}, error) {
			assert.Equal(t, int64(91), staffID)
			return map[int64]struct{}{12: {}}, nil
		},
	}
	svc := newService(f)

	groups, err := svc.resolveGroups(ctx, Caller{OperationalOverview: true, AdminScope: true}, int64Ptr(91))
	require.NoError(t, err)
	require.Len(t, groups, 3)
	assert.Equal(t, "Adler", groups[0].RoomName)
	assert.Equal(t, "Adler", groups[1].RoomName)
	assert.Equal(t, "Zebra", groups[2].RoomName)
	assert.Equal(t, &color, groups[2].RoomColor)
	assert.Equal(t, "Malen", groups[2].Name)
	assert.Equal(t, "Adler", groups[0].Name, "sessions without an activity carry the room name")
	assert.True(t, groups[0].IsCurrentUserSupervising)
	assert.False(t, groups[1].IsCurrentUserSupervising)
	assert.True(t, groups[0].CanAssign)
	assert.True(t, groups[1].CanAssign, "admins may assign every session")

	groups, err = svc.resolveGroups(ctx, Caller{OperationalOverview: true}, int64Ptr(91))
	require.NoError(t, err)
	assert.True(t, groups[0].CanAssign)
	assert.False(t, groups[1].CanAssign, "without admin scope only supervised sessions accept assignments")

	groups, err = svc.resolveGroups(ctx, Caller{OperationalOverview: true}, nil)
	require.NoError(t, err)
	assert.False(t, groups[0].IsCurrentUserSupervising, "without a staff identity nothing is owned")

	f.running = func(context.Context) ([]Session, error) { return nil, errors.New("boom") }
	_, err = svc.resolveGroups(ctx, Caller{OperationalOverview: true}, int64Ptr(91))
	require.ErrorContains(t, err, "load active groups")

	f.running = func(context.Context) ([]Session, error) { return []Session{{ID: 1}}, nil }
	f.supervisedByStaff = func(context.Context, int64) (map[int64]struct{}, error) { return nil, errors.New("boom") }
	_, err = svc.resolveGroups(ctx, Caller{OperationalOverview: true}, int64Ptr(91))
	require.ErrorContains(t, err, "load current staff supervisions")
}

func TestResolveGroupsSupervisedScope(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	f := &fakes{
		supervised: func(context.Context) ([]Session, error) { return nil, errors.New("boom") },
		supervisedByStaff: func(context.Context, int64) (map[int64]struct{}, error) {
			t.Fatal("the supervised scope must not resolve ownership separately")
			return nil, nil
		},
	}
	svc := newService(f)
	_, err := svc.resolveGroups(ctx, Caller{}, int64Ptr(91))
	require.ErrorContains(t, err, "load supervised groups")

	f.supervised = func(context.Context) ([]Session, error) { return []Session{{ID: 11}, {ID: 12}}, nil }
	groups, err := svc.resolveGroups(ctx, Caller{}, int64Ptr(91))
	require.NoError(t, err)
	require.Len(t, groups, 2)
	assert.True(t, groups[0].IsCurrentUserSupervising)
	assert.True(t, groups[1].IsCurrentUserSupervising)
	assert.True(t, groups[0].CanAssign)
	assert.True(t, groups[1].CanAssign)
}

func TestSelectGroup(t *testing.T) {
	t.Parallel()

	groups := []Group{{ID: 10}, {ID: 20}}
	yardGroupID := int64(30)

	tests := []struct {
		name      string
		requested int64
		yard      *SchulhofStatus
		want      *int64
		wantErr   error
	}{
		{name: "defaults to the first group", want: int64Ptr(10)},
		{name: "uses a requested group", requested: 20, want: int64Ptr(20)},
		{name: "permits the supervised yard group", requested: 30, yard: &SchulhofStatus{IsUserSupervising: true, ActiveGroupID: &yardGroupID}, want: int64Ptr(30)},
		{name: "rejects an unsupervised yard group", requested: 30, yard: &SchulhofStatus{ActiveGroupID: &yardGroupID}, wantErr: ErrForbiddenGroup},
		{name: "rejects an unsupervised group", requested: 40, wantErr: ErrForbiddenGroup},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectGroup(groups, tt.yard, tt.requested)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}

	got, err := selectGroup(nil, nil, 0)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestLoadStaticSectionsBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	f := &fakes{
		unclaimed: func(context.Context) ([]UnclaimedGroup, error) {
			return []UnclaimedGroup{{ID: 11, RoomName: "Adler"}, {ID: 12}}, nil
		},
		myGroups: func(context.Context) ([]EducationalGroup, error) {
			return []EducationalGroup{{ID: 41, Name: "Bären", RoomName: "Igel"}, {ID: 42, Name: "Füchse"}}, nil
		},
	}
	svc := newService(f)

	projection := emptyProjection()
	require.NoError(t, svc.loadStaticSections(ctx, projection))
	assert.Equal(t, []UnclaimedGroup{{ID: 11, RoomName: "Adler"}, {ID: 12}}, projection.UnclaimedGroups)
	assert.Equal(t, []EducationalGroup{{ID: 41, Name: "Bären", RoomName: "Igel"}, {ID: 42, Name: "Füchse"}}, projection.EducationalGroups)

	f.myGroups = func(context.Context) ([]EducationalGroup, error) { return nil, errors.New("boom") }
	require.ErrorContains(t, svc.loadStaticSections(ctx, emptyProjection()), "load educational groups")

	f.unclaimed = func(context.Context) ([]UnclaimedGroup, error) { return nil, errors.New("boom") }
	require.ErrorContains(t, svc.loadStaticSections(ctx, emptyProjection()), "load unclaimed groups")

	f.unclaimed = nil
	f.myGroups = nil
	projection = emptyProjection()
	require.NoError(t, svc.loadStaticSections(ctx, projection))
	assert.Equal(t, []UnclaimedGroup{}, projection.UnclaimedGroups, "empty sections stay [] on the wire")
	assert.Equal(t, []EducationalGroup{}, projection.EducationalGroups)
}

func TestLoadTrackingBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	studentIDs := []int64{11}
	f := &fakes{trackingLabels: func(context.Context) ([]string, error) { return nil, errors.New("boom") }}
	svc := newService(f)

	_, err := svc.loadTracking(ctx, studentIDs)
	require.Error(t, err)

	f.trackingLabels = func(context.Context) ([]string, error) { return nil, nil }
	tracking, err := svc.loadTracking(ctx, studentIDs)
	require.NoError(t, err)
	assert.Empty(t, tracking.Labels)
	assert.NotNil(t, tracking.Results)

	f.trackingLabels = func(context.Context) ([]string, error) { return []string{"Hausaufgaben"}, nil }
	f.tracking = func(context.Context, []int64, []string) (map[int64][]bool, error) { return nil, errors.New("boom") }
	_, err = svc.loadTracking(ctx, studentIDs)
	require.Error(t, err)

	f.tracking = func(_ context.Context, ids []int64, labels []string) (map[int64][]bool, error) {
		assert.Equal(t, studentIDs, ids)
		assert.Equal(t, []string{"Hausaufgaben"}, labels)
		return map[int64][]bool{11: {true}}, nil
	}
	tracking, err = svc.loadTracking(ctx, studentIDs)
	require.NoError(t, err)
	assert.Equal(t, []string{"Hausaufgaben"}, tracking.Labels)
	assert.Equal(t, map[int64][]bool{11: {true}}, tracking.Results)

	f.trackingLabels = func(context.Context) ([]string, error) {
		t.Fatal("no students, no settings read")
		return nil, nil
	}
	tracking, err = svc.loadTracking(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, tracking.Labels)
}

func TestBuildVisitsAndEffectiveTimes(t *testing.T) {
	t.Parallel()

	checkedIn := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	checkedOut := checkedIn.Add(time.Hour)
	storedPhoto := studentPhotoStoredURLPrefix + "portrait.jpg"
	rows := []VisitRecord{{
		StudentID: 1, ActiveGroupID: 2, EntryTime: checkedIn,
		FirstName: "  Erika", LastName: "Mustermann ", SchoolClass: "4a", GroupName: "Bären",
		Sick: true, SickSince: &checkedIn, Excused: true, ExcusedSince: &checkedOut, PhotoPath: &storedPhoto,
	}}
	svc := newService(&fakes{})

	visits := svc.buildVisits(rows, map[int64]Attendance{1: {CheckInTime: &checkedIn, CheckOutTime: &checkedOut}}, true, true)
	require.Len(t, visits, 1)
	assert.Equal(t, "Erika Mustermann", visits[0].StudentName)
	assert.True(t, visits[0].Sick)
	assert.True(t, visits[0].Excused)
	assert.Equal(t, &checkedIn, visits[0].SickSince)
	assert.Equal(t, "/api/students/1/photo/portrait.jpg", visits[0].PhotoURL)
	require.NotNil(t, visits[0].ActualArrivalTime)
	assert.Equal(t, "12:00", *visits[0].ActualArrivalTime, "actual clocks render in Berlin summer time")
	assert.Equal(t, "13:00", *visits[0].ActualPickupTime)

	privateVisits := svc.buildVisits(rows, nil, false, true)
	assert.Nil(t, privateVisits[0].ActualArrivalTime)
	assert.Empty(t, privateVisits[0].PhotoURL)

	noPhotos := svc.buildVisits(rows, nil, true, false)
	assert.Empty(t, noPhotos[0].PhotoURL)

	pickup := "15:30"
	pickupItem := pickupTimeFromEffective(1, Pickup{
		Date: "2026-08-19", PickupTime: &pickup, WeekdayName: "Mittwoch", IsException: true, Notes: "Oma", DayNotes: []DayNote{{ID: 3, Content: "Klingeln"}},
	})
	assert.Equal(t, "15:30", *pickupItem.PickupTime)
	assert.Equal(t, "2026-08-19", pickupItem.Date)
	assert.Equal(t, []DayNote{{ID: 3, Content: "Klingeln"}}, pickupItem.DayNotes)

	arrival := "11:45"
	arrivalItem := arrivalTimeFromEffective(1, Arrival{
		Date: "2026-08-19", ArrivalTime: &arrival, WeekdayName: "Mittwoch", Notes: "Bus", DayNotes: []DayNote{{ID: 4, Content: "Verspätung"}},
	})
	assert.Equal(t, "11:45", *arrivalItem.ExpectedArrival)
	assert.Equal(t, []DayNote{{ID: 4, Content: "Verspätung"}}, arrivalItem.DayNotes)
}

func TestBuildPhotoURL(t *testing.T) {
	t.Parallel()

	assert.Empty(t, buildPhotoURL(1, ""))
	assert.Equal(t, "https://example.test/photo.jpg", buildPhotoURL(1, "https://example.test/photo.jpg"))
}

func TestEmptyAndPermissionRedactedProjection(t *testing.T) {
	t.Parallel()

	projection := emptyProjection()
	assert.Empty(t, projection.Groups)
	assert.Empty(t, projection.UnclaimedGroups)
	assert.Empty(t, projection.EducationalGroups)
	assert.Empty(t, projection.ActiveSessions)
	assert.Empty(t, projection.PlannedNow)
	assert.Empty(t, projection.Visits)
	assert.Empty(t, projection.TrackingIndicators.Labels)
	assert.Empty(t, projection.TrackingIndicators.Results)
	assert.Empty(t, projection.PickupTimes)
	assert.Empty(t, projection.ArrivalTimes)

	f := &fakes{
		plannedNow: func(context.Context, PlannedNowQuery) ([]PlannedInstance, error) {
			t.Fatal("schedule sections must not load without schedules:read")
			return nil, nil
		},
		pickups: func(context.Context, []int64, Date) (map[int64]Pickup, error) {
			t.Fatal("planning times must not load without users:read")
			return nil, nil
		},
	}
	svc := newService(f)
	ctx := context.Background()
	snapshot := newDaySnapshot(time.Date(2026, time.August, 19, 12, 0, 0, 0, berlin), testCalendar{})
	require.NoError(t, svc.loadScheduleSections(ctx, projection, Caller{}, snapshot))
	require.NoError(t, svc.loadPlanningTimes(ctx, projection, Caller{}, []int64{1}, true, snapshot.businessDay))
	require.NoError(t, svc.loadPlanningTimes(ctx, projection, Caller{CanReadStudents: true}, []int64{1}, false, snapshot.businessDay))
	require.NoError(t, svc.loadPlanningTimes(ctx, projection, Caller{CanReadStudents: true}, nil, true, snapshot.businessDay))
	assert.Empty(t, projection.PlannedNow)
	assert.Empty(t, projection.PickupTimes)
}

func TestLoadPlanningTimesProjectsPickupException(t *testing.T) {
	t.Parallel()

	const date Date = "2026-08-19"
	pickup := "15:30"
	f := &fakes{
		now: func() time.Time { panic("planning-time loads must use the aggregate day snapshot") },
		pickups: func(_ context.Context, studentIDs []int64, gotDate Date) (map[int64]Pickup, error) {
			assert.Equal(t, []int64{42, 43}, studentIDs)
			assert.Equal(t, date, gotDate)
			return map[int64]Pickup{42: {Date: date, PickupTime: &pickup, IsException: true}}, nil
		},
		arrivals: func(_ context.Context, studentIDs []int64, gotDate Date) (map[int64]Arrival, error) {
			assert.Equal(t, date, gotDate)
			return map[int64]Arrival{43: {Date: date, IsException: true, Notes: "Zahnarzt"}}, nil
		},
	}
	svc := newService(f)
	projection := emptyProjection()

	require.NoError(t, svc.loadPlanningTimes(context.Background(), projection, Caller{CanReadStudents: true}, []int64{42, 43}, true, date))
	require.Len(t, projection.PickupTimes, 1)
	assert.Equal(t, int64(42), projection.PickupTimes[0].StudentID)
	assert.Equal(t, "15:30", *projection.PickupTimes[0].PickupTime)
	assert.True(t, projection.PickupTimes[0].IsException)
	require.Len(t, projection.ArrivalTimes, 1)
	assert.Equal(t, int64(43), projection.ArrivalTimes[0].StudentID)
	assert.Nil(t, projection.ArrivalTimes[0].ExpectedArrival, "a timeless exception keeps its time nil")
	assert.Equal(t, "Zahnarzt", projection.ArrivalTimes[0].Notes)

	f.arrivals = func(context.Context, []int64, Date) (map[int64]Arrival, error) { return nil, errors.New("boom") }
	require.ErrorContains(t, svc.loadPlanningTimes(context.Background(), emptyProjection(), Caller{CanReadStudents: true}, []int64{42, 43}, true, date), "load arrival times")
	f.pickups = func(context.Context, []int64, Date) (map[int64]Pickup, error) { return nil, errors.New("boom") }
	require.ErrorContains(t, svc.loadPlanningTimes(context.Background(), emptyProjection(), Caller{CanReadStudents: true}, []int64{42, 43}, true, date), "load pickup times")
}

func TestLoadScheduleSectionsRedactsPickupTimesWithoutStudentRead(t *testing.T) {
	t.Parallel()

	pickupTime := "15:00"
	var lastQuery PlannedNowQuery
	f := &fakes{
		plannedNow: func(_ context.Context, query PlannedNowQuery) ([]PlannedInstance, error) {
			lastQuery = query
			return []PlannedInstance{{
				PickupTimesLoaded: true,
				RosterPreview:     []RosterRow{{PickupTime: &pickupTime}},
			}}, nil
		},
		activeSessions: func(context.Context, Date) ([]ActiveSession, error) { return []ActiveSession{}, nil },
		spontaneous:    func(context.Context) (bool, error) { return true, nil },
	}
	svc := newService(f)
	projection := emptyProjection()
	snapshot := newDaySnapshot(time.Date(2026, time.August, 19, 12, 0, 0, 0, berlin), testCalendar{})

	require.NoError(t, svc.loadScheduleSections(context.Background(), projection, Caller{AccountID: 99, TokenAdmin: true, CanReadSchedules: true}, snapshot))
	assert.Equal(t, PlannedNowQuery{
		AccountID: 99, TokenAdmin: true, Date: snapshot.businessDay, Now: snapshot.instant,
		HorizonMinutes: plannedNowHorizonMinutes, Limit: plannedNowLimit, IncludeRoster: true,
	}, lastQuery, "the page contract the former BFF hardcoded")
	require.Len(t, projection.PlannedNow, 1)
	assert.False(t, projection.PlannedNow[0].PickupTimesLoaded)
	assert.True(t, projection.PlannedNow[0].PickupTimesRedacted)
	assert.Nil(t, projection.PlannedNow[0].RosterPreview[0].PickupTime)
	assert.True(t, projection.Capabilities.WebSpontaneousActivitiesEnabled)

	projection = emptyProjection()
	require.NoError(t, svc.loadScheduleSections(context.Background(), projection, Caller{CanReadSchedules: true, CanReadStudents: true}, snapshot))
	assert.True(t, projection.PlannedNow[0].PickupTimesLoaded)
	assert.Equal(t, &pickupTime, projection.PlannedNow[0].RosterPreview[0].PickupTime)

	f.spontaneous = func(context.Context) (bool, error) {
		return false, errors.New("resolve spontaneous activities setting: boom")
	}
	require.ErrorContains(t, svc.loadScheduleSections(context.Background(), emptyProjection(), Caller{CanReadSchedules: true}, snapshot), "spontaneous activities")
	f.activeSessions = func(context.Context, Date) ([]ActiveSession, error) { return nil, errors.New("boom") }
	require.ErrorContains(t, svc.loadScheduleSections(context.Background(), emptyProjection(), Caller{CanReadSchedules: true}, snapshot), "load active sessions")
	f.plannedNow = func(context.Context, PlannedNowQuery) ([]PlannedInstance, error) { return nil, errors.New("boom") }
	require.ErrorContains(t, svc.loadScheduleSections(context.Background(), emptyProjection(), Caller{CanReadSchedules: true}, snapshot), "load planned instances")
}

func TestSpontaneousStartAvailabilityUsesBerlinSchoolDay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		now           time.Time
		wantDay       Date
		wantAvailable bool
		wantReason    string
	}{
		{name: "Friday", now: time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC), wantDay: "2026-08-28", wantAvailable: true},
		{name: "Saturday", now: time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC), wantDay: "2026-08-29", wantReason: SpontaneousStartBlockedWeekend},
		{name: "Sunday", now: time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC), wantDay: "2026-08-30", wantReason: SpontaneousStartBlockedWeekend},
		{name: "Monday", now: time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC), wantDay: "2026-08-31", wantAvailable: true},
		{
			name:          "Sunday in New York is already Monday in Berlin",
			now:           time.Date(2026, time.August, 30, 18, 30, 0, 0, time.FixedZone("America/New_York", -4*60*60)),
			wantDay:       "2026-08-31",
			wantAvailable: true,
		},
		{
			name:       "Monday in Tokyo is still Sunday in Berlin",
			now:        time.Date(2026, time.August, 31, 6, 30, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60)),
			wantDay:    "2026-08-30",
			wantReason: SpontaneousStartBlockedWeekend,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := newDaySnapshot(tt.now, testCalendar{})

			assert.Equal(t, tt.wantDay, snapshot.businessDay)
			assert.Equal(t, tt.wantAvailable, snapshot.spontaneousStart.Available)
			assert.Equal(t, tt.wantReason, snapshot.spontaneousStart.BlockedReason)
		})
	}
}

func TestDashboardCapturesOneServerTimeSnapshot(t *testing.T) {
	t.Parallel()

	beforeMidnight := time.Date(2026, time.August, 30, 23, 59, 59, 0, berlin)
	afterMidnight := beforeMidnight.Add(time.Second)
	nowCalls := 0
	var plannedDay, activeSessionsDay, pickupDay, arrivalDay Date
	var plannedInstant time.Time
	const activeGroupID int64 = 401
	const studentID int64 = 501

	f := &fakes{
		now: func() time.Time {
			nowCalls++
			if nowCalls == 1 {
				return beforeMidnight
			}
			return afterMidnight
		},
		caller: func(context.Context) (Caller, error) {
			return Caller{CanReadSchedules: true, CanReadStudents: true}, nil
		},
		fullStudentAccess: func(context.Context) (bool, error) { return true, nil },
		supervised: func(context.Context) ([]Session, error) {
			return []Session{{ID: activeGroupID}}, nil
		},
		groupVisits: func(_ context.Context, gotActiveGroupID int64) ([]VisitRecord, error) {
			assert.Equal(t, activeGroupID, gotActiveGroupID)
			return []VisitRecord{{StudentID: studentID, ActiveGroupID: activeGroupID, EntryTime: beforeMidnight}}, nil
		},
		attendanceTimes: func(_ context.Context, studentIDs []int64) (map[int64]Attendance, error) {
			assert.Equal(t, []int64{studentID}, studentIDs)
			return map[int64]Attendance{}, nil
		},
		plannedNow: func(_ context.Context, query PlannedNowQuery) ([]PlannedInstance, error) {
			plannedDay = query.Date
			plannedInstant = query.Now
			return nil, nil
		},
		activeSessions: func(_ context.Context, day Date) ([]ActiveSession, error) {
			activeSessionsDay = day
			return nil, nil
		},
		spontaneous: func(context.Context) (bool, error) { return true, nil },
		pickups: func(_ context.Context, studentIDs []int64, day Date) (map[int64]Pickup, error) {
			assert.Equal(t, []int64{studentID}, studentIDs)
			pickupDay = day
			return map[int64]Pickup{}, nil
		},
		arrivals: func(_ context.Context, studentIDs []int64, day Date) (map[int64]Arrival, error) {
			assert.Equal(t, []int64{studentID}, studentIDs)
			arrivalDay = day
			return map[int64]Arrival{}, nil
		},
	}

	projection, err := newService(f).Dashboard(context.Background(), 0)

	require.NoError(t, err)
	assert.Equal(t, 1, nowCalls, "one aggregate request must capture the injected server clock exactly once")
	assert.Equal(t, Date("2026-08-30"), projection.BusinessDay)
	assert.Equal(t, SpontaneousStartAvailability{Available: false, BlockedReason: SpontaneousStartBlockedWeekend}, projection.SpontaneousStartAvailability)
	assert.Equal(t, projection.BusinessDay, plannedDay)
	assert.Equal(t, projection.BusinessDay, activeSessionsDay)
	assert.Equal(t, projection.BusinessDay, pickupDay)
	assert.Equal(t, projection.BusinessDay, arrivalDay)
	assert.Equal(t, beforeMidnight, plannedInstant)
	assert.Nil(t, projection.Schulhof, "no staff identity, no Schulhof workflow")
	assert.Equal(t, int64Ptr(activeGroupID), projection.SelectedGroupID)
	require.Len(t, projection.Visits, 1)
	assert.True(t, projection.Capabilities.WebSpontaneousActivitiesEnabled)
}

func TestDashboardSelectedGroupSectionsFollowAccess(t *testing.T) {
	t.Parallel()

	entry := time.Date(2026, time.August, 19, 8, 0, 0, 0, time.UTC)
	exit := entry.Add(time.Hour)
	checkIn := time.Date(2026, time.August, 19, 6, 5, 0, 0, time.UTC)
	yardGroupID := int64(30)
	f := &fakes{
		currentStaffID: func(context.Context) (*int64, error) { return int64Ptr(77), nil },
		caller: func(context.Context) (Caller, error) {
			return Caller{CanReadStudents: true}, nil
		},
		fullStudentAccess: func(context.Context) (bool, error) { return true, nil },
		supervised: func(context.Context) ([]Session, error) {
			return []Session{{ID: 10, RoomName: "Adler"}}, nil
		},
		yard: func(_ context.Context, staffID int64) (*SchulhofStatus, error) {
			assert.Equal(t, int64(77), staffID)
			return &SchulhofStatus{Exists: true, IsUserSupervising: true, ActiveGroupID: &yardGroupID}, nil
		},
		groupVisits: func(_ context.Context, activeGroupID int64) ([]VisitRecord, error) {
			assert.Equal(t, yardGroupID, activeGroupID)
			return []VisitRecord{
				{StudentID: 1, ActiveGroupID: activeGroupID, EntryTime: entry, FirstName: "Erika", LastName: "Muster"},
				{StudentID: 1, ActiveGroupID: activeGroupID, EntryTime: entry, FirstName: "Erika", LastName: "Muster"},
				{StudentID: 2, ActiveGroupID: activeGroupID, EntryTime: entry, ExitTime: &exit, FirstName: "Max", LastName: "Weg"},
			}, nil
		},
		attendanceTimes: func(_ context.Context, studentIDs []int64) (map[int64]Attendance, error) {
			assert.Equal(t, []int64{1}, studentIDs, "closed visits and duplicates do not widen the student set")
			return map[int64]Attendance{1: {CheckInTime: &checkIn}}, nil
		},
		trackingLabels: func(context.Context) ([]string, error) { return []string{"Hausaufgaben"}, nil },
		tracking: func(context.Context, []int64, []string) (map[int64][]bool, error) {
			return map[int64][]bool{1: {true}}, nil
		},
		pickups: func(_ context.Context, studentIDs []int64, _ Date) (map[int64]Pickup, error) {
			assert.Equal(t, []int64{1}, studentIDs)
			return map[int64]Pickup{1: {WeekdayName: "Mittwoch"}}, nil
		},
	}

	projection, err := newService(f).Dashboard(context.Background(), yardGroupID)

	require.NoError(t, err)
	assert.Equal(t, int64Ptr(77), projection.CurrentStaffID)
	assert.Equal(t, int64Ptr(yardGroupID), projection.SelectedGroupID, "a supervised Schulhof session is selectable")
	require.Len(t, projection.Visits, 2)
	assert.Equal(t, "Erika Muster", projection.Visits[0].StudentName)
	assert.Equal(t, "08:05", *projection.Visits[0].ActualArrivalTime)
	assert.Equal(t, []string{"Hausaufgaben"}, projection.TrackingIndicators.Labels)
	require.Len(t, projection.PickupTimes, 1)
	assert.Equal(t, "Mittwoch", projection.PickupTimes[0].WeekdayName)
	assert.Empty(t, projection.ArrivalTimes)
	assert.Empty(t, projection.PlannedNow, "no schedules:read, no planned section")

	_, err = newService(f).Dashboard(context.Background(), 99)
	require.ErrorIs(t, err, ErrForbiddenGroup)

	f.yard = func(context.Context, int64) (*SchulhofStatus, error) { return nil, errors.New("boom") }
	_, err = newService(f).Dashboard(context.Background(), 0)
	require.ErrorContains(t, err, "load schulhof status")

	f.yard = nil
	f.groupVisits = func(context.Context, int64) ([]VisitRecord, error) { return nil, errors.New("boom") }
	_, err = newService(f).Dashboard(context.Background(), 0)
	require.ErrorContains(t, err, "load group visits")

	f.groupVisits = func(context.Context, int64) ([]VisitRecord, error) { return []VisitRecord{{StudentID: 1}}, nil }
	f.attendanceTimes = func(context.Context, []int64) (map[int64]Attendance, error) { return nil, errors.New("boom") }
	_, err = newService(f).Dashboard(context.Background(), 0)
	require.ErrorContains(t, err, "load attendance statuses")

	f.attendanceTimes = nil
	f.photosEnabled = func(context.Context) (bool, error) { return false, errors.New("boom") }
	_, err = newService(f).Dashboard(context.Background(), 0)
	require.ErrorContains(t, err, "resolve student photos setting")

	f.photosEnabled = nil
	f.tracking = func(context.Context, []int64, []string) (map[int64][]bool, error) { return nil, errors.New("boom") }
	_, err = newService(f).Dashboard(context.Background(), 0)
	require.ErrorContains(t, err, "load tracking indicators")

	f.tracking = nil
	f.fullStudentAccess = func(context.Context) (bool, error) { return false, errors.New("boom") }
	_, err = newService(f).Dashboard(context.Background(), 0)
	require.ErrorContains(t, err, "resolve student access")
}

// TestProjectionWireShape pins the JSON contract of the aggregate: every
// section, the string-encoded ids, the omitted optionals and the sections
// that stay [] instead of null. The frontend mapping depends on every key.
func TestProjectionWireShape(t *testing.T) {
	t.Parallel()

	entry := time.Date(2026, time.August, 19, 6, 0, 0, 0, time.UTC)
	clock := "08:00"
	roomID := int64(21)
	color := "#83CD2D"
	projection := emptyProjection()
	projection.BusinessDay = "2026-08-19"
	projection.SpontaneousStartAvailability = SpontaneousStartAvailability{Available: true}
	projection.Groups = []Group{{ID: 11, Name: "Malen", RoomID: &roomID, RoomName: "Zebra", RoomColor: &color, IsCurrentUserSupervising: true, CanAssign: true}, {ID: 12, Name: "Adler"}}
	projection.SelectedGroupID = int64Ptr(11)
	projection.UnclaimedGroups = []UnclaimedGroup{{ID: 13, RoomName: "Igel"}}
	projection.CurrentStaffID = int64Ptr(7)
	projection.EducationalGroups = []EducationalGroup{{ID: 41, Name: "Bären", RoomName: "Igel"}}
	projection.Schulhof = &SchulhofStatus{Exists: true, RoomName: "Schulhof", Supervisors: []Supervisor{{ID: 1, StaffID: 7, Name: "Erika", IsCurrentUser: true}}}
	projection.Capabilities = Capabilities{WebSpontaneousActivitiesEnabled: true}
	projection.ActiveSessions = []ActiveSession{{ActiveGroupID: 11, InstanceID: 5, Title: "Malen", StartTime: "14:00", EndTime: "15:00"}}
	projection.PlannedNow = []PlannedInstance{{ID: 5, Title: "Malen", Date: "2026-08-19", StartTime: "14:00", EndTime: "15:00", RoomID: 21, Status: "planned",
		AssignedStaffIDs: []int64{7}, RosterPreview: []RosterRow{{StudentID: 1, StudentName: "Erika Muster", Status: "expected", PickupTime: &clock, CareDayStatus: "scheduled"}},
		PickupTimesLoaded: true, Warnings: []ConflictWarning{}, StartAvailableAt: "13:45", StartExpiresAt: "15:00"}}
	projection.Visits = []Visit{{StudentID: 1, StudentName: "Erika Muster", SchoolClass: "4a", GroupName: "Bären", ActiveGroupID: 11, CheckInTime: entry, ActualArrivalTime: &clock, Sick: false, PhotoURL: "/api/students/1/photo/p.jpg"}}
	projection.TrackingIndicators = TrackingIndicators{Labels: []string{"Hausaufgaben"}, Results: map[int64][]bool{1: {true}}}
	projection.PickupTimes = []PickupTime{{StudentID: 1, Date: "2026-08-19", WeekdayName: "Mittwoch", PickupTime: &clock, IsException: true, Notes: "Oma", DayNotes: []DayNote{{ID: 3, Content: "Klingeln"}}}}
	projection.ArrivalTimes = []ArrivalTime{{StudentID: 1, Date: "2026-08-19", WeekdayName: "Mittwoch"}}

	got, err := json.Marshal(projection)
	require.NoError(t, err)

	want := `{"business_day":"2026-08-19","spontaneous_start_availability":{"available":true},` +
		`"groups":[{"id":"11","name":"Malen","room_id":"21","room_name":"Zebra","room_color":"#83CD2D","is_current_user_supervising":true,"can_assign":true},` +
		`{"id":"12","name":"Adler","is_current_user_supervising":false,"can_assign":false}],` +
		`"selected_group_id":"11","unclaimed_groups":[{"id":"13","room_name":"Igel"}],"current_staff_id":"7",` +
		`"educational_groups":[{"id":"41","name":"Bären","room_name":"Igel"}],` +
		`"schulhof_status":{"exists":true,"room_name":"Schulhof","is_user_supervising":false,"supervisor_count":0,"student_count":0,"supervisors":[{"id":1,"staff_id":7,"name":"Erika","is_current_user":true}]},` +
		`"capabilities":{"web_spontaneous_activities_enabled":true},` +
		`"active_sessions":[{"active_group_id":11,"instance_id":5,"title":"Malen","start_time":"14:00","end_time":"15:00"}],` +
		`"planned_now":[{"id":5,"title":"Malen","date":"2026-08-19","start_time":"14:00","end_time":"15:00","room_id":21,"status":"planned","is_overdue":false,"minutes_until_start":0,` +
		`"expected_students_count":0,"present_students_count":0,"not_scheduled_students_count":0,"assigned_staff_ids":[7],"is_assigned":false,"is_primary":false,"is_substitute":false,"is_absent":false,` +
		`"roster_preview":[{"student_id":1,"student_name":"Erika Muster","school_class":"","group_name":"","planned":false,"is_unplanned":false,"currently_present":false,"status":"expected","pickup_time":"08:00","care_day_status":"scheduled"}],` +
		`"pickup_times_loaded":true,"warnings":[],"can_start":false,"start_available_at":"13:45","start_expires_at":"15:00"}],` +
		`"visits":[{"student_id":"1","student_name":"Erika Muster","school_class":"4a","group_name":"Bären","active_group_id":"11","check_in_time":"2026-08-19T06:00:00Z","actual_arrival_time":"08:00","sick":false,"excused":false,"photo_url":"/api/students/1/photo/p.jpg"}],` +
		`"tracking_indicators":{"labels":["Hausaufgaben"],"results":{"1":[true]}},` +
		`"pickup_times":[{"student_id":"1","date":"2026-08-19","weekday_name":"Mittwoch","pickup_time":"08:00","is_exception":true,"notes":"Oma","day_notes":[{"id":"3","content":"Klingeln"}]}],` +
		`"arrival_times":[{"student_id":"1","date":"2026-08-19","weekday_name":"Mittwoch","is_exception":false}]}`
	assert.JSONEq(t, want, string(got))
	assert.Equal(t, want, string(got), "key order is part of the golden contract")

	empty, err := json.Marshal(emptyProjection())
	require.NoError(t, err)
	assert.Equal(t, `{"business_day":"","spontaneous_start_availability":{"available":false},"groups":[],"unclaimed_groups":[],"educational_groups":[],"schulhof_status":null,`+
		`"capabilities":{"web_spontaneous_activities_enabled":false},"active_sessions":[],"planned_now":[],"visits":[],"tracking_indicators":{"labels":[],"results":{}},"pickup_times":[],"arrival_times":[]}`, string(empty))
}
