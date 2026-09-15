package active

import "time"

// SessionActivity is timetable-owned activity data projected onto a presence session.
// It has no persistence or activity mutation behavior.
type SessionActivity struct {
	ID                    int64                    `json:"id"`
	TenantID              int64                    `json:"tenant_id"`
	CreatedAt             time.Time                `json:"created_at"`
	UpdatedAt             time.Time                `json:"updated_at"`
	Name                  string                   `json:"name"`
	MaxParticipants       int                      `json:"max_participants"`
	RequiredStaff         *int                     `json:"required_staff,omitempty"`
	IsOpen                bool                     `json:"is_open"`
	CategoryID            int64                    `json:"category_id"`
	PlanningTrackID       *int64                   `json:"planning_track_id,omitempty"`
	PlannedRoomID         *int64                   `json:"planned_room_id,omitempty"`
	CreatedBy             *int64                   `json:"created_by"`
	Type                  string                   `json:"type"`
	EducationGroupID      *int64                   `json:"education_group_id,omitempty"`
	ListKind              *string                  `json:"list_kind,omitempty"`
	IsTemplate            bool                     `json:"is_template"`
	IsSystem              bool                     `json:"is_system"`
	ArchivedAt            *time.Time               `json:"archived_at,omitempty"`
	SeriesRootID          *int64                   `json:"-"`
	CalendarPeriodID      *int64                   `json:"calendar_period_id,omitempty"`
	TargetGroupType       string                   `json:"target_group_type"`
	TargetGradeLevel      *int16                   `json:"target_grade_level,omitempty"`
	TargetSchoolClass     *string                  `json:"target_school_class,omitempty"`
	SourceCareOfferingIDs []int64                  `json:"source_care_offering_ids,omitempty"`
	SourceGradeLevels     []int                    `json:"source_grade_levels,omitempty"`
	SourceSchoolClasses   []string                 `json:"source_school_classes,omitempty"`
	Notes                 *string                  `json:"notes,omitempty"`
	Category              *SessionActivityCategory `json:"category,omitempty"`
}

type SessionActivityCategory struct {
	ID          int64      `json:"id"`
	TenantID    int64      `json:"tenant_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Color       string     `json:"color,omitempty"`
	IsSystem    bool       `json:"is_system"`
	ShiftTypeID *int64     `json:"shift_type_id,omitempty"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
}

func (g *SessionActivity) ParticipantLimit() *int {
	if g.MaxParticipants <= 0 {
		return nil
	}
	limit := g.MaxParticipants
	return &limit
}
