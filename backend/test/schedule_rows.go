package test

import (
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// The Timetable & Activities schedule rows the fixtures write and the suites
// read back. The owner keeps its row mapping in models/schedule, which the
// shared fixtures may not import (#2748), so test support maps the tables
// itself, like the School Structure rows.

// The schedule values the fixtures write.
const (
	ScheduleAttendanceStatusExpected = "expected"
	ScheduleAttendanceStatusPresent  = "present"
	ScheduleAttendanceStatusAbsent   = "absent"
	ScheduleInstanceStatusPlanned    = "planned"
	ScheduleInstanceStatusActive     = "active"
	ScheduleInstanceStatusCompleted  = "completed"
	ScheduleInstanceStatusCancelled  = "cancelled"
	ScheduleExceptionCancelled       = "cancelled"
	ScheduleExceptionModified        = "modified"
	SchedulePeriodTypeCustom         = "custom"
	SchedulePeriodTypeSchoolYear     = "school_year"
	SchedulePickupSourceCareOffering = "care_offering"
	ScheduleWeekdayMonday            = 1
)

// ScheduleDate is the calendar day of a schedule row.
type ScheduleDate = calendar.Date

// ActivityInstance is one schedule.activity_instances row.
type ActivityInstance struct {
	bun.BaseModel          `bun:"table:schedule.activity_instances,alias:activity_instance"`
	ID                     int64           `bun:"id,pk,autoincrement"`
	CreatedAt              time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt              time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID               int64           `bun:"tenant_id,notnull"`
	Date                   calendar.Date   `bun:"date,notnull"`
	ActivityGroupID        *int64          `bun:"activity_group_id"`
	CalendarPeriodID       *int64          `bun:"calendar_period_id"`
	Title                  string          `bun:"title,notnull"`
	Description            *string         `bun:"description"`
	StartTime              time.Time       `bun:"start_time,notnull"`
	EndTime                time.Time       `bun:"end_time,notnull"`
	RoomID                 int64           `bun:"room_id,nullzero"`
	RequiredStaff          *int            `bun:"required_staff"`
	Status                 string          `bun:"status,notnull,default:'planned'"`
	ActiveGroupID          *int64          `bun:"active_group_id"`
	ListKind               *string         `bun:"list_kind"`
	IsSpontaneous          bool            `bun:"is_spontaneous,notnull,default:false"`
	UnderstaffedAck        bool            `bun:"understaffed_ack,notnull,default:false"`
	UnderstaffedNote       *string         `bun:"understaffed_note"`
	CancelReason           *string         `bun:"cancel_reason"`
	Notes                  *string         `bun:"notes"`
	IdempotencyKey         *string         `bun:"idempotency_key"`
	IdempotencyFingerprint *string         `bun:"idempotency_fingerprint"`
	CreatedBy              *int64          `bun:"created_by"`
	StartedBy              *int64          `bun:"started_by"`
	StartedAt              *time.Time      `bun:"started_at"`
	CompletedAt            *time.Time      `bun:"completed_at"`
	CompletedBy            *int64          `bun:"completed_by"`
	ReopenUntil            *time.Time      `bun:"reopen_until"`
	CompletionSnapshot     json.RawMessage `bun:"completion_snapshot,type:jsonb"`
}

// InstanceStudent is one schedule.instance_students row.
type InstanceStudent struct {
	bun.BaseModel      `bun:"table:schedule.instance_students,alias:instance_student"`
	ID                 int64      `bun:"id,pk,autoincrement"`
	CreatedAt          time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID           int64      `bun:"tenant_id,notnull"`
	InstanceID         int64      `bun:"instance_id,notnull"`
	StudentID          int64      `bun:"student_id,notnull"`
	RoomID             *int64     `bun:"room_id"`
	Status             string     `bun:"status,notnull,default:'expected'"`
	Substatus          *string    `bun:"substatus"`
	Note               *string    `bun:"note"`
	CheckedInAt        *time.Time `bun:"checked_in_at"`
	CheckedOutAt       *time.Time `bun:"checked_out_at"`
	IsUnplanned        bool       `bun:"is_unplanned,notnull,default:false"`
	NotScheduled       bool       `bun:"not_scheduled,notnull,default:false"`
	ManualStatusAt     *time.Time `bun:"manual_status_at"`
	StudentStatusDayID *int64     `bun:"student_status_day_id"`
	PickupExceptionID  *int64     `bun:"pickup_exception_id"`
}

// InstanceStaff is one schedule.instance_staff row.
type InstanceStaff struct {
	bun.BaseModel `bun:"table:schedule.instance_staff,alias:instance_staff"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	InstanceID    int64     `bun:"instance_id,notnull"`
	StaffID       int64     `bun:"staff_id,notnull"`
	RoomID        *int64    `bun:"room_id"`
	IsPrimary     bool      `bun:"is_primary,notnull,default:false"`
	IsSubstitute  bool      `bun:"is_substitute,notnull,default:false"`
	IsAbsent      bool      `bun:"is_absent,notnull,default:false"`
	AbsenceReason *string   `bun:"absence_reason"`
	SickAbsenceID *int64    `bun:"sick_absence_id"`
}

// StudentPickupSchedule is one schedule.student_pickup_schedules row.
type StudentPickupSchedule struct {
	bun.BaseModel  `bun:"table:schedule.student_pickup_schedules,alias:student_pickup_schedule"`
	ID             int64     `bun:"id,pk,autoincrement"`
	CreatedAt      time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID       int64     `bun:"tenant_id,notnull"`
	StudentID      int64     `bun:"student_id,notnull"`
	Weekday        int       `bun:"weekday,notnull"`
	PickupTime     time.Time `bun:"pickup_time,notnull"`
	Notes          *string   `bun:"notes"`
	CreatedBy      int64     `bun:"created_by,nullzero"`
	Source         string    `bun:"source,nullzero,notnull,default:'staff'"`
	CareOfferingID *int64    `bun:"care_offering_id,nullzero"`
}

// StudentPickupNote is one schedule.student_pickup_notes row.
type StudentPickupNote struct {
	bun.BaseModel `bun:"table:schedule.student_pickup_notes,alias:student_pickup_note"`
	ID            int64         `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64         `bun:"tenant_id,notnull"`
	StudentID     int64         `bun:"student_id,notnull"`
	NoteDate      calendar.Date `bun:"note_date,nullzero"`
	Weekday       int           `bun:"weekday,nullzero"`
	Content       string        `bun:"content,notnull"`
	CreatedBy     int64         `bun:"created_by,notnull"`
}

// StudentPickupException is one schedule.student_pickup_exceptions row.
type StudentPickupException struct {
	bun.BaseModel         `bun:"table:schedule.student_pickup_exceptions,alias:student_pickup_exception"`
	ID                    int64         `bun:"id,pk,autoincrement"`
	CreatedAt             time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID              int64         `bun:"tenant_id,notnull"`
	StudentID             int64         `bun:"student_id,notnull"`
	ExceptionDate         calendar.Date `bun:"exception_date,notnull"`
	PickupTime            *time.Time    `bun:"pickup_time"`
	Reason                *string       `bun:"reason"`
	ExcusedFrom           *time.Time    `bun:"excused_from"`
	ExcusedReason         *string       `bun:"excused_reason"`
	ExcusedCreatedBy      *int64        `bun:"excused_created_by"`
	ExcusedOwnsPickupTime bool          `bun:"excused_owns_pickup_time,notnull,default:false"`
	ExcusedAuto           bool          `bun:"excused_auto,notnull,default:false"`
	Source                string        `bun:"source,nullzero,notnull,default:'staff'"`
	CreatedBy             int64         `bun:"created_by,nullzero"`
	CreatedByGuardian     *int64        `bun:"created_by_guardian,nullzero"`
}

// StudentArrivalSchedule is one schedule.student_arrival_schedules row.
type StudentArrivalSchedule struct {
	bun.BaseModel   `bun:"table:schedule.student_arrival_schedules,alias:student_arrival_schedule"`
	ID              int64     `bun:"id,pk,autoincrement"`
	CreatedAt       time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID        int64     `bun:"tenant_id,notnull"`
	StudentID       int64     `bun:"student_id,notnull"`
	Weekday         int       `bun:"weekday,notnull"`
	ExpectedArrival time.Time `bun:"expected_arrival,nullzero"`
	Notes           *string   `bun:"notes"`
	CreatedBy       int64     `bun:"created_by,notnull"`
}

// StudentArrivalException is one schedule.student_arrival_exceptions row.
type StudentArrivalException struct {
	bun.BaseModel     `bun:"table:schedule.student_arrival_exceptions,alias:student_arrival_exception"`
	ID                int64         `bun:"id,pk,autoincrement"`
	CreatedAt         time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID          int64         `bun:"tenant_id,notnull"`
	StudentID         int64         `bun:"student_id,notnull"`
	ExceptionDate     calendar.Date `bun:"exception_date,notnull"`
	ExpectedArrival   *time.Time    `bun:"expected_arrival"`
	Reason            *string       `bun:"reason"`
	Source            string        `bun:"source,nullzero,notnull,default:'staff'"`
	CreatedBy         int64         `bun:"created_by,nullzero"`
	CreatedByGuardian *int64        `bun:"created_by_guardian,nullzero"`
}

// StudentArrivalNote is one schedule.student_arrival_notes row.
type StudentArrivalNote struct {
	bun.BaseModel `bun:"table:schedule.student_arrival_notes,alias:student_arrival_note"`
	ID            int64         `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64         `bun:"tenant_id,notnull"`
	StudentID     int64         `bun:"student_id,notnull"`
	NoteDate      calendar.Date `bun:"note_date,notnull"`
	Content       string        `bun:"content,notnull"`
	CreatedBy     int64         `bun:"created_by,notnull"`
}

// CalendarPeriod is one schedule.calendar_periods row.
type CalendarPeriod struct {
	bun.BaseModel   `bun:"table:schedule.calendar_periods,alias:calendar_period"`
	ID              int64          `bun:"id,pk,autoincrement"`
	CreatedAt       time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID        int64          `bun:"tenant_id,notnull"`
	Name            string         `bun:"name,notnull"`
	PeriodType      string         `bun:"period_type,notnull"`
	StartDate       calendar.Date  `bun:"start_date,notnull"`
	EndDate         calendar.Date  `bun:"end_date,notnull"`
	WeekCycleLength int            `bun:"week_cycle_length,notnull,default:1"`
	WeekCycleAnchor *calendar.Date `bun:"week_cycle_anchor"`
	IsActive        bool           `bun:"is_active,notnull,default:false"`
}

// Timeframe is one schedule.timeframes row.
type Timeframe struct {
	bun.BaseModel `bun:"table:schedule.timeframes,alias:timeframe"`
	ID            int64      `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	StartTime     time.Time  `bun:"start_time,notnull"`
	EndTime       *time.Time `bun:"end_time"`
	IsActive      bool       `bun:"is_active,notnull,default:false"`
	Description   string     `bun:"description"`
}

// StaffShift is one schedule.staff_shifts row.
type StaffShift struct {
	bun.BaseModel        `bun:"table:schedule.staff_shifts,alias:staff_shift"`
	ID                   int64          `bun:"id,pk,autoincrement"`
	CreatedAt            time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt            time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID             int64          `bun:"tenant_id,notnull"`
	StaffID              int64          `bun:"staff_id,notnull"`
	Date                 calendar.Date  `bun:"date,notnull,type:date"`
	StartTime            time.Time      `bun:"start_time,notnull"`
	EndTime              time.Time      `bun:"end_time,notnull"`
	BreakMinutes         int            `bun:"break_minutes,notnull,default:0"`
	ShiftTypeID          *int64         `bun:"shift_type_id"`
	Notes                string         `bun:"notes"`
	SeriesID             *int64         `bun:"series_id"`
	Detached             bool           `bun:"detached,notnull,default:false"`
	SeriesOccurrenceDate *calendar.Date `bun:"series_occurrence_date,type:date"`
	Cancelled            bool           `bun:"cancelled,notnull,default:false"`
	ChangeReason         *string        `bun:"change_reason"`
	OriginShiftID        *int64         `bun:"origin_shift_id"`
	SickAbsenceID        *int64         `bun:"sick_absence_id"`
	CreatedBy            int64          `bun:"created_by,notnull"`
	UpdatedBy            *int64         `bun:"updated_by"`
}

// Dateframe is one schedule.dateframes row.
type Dateframe struct {
	bun.BaseModel `bun:"table:schedule.dateframes,alias:dateframe"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	StartDate     time.Time `bun:"start_date,notnull"`
	EndDate       time.Time `bun:"end_date,notnull"`
	Name          string    `bun:"name"`
	Description   string    `bun:"description"`
}

// ClosingDay is one schedule.closing_days row.
type ClosingDay struct {
	bun.BaseModel `bun:"table:schedule.closing_days,alias:closing_day"`
	ID            int64         `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64         `bun:"tenant_id,notnull"`
	StartDate     calendar.Date `bun:"start_date,notnull"`
	EndDate       calendar.Date `bun:"end_date,notnull"`
	Reason        string        `bun:"reason,notnull"`
}

// ActivityException is one schedule.activity_exceptions row.
type ActivityException struct {
	bun.BaseModel   `bun:"table:schedule.activity_exceptions,alias:activity_exception"`
	ID              int64         `bun:"id,pk,autoincrement"`
	CreatedAt       time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID        int64         `bun:"tenant_id,notnull"`
	ActivityGroupID int64         `bun:"activity_group_id,notnull"`
	ExceptionDate   calendar.Date `bun:"exception_date,notnull"`
	ExceptionType   string        `bun:"exception_type,notnull"`
	StartTime       *time.Time    `bun:"start_time"`
	EndTime         *time.Time    `bun:"end_time"`
	RoomID          *int64        `bun:"room_id"`
	Reason          *string       `bun:"reason"`
	CreatedBy       *int64        `bun:"created_by"`
}
