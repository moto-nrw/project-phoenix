package iot

import (
	"time"

	iotapi "github.com/moto-nrw/project-phoenix/api/iot"
)

// DeviceState caches the most recent snapshot of device-related data.
type DeviceState struct {
	Session                 *iotapi.SessionCurrentResponse
	Students                []iotapi.TeacherStudentResponse
	Rooms                   []iotapi.DeviceRoomResponse
	Activities              []iotapi.TeacherActivityResponse
	LastRefreshed           time.Time
	SessionManaged          bool
	ManagedSessionID        *int64
	LastSessionStartAttempt time.Time
	StudentStates           map[int64]*StudentState
	StaffRoster             map[int64]*StaffState
	RoomsByID               map[int64]iotapi.DeviceRoomResponse
	ActivitiesByID          map[int64]iotapi.TeacherActivityResponse
	ActiveSupervisors       map[int64]SupervisorAssignment
}

func (s *DeviceState) sessionActive() bool {
	return s != nil && s.Session != nil && s.Session.IsActive
}

// StudentState captures mutable data about a simulated student.
type StudentState struct {
	StudentID          int64
	PersonID           int64
	FirstName          string
	LastName           string
	RFIDTag            string
	CurrentPhase       RotationPhase
	NextPhase          RotationPhase
	CurrentRoomID      *int64
	RotationIndex      int
	AGHopCount         int
	AGHopTarget        int
	VisitedAGs         map[int64]time.Time
	AttendanceStatus   string
	LastAttendance     time.Time
	LastEventAt        time.Time
	HomeRoomID         *int64
	HomeDeviceID       string
	HasActiveVisit     bool
	VisitCooldownUntil time.Time
}

// StaffState captures roster info for supervisor rotation.
type StaffState struct {
	StaffID     int64
	PersonID    int64
	FirstName   string
	LastName    string
	DisplayName string
	IsLead      bool
	LastActive  time.Time
}

// SupervisorAssignment tracks who is currently supervising a session.
type SupervisorAssignment struct {
	StaffID     int64
	IsLead      bool
	LastUpdated time.Time
}

func (s *DeviceState) ensureIndexes() {
	if s.StudentStates == nil {
		s.StudentStates = make(map[int64]*StudentState)
	}
	if s.StaffRoster == nil {
		s.StaffRoster = make(map[int64]*StaffState)
	}
	if s.RoomsByID == nil {
		s.RoomsByID = make(map[int64]iotapi.DeviceRoomResponse)
	}
	if s.ActivitiesByID == nil {
		s.ActivitiesByID = make(map[int64]iotapi.TeacherActivityResponse)
	}
	if s.ActiveSupervisors == nil {
		s.ActiveSupervisors = make(map[int64]SupervisorAssignment)
	}
}

func cloneVisitedAGs(src map[int64]time.Time) map[int64]time.Time {
	if len(src) == 0 {
		return make(map[int64]time.Time)
	}
	dst := make(map[int64]time.Time, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
