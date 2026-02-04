package fixed

import (
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Result contains all created fixed data entities
type Result struct {
	// Core entities
	Rooms     []*facilities.Room
	Persons   []*users.Person
	RFIDCards []*users.RFIDCard
	Staff     []*users.Staff
	Teachers  []*users.Teacher
	Students  []*users.Student

	// Auth entities
	Accounts      []*auth.Account
	Roles         []*auth.Role
	AdminAccount  *auth.Account
	StaffWithPINs map[string]string // email -> PIN for testing

	// Education entities
	EducationGroups   []*education.Group
	ClassGroups       []*education.Group // Subset: grade classes
	SupervisionGroups []*education.Group // Subset: OGS groups

	// Activity entities
	ActivityCategories []*activities.Category
	ActivityGroups     []*activities.Group
	Enrollments        []*activities.StudentEnrollment

	// Schedule entities
	Timeframes []*schedule.Timeframe
	Schedules  []*activities.Schedule

	// IoT entities
	Devices       []*iot.Device
	DevicesByRoom map[int64]*iot.Device // room_id -> device

	// Time tracking
	WorkSessions      []*active.WorkSession
	WorkSessionBreaks []*active.WorkSessionBreak

	// Lookup maps for relationships
	PersonByID        map[int64]*users.Person
	StudentByPersonID map[int64]*users.Student
	TeacherByStaffID  map[int64]*users.Teacher
	RoomByID          map[int64]*facilities.Room
	GroupByID         map[int64]*education.Group
	ActivityByID      map[int64]*activities.Group
}

// NewResult creates a new result instance with initialized maps
func NewResult() *Result {
	return &Result{
		StaffWithPINs:     make(map[string]string),
		DevicesByRoom:     make(map[int64]*iot.Device),
		PersonByID:        make(map[int64]*users.Person),
		StudentByPersonID: make(map[int64]*users.Student),
		TeacherByStaffID:  make(map[int64]*users.Teacher),
		RoomByID:          make(map[int64]*facilities.Room),
		GroupByID:         make(map[int64]*education.Group),
		ActivityByID:      make(map[int64]*activities.Group),
	}
}
