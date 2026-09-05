package iot

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/device"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestAvailableRoomsUsesDeviceTenantComposition(t *testing.T) {
	t.Parallel()

	db, services, feedback := testutil.SetupFeedbackAPITest(t)
	room := testpkg.CreateTestRoom(t, db, "Igelraum")
	testDevice := testpkg.CreateTestDevice(t, db, "tenant-room-device")
	resource := NewResource(ServiceDependencies{
		IoTService:               services.IoT,
		StaffPINAuthenticator:    services.StaffPINAuth,
		CheckinService:           services.Checkin,
		StaffClockService:        services.StaffClock,
		UsersService:             services.Users,
		ActiveService:            services.Active,
		ActivitiesService:        services.Activities,
		SettingsService:          services.Settings,
		FacilityService:          services.Facilities,
		EducationService:         services.Education,
		FeedbackService:          feedback,
		FeedbackResponseObserver: func(int, string) {},
		PickupScheduleService:    services.PickupSchedule,
		SchoolService:            services.Schools,
		TimetableDataService:     services.TimetableData,
		TimetableBridge:          services.TimetableBridge,
		UnregisteredTagScans:     services.UnregisteredTagScans,
		Broadcaster:              services.RealtimeHub,
		DevicePINFallback:        "1234",
		DB:                       db,
		DeviceLastSeenDebouncer:  device.NewLastSeenDebouncer(),
	})
	handler := testpkg.TenantRuntimeMiddleware(t, db)(resource.Router())

	request := httptest.NewRequest(http.MethodGet, "/rooms/available", nil)
	request.Header.Set("Authorization", "Bearer "+*testDevice.APIKey)
	request.Header.Set("X-Staff-PIN", "1234")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), room.Name)
}
