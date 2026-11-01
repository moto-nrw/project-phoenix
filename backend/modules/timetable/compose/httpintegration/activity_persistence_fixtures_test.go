package httpintegration_test

import (
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

// These private persistence fixtures restore the original Bun column metadata.
// Native HTTP contracts do not carry ORM tags; pointer conversions preserve the
// exact field values and insert-returned IDs without changing test assertions.

type Group struct {
	ID                    int64                                 `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt             time.Time                             `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt             time.Time                             `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	TenantID              int64                                 `bun:"tenant_id,notnull" json:"tenant_id"`
	Name                  string                                `bun:"name,notnull" json:"name"`
	MaxParticipants       int                                   `bun:"max_participants,nullzero" json:"max_participants"`
	RequiredStaff         *int                                  `bun:"required_staff" json:"required_staff,omitempty"`
	IsOpen                bool                                  `bun:"is_open,notnull,default:false" json:"is_open"`
	CategoryID            int64                                 `bun:"category_id,notnull" json:"category_id"`
	PlanningTrackID       *int64                                `bun:"planning_track_id" json:"planning_track_id,omitempty"`
	PlannedRoomID         *int64                                `bun:"planned_room_id" json:"planned_room_id,omitempty"`
	CreatedBy             *int64                                `bun:"created_by" json:"created_by"`
	Type                  string                                `bun:"type,notnull,default:'activity'" json:"type"`
	EducationGroupID      *int64                                `bun:"education_group_id" json:"education_group_id,omitempty"`
	ListKind              *string                               `bun:"list_kind" json:"list_kind,omitempty"`
	IsTemplate            bool                                  `bun:"is_template,notnull,default:false" json:"is_template"`
	IsSystem              bool                                  `bun:"is_system,notnull,default:false" json:"is_system"`
	ArchivedAt            *time.Time                            `bun:"archived_at" json:"archived_at,omitempty"`
	SeriesRootID          *int64                                `bun:"series_root_id" json:"-"`
	CalendarPeriodID      *int64                                `bun:"calendar_period_id" json:"calendar_period_id,omitempty"`
	TargetGroupType       string                                `bun:"target_group_type,notnull,default:'none'" json:"target_group_type"`
	TargetGradeLevel      *int16                                `bun:"target_grade_level" json:"target_grade_level,omitempty"`
	TargetSchoolClass     *string                               `bun:"target_school_class" json:"target_school_class,omitempty"`
	SourceCareOfferingIDs []int64                               `bun:"source_care_offering_ids,type:jsonb,nullzero" json:"source_care_offering_ids,omitempty"`
	SourceGradeLevels     []int                                 `bun:"source_grade_levels,type:jsonb,nullzero" json:"source_grade_levels,omitempty"`
	SourceSchoolClasses   []string                              `bun:"source_school_classes,type:jsonb,nullzero" json:"source_school_classes,omitempty"`
	Notes                 *string                               `bun:"notes" json:"notes,omitempty"`
	IncludeClosingDays    bool                                  `bun:"include_closing_days,notnull,default:false" json:"-"`
	SeriesLastDay         *activityContract.Date                `bun:"series_last_day" json:"-"`
	Category              *activityContract.Category            `bun:"-" json:"category,omitempty"`
	Supervisors           []*activityContract.SupervisorPlanned `bun:"-" json:"supervisors,omitempty"`
	Schedules             []*activityContract.Schedule          `bun:"-" json:"schedules,omitempty"`
}

func activityGroupFixture(v *activityContract.Group) *Group { return (*Group)(v) }

type Schedule struct {
	ID               int64                  `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt        time.Time              `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt        time.Time              `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	TenantID         int64                  `bun:"tenant_id,notnull" json:"tenant_id"`
	Weekday          int                    `bun:"weekday,notnull" json:"weekday"`
	TimeframeID      *int64                 `bun:"timeframe_id" json:"timeframe_id,omitempty"`
	ActivityGroupID  int64                  `bun:"activity_group_id,notnull" json:"activity_group_id"`
	WeekPattern      int                    `bun:"week_pattern,notnull,default:0" json:"week_pattern"`
	CalendarPeriodID *int64                 `bun:"calendar_period_id" json:"calendar_period_id,omitempty"`
	ValidUntil       *activityContract.Date `bun:"valid_until" json:"valid_until,omitempty"`
	ValidFrom        *activityContract.Date `bun:"valid_from" json:"valid_from,omitempty"`
}

func activityScheduleFixture(v *activityContract.Schedule) *Schedule { return (*Schedule)(v) }

type SupervisorPlanned struct {
	ID               int64                   `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt        time.Time               `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt        time.Time               `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	TenantID         int64                   `bun:"tenant_id,notnull" json:"tenant_id"`
	StaffID          int64                   `bun:"staff_id,notnull" json:"staff_id"`
	GroupID          int64                   `bun:"group_id,notnull" json:"group_id"`
	IsPrimary        bool                    `bun:"is_primary,notnull,default:false" json:"is_primary"`
	ValidFrom        activityContract.Date   `bun:"valid_from,notnull,default:current_date" json:"valid_from"`
	ValidUntil       *activityContract.Date  `bun:"valid_until" json:"valid_until,omitempty"`
	CalendarPeriodID *int64                  `bun:"calendar_period_id" json:"calendar_period_id,omitempty"`
	Weekday          *int                    `bun:"weekday" json:"weekday,omitempty"`
	StaffPersonID    int64                   `bun:"-" json:"-"`
	FirstName        string                  `bun:"-" json:"first_name,omitempty"`
	LastName         string                  `bun:"-" json:"last_name,omitempty"`
	Group            *activityContract.Group `bun:"-" json:"group,omitempty"`
}

func activitySupervisorPlannedFixture(v *activityContract.SupervisorPlanned) *SupervisorPlanned {
	return (*SupervisorPlanned)(v)
}

type StudentEnrollment struct {
	ID                        int64                   `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt                 time.Time               `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt                 time.Time               `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	TenantID                  int64                   `bun:"tenant_id,notnull" json:"tenant_id"`
	StudentID                 int64                   `bun:"student_id,notnull" json:"student_id"`
	ActivityGroupID           int64                   `bun:"activity_group_id,notnull" json:"activity_group_id"`
	ValidFrom                 activityContract.Date   `bun:"valid_from,notnull" json:"valid_from"`
	ValidUntil                *activityContract.Date  `bun:"valid_until" json:"valid_until,omitempty"`
	CalendarPeriodID          *int64                  `bun:"calendar_period_id" json:"calendar_period_id,omitempty"`
	EnrollmentRequestChildID  *int64                  `bun:"enrollment_request_child_id" json:"enrollment_request_child_id,omitempty"`
	SelectedWeekdays          []int                   `bun:"selected_weekdays,type:jsonb,nullzero" json:"selected_weekdays,omitempty"`
	AttendanceStatus          *string                 `bun:"attendance_status" json:"attendance_status,omitempty"`
	Weekday                   *int                    `bun:"weekday" json:"weekday,omitempty"`
	KeptWithoutBookingThrough *activityContract.Date  `bun:"kept_without_booking_through" json:"kept_without_booking_through,omitempty"`
	StudentAlumnus            bool                    `bun:"-" json:"-"`
	StudentFirstName          string                  `bun:"-" json:"-"`
	StudentLastName           string                  `bun:"-" json:"-"`
	ActivityGroup             *activityContract.Group `bun:"-" json:"activity_group,omitempty"`
}

func activityStudentEnrollmentFixture(v *activityContract.StudentEnrollment) *StudentEnrollment {
	return (*StudentEnrollment)(v)
}

func nativeScheduleRows(rows []*Schedule) []*activityContract.Schedule {
	if rows == nil {
		return nil
	}
	result := make([]*activityContract.Schedule, len(rows))
	for i, row := range rows {
		result[i] = (*activityContract.Schedule)(row)
	}
	return result
}

func nativeStudentEnrollmentRows(rows []*StudentEnrollment) []*activityContract.StudentEnrollment {
	if rows == nil {
		return nil
	}
	result := make([]*activityContract.StudentEnrollment, len(rows))
	for i, row := range rows {
		result[i] = (*activityContract.StudentEnrollment)(row)
	}
	return result
}

func nativeSupervisorPlannedRows(rows []*SupervisorPlanned) []*activityContract.SupervisorPlanned {
	if rows == nil {
		return nil
	}
	result := make([]*activityContract.SupervisorPlanned, len(rows))
	for i, row := range rows {
		result[i] = (*activityContract.SupervisorPlanned)(row)
	}
	return result
}
