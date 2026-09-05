package domain

import (
	"errors"
	"time"
)

var ErrInstanceStudentNotFound = errors.New("instance student assignment not found")

const InstanceStudentUniqueConstraint = "unique_instance_student"

const (
	AttendanceExpected         = "expected"
	AttendanceAbsent           = "absent"
	AttendanceSubstatusSick    = "sick"
	AttendanceSubstatusExcused = "excused"
	AttendanceSubstatusTrip    = "field_trip"
)

type InstanceStudent struct {
	ID                 int64
	TenantID           int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	InstanceID         int64
	StudentID          int64
	RoomID             *int64
	Status             string
	Substatus          *string
	Note               *string
	CheckedInAt        *time.Time
	CheckedOutAt       *time.Time
	IsUnplanned        bool
	NotScheduled       bool
	ManualStatusAt     *time.Time
	StudentStatusDayID *int64
	PickupExceptionID  *int64
}

type InstanceStudentFields struct {
	InstanceID         int64
	StudentID          int64
	RoomID             *int64
	Status             string
	Substatus          *string
	Note               *string
	CheckedInAt        *time.Time
	CheckedOutAt       *time.Time
	IsUnplanned        bool
	NotScheduled       bool
	ManualStatusAt     *time.Time
	StudentStatusDayID *int64
	PickupExceptionID  *int64
}

type InstanceStudentFilter struct {
	IDs                        []int64
	InstanceIDs                []int64
	StudentIDs                 []int64
	Status                     *string
	Date                       *string
	FromDate                   *string
	ToDate                     *string
	CurrentTime                *string
	NotScheduledCandidatesOnly bool
	OrderByCreated             bool
	OrderByInstanceStudent     bool
	OrderByStudentActivityTime bool
	OrderByActivityDateTime    bool
	Limit                      int
	Offset                     int
}

type ParallelPresence struct {
	StudentID  int64
	InstanceID int64
	Title      string
	StartTime  time.Time
	EndTime    time.Time
}

type InstanceStudentKey struct {
	InstanceID int64
	StudentID  int64
}

type StudentInstanceRef struct {
	StudentID  int64
	InstanceID int64
}

type ScheduledInstanceRow struct {
	Instance   ActivityInstance
	Attendance InstanceStudent
}

type PickupException struct {
	ID            int64
	StudentID     int64
	ExceptionDate string
	ExcusedFrom   *time.Time
	ExcusedAuto   bool
}

type PickupExceptionFilter struct {
	IDs        []int64
	StudentIDs []int64
	Date       string
	From       string
}

type StudentStatusDay struct {
	ID        int64
	StudentID int64
	Date      string
	Status    string
}

type StudentStatusDayFilter struct {
	IDs        []int64
	StudentIDs []int64
	Date       string
	From       string
	ActiveOnly bool
	LatestOnly bool
}

type PartialAbsenceBlock struct {
	ID        int64
	Title     string
	StartTime time.Time
	EndTime   time.Time
}

type RosterRemoval struct {
	ID                 int64
	TenantID           int64
	TransitionID       int64
	InstanceID         int64
	StudentID          int64
	RoomID             *int64
	Status             string
	Substatus          *string
	Note               *string
	IsUnplanned        bool
	NotScheduled       bool
	ManualStatusAt     *time.Time
	StudentStatusDayID *int64
	CreatedAt          time.Time
}

type CareDay struct {
	StudentID int64
	Date      string
}

type RoomRef struct {
	ID       int64
	TenantID int64
}

type AttendanceFieldPatch struct {
	Status         *string
	Substatus      *string
	SubstatusClear bool
	Note           *string
	NoteClear      bool
}
