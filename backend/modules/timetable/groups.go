package timetable

import "time"

const (
	TargetGroupTypeGrade          = "jahrgang"
	TargetGroupTypeSchoolClass    = "klasse"
	TargetGroupTypeEducationGroup = "gruppe"
	TargetGroupTypeOffering       = "angebot"
	TargetGroupTypeNone           = "none"

	GroupTypeActivity = "activity"
	GroupTypeCare     = "care"
	GroupTypeExternal = "external"

	ListKindEdgeHours    = "edge_hours"
	ListKindLearningTime = "learning_time"
	ListKindActivity     = "activity"
	ListKindMensa        = "mensa"
)

// Group is the owner view of one activities.groups row. Cross-owner people
// projections are deliberately absent; composition attaches them where a
// transport needs names.
type Group struct {
	ID                    int64      `json:"id"`
	TenantID              int64      `json:"tenant_id"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	Name                  string     `json:"name"`
	MaxParticipants       int        `json:"max_participants"`
	RequiredStaff         *int       `json:"required_staff,omitempty"`
	IsOpen                bool       `json:"is_open"`
	CategoryID            int64      `json:"category_id"`
	PlanningTrackID       *int64     `json:"planning_track_id,omitempty"`
	PlannedRoomID         *int64     `json:"planned_room_id,omitempty"`
	CreatedBy             *int64     `json:"created_by"`
	Type                  string     `json:"type"`
	EducationGroupID      *int64     `json:"education_group_id,omitempty"`
	ListKind              *string    `json:"list_kind,omitempty"`
	IsTemplate            bool       `json:"is_template"`
	IsSystem              bool       `json:"is_system"`
	ArchivedAt            *time.Time `json:"archived_at,omitempty"`
	SeriesRootID          *int64     `json:"-"`
	CalendarPeriodID      *int64     `json:"calendar_period_id,omitempty"`
	TargetGroupType       string     `json:"target_group_type"`
	TargetGradeLevel      *int16     `json:"target_grade_level,omitempty"`
	TargetSchoolClass     *string    `json:"target_school_class,omitempty"`
	SourceCareOfferingIDs []int64    `json:"source_care_offering_ids,omitempty"`
	SourceGradeLevels     []int      `json:"source_grade_levels,omitempty"`
	SourceSchoolClasses   []string   `json:"source_school_classes,omitempty"`
	Notes                 *string    `json:"notes,omitempty"`
	Category              *Category  `json:"category,omitempty"`
}

// GroupFilter is the bounded compatibility filter used by activity listings.
type GroupFilter struct {
	Name              string
	CategoryID        *int64
	IsOpen            *bool
	IsSystem          *bool
	IsTemplate        *bool
	IDs               []int64
	SeriesForGroupID  *int64
	SourceOfferingIDs []int64
	HasOfferingSource bool
	ActiveOnly        bool
	OrderByName       bool
	OrderByID         bool
}

// GroupInput contains the writable scalar fields of an activities.groups row.
// Tenant identity and persistence metadata always come from the owner runtime.
type GroupInput struct {
	Name                  string
	MaxParticipants       int
	RequiredStaff         *int
	IsOpen                bool
	CategoryID            int64
	PlanningTrackID       *int64
	PlannedRoomID         *int64
	CreatedBy             *int64
	Type                  string
	EducationGroupID      *int64
	ListKind              *string
	IsTemplate            bool
	IsSystem              bool
	ArchivedAt            *time.Time
	SeriesRootID          *int64
	CalendarPeriodID      *int64
	TargetGroupType       string
	TargetGradeLevel      *int16
	TargetSchoolClass     *string
	SourceCareOfferingIDs []int64
	SourceGradeLevels     []int
	SourceSchoolClasses   []string
	Notes                 *string
}

type TemplateUpdate struct {
	Name                    string
	Type                    string
	CategoryID              int64
	PlanningTrackID         *int64
	PlanningTrackIDProvided bool
	RoomID                  int64
	EducationGroupID        *int64
	MaxParticipants         int
	MaxParticipantsProvided bool
	RequiredStaff           *int
	CalendarPeriodID        *int64
	TargetGroupType         string
	TargetGradeLevel        *int16
	TargetSchoolClass       *string
	ListKind                *string
	Notes                   *string
	SourceCareOfferingIDs   []int64
	SourceGradeLevels       []int
	SourceSchoolClasses     []string
}

type OfferingSourceInput struct {
	CareOfferingIDs []int64
	GradeLevels     []int
	SchoolClasses   []string
}

// GroupTarget is one dynamic cohort attached to a timetable template.
type GroupTarget struct {
	ID                 int64     `json:"id"`
	TenantID           int64     `json:"tenant_id"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	ActivityGroupID    int64     `json:"activity_group_id"`
	TargetGroupType    string    `json:"target_group_type"`
	TargetGradeLevel   *int16    `json:"target_grade_level,omitempty"`
	TargetSchoolClass  *string   `json:"target_school_class,omitempty"`
	EducationGroupID   *int64    `json:"education_group_id,omitempty"`
	EducationGroupName string    `json:"education_group_name,omitempty"`
}

type GroupTargetInput struct {
	TargetGroupType   string
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	EducationGroupID  *int64
}
