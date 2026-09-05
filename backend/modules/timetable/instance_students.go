package timetable

import (
	"context"
	"time"
)

const (
	InstanceAttendanceExpected = "expected"
	InstanceAttendancePresent  = "present"
	InstanceAttendanceAbsent   = "absent"
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

type InstanceStudentInput struct {
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

type InstanceStudentQuery interface {
	FindInstanceStudent(context.Context, int64) (InstanceStudent, error)
	ListInstanceStudents(context.Context, InstanceStudentFilter) ([]InstanceStudent, error)
	CountNonAbsentInstanceStudents(context.Context, []int64) (map[int64]int, error)
	ListParallelStudentPresence(context.Context, int64, string, []int64) ([]ParallelPresence, error)
}

type InstanceStudentCommand interface {
	CreateInstanceStudent(context.Context, InstanceStudentInput) (InstanceStudent, error)
	UpdateInstanceStudent(context.Context, int64, InstanceStudentInput) (InstanceStudent, error)
	DeleteInstanceStudent(context.Context, int64) error
	DeleteInstanceStudentsByInstance(context.Context, int64) error
}

type InstanceStudentCapability interface {
	InstanceStudentQuery
	InstanceStudentCommand
}

func validInstanceStudent(input InstanceStudentInput) bool {
	return input.InstanceID > 0 && input.StudentID > 0 &&
		(input.RoomID == nil || *input.RoomID > 0) && validInstanceAttendanceStatus(input.Status) &&
		(input.Substatus == nil || validInstanceAttendanceSubstatus(*input.Substatus)) &&
		(input.StudentStatusDayID == nil || *input.StudentStatusDayID > 0) &&
		(input.PickupExceptionID == nil || *input.PickupExceptionID > 0) &&
		(input.Note == nil || len(*input.Note) <= 500)
}

func validInstanceAttendanceSubstatus(value string) bool {
	switch value {
	case "late", "excused", "sick", "field_trip", "other":
		return true
	default:
		return false
	}
}

func validInstanceAttendanceStatus(value string) bool {
	return value == InstanceAttendanceExpected || value == InstanceAttendancePresent || value == InstanceAttendanceAbsent
}

func validInstanceStudentFilter(filter InstanceStudentFilter) bool {
	orders := 0
	for _, ordered := range []bool{filter.OrderByCreated, filter.OrderByInstanceStudent,
		filter.OrderByStudentActivityTime, filter.OrderByActivityDateTime} {
		if ordered {
			orders++
		}
	}
	return filter.Limit >= 0 && filter.Offset >= 0 && orders <= 1 &&
		!hasInvalidID(filter.IDs) && !hasInvalidID(filter.InstanceIDs) && !hasInvalidID(filter.StudentIDs) &&
		(filter.Status == nil || validInstanceAttendanceStatus(*filter.Status)) &&
		validOptionalDate(filter.Date) && validOptionalDate(filter.FromDate) && validOptionalDate(filter.ToDate) &&
		(filter.CurrentTime == nil || (filter.Date != nil && validClock(*filter.CurrentTime)))
}
