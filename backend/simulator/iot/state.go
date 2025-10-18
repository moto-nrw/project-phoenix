package iot

import (
	"time"

	iotapi "github.com/moto-nrw/project-phoenix/api/iot"
)

// DeviceState caches the most recent snapshot of device-related data.
type DeviceState struct {
	Session       *iotapi.SessionCurrentResponse
	Students      []iotapi.TeacherStudentResponse
	Rooms         []iotapi.DeviceRoomResponse
	Activities    []iotapi.TeacherActivityResponse
	LastRefreshed time.Time
}

func (s *DeviceState) sessionActive() bool {
	return s != nil && s.Session != nil && s.Session.IsActive
}
