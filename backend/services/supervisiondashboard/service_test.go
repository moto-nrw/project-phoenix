package supervisiondashboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

func TestSelectGroup(t *testing.T) {
	t.Parallel()

	groups := []Group{{ID: 10}, {ID: 20}}
	yardGroupID := int64(30)

	tests := []struct {
		name      string
		requested int64
		yard      *facilitiesService.SchulhofStatus
		want      *int64
		wantErr   error
	}{
		{name: "defaults to the first group", want: int64Ptr(10)},
		{name: "uses a requested group", requested: 20, want: int64Ptr(20)},
		{name: "permits the supervised yard group", requested: 30, yard: &facilitiesService.SchulhofStatus{IsUserSupervising: true, ActiveGroupID: &yardGroupID}, want: int64Ptr(30)},
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

func TestBuildVisitsAndEffectiveTimes(t *testing.T) {
	t.Parallel()

	checkedIn := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	checkedOut := checkedIn.Add(time.Hour)
	sick, excused := true, true
	storedPhoto := studentPhotoStoredURLPrefix + "portrait.jpg"
	rows := []*activeModels.VisitWithStudentDisplay{{
		StudentID: 1, ActiveGroupID: 2, EntryTime: checkedIn,
		FirstName: "  Erika", LastName: "Mustermann ", SchoolClass: "4a", OGSGroupName: "Bären",
		Sick: &sick, SickSince: &checkedIn, Excused: &excused, ExcusedSince: &checkedOut, PhotoPath: &storedPhoto,
	}}

	visits := buildVisits(rows, map[int64]*activeService.AttendanceStatus{1: {CheckInTime: &checkedIn, CheckOutTime: &checkedOut}}, true, true)
	require.Len(t, visits, 1)
	assert.Equal(t, "Erika Mustermann", visits[0].StudentName)
	assert.True(t, visits[0].Sick)
	assert.True(t, visits[0].Excused)
	assert.Equal(t, "/api/students/1/photo/portrait.jpg", visits[0].PhotoURL)
	assert.Equal(t, timezone.FormatBerlinClock(&checkedIn), visits[0].ActualArrivalTime)
	assert.Equal(t, timezone.FormatBerlinClock(&checkedOut), visits[0].ActualPickupTime)

	privateVisits := buildVisits(rows, nil, false, true)
	assert.Nil(t, privateVisits[0].ActualArrivalTime)
	assert.Empty(t, privateVisits[0].PhotoURL)

	pickup := time.Date(2026, time.August, 19, 15, 30, 0, 0, time.UTC)
	arrival := time.Date(2026, time.August, 19, 11, 45, 0, 0, time.UTC)
	pickupItem := pickupTimeFromEffective(1, &scheduleService.EffectivePickupTime{
		Date: timezone.DateFromTime(checkedIn), PickupTime: &pickup, WeekdayName: "Mittwoch", IsException: true, Notes: "Oma", DayNotes: []scheduleService.NoteData{{ID: 3, Content: "Klingeln"}},
	})
	assert.Equal(t, "15:30", *pickupItem.PickupTime)
	assert.Equal(t, []DayNote{{ID: 3, Content: "Klingeln"}}, pickupItem.DayNotes)

	arrivalItem := arrivalTimeFromEffective(1, &scheduleService.EffectiveArrivalTime{
		Date: timezone.DateFromTime(checkedIn), ArrivalTime: &arrival, WeekdayName: "Mittwoch", Notes: "Bus", DayNotes: []scheduleService.ArrivalNoteData{{ID: 4, Content: "Verspätung"}},
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

	service := &service{}
	ctx := context.Background()
	require.NoError(t, service.loadScheduleSections(ctx, projection))
	require.NoError(t, service.loadPlanningTimes(ctx, projection, nil, false))
	tracking, err := service.loadTracking(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, tracking.Labels)
	assert.Empty(t, tracking.Results)

	rooms, err := service.loadEducationRooms(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, rooms)
}

func TestLoadPlanningTimesProjectsPickupException(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 8, 19)
	pickup := time.Date(2026, time.August, 19, 15, 30, 0, 0, time.UTC)
	service := &service{deps: Dependencies{
		Now: func() time.Time { return date.BerlinMidnight().Add(12 * time.Hour) },
		Pickups: &mockPickupService{getBulkEffectivePickupTimesForDateFn: func(studentIDs []int64, gotDate timezone.Date) (map[int64]*scheduleService.EffectivePickupTime, error) {
			assert.Equal(t, []int64{42}, studentIDs)
			assert.Equal(t, date, gotDate)
			return map[int64]*scheduleService.EffectivePickupTime{42: {Date: date, PickupTime: &pickup, IsException: true}}, nil
		}},
		Arrivals: &mockArrivalService{getBulkEffectiveArrivalTimesForDateFn: func([]int64, timezone.Date) (map[int64]*scheduleService.EffectiveArrivalTime, error) {
			return map[int64]*scheduleService.EffectiveArrivalTime{}, nil
		}},
	}}
	ctx := context.WithValue(context.Background(), jwt.CtxPermissions, []string{permissions.UsersRead})
	projection := emptyProjection()

	require.NoError(t, service.loadPlanningTimes(ctx, projection, []int64{42}, true))
	require.Len(t, projection.PickupTimes, 1)
	assert.Equal(t, "15:30", *projection.PickupTimes[0].PickupTime)
	assert.True(t, projection.PickupTimes[0].IsException)
}

func TestLoadScheduleSectionsRedactsPickupTimesWithoutStudentRead(t *testing.T) {
	t.Parallel()

	pickupTime := "15:00"
	operations := &mockOperationsService{
		plannedNowFn: func(opts scheduleService.PlannedNowOptions) ([]scheduleService.OperationPlannedInstance, error) {
			assert.True(t, opts.IncludeRoster)
			return []scheduleService.OperationPlannedInstance{{
				PickupTimesLoaded: true,
				RosterPreview:     []scheduleService.OperationRosterRow{{PickupTime: &pickupTime}},
			}}, nil
		},
		activeSessionsFn: func() ([]scheduleService.OperationActiveSession, error) {
			return []scheduleService.OperationActiveSession{}, nil
		},
	}
	service := &service{deps: Dependencies{
		Operations: operations,
		Settings:   &configtest.Mock{},
		Now:        func() time.Time { return time.Date(2026, time.August, 19, 12, 0, 0, 0, timezone.Berlin) },
	}}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{ID: 99})
	ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{permissions.SchedulesRead})
	projection := emptyProjection()

	require.NoError(t, service.loadScheduleSections(ctx, projection))
	require.Len(t, projection.PlannedNow, 1)
	assert.False(t, projection.PlannedNow[0].PickupTimesLoaded)
	assert.True(t, projection.PlannedNow[0].PickupTimesRedacted)
	assert.Nil(t, projection.PlannedNow[0].RosterPreview[0].PickupTime)
}

func TestServiceDefaults(t *testing.T) {
	t.Parallel()

	service := NewService(Dependencies{}).(*service)
	assert.NotNil(t, service.deps.Now)
	assert.Error(t, service.validateDependencies())

	ctx, err := service.prefetchSettings(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, ctx)
}

func TestSettingsDrivenProjectionBranches(t *testing.T) {
	t.Parallel()

	settings := &configtest.Mock{
		ResolveStringFn: func(_ context.Context, key string) (string, error) {
			switch key {
			case configModel.KeyOperationalOverviewScope:
				return configModel.OverviewScopeAdmins, nil
			case configModel.KeyCareConcept:
				return configModel.CareConceptOpenRooms, nil
			default:
				return "", nil
			}
		},
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			return key == configModel.KeyWebSpontaneousActivities, nil
		},
	}
	service := &service{deps: Dependencies{Settings: settings}}

	overview, err := service.hasOperationalOverview(adminContext())
	require.NoError(t, err)
	assert.True(t, overview)

	capabilities, err := service.resolveCapabilities(context.Background())
	require.NoError(t, err)
	assert.True(t, capabilities.WebSpontaneousActivitiesEnabled)

	rooms, err := service.loadRooms(context.Background(), []*activeModels.Group{{}})
	require.NoError(t, err)
	assert.Empty(t, rooms)

	settings.ResolveStringFn = func(_ context.Context, _ string) (string, error) { return "", errors.New("settings unavailable") }
	_, err = service.resolveCapabilities(context.Background())
	require.Error(t, err)
}

func int64Ptr(value int64) *int64 { return &value }
