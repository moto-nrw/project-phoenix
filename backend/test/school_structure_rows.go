package test

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

// The School Structure, School Membership and Workforce rows the fixtures
// write and the suites read back directly. models/education is gone (#3556)
// and the owners keep their row mappings private, so test support maps the
// tables itself, like ClassArrivalTimeRow.

// EducationGroup is one education.groups row.
type EducationGroup struct {
	bun.BaseModel `bun:"table:education.groups,alias:group"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	Name          string    `bun:"name,notnull"`
	RoomID        *int64    `bun:"room_id"`
}

// EducationGroupTeacher is one education.group_teacher row.
type EducationGroupTeacher struct {
	bun.BaseModel `bun:"table:education.group_teacher,alias:group_teacher"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	GroupID       int64     `bun:"group_id,notnull"`
	TeacherID     int64     `bun:"teacher_id,notnull"`
}

// EducationClassTeacher is one education.class_teachers row (#1772).
type EducationClassTeacher struct {
	bun.BaseModel `bun:"table:education.class_teachers,alias:class_teacher"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	StaffID       int64     `bun:"staff_id,notnull"`
	SchoolClass   string    `bun:"school_class,notnull"`
}

// The target types of education.group_substitution.
const (
	EducationGroupSubstitutionTypeGroupHandover = "group_handover"
	EducationGroupSubstitutionTypeLegacy        = "legacy_personnel_substitution"
)

// EducationGroupSubstitution is one education.group_substitution row.
type EducationGroupSubstitution struct {
	bun.BaseModel     `bun:"table:education.group_substitution,alias:group_substitution"`
	ID                int64         `bun:"id,pk,autoincrement"`
	CreatedAt         time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID          int64         `bun:"tenant_id,notnull"`
	TargetType        string        `bun:"target_type,notnull"`
	GroupID           int64         `bun:"group_id,notnull"`
	RegularStaffID    *int64        `bun:"regular_staff_id"`
	SubstituteStaffID int64         `bun:"substitute_staff_id,notnull"`
	StartDate         timezone.Date `bun:"start_date,notnull"`
	EndDate           timezone.Date `bun:"end_date,notnull"`
	Reason            string        `bun:"reason"`
}

// EducationGradeTransition is one education.grade_transitions row.
type EducationGradeTransition struct {
	bun.BaseModel `bun:"table:education.grade_transitions,alias:grade_transition"`
	ID            int64      `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	AcademicYear  string     `bun:"academic_year,notnull"`
	Status        string     `bun:"status,notnull,default:'draft'"`
	AppliedAt     *time.Time `bun:"applied_at"`
	AppliedBy     *int64     `bun:"applied_by"`
	RevertedAt    *time.Time `bun:"reverted_at"`
	RevertedBy    *int64     `bun:"reverted_by"`
	CreatedBy     int64      `bun:"created_by,notnull"`
	Notes         *string    `bun:"notes"`
	// RosterBaselineInstanceID is the highest schedule.activity_instances id
	// visible when the transition was applied (#405).
	RosterBaselineInstanceID *int64         `bun:"roster_baseline_instance_id"`
	Metadata                 map[string]any `bun:"metadata,type:jsonb,default:'{}'"`
}

// EducationGradeTransitionMapping is one class rename of a transition draft;
// a NULL target graduates the class.
type EducationGradeTransitionMapping struct {
	bun.BaseModel `bun:"table:education.grade_transition_mappings,alias:grade_transition_mapping"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	TransitionID  int64     `bun:"transition_id,notnull"`
	FromClass     string    `bun:"from_class,notnull"`
	ToClass       *string   `bun:"to_class"`
}

// The draft status and the history actions of the grade transition tables.
const (
	EducationTransitionStatusDraft     = "draft"
	EducationTransitionActionPromoted  = "promoted"
	EducationTransitionActionGraduated = "graduated"
)

// EducationGradeTransitionHistory is one child's row in a transition's
// history.
type EducationGradeTransitionHistory struct {
	bun.BaseModel `bun:"table:education.grade_transition_history,alias:grade_transition_history"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	TransitionID  int64     `bun:"transition_id,notnull"`
	StudentID     int64     `bun:"student_id,notnull"`
	PersonName    string    `bun:"person_name,notnull"`
	FromClass     string    `bun:"from_class,notnull"`
	ToClass       *string   `bun:"to_class"`
	Action        string    `bun:"action,notnull"`
	FromStatus    *string   `bun:"from_status"`
	RFIDTag       *string   `bun:"rfid_tag"`
}
