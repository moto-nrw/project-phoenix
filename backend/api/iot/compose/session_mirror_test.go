package compose

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStartMirrorsTenantAndWallClock(t *testing.T) {
	t.Parallel()
	db, active := testutil.SetupActiveModule(t)
	_, timetable := testutil.SetupTimetableModule(t)
	_, auth := testutil.SetupAuthModule(t)
	activity := testpkg.CreateTestActivityGroup(t, db, "Mirror Activity")
	room := testpkg.CreateTestRoom(t, db, "Mirror Room")
	staff := testpkg.CreateTestStaff(t, db, "Mirror", "Supervisor")
	device := testpkg.CreateTestDevice(t, db, "mirror-device")
	deviceAuth := testutil.NewDeviceAuthenticators(active.IoT.Fleet(), auth.Schools, auth.StaffPINAuth.AuthenticateStaffPIN, auth.Settings, "1234")
	mirror := devicescanCompose.NewSessionMirror(timetable.TimetableData, active.Activities, nil, nil)
	resource := newRouterTestResource()
	resource.DB = db
	resource.DeviceAuthenticator = deviceAuth.Device()
	resource.DeviceOnlyAuthenticator = deviceAuth.DeviceOnly()
	resource.SessionLifecycle = devicescanCompose.NewSessionLifecycle(active.Active, active.Users, active.IoT, mirror, nil)
	handler := testpkg.TenantRuntimeMiddleware(t, db)(resource.Router())
	request := testutil.NewAuthenticatedRequest(t, "POST", "/session/start", map[string]any{
		"activity_id": activity.ID, "room_id": room.ID, "supervisor_ids": []int64{staff.ID},
	})
	request.Header.Set("Authorization", "Bearer "+*device.APIKey)
	request.Header.Set("X-Staff-PIN", "1234")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	payload := testutil.ParseJSONResponse(t, response.Body.Bytes())["data"].(map[string]any)
	groupID := int64(payload["active_group_id"].(float64))
	ctx := testpkg.TenantContext(testpkg.Tenant(t))
	instance, err := timetable.TimetableData.GetInstanceByActiveGroupID(ctx, groupID)
	require.NoError(t, err)
	require.NotNil(t, instance, "a successful start persists its timetable mirror")
	assert.Equal(t, testpkg.Tenant(t), instance.GetTenantID())
	assert.Equal(t, "active", string(instance.Status))
	assert.True(t, instance.IsSpontaneous)
	assert.Equal(t, activity.Name, instance.Title)
	assert.Equal(t, room.ID, instance.RoomID)
	require.NotNil(t, instance.StartedAt)
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	local := instance.StartedAt.In(berlin)
	assert.Equal(t, local.Format(time.DateOnly), instance.Date.String())
	minutes := min(local.Hour()*60+local.Minute(), 23*60+30)
	assert.Equal(t, minutes, instance.StartTime.Hour()*60+instance.StartTime.Minute())
	assert.Equal(t, min(minutes+60, 23*60+59), instance.EndTime.Hour()*60+instance.EndTime.Minute())
	assert.Equal(t, 0, instance.StartTime.Second())
	rows, err := timetable.TimetableData.GetInstanceStaff(ctx, instance.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, staff.ID, rows[0].StaffID)
	assert.Equal(t, testpkg.Tenant(t), rows[0].GetTenantID())
}
