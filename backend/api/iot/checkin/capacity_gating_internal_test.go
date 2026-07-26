// Package checkin internal tests for the per-tenant capacity-detail gating in
// renderCheckinError (issue #1879). Pure renderer tests — no database; the
// settings service is the shared configtest.Mock.
package checkin

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	checkinSvc "github.com/moto-nrw/project-phoenix/services/iot/checkin"
)

// renderGatedError drives rs.renderCheckinError with the given typed error and
// returns the recorded response body.
func renderGatedError(t *testing.T, rs *Resource, err error) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	rs.renderCheckinError(w, req, err)
	assert.Equal(t, 409, w.Code)
	return w.Body.String()
}

// settingsMock returns a SettingsService mock resolving the two capacity
// detail keys to the given values.
func settingsMock(activityEnabled, roomEnabled bool) *configtest.Mock {
	return &configtest.Mock{
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			switch key {
			case configModel.KeyCheckinActivityCapacityDetailsEnabled:
				return activityEnabled, nil
			case configModel.KeyCheckinRoomCapacityDetailsEnabled:
				return roomEnabled, nil
			default:
				return false, nil
			}
		},
	}
}

func activityCapacityErr() error {
	return &checkinSvc.ActivityCapacityError{
		ActivityID:       7,
		ActivityName:     "Bastelraum",
		CurrentOccupancy: 5,
		MaxCapacity:      4,
	}
}

func roomCapacityErr() error {
	return &checkinSvc.RoomCapacityError{
		RoomID:           42,
		RoomName:         "Room A",
		CurrentOccupancy: 15,
		MaxCapacity:      10,
	}
}

func TestRenderCheckinError_ActivityCapacity_SettingOn_IncludesDetails(t *testing.T) {
	rs := &Resource{SettingsService: settingsMock(true, true)}
	body := renderGatedError(t, rs, activityCapacityErr())

	assert.Contains(t, body, "\"details\"")
	assert.Contains(t, body, "\"current_occupancy\":5")
	assert.Contains(t, body, "\"max_capacity\":4")
	assert.Contains(t, body, "\"activity_name\":\"Bastelraum\"")
	assert.Contains(t, body, "ACTIVITY_CAPACITY_EXCEEDED")
}

func TestRenderCheckinError_ActivityCapacity_SettingOff_OmitsDetails(t *testing.T) {
	rs := &Resource{SettingsService: settingsMock(false, true)}
	body := renderGatedError(t, rs, activityCapacityErr())

	assert.NotContains(t, body, "\"details\"")
	assert.Contains(t, body, "\"code\":\"ACTIVITY_CAPACITY_EXCEEDED\"")
	assert.Contains(t, body, "\"message\":\"Activity capacity exceeded\"")
}

func TestRenderCheckinError_RoomCapacity_SettingOn_IncludesDetails(t *testing.T) {
	rs := &Resource{SettingsService: settingsMock(false, true)}
	body := renderGatedError(t, rs, roomCapacityErr())

	assert.Contains(t, body, "\"details\"")
	assert.Contains(t, body, "\"current_occupancy\":15")
	assert.Contains(t, body, "\"max_capacity\":10")
	assert.Contains(t, body, "\"room_name\":\"Room A\"")
	assert.Contains(t, body, "ROOM_CAPACITY_EXCEEDED")
}

func TestRenderCheckinError_RoomCapacity_SettingOff_OmitsDetails(t *testing.T) {
	rs := &Resource{SettingsService: settingsMock(false, false)}
	body := renderGatedError(t, rs, roomCapacityErr())

	assert.NotContains(t, body, "\"details\"")
	assert.Contains(t, body, "\"code\":\"ROOM_CAPACITY_EXCEEDED\"")
	assert.Contains(t, body, "\"message\":\"Room capacity exceeded\"")
}

// A nil settings service (bare-struct construction) must fall back to the
// registry defaults: activity details hidden, room details shown.
func TestRenderCheckinError_NilSettingsService_UsesPerKeyDefaults(t *testing.T) {
	rs := &Resource{}

	activityBody := renderGatedError(t, rs, activityCapacityErr())
	assert.NotContains(t, activityBody, "\"details\"", "activity details must default to hidden")

	roomBody := renderGatedError(t, rs, roomCapacityErr())
	assert.Contains(t, roomBody, "\"details\"", "room details must default to shown")
	assert.Contains(t, roomBody, "\"current_occupancy\":15")
}
