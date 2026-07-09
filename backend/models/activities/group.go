package activities

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Group type constants
const (
	GroupTypeActivity = "activity"
	GroupTypeCare     = "care"
	GroupTypeExternal = "external"
)

// Target-group ("Zielgruppe") type constants for Betreuungsplan templates.
// "gruppe" deliberately has no dedicated value column - it reuses
// EducationGroupID. "angebot" (Angebotsauswahl) needs no value column
// either: its roster comes from the existing CareOffering.ActivityGroupID
// bridge, not from a value stored on the Group.
const (
	TargetGroupTypeJahrgang = "jahrgang"
	TargetGroupTypeKlasse   = "klasse"
	TargetGroupTypeGruppe   = "gruppe"
	TargetGroupTypeAngebot  = "angebot"
	TargetGroupTypeNone     = "none"
)

// Group represents an activity group
type Group struct {
	base.Model `bun:"schema:activities,table:groups"`
	base.TenantModel
	Name             string     `bun:"name,notnull" json:"name"`
	MaxParticipants  int        `bun:"max_participants,notnull" json:"max_participants"`
	IsOpen           bool       `bun:"is_open,notnull,default:false" json:"is_open"`
	CategoryID       int64      `bun:"category_id,notnull" json:"category_id"`
	PlannedRoomID    *int64     `bun:"planned_room_id" json:"planned_room_id,omitempty"`
	CreatedBy        *int64     `bun:"created_by" json:"created_by"`
	Type             string     `bun:"type,notnull,default:'activity'" json:"type"`
	EducationGroupID *int64     `bun:"education_group_id" json:"education_group_id,omitempty"`
	IsTemplate       bool       `bun:"is_template,notnull,default:false" json:"is_template"`
	IsSystem         bool       `bun:"is_system,notnull,default:false" json:"is_system"`
	ArchivedAt       *time.Time `bun:"archived_at" json:"archived_at,omitempty"`

	// CalendarPeriodID pins this template to a calendar period (e.g. "1.
	// Halbjahr 2026/27"). The materialization service's selectPeriod()
	// prefers a schedule row's own pin over this one; this is the
	// template-level fallback and the value read by list responses that
	// only have the Group row.
	CalendarPeriodID *int64 `bun:"calendar_period_id" json:"calendar_period_id,omitempty"`

	// Zielgruppe (target-group) fields. TargetGroupType is one of the
	// TargetGroupType* constants; exactly one of the following holds the
	// type's value, enforced by Validate():
	//   jahrgang -> TargetGradeLevel
	//   klasse   -> TargetSchoolClass
	//   gruppe   -> EducationGroupID (existing field, no new column)
	//   angebot  -> none (roster derives from CareOffering.ActivityGroupID)
	//   none     -> none (today's default: manually curated roster)
	TargetGroupType   string  `bun:"target_group_type,notnull,default:'none'" json:"target_group_type"`
	TargetGradeLevel  *int16  `bun:"target_grade_level" json:"target_grade_level,omitempty"`
	TargetSchoolClass *string `bun:"target_school_class" json:"target_school_class,omitempty"`

	// Relations - populated when using the ORM's relations
	Category       *Category            `bun:"rel:belongs-to,join:category_id=id" json:"category,omitempty"`
	CreatedByStaff *users.Staff         `bun:"rel:belongs-to,join:created_by=id" json:"created_by_staff,omitempty"`
	Supervisors    []*SupervisorPlanned `bun:"rel:has-many,join:id=group_id" json:"supervisors,omitempty"`
	Schedules      []*Schedule          `bun:"rel:has-many,join:id=activity_group_id" json:"schedules,omitempty"`
}

// Validate ensures group data is valid
func (g *Group) Validate() error {
	if g.Name == "" {
		return errors.New("group name is required")
	}

	if g.MaxParticipants <= 0 {
		return errors.New("max participants must be greater than zero")
	}

	if g.CategoryID <= 0 {
		return errors.New("category ID is required")
	}

	if err := g.ValidateTargetGroup(); err != nil {
		return err
	}

	return nil
}

// ValidateTargetGroup enforces that TargetGroupType and its matching value
// field are consistent - the same type-conditional-invariant style used by
// ActivityException.Validate() (models/schedule/activity_exception.go).
// Exported so API handlers can validate the Zielgruppe fields from a request
// body up front (400) before constructing/persisting a full Group, without
// duplicating this switch.
//
// An empty TargetGroupType (the Go zero value for callers that construct a
// Group{} literal without ever mentioning Zielgruppe, which is most of the
// codebase) is treated the same as TargetGroupTypeNone: the DB column
// defaults to 'none', and callers predating this field must not be forced
// to set it explicitly.
func (g *Group) ValidateTargetGroup() error {
	if !IsValidTargetGroupType(g.TargetGroupType) {
		return errors.New("invalid target group type")
	}

	targetGroupType := g.TargetGroupType
	if targetGroupType == "" {
		targetGroupType = TargetGroupTypeNone
	}

	switch targetGroupType {
	case TargetGroupTypeJahrgang:
		if g.TargetGradeLevel == nil {
			return errors.New("jahrgang target group requires target_grade_level")
		}
		if g.TargetSchoolClass != nil {
			return errors.New("jahrgang target group must not set target_school_class")
		}
	case TargetGroupTypeKlasse:
		if g.TargetSchoolClass == nil || *g.TargetSchoolClass == "" {
			return errors.New("klasse target group requires target_school_class")
		}
		if g.TargetGradeLevel != nil {
			return errors.New("klasse target group must not set target_grade_level")
		}
	case TargetGroupTypeGruppe:
		if g.EducationGroupID == nil {
			return errors.New("gruppe target group requires education_group_id")
		}
		if g.TargetGradeLevel != nil || g.TargetSchoolClass != nil {
			return errors.New("gruppe target group must not set target_grade_level or target_school_class")
		}
	case TargetGroupTypeAngebot, TargetGroupTypeNone:
		if g.TargetGradeLevel != nil || g.TargetSchoolClass != nil {
			return errors.New("target_grade_level and target_school_class must be empty for this target group type")
		}
	}

	return nil
}

// IsValidTargetGroupType reports whether t is a permitted Zielgruppe type.
// The empty string is accepted as an alias for TargetGroupTypeNone (the Go
// zero value for callers predating this field).
func IsValidTargetGroupType(t string) bool {
	switch t {
	case "", TargetGroupTypeJahrgang, TargetGroupTypeKlasse, TargetGroupTypeGruppe, TargetGroupTypeAngebot, TargetGroupTypeNone:
		return true
	}
	return false
}

// IsOwnedBy checks if the group was created by the given staff member
func (g *Group) IsOwnedBy(staffID int64) bool {
	return g.CreatedBy != nil && *g.CreatedBy == staffID
}

// IsSupervisedBy checks if the given staff member is a supervisor of this group
func (g *Group) IsSupervisedBy(staffID int64) bool {
	for _, supervisor := range g.Supervisors {
		if supervisor != nil && supervisor.StaffID == staffID {
			return true
		}
	}
	return false
}

// HasAvailableSpots checks if the group has available spots based on current enrollment count
func (g *Group) HasAvailableSpots(currentEnrollmentCount int) bool {
	return g.MaxParticipants > currentEnrollmentCount
}
