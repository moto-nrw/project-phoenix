package legacy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
)

// The retained interfaces are wide; the mocks embed them and override only
// what a test calls — an unexpected call panics on the nil embedded interface.
type mockActiveService struct {
	activeService.Service
	listActiveGroupsFn         func() ([]*activeModels.Group, error)
	getActiveGroupsByIDsFn     func(ids []int64) (map[int64]*activeModels.Group, error)
	getRoomsByIDsFn            func(ids []int64) ([]*facilitiesModels.Room, error)
	getUnclaimedActiveGroupsFn func() ([]*activeModels.Group, error)
	getActiveGroupVisitsFn     func(activeGroupID int64) ([]*activeService.VisitWithStudentDisplay, error)
	getAttendanceStatusesFn    func(studentIDs []int64) (map[int64]*activeService.AttendanceStatus, error)
	getStaffSupervisionsFn     func(staffID int64) ([]*activeModels.GroupSupervisor, error)
}

func (m *mockActiveService) ListActiveGroups(_ context.Context, _ *base.QueryOptions) ([]*activeModels.Group, error) {
	return m.listActiveGroupsFn()
}

func (m *mockActiveService) GetActiveGroupsByIDs(_ context.Context, ids []int64) (map[int64]*activeModels.Group, error) {
	return m.getActiveGroupsByIDsFn(ids)
}

func (m *mockActiveService) GetRoomsByIDs(_ context.Context, ids []int64) ([]*facilitiesModels.Room, error) {
	return m.getRoomsByIDsFn(ids)
}

func (m *mockActiveService) GetUnclaimedActiveGroups(_ context.Context) ([]*activeModels.Group, error) {
	return m.getUnclaimedActiveGroupsFn()
}

func (m *mockActiveService) GetActiveGroupVisitsWithDisplay(_ context.Context, activeGroupID int64) ([]*activeService.VisitWithStudentDisplay, error) {
	return m.getActiveGroupVisitsFn(activeGroupID)
}

func (m *mockActiveService) GetStudentsAttendanceStatuses(_ context.Context, studentIDs []int64) (map[int64]*activeService.AttendanceStatus, error) {
	return m.getAttendanceStatusesFn(studentIDs)
}

func (m *mockActiveService) GetStaffActiveSupervisions(_ context.Context, staffID int64) ([]*activeModels.GroupSupervisor, error) {
	return m.getStaffSupervisionsFn(staffID)
}

type mockUserContextService struct {
	userContextService.UserContextService
	getCurrentStaffFn       func() (*usersModels.Staff, error)
	getMySupervisedGroupsFn func() ([]*activeModels.Group, error)
	getMyGroupsFn           func() ([]*educationModels.Group, error)
}

func (m *mockUserContextService) GetCurrentStaff(_ context.Context) (*usersModels.Staff, error) {
	return m.getCurrentStaffFn()
}

func (m *mockUserContextService) GetMySupervisedGroups(_ context.Context) ([]*activeModels.Group, error) {
	return m.getMySupervisedGroupsFn()
}

func (m *mockUserContextService) GetMyGroups(_ context.Context) ([]*educationModels.Group, error) {
	return m.getMyGroupsFn()
}

type mockEducationService struct {
	educationService.Service
	getGroupsWithRoomsByIDsFn func(ids []int64) (map[int64]*educationModels.Group, error)
}

func (m *mockEducationService) GetGroupsWithRoomsByIDs(_ context.Context, ids []int64) (map[int64]*educationModels.Group, error) {
	return m.getGroupsWithRoomsByIDsFn(ids)
}

type mockSchulhofService struct {
	facilitiesService.SchulhofService
	statusFn func(staffID int64) (*facilitiesService.SchulhofStatus, error)
}

func (m *mockSchulhofService) GetSchulhofStatus(_ context.Context, staffID int64) (*facilitiesService.SchulhofStatus, error) {
	return m.statusFn(staffID)
}

type mockOperationsService struct {
	scheduleService.TimetableOperationsService
	plannedNowFn     func(timezone.Date, time.Time, scheduleService.PlannedNowOptions) ([]scheduleService.OperationPlannedInstance, error)
	activeSessionsFn func(timezone.Date) ([]scheduleService.OperationActiveSession, error)
}

func (m *mockOperationsService) PlannedNow(_ context.Context, _ int64, _ bool, day timezone.Date, now time.Time, opts scheduleService.PlannedNowOptions) ([]scheduleService.OperationPlannedInstance, error) {
	return m.plannedNowFn(day, now, opts)
}

func (m *mockOperationsService) ActiveSessions(_ context.Context, day timezone.Date) ([]scheduleService.OperationActiveSession, error) {
	return m.activeSessionsFn(day)
}

type mockPickupService struct {
	scheduleService.PickupScheduleService
	getBulkEffectivePickupTimesForDateFn func([]int64, timezone.Date) (map[int64]*scheduleService.EffectivePickupTime, error)
}

func (m *mockPickupService) GetBulkEffectivePickupTimesForDate(_ context.Context, studentIDs []int64, date timezone.Date) (map[int64]*scheduleService.EffectivePickupTime, error) {
	return m.getBulkEffectivePickupTimesForDateFn(studentIDs, date)
}

type mockArrivalService struct {
	scheduleService.ArrivalScheduleService
	getBulkEffectiveArrivalTimesForDateFn func([]int64, timezone.Date) (map[int64]*scheduleService.EffectiveArrivalTime, error)
}

func (m *mockArrivalService) GetBulkEffectiveArrivalTimesForDate(_ context.Context, studentIDs []int64, date timezone.Date) (map[int64]*scheduleService.EffectiveArrivalTime, error) {
	return m.getBulkEffectiveArrivalTimesForDateFn(studentIDs, date)
}

func fullSources() Sources {
	return Sources{
		Active:      &mockActiveService{},
		UserContext: &mockUserContextService{},
		Education:   &mockEducationService{},
		Schulhof:    &mockSchulhofService{},
		Operations:  &mockOperationsService{},
		Settings:    &configtest.Mock{},
		Pickups:     &mockPickupService{},
		Arrivals:    &mockArrivalService{},
	}
}

func adminContext() context.Context {
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
		AccountID: 99,
		TenantID:  23,
		Scope:     string(permissions.ScopeTenant),
		Admin:     true,
	})
	if err != nil {
		panic(err)
	}
	return permissions.WithPrincipal(context.Background(), principal)
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

func TestCurrentStaffIDBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	staff := &usersModels.Staff{Model: base.Model{ID: 42}}

	tests := []struct {
		name    string
		result  *usersModels.Staff
		err     error
		want    *int64
		wantErr bool
	}{
		{name: "not linked to staff", err: userContextService.ErrUserNotLinkedToStaff},
		{name: "not linked to person", err: userContextService.ErrUserNotLinkedToPerson},
		{name: "lookup error", err: errors.New("db down"), wantErr: true},
		{name: "nil staff", result: nil},
		{name: "staff found", result: staff, want: int64Ptr(42)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := access{userContext: &mockUserContextService{
				getCurrentStaffFn: func() (*usersModels.Staff, error) { return tt.result, tt.err },
			}}
			got, err := a.CurrentStaffID(ctx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCallerResolvesOverviewAndPermissions(t *testing.T) {
	t.Parallel()

	settings := &configtest.Mock{ResolveStringFn: func(_ context.Context, key string) (string, error) {
		if key != configModel.KeyOperationalOverviewScope {
			return "", errors.New("unexpected settings key: " + key)
		}
		return configModel.OverviewScopeAdmins, nil
	}}
	a := access{settings: settings}

	ctx := context.WithValue(adminContext(), jwt.CtxClaims, jwt.AppClaims{ID: 99, IsAdmin: true})
	ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{permissions.SchedulesRead})
	caller, err := a.Caller(ctx)
	require.NoError(t, err)
	assert.Equal(t, supervisiondashboard.Caller{
		AccountID: 99, TokenAdmin: true, AdminScope: true, OperationalOverview: true, CanReadSchedules: true,
	}, caller)

	settings.ResolveStringFn = func(context.Context, string) (string, error) {
		return configModel.OverviewScopeOwn, nil
	}
	caller, err = a.Caller(ctx)
	require.NoError(t, err)
	assert.True(t, caller.OperationalOverview, "admins always have the school-wide overview")

	settings.ResolveStringFn = func(context.Context, string) (string, error) { return "", errors.New("boom") }
	_, err = a.Caller(ctx)
	require.ErrorContains(t, err, "operational overview scope")

	plain := context.WithValue(context.Background(), jwt.CtxPermissions, []string{permissions.UsersRead})
	settings.ResolveStringFn = func(context.Context, string) (string, error) { return configModel.OverviewScopeOwn, nil }
	caller, err = a.Caller(plain)
	require.NoError(t, err)
	assert.Equal(t, supervisiondashboard.Caller{CanReadStudents: true}, caller)
}

func TestRunningSessionsResolveRelationsAndRooms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	color := "#83CD2D"
	endedAt := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)

	active := &mockActiveService{
		listActiveGroupsFn: func() ([]*activeModels.Group, error) {
			return []*activeModels.Group{
				{Model: base.Model{ID: 11}, RoomID: 21, ActualGroup: &activityModels.Group{Name: "Malen"}, Room: &facilitiesModels.Room{ID: 21, Name: "Zebra", Color: &color}},
				{Model: base.Model{ID: 12}, RoomID: 22},
				{Model: base.Model{ID: 13}, RoomID: 22},
				{Model: base.Model{ID: 14}, RoomID: 23, EndTime: &endedAt},
			}, nil
		},
		getRoomsByIDsFn: func(ids []int64) ([]*facilitiesModels.Room, error) {
			assert.Equal(t, []int64{22}, ids, "rooms are bulk-loaded once per missing relation")
			return []*facilitiesModels.Room{{ID: 22, Name: "Adler"}}, nil
		},
		getActiveGroupsByIDsFn: func(ids []int64) (map[int64]*activeModels.Group, error) {
			assert.Equal(t, []int64{11, 12, 13}, ids, "ended sessions are not re-read")
			return map[int64]*activeModels.Group{
				11: {Model: base.Model{ID: 11}, RoomID: 21, ActualGroup: &activityModels.Group{Name: "Malen"}, Room: &facilitiesModels.Room{ID: 21, Name: "Zebra", Color: &color}},
				12: {Model: base.Model{ID: 12}, RoomID: 22},
				13: {Model: base.Model{ID: 13}, RoomID: 22},
			}, nil
		},
		getStaffSupervisionsFn: func(staffID int64) ([]*activeModels.GroupSupervisor, error) {
			assert.Equal(t, int64(91), staffID)
			return []*activeModels.GroupSupervisor{{GroupID: 12}}, nil
		},
	}
	s := sessions{active: active}

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

	active.getRoomsByIDsFn = func([]int64) ([]*facilitiesModels.Room, error) { return nil, errors.New("boom") }
	_, err = s.Running(ctx)
	require.ErrorContains(t, err, "bulk load rooms")

	active.getActiveGroupsByIDsFn = func([]int64) (map[int64]*activeModels.Group, error) { return nil, errors.New("boom") }
	_, err = s.Running(ctx)
	require.ErrorContains(t, err, "load active group relations")

	active.listActiveGroupsFn = func() ([]*activeModels.Group, error) { return nil, errors.New("boom") }
	_, err = s.Running(ctx)
	require.EqualError(t, err, "boom")
}

func TestSupervisedSessionsAndUnclaimed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userContext := &mockUserContextService{getMySupervisedGroupsFn: func() ([]*activeModels.Group, error) {
		return []*activeModels.Group{{Model: base.Model{ID: 11}, RoomID: 21}, {Model: base.Model{ID: 12}}}, nil
	}}
	active := &mockActiveService{
		getRoomsByIDsFn: func(ids []int64) ([]*facilitiesModels.Room, error) {
			assert.Equal(t, []int64{21}, ids)
			return []*facilitiesModels.Room{{ID: 21, Name: "Zebra"}}, nil
		},
		getUnclaimedActiveGroupsFn: func() ([]*activeModels.Group, error) {
			return []*activeModels.Group{
				{Model: base.Model{ID: 11}, Room: &facilitiesModels.Room{ID: 21, Name: "Adler"}},
				{Model: base.Model{ID: 12}},
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

	userContext.getMySupervisedGroupsFn = func() ([]*activeModels.Group, error) { return nil, errors.New("boom") }
	_, err = s.Supervised(ctx)
	require.EqualError(t, err, "boom")
}

func TestMyGroupsResolvesRoomsOnlyForGroupsWithRooms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	roomID := int64(31)
	userContext := &mockUserContextService{getMyGroupsFn: func() ([]*educationModels.Group, error) {
		return []*educationModels.Group{
			{Model: base.Model{ID: 41}, Name: "Bären", RoomID: &roomID},
			{Model: base.Model{ID: 42}, Name: "Füchse"},
		}, nil
	}}
	education := &mockEducationService{getGroupsWithRoomsByIDsFn: func(ids []int64) (map[int64]*educationModels.Group, error) {
		assert.Equal(t, []int64{41}, ids)
		return map[int64]*educationModels.Group{
			41: {Model: base.Model{ID: 41}, Room: &facilitiesModels.Room{ID: 31, Name: "Igel"}},
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

	userContext.getMyGroupsFn = func() ([]*educationModels.Group, error) { return nil, nil }
	education.getGroupsWithRoomsByIDsFn = func([]int64) (map[int64]*educationModels.Group, error) {
		t.Fatal("no groups with rooms, no room lookup")
		return nil, nil
	}
	got, err = g.MyGroups(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSettingsBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := &configtest.Mock{
		ResolveBoolFn:   func(context.Context, string) (bool, error) { return true, nil },
		ResolveStringFn: func(context.Context, string) (string, error) { return "", errors.New("boom") },
	}
	s := settings{settings: mock}

	_, err := s.TrackingIndicatorLabels(ctx)
	require.Error(t, err)

	mock.ResolveStringFn = func(context.Context, string) (string, error) { return "  ", nil }
	labels, err := s.TrackingIndicatorLabels(ctx)
	require.NoError(t, err)
	assert.Empty(t, labels)

	mock.ResolveStringFn = func(_ context.Context, key string) (string, error) {
		if key == configModel.KeyTrackingIndicator1 {
			return "Hausaufgaben", nil
		}
		return "", nil
	}
	labels, err = s.TrackingIndicatorLabels(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"Hausaufgaben"}, labels)

	mock.ResolveBoolFn = func(_ context.Context, key string) (bool, error) {
		return key != configModel.KeyTrackingIndicatorsEnabled, nil
	}
	labels, err = s.TrackingIndicatorLabels(ctx)
	require.NoError(t, err)
	assert.Nil(t, labels, "disabled indicators carry no labels")

	mock.ResolveStringFn = func(context.Context, string) (string, error) { return "standard", nil }
	enabled, err := s.SpontaneousActivitiesEnabled(ctx)
	require.NoError(t, err)
	assert.False(t, enabled, "closed care concepts never start spontaneous activities")

	mock.ResolveStringFn = func(context.Context, string) (string, error) { return configModel.CareConceptOpenRooms, nil }
	enabled, err = s.SpontaneousActivitiesEnabled(ctx)
	require.NoError(t, err)
	assert.True(t, enabled)

	mock.ResolveBoolFn = func(context.Context, string) (bool, error) { return false, errors.New("boom") }
	_, err = s.SpontaneousActivitiesEnabled(ctx)
	require.ErrorContains(t, err, "spontaneous activities")

	mock.ResolveStringFn = func(context.Context, string) (string, error) { return "", errors.New("settings unavailable") }
	_, err = s.SpontaneousActivitiesEnabled(ctx)
	require.ErrorContains(t, err, "care concept")

	prepared, err := s.Prepare(ctx)
	require.NoError(t, err)
	assert.NotNil(t, prepared)
	mock.ResolveManyFn = func(context.Context, []string) (*configService.SettingsSnapshot, error) {
		return nil, errors.New("settings down")
	}
	_, err = s.Prepare(ctx)
	require.EqualError(t, err, "settings down")
}

func TestPresenceMapsVisitsAndAttendance(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	checkedIn := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	checkedOut := checkedIn.Add(time.Hour)
	sick := true
	photo := "/uploads/student-photos/p.jpg"
	active := &mockActiveService{
		getActiveGroupVisitsFn: func(activeGroupID int64) ([]*activeService.VisitWithStudentDisplay, error) {
			assert.Equal(t, int64(55), activeGroupID)
			return []*activeService.VisitWithStudentDisplay{
				{StudentID: 1, ActiveGroupID: 55, EntryTime: checkedIn, FirstName: "Erika", LastName: "Muster", SchoolClass: "4a", OGSGroupName: "Bären", Sick: &sick, SickSince: &checkedIn, PhotoPath: &photo},
				{StudentID: 2, ActiveGroupID: 55, EntryTime: checkedIn, ExitTime: &checkedOut},
				nil,
			}, nil
		},
		getAttendanceStatusesFn: func([]int64) (map[int64]*activeService.AttendanceStatus, error) {
			return map[int64]*activeService.AttendanceStatus{1: {CheckInTime: &checkedIn, CheckOutTime: &checkedOut}, 2: nil}, nil
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
	date := timezone.NewDate(2026, 8, 19)
	pickup := timezone.NormalizeWallClock(time.Date(2026, time.August, 19, 15, 30, 0, 0, time.UTC))
	arrival := timezone.NormalizeWallClock(time.Date(2026, time.August, 19, 11, 45, 0, 0, time.UTC))
	p := planning{
		pickups: &mockPickupService{getBulkEffectivePickupTimesForDateFn: func(studentIDs []int64, gotDate timezone.Date) (map[int64]*scheduleService.EffectivePickupTime, error) {
			assert.Equal(t, []int64{42}, studentIDs)
			assert.Equal(t, date, gotDate)
			return map[int64]*scheduleService.EffectivePickupTime{
				42: {Date: date, PickupTime: &pickup, WeekdayName: "Mittwoch", IsException: true, Notes: "Oma", DayNotes: []scheduleService.NoteData{{ID: 3, Content: "Klingeln"}}},
				43: nil,
			}, nil
		}},
		arrivals: &mockArrivalService{getBulkEffectiveArrivalTimesForDateFn: func([]int64, timezone.Date) (map[int64]*scheduleService.EffectiveArrivalTime, error) {
			return map[int64]*scheduleService.EffectiveArrivalTime{
				42: {Date: date, ArrivalTime: &arrival, WeekdayName: "Mittwoch", Notes: "Bus", DayNotes: []scheduleService.ArrivalNoteData{{ID: 4, Content: "Verspätung"}}},
			}, nil
		}},
	}

	pickups, err := p.Pickups(ctx, []int64{42}, "2026-08-19")
	require.NoError(t, err)
	clock := "15:30"
	assert.Equal(t, map[int64]supervisiondashboard.Pickup{42: {
		Date: "2026-08-19", WeekdayName: "Mittwoch", PickupTime: &clock, IsException: true, Notes: "Oma",
		DayNotes: []supervisiondashboard.DayNote{{ID: 3, Content: "Klingeln"}},
	}}, pickups, "nil plans are dropped, not projected as empty")

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
	now := time.Date(2026, time.August, 19, 12, 0, 0, 0, timezone.Berlin)
	pickup := "15:00"
	note := "Nachmittag"
	activeGroupID := int64(88)
	roomName := "Zebra"
	retained := scheduleService.OperationPlannedInstance{
		ID: 5, Title: "Malen", Date: "2026-08-19", StartTime: "14:00", EndTime: "15:00", RoomID: 21, RoomName: &roomName, Status: "planned",
		IsOverdue: true, MinutesUntilStart: 3, ExpectedStudentsCount: 4, PresentStudentsCount: 2, NotScheduledCount: 1,
		AssignedStaffIDs: []int64{7, 9}, IsAssigned: true, IsPrimary: true, IsSubstitute: false, IsAbsent: false,
		RosterPreview: []scheduleService.OperationRosterRow{{
			StudentID: 1, StudentName: "Erika Muster", SchoolClass: "4a", GroupName: "Bären", Planned: true, CurrentlyPresent: true,
			VisitID: int64Ptr(77), Status: "present", Substatus: &note, Note: &note, CheckedInAt: &pickup, VisitEntryTime: &pickup, PickupTime: &pickup,
			Warnings:          []scheduleService.OperationRosterWarning{{Kind: "arrival", Message: "zu früh", ExpectedArrival: &pickup, ExpectedGroupID: int64Ptr(3)}},
			ParallelPresentIn: &scheduleService.OperationParallelPresence{InstanceID: 6, Title: "Basteln", StartTime: "14:00", EndTime: "15:00"},
			CareDayStatus:     "scheduled",
		}},
		PickupTimesLoaded: true, PickupTimesRedacted: false,
		Warnings: []scheduleService.InstanceConflictWarning{{Kind: "staff", ResourceID: 7, Message: "doppelt", CanOverride: true, Fingerprint: "abc", ConflictingInstanceID: 6, ConflictingTitle: "Basteln", OverlapStart: "14:00", OverlapEnd: "14:30"}},
		CanStart: true, StartAvailableAt: "13:45", StartExpiresAt: "15:00", ActiveGroupID: &activeGroupID, CancelReason: &note,
		PlanningTrackName: &note, PlanningTrackColor: &note, GroupName: &note,
		StaffNames: []scheduleService.OperationStaffName{{StaffID: 7, DisplayName: "Erika", IsSubstitute: true}},
	}
	minimal := scheduleService.OperationPlannedInstance{ID: 6}
	operations := &mockOperationsService{
		plannedNowFn: func(day timezone.Date, gotNow time.Time, opts scheduleService.PlannedNowOptions) ([]scheduleService.OperationPlannedInstance, error) {
			assert.Equal(t, timezone.NewDate(2026, 8, 19), day)
			assert.Equal(t, now, gotNow)
			assert.Equal(t, scheduleService.PlannedNowOptions{HorizonMinutes: 480, Limit: 5, IncludeRoster: true}, opts)
			return []scheduleService.OperationPlannedInstance{retained, minimal}, nil
		},
		activeSessionsFn: func(day timezone.Date) ([]scheduleService.OperationActiveSession, error) {
			assert.Equal(t, timezone.NewDate(2026, 8, 19), day)
			return []scheduleService.OperationActiveSession{{ActiveGroupID: 88, InstanceID: 5, Title: "Malen", StartTime: "14:00", EndTime: "15:00"}}, nil
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
	assertSameJSON(t, []scheduleService.OperationActiveSession{{ActiveGroupID: 88, InstanceID: 5, Title: "Malen", StartTime: "14:00", EndTime: "15:00"}}, sessions)

	_, err = s.PlannedNow(ctx, supervisiondashboard.PlannedNowQuery{Date: "today"})
	require.Error(t, err)
}

func TestYardPreservesWireShape(t *testing.T) {
	t.Parallel()

	roomID := int64(21)
	retained := &facilitiesService.SchulhofStatus{
		Exists: true, RoomID: &roomID, RoomName: "Schulhof", ActivityGroupID: int64Ptr(3), ActiveGroupID: int64Ptr(8), IsUserSupervising: true,
		SupervisionID: int64Ptr(4), SupervisorCount: 1, StudentCount: 12,
		Supervisors: []facilitiesService.SupervisorInfo{{ID: 4, StaffID: 7, Name: "Erika Muster", IsCurrentUser: true}},
	}
	y := yard{schulhof: &mockSchulhofService{statusFn: func(staffID int64) (*facilitiesService.SchulhofStatus, error) {
		assert.Equal(t, int64(77), staffID)
		return retained, nil
	}}}

	status, err := y.Status(context.Background(), 77)
	require.NoError(t, err)
	assertSameJSON(t, retained, status)
	assertSameJSON(t, &facilitiesService.SchulhofStatus{}, schulhofStatus(&facilitiesService.SchulhofStatus{}))
	assert.Nil(t, schulhofStatus(nil))
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
