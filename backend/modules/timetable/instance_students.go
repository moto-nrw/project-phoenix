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

type CarePlanDirectory interface {
	FindPickupException(context.Context, int64) (*PickupException, error)
	ListPickupExceptions(context.Context, PickupExceptionFilter) ([]PickupException, error)
	FindStudentStatusDay(context.Context, int64, bool) (*StudentStatusDay, error)
	ListStudentStatusDays(context.Context, StudentStatusDayFilter) ([]StudentStatusDay, error)
}

type RoomRef struct {
	ID       int64
	TenantID int64
}

type RoomDirectory interface {
	LockRoomsByID(context.Context, []int64) ([]RoomRef, error)
}

type RoomDirectoryFunc func(context.Context, []int64) ([]RoomRef, error)

func (f RoomDirectoryFunc) LockRoomsByID(ctx context.Context, ids []int64) ([]RoomRef, error) {
	return f(ctx, ids)
}

type CareDayLocker interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
	LockExceptionDay(context.Context, int64, string) error
}

type careDayLockerFuncs struct {
	lockStudentAndDay func(context.Context, int64, string) error
	lockDay           func(context.Context, int64, string) error
}

func NewCareDayLocker(lockStudentAndDay, lockDay func(context.Context, int64, string) error) CareDayLocker {
	if lockStudentAndDay == nil || lockDay == nil {
		panic("timetable: care-day lock functions are required")
	}
	return careDayLockerFuncs{lockStudentAndDay: lockStudentAndDay, lockDay: lockDay}
}

func (f careDayLockerFuncs) LockStudentAndExceptionDay(ctx context.Context, studentID int64, date string) error {
	return f.lockStudentAndDay(ctx, studentID, date)
}

func (f careDayLockerFuncs) LockExceptionDay(ctx context.Context, studentID int64, date string) error {
	return f.lockDay(ctx, studentID, date)
}

type AttendanceFieldPatch struct {
	Status         *string
	Substatus      *string
	SubstatusClear bool
	Note           *string
	NoteClear      bool
}

func (p AttendanceFieldPatch) HasChanges() bool {
	return p.Status != nil || p.Substatus != nil || p.SubstatusClear || p.Note != nil || p.NoteClear
}

type InstanceStudentQuery interface {
	FindInstanceStudent(context.Context, int64) (InstanceStudent, error)
	ListInstanceStudents(context.Context, InstanceStudentFilter) ([]InstanceStudent, error)
	CountNonAbsentInstanceStudents(context.Context, []int64) (map[int64]int, error)
	ListParallelStudentPresence(context.Context, int64, string, []int64) ([]ParallelPresence, error)
	ListStudentInstanceRefsBefore(context.Context, string) ([]StudentInstanceRef, error)
	ListScheduledInstancesForStudent(context.Context, int64, string, string) ([]ScheduledInstanceRow, error)
	HasPlannedStudentSlots(context.Context, string, string) (bool, error)
	ListPlannedStudentIDs(context.Context, []int64, string) ([]int64, error)
	ListPartialAbsenceBlocks(context.Context, int64, string, time.Time) ([]PartialAbsenceBlock, error)
}

type InstanceStudentCommand interface {
	CreateInstanceStudent(context.Context, InstanceStudentInput) (InstanceStudent, error)
	UpdateInstanceStudent(context.Context, int64, InstanceStudentInput) (InstanceStudent, error)
	DeleteInstanceStudent(context.Context, int64) error
	DeleteInstanceStudentsByInstance(context.Context, int64) error
	UpdateAttendanceFromCheckin(context.Context, int64, int64, time.Time) (bool, error)
	UpdateAttendanceFromCheckinBatch(context.Context, []InstanceStudentKey, time.Time) error
	UpdateAttendanceCheckout(context.Context, int64, int64, time.Time) error
	UpdateAttendanceCheckoutBatch(context.Context, []InstanceStudentKey, time.Time) error
	CreateUnplannedPresentIfAbsent(context.Context, int64, int64, time.Time) (InstanceStudent, error)
	ReconcileAttendanceInterval(context.Context, int64, int64, time.Time, *time.Time, time.Time, *time.Time) (bool, error)
	UpdateAttendanceFields(context.Context, int64, AttendanceFieldPatch) error
	BulkUpdateStatus(context.Context, int64, string, string, []int64) (int, error)
	MarkNotScheduled(context.Context, []StudentInstanceRef) error
	MarkExpectedAbsentByActiveGroupIDs(context.Context, []int64, time.Time, []StudentInstanceRef) error
	CloseOpenCheckoutsByActiveGroupIDs(context.Context, []int64, time.Time) (int, error)
	ApplyStatusDay(context.Context, int64, string, int64, string) (int, error)
	ReleaseStatusDay(context.Context, int64) (int, error)
	ApplyActiveStatusDaysForInstance(context.Context, int64, string) (int, error)
	ApplyPartialAbsence(context.Context, int64) (int, error)
	ReleasePartialAbsence(context.Context, int64) (int, error)
	ApplyActivePartialAbsencesForInstance(context.Context, int64, string) (int, error)
	ArchivePlannedInstanceStudents(context.Context, int64, []int64, string, time.Time) (int, error)
	RestoreArchivedInstanceStudents(context.Context, int64, []int64, string) (int, error)
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

func validInstanceStudentKeys(keys []InstanceStudentKey) bool {
	for _, key := range keys {
		if key.InstanceID <= 0 || key.StudentID <= 0 {
			return false
		}
	}
	return true
}

func validStudentInstanceRefs(refs []StudentInstanceRef) bool {
	for _, ref := range refs {
		if ref.InstanceID <= 0 || ref.StudentID <= 0 {
			return false
		}
	}
	return true
}

func validAttendanceFieldPatch(patch AttendanceFieldPatch) bool {
	return patch.HasChanges() && (patch.Status == nil || validInstanceAttendanceStatus(*patch.Status)) &&
		(patch.Substatus == nil || validInstanceAttendanceSubstatus(*patch.Substatus)) &&
		(patch.Note == nil || len(*patch.Note) <= 500)
}
