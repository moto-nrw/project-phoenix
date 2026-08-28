package supervisiondashboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
)

// The dependency interfaces are wide; the mocks embed them and override only
// what a test calls — an unexpected call panics on the nil embedded interface.
type mockActiveService struct {
	activeService.Service
	listActiveGroupsFn         func() ([]*activeModels.Group, error)
	getActiveGroupsByIDsFn     func(ids []int64) (map[int64]*activeModels.Group, error)
	getRoomsByIDsFn            func(ids []int64) ([]*facilitiesModels.Room, error)
	getUnclaimedActiveGroupsFn func() ([]*activeModels.Group, error)
	getTrackingIndicatorsFn    func(studentIDs []int64, labels []string) (map[int64][]bool, error)
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

func (m *mockActiveService) GetTrackingIndicators(_ context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	return m.getTrackingIndicatorsFn(studentIDs, labels)
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
}
type mockOperationsService struct {
	scheduleService.TimetableOperationsService
	plannedNowFn     func(scheduleService.PlannedNowOptions) ([]scheduleService.OperationPlannedInstance, error)
	activeSessionsFn func() ([]scheduleService.OperationActiveSession, error)
}

func (m *mockOperationsService) PlannedNow(_ context.Context, _ int64, _ bool, _ timezone.Date, _ time.Time, opts scheduleService.PlannedNowOptions) ([]scheduleService.OperationPlannedInstance, error) {
	return m.plannedNowFn(opts)
}

func (m *mockOperationsService) ActiveSessions(_ context.Context, _ timezone.Date) ([]scheduleService.OperationActiveSession, error) {
	return m.activeSessionsFn()
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

func fullDependencies() Dependencies {
	return Dependencies{
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
	return context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{ID: 99, IsAdmin: true})
}

func TestGetFailsFast(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, err := NewService(Dependencies{}).Get(ctx, 0)
	require.ErrorContains(t, err, "not fully configured")

	deps := fullDependencies()
	deps.Settings = &configtest.Mock{ResolveManyFn: func(context.Context, []string) (*configService.SettingsSnapshot, error) {
		return nil, errors.New("settings down")
	}}
	_, err = NewService(deps).Get(ctx, 0)
	require.ErrorContains(t, err, "resolve dashboard settings")

	deps = fullDependencies()
	deps.UserContext = &mockUserContextService{getCurrentStaffFn: func() (*usersModels.Staff, error) {
		return nil, errors.New("staff lookup failed")
	}}
	_, err = NewService(deps).Get(ctx, 0)
	require.ErrorContains(t, err, "load current staff")
}

func TestLoadCurrentStaffIDBranches(t *testing.T) {
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
			svc := &service{deps: Dependencies{UserContext: &mockUserContextService{
				getCurrentStaffFn: func() (*usersModels.Staff, error) { return tt.result, tt.err },
			}}}
			got, err := svc.loadCurrentStaffID(ctx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHasOperationalOverviewAdmin(t *testing.T) {
	t.Parallel()

	ctx := adminContext()

	settings := &configtest.Mock{ResolveStringFn: func(_ context.Context, key string) (string, error) {
		if key != configModel.KeyOperationalOverviewScope {
			return "", errors.New("unexpected settings key: " + key)
		}
		return configModel.OverviewScopeAdmins, nil
	}}
	svc := &service{deps: Dependencies{Settings: settings}}
	broad, err := svc.hasOperationalOverview(ctx)
	require.NoError(t, err)
	assert.True(t, broad)

	settings.ResolveStringFn = func(context.Context, string) (string, error) {
		return configModel.OverviewScopeOwn, nil
	}
	broad, err = svc.hasOperationalOverview(ctx)
	require.NoError(t, err)
	assert.False(t, broad, "the own scope keeps even admins on their own supervisions")

	settings.ResolveStringFn = func(context.Context, string) (string, error) { return "", errors.New("boom") }
	_, err = svc.hasOperationalOverview(ctx)
	require.ErrorContains(t, err, "operational overview scope")
}

func TestResolveGroupsBroadScope(t *testing.T) {
	t.Parallel()

	ctx := adminContext()
	color := "#83CD2D"
	endedAt := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)

	active := &mockActiveService{
		listActiveGroupsFn: func() ([]*activeModels.Group, error) {
			return []*activeModels.Group{
				{Model: base.Model{ID: 11}, RoomID: 21, ActualGroup: &activityModels.Group{Name: "Malen"}, Room: &facilitiesModels.Room{Model: base.Model{ID: 21}, Name: "Zebra", Color: &color}},
				{Model: base.Model{ID: 12}, RoomID: 22},
				{Model: base.Model{ID: 13}, RoomID: 22},
				{Model: base.Model{ID: 14}, RoomID: 23, EndTime: &endedAt},
			}, nil
		},
		getRoomsByIDsFn: func(ids []int64) ([]*facilitiesModels.Room, error) {
			assert.Equal(t, []int64{22}, ids)
			return []*facilitiesModels.Room{{Model: base.Model{ID: 22}, Name: "Adler"}}, nil
		},
		getActiveGroupsByIDsFn: func(ids []int64) (map[int64]*activeModels.Group, error) {
			assert.Equal(t, []int64{11, 12, 13}, ids)
			return map[int64]*activeModels.Group{
				11: {Model: base.Model{ID: 11}, RoomID: 21, ActualGroup: &activityModels.Group{Name: "Malen"}, Room: &facilitiesModels.Room{Model: base.Model{ID: 21}, Name: "Zebra", Color: &color}},
				12: {Model: base.Model{ID: 12}, RoomID: 22, Room: &facilitiesModels.Room{Model: base.Model{ID: 22}, Name: "Adler"}},
				13: {Model: base.Model{ID: 13}, RoomID: 22, Room: &facilitiesModels.Room{Model: base.Model{ID: 22}, Name: "Adler"}},
			}, nil
		},
	}
	settings := &configtest.Mock{ResolveStringFn: func(context.Context, string) (string, error) {
		return configModel.OverviewScopeAdmins, nil
	}}
	svc := &service{deps: Dependencies{Active: active, Settings: settings}}

	groups, err := svc.resolveGroups(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 3)
	assert.Equal(t, "Adler", groups[0].RoomName)
	assert.Equal(t, "Adler", groups[1].RoomName)
	assert.Equal(t, "Zebra", groups[2].RoomName)
	assert.Equal(t, &color, groups[2].RoomColor)
	assert.Equal(t, "Malen", groups[2].Name)
	assert.Equal(t, "Adler", groups[0].Name)

	active.listActiveGroupsFn = func() ([]*activeModels.Group, error) { return nil, errors.New("boom") }
	_, err = svc.resolveGroups(ctx)
	require.ErrorContains(t, err, "load active groups")
}

func TestResolveGroupsSupervisedErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	settings := &configtest.Mock{}

	userContext := &mockUserContextService{getMySupervisedGroupsFn: func() ([]*activeModels.Group, error) {
		return nil, errors.New("boom")
	}}
	svc := &service{deps: Dependencies{UserContext: userContext, Settings: settings}}
	_, err := svc.resolveGroups(ctx)
	require.ErrorContains(t, err, "load supervised groups")

	userContext.getMySupervisedGroupsFn = func() ([]*activeModels.Group, error) {
		return []*activeModels.Group{{Model: base.Model{ID: 11}, RoomID: 21}}, nil
	}
	svc.deps.Active = &mockActiveService{getRoomsByIDsFn: func([]int64) ([]*facilitiesModels.Room, error) {
		return nil, errors.New("boom")
	}}
	_, err = svc.resolveGroups(ctx)
	require.ErrorContains(t, err, "bulk load rooms")
}

func TestLoadStaticSectionsBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	roomID := int64(31)

	active := &mockActiveService{getUnclaimedActiveGroupsFn: func() ([]*activeModels.Group, error) {
		return []*activeModels.Group{
			{Model: base.Model{ID: 11}, Room: &facilitiesModels.Room{Model: base.Model{ID: 21}, Name: "Adler"}},
			{Model: base.Model{ID: 12}},
		}, nil
	}}
	userContext := &mockUserContextService{getMyGroupsFn: func() ([]*educationModels.Group, error) {
		return []*educationModels.Group{
			{Model: base.Model{ID: 41}, Name: "Bären", RoomID: &roomID},
			{Model: base.Model{ID: 42}, Name: "Füchse"},
		}, nil
	}}
	education := &mockEducationService{getGroupsWithRoomsByIDsFn: func(ids []int64) (map[int64]*educationModels.Group, error) {
		assert.Equal(t, []int64{41}, ids)
		return map[int64]*educationModels.Group{
			41: {Model: base.Model{ID: 41}, Room: &facilitiesModels.Room{Model: base.Model{ID: 31}, Name: "Igel"}},
			42: nil,
		}, nil
	}}
	svc := &service{deps: Dependencies{Active: active, UserContext: userContext, Education: education}}

	projection := emptyProjection()
	require.NoError(t, svc.loadStaticSections(ctx, projection))
	assert.Equal(t, []UnclaimedGroup{{ID: 11, RoomName: "Adler"}, {ID: 12}}, projection.UnclaimedGroups)
	assert.Equal(t, []EducationalGroup{{ID: 41, Name: "Bären", RoomName: "Igel"}, {ID: 42, Name: "Füchse"}}, projection.EducationalGroups)

	education.getGroupsWithRoomsByIDsFn = func([]int64) (map[int64]*educationModels.Group, error) { return nil, errors.New("boom") }
	require.ErrorContains(t, svc.loadStaticSections(ctx, emptyProjection()), "load education group rooms")

	userContext.getMyGroupsFn = func() ([]*educationModels.Group, error) { return nil, errors.New("boom") }
	require.ErrorContains(t, svc.loadStaticSections(ctx, emptyProjection()), "load educational groups")

	active.getUnclaimedActiveGroupsFn = func() ([]*activeModels.Group, error) { return nil, errors.New("boom") }
	require.ErrorContains(t, svc.loadStaticSections(ctx, emptyProjection()), "load unclaimed groups")
}

func TestLoadTrackingBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	studentIDs := []int64{11}

	settings := &configtest.Mock{
		ResolveBoolFn:   func(context.Context, string) (bool, error) { return true, nil },
		ResolveStringFn: func(context.Context, string) (string, error) { return "", errors.New("boom") },
	}
	svc := &service{deps: Dependencies{Settings: settings}}
	_, err := svc.loadTracking(ctx, studentIDs)
	require.Error(t, err)

	settings.ResolveStringFn = func(context.Context, string) (string, error) { return "  ", nil }
	tracking, err := svc.loadTracking(ctx, studentIDs)
	require.NoError(t, err)
	assert.Empty(t, tracking.Labels)

	settings.ResolveStringFn = func(_ context.Context, key string) (string, error) {
		if key == configModel.KeyTrackingIndicator1 {
			return "Hausaufgaben", nil
		}
		return "", nil
	}
	svc.deps.Active = &mockActiveService{getTrackingIndicatorsFn: func([]int64, []string) (map[int64][]bool, error) {
		return nil, errors.New("boom")
	}}
	_, err = svc.loadTracking(ctx, studentIDs)
	require.Error(t, err)

	svc.deps.Active = &mockActiveService{getTrackingIndicatorsFn: func(ids []int64, labels []string) (map[int64][]bool, error) {
		assert.Equal(t, studentIDs, ids)
		assert.Equal(t, []string{"Hausaufgaben"}, labels)
		return map[int64][]bool{11: {true}}, nil
	}}
	tracking, err = svc.loadTracking(ctx, studentIDs)
	require.NoError(t, err)
	assert.Equal(t, []string{"Hausaufgaben"}, tracking.Labels)
	assert.Equal(t, map[int64][]bool{11: {true}}, tracking.Results)
}

func TestResolveCapabilitiesClosedConcept(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	settings := &configtest.Mock{ResolveStringFn: func(context.Context, string) (string, error) { return "standard", nil }}
	svc := &service{deps: Dependencies{Settings: settings}}
	capabilities, err := svc.resolveCapabilities(ctx)
	require.NoError(t, err)
	assert.False(t, capabilities.WebSpontaneousActivitiesEnabled)

	settings.ResolveStringFn = func(context.Context, string) (string, error) { return configModel.CareConceptOpenRooms, nil }
	settings.ResolveBoolFn = func(context.Context, string) (bool, error) { return false, errors.New("boom") }
	_, err = svc.resolveCapabilities(ctx)
	require.ErrorContains(t, err, "spontaneous activities")
}
