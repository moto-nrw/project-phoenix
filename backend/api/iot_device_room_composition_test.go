package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"

	iotcompose "github.com/moto-nrw/project-phoenix/api/iot/compose"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestAvailableRoomsUsesDeviceTenantComposition(t *testing.T) {
	t.Parallel()

	db, data, devices := setupIoTDataModule(t)
	_, auth := setupIoTSettingsModule(t)
	feedback, err := NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	identity, err := NewRFIDTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	room := testpkg.CreateTestRoom(t, db, "Igelraum")
	testDevice := testpkg.CreateTestDevice(t, db, "tenant-room-device")
	schools, err := NewDeviceSchools(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	deviceAuth := testutil.NewDeviceAuthenticators(devices.IoT.Fleet(), schools, auth)
	resource := iotcompose.NewResource(iotcompose.ServiceDependencies{Runtime: NewIoTHTTPRuntime(), ErrorReports: NewIoTErrorReporter(nil),
		Administration:           NewIoTAdministration(devicefleet.NewAdministration(devices.IoT.Fleet())),
		Configuration:            data.Configuration,
		Rooms:                    data.RoomAvailability,
		FeedbackService:          feedback.Feedback,
		FeedbackStudents:         identity.FeedbackStudents,
		FeedbackResponseObserver: func(int, string) {},
		DeviceScan:               routerDeviceScan{},
		StaffClock:               routerStaffClock{},
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
