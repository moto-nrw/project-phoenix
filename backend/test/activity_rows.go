package test

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// The Timetable & Activities rows the fixtures write and the suites arrange
// directly. The owner keeps its row mapping in models/activities, which the
// shared fixtures may not import (#2748), so test support maps the tables
// itself, like the School Structure rows.

// WeekdaySunday is the ISO weekday number of Sunday in activity schedules.
const WeekdaySunday = 7

// ActivityGroupTypeCare is the type of a care activity group.
const ActivityGroupTypeCare = "care"

// ActivityDate is the calendar day of an activity row.
type ActivityDate = calendar.Date

// ActivityCategory is one activities.categories row.
type ActivityCategory struct {
	bun.BaseModel `bun:"table:activities.categories,alias:category"`
	ID            int64      `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	Name          string     `bun:"name,notnull"`
	Description   string     `bun:"description"`
	Color         string     `bun:"color"`
	IsSystem      bool       `bun:"is_system,notnull,default:false"`
	ShiftTypeID   *int64     `bun:"shift_type_id"`
	ArchivedAt    *time.Time `bun:"archived_at"`
}

// ActivityGroup is one activities.groups row.
type ActivityGroup struct {
	bun.BaseModel    `bun:"table:activities.groups,alias:group"`
	ID               int64      `bun:"id,pk,autoincrement"`
	CreatedAt        time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID         int64      `bun:"tenant_id,notnull"`
	Name             string     `bun:"name,notnull"`
	MaxParticipants  int        `bun:"max_participants,nullzero"`
	RequiredStaff    *int       `bun:"required_staff"`
	IsOpen           bool       `bun:"is_open,notnull,default:false"`
	CategoryID       int64      `bun:"category_id,notnull"`
	PlanningTrackID  *int64     `bun:"planning_track_id"`
	PlannedRoomID    *int64     `bun:"planned_room_id"`
	CreatedBy        *int64     `bun:"created_by"`
	Type             string     `bun:"type,notnull,default:'activity'"`
	EducationGroupID *int64     `bun:"education_group_id"`
	ListKind         *string    `bun:"list_kind"`
	IsTemplate       bool       `bun:"is_template,notnull,default:false"`
	IsSystem         bool       `bun:"is_system,notnull,default:false"`
	ArchivedAt       *time.Time `bun:"archived_at"`
	CalendarPeriodID *int64     `bun:"calendar_period_id"`
	TargetGroupType  string     `bun:"target_group_type,notnull,default:'none'"`
}

// StudentEnrollment is one activities.student_enrollments row.
type StudentEnrollment struct {
	bun.BaseModel            `bun:"table:activities.student_enrollments,alias:enrollment"`
	ID                       int64         `bun:"id,pk,autoincrement"`
	CreatedAt                time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt                time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID                 int64         `bun:"tenant_id,notnull"`
	StudentID                int64         `bun:"student_id,notnull"`
	ActivityGroupID          int64         `bun:"activity_group_id,notnull"`
	ValidFrom                ActivityDate  `bun:"valid_from,notnull"`
	ValidUntil               *ActivityDate `bun:"valid_until"`
	CalendarPeriodID         *int64        `bun:"calendar_period_id"`
	EnrollmentRequestChildID *int64        `bun:"enrollment_request_child_id"`
	SelectedWeekdays         []int         `bun:"selected_weekdays,type:jsonb,nullzero"`
	AttendanceStatus         *string       `bun:"attendance_status"`
	Weekday                  *int          `bun:"weekday"`
}

// ActivitySchedule is one activities.schedules row.
type ActivitySchedule struct {
	bun.BaseModel    `bun:"table:activities.schedules,alias:schedule"`
	ID               int64         `bun:"id,pk,autoincrement"`
	CreatedAt        time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID         int64         `bun:"tenant_id,notnull"`
	Weekday          int           `bun:"weekday,notnull"`
	TimeframeID      *int64        `bun:"timeframe_id"`
	ActivityGroupID  int64         `bun:"activity_group_id,notnull"`
	WeekPattern      int           `bun:"week_pattern,notnull,default:0"`
	CalendarPeriodID *int64        `bun:"calendar_period_id"`
	ValidUntil       *ActivityDate `bun:"valid_until"`
	ValidFrom        *ActivityDate `bun:"valid_from"`
}

// ActivitySupervisor is one activities.supervisors row: a planned
// supervisor of an activity.
type ActivitySupervisor struct {
	bun.BaseModel    `bun:"table:activities.supervisors,alias:supervisor"`
	ID               int64         `bun:"id,pk,autoincrement"`
	CreatedAt        time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID         int64         `bun:"tenant_id,notnull"`
	StaffID          int64         `bun:"staff_id,notnull"`
	GroupID          int64         `bun:"group_id,notnull"`
	IsPrimary        bool          `bun:"is_primary,notnull,default:false"`
	ValidFrom        ActivityDate  `bun:"valid_from,notnull,default:current_date"`
	ValidUntil       *ActivityDate `bun:"valid_until"`
	CalendarPeriodID *int64        `bun:"calendar_period_id"`
	Weekday          *int          `bun:"weekday"`
}
