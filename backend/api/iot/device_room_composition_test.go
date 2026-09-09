package iot

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestAvailableRoomsUsesDeviceTenantComposition(t *testing.T) {
	t.Parallel()

	db, data := testutil.SetupIoTDataModule(t)
	_, auth := testutil.SetupAuthModule(t)
	_, feedback := testutil.SetupFeedbackModule(t)
	room := testpkg.CreateTestRoom(t, db, "Igelraum")
	testDevice := testpkg.CreateTestDevice(t, db, "tenant-room-device")
	deviceAuth := testutil.NewDeviceAuthenticators(data.IoT.Fleet(), auth.Schools, auth.StaffPINAuth.AuthenticateStaffPIN, auth.Settings, "1234")
	resource := NewResource(ServiceDependencies{
		IoTService:               data.IoT,
		UsersService:             data.Users,
		ActivitiesService:        data.Activities,
		SettingsService:          auth.Settings,
		FacilityService:          data.Facilities,
		FeedbackService:          feedback.Feedback,
		FeedbackResponseObserver: func(int, string) {},
		StaffClock:               routerStaffClock{},
		SchoolService:            auth.Schools,
		DB:                       db,
		DeviceAuthenticator:      deviceAuth.Device(),
		DeviceOnlyAuthenticator:  deviceAuth.DeviceOnly(),
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
