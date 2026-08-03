package activities

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
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
// EducationGroupID. "angebot" (Angebotsauswahl) has two sourcing modes:
// with SourceCareOfferingID set the template declares the offering (plus an
// optional SourceGradeLevels filter) as its dynamic roster source (#2137);
// without it the roster comes from the legacy CareOffering.ActivityGroupID
// bridge (#1651), where the offering points at the template instead.
const (
	TargetGroupTypeJahrgang = "jahrgang"
	TargetGroupTypeKlasse   = "klasse"
	TargetGroupTypeGruppe   = "gruppe"
	TargetGroupTypeAngebot  = "angebot"
	TargetGroupTypeNone     = "none"
)

// List kind constants classify timetable templates for printable daily lists.
// Empty means the template is not tied to a dedicated list kind.
const (
	ListKindEdgeHours    = "edge_hours"
	ListKindLearningTime = "learning_time"
	ListKindActivity     = "activity"
	ListKindMensa        = "mensa"
)

func IsValidListKind(kind string) bool {
	switch kind {
	case "", ListKindEdgeHours, ListKindLearningTime, ListKindActivity, ListKindMensa:
		return true
	default:
		return false
	}
}

func ListKindLabel(kind string) string {
	switch kind {
	case ListKindEdgeHours:
		return "Randstunden"
	case ListKindLearningTime:
		return "Lernzeit"
	case ListKindActivity:
		return "AG-Angebote"
	case ListKindMensa:
		return "Mensa"
	default:
		return ""
	}
}

// Group represents an activity group
type Group struct {
	base.Model `bun:"schema:activities,table:groups"`
	base.TenantModel
	Name            string `bun:"name,notnull" json:"name"`
	MaxParticipants int    `bun:"max_participants,notnull" json:"max_participants"`
	// RequiredStaff is the manual Personalbedarf override for the template
	// (issue #1839). NULL means "derive from the Betreuungsschlüssel" (#1869);
	// a set value (>= 0) is inherited by materialized instances at read time
	// (their own column stays NULL unless individually pinned), so later
	// template edits keep propagating. Mirrors MaxParticipants plumbing.
	RequiredStaff    *int       `bun:"required_staff" json:"required_staff,omitempty"`
	IsOpen           bool       `bun:"is_open,notnull,default:false" json:"is_open"`
	CategoryID       int64      `bun:"category_id,notnull" json:"category_id"`
	PlannedRoomID    *int64     `bun:"planned_room_id" json:"planned_room_id,omitempty"`
	CreatedBy        *int64     `bun:"created_by" json:"created_by"`
	Type             string     `bun:"type,notnull,default:'activity'" json:"type"`
	EducationGroupID *int64     `bun:"education_group_id" json:"education_group_id,omitempty"`
	ListKind         *string    `bun:"list_kind" json:"list_kind,omitempty"`
	IsTemplate       bool       `bun:"is_template,notnull,default:false" json:"is_template"`
	IsSystem         bool       `bun:"is_system,notnull,default:false" json:"is_system"`
	ArchivedAt       *time.Time `bun:"archived_at" json:"archived_at,omitempty"`
	// SeriesRootID identifies all segments produced by recurring-template
	// splits. The original segment keeps NULL (it is its own root); every
	// successor points to that original row, including successors split again.
	SeriesRootID *int64 `bun:"series_root_id" json:"-"`

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
	//   angebot  -> SourceCareOfferingID (+ optional SourceGradeLevels), or
	//               none for the legacy CareOffering.ActivityGroupID bridge
	//   none     -> none (today's default: manually curated roster)
	TargetGroupType   string  `bun:"target_group_type,notnull,default:'none'" json:"target_group_type"`
	TargetGradeLevel  *int16  `bun:"target_grade_level" json:"target_grade_level,omitempty"`
	TargetSchoolClass *string `bun:"target_school_class" json:"target_school_class,omitempty"`

	// Offering-source fields (#2137): an "angebot" template may declare one
	// care offering as its dynamic roster source plus an optional grade
	// filter, so one Betreuungsangebot can feed several parallel Regeltermine
	// (one per Jahrgang). Empty/nil SourceGradeLevels means "all enrolled
	// children of the offering". An "angebot" template WITHOUT a source keeps
	// the legacy behavior (roster fed via CareOffering.ActivityGroupID).
	SourceCareOfferingID *int64 `bun:"source_care_offering_id" json:"source_care_offering_id,omitempty"`
	SourceGradeLevels    []int  `bun:"source_grade_levels,type:jsonb,nullzero" json:"source_grade_levels,omitempty"`

	// Notes is the durable Wochennotiz for a recurring template (issue #1837
	// follow-up). NULL means no series note. It is joined onto every
	// materialized instance at read time (the instance table has no such
	// column), so it survives ReplanWeek and series splits and keeps
	// propagating on later edits. The per-occurrence Tagesnotiz stays on
	// schedule.activity_instances.notes.
	Notes *string `bun:"notes" json:"notes,omitempty"`

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
	// A non-nil pointer to "" means "no list kind" for callers that build a
	// Group{} literal; canonicalize it to NULL so it satisfies the DB's
	// `list_kind IS NULL OR list_kind IN (...)` CHECK instead of hitting a
	// constraint error (IsValidListKind("") stays true for the slot-list
	// filter, where empty means "any kind").
	if g.ListKind != nil && *g.ListKind == "" {
		g.ListKind = nil
	}
	if g.ListKind != nil && !IsValidListKind(*g.ListKind) {
		return errors.New("invalid list kind")
	}

	if g.RequiredStaff != nil && *g.RequiredStaff < 0 {
		return errors.New("required staff cannot be negative")
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
// codebase) is canonicalized to TargetGroupTypeNone. This keeps generic
// repository creates and updates consistent with the DB's non-empty CHECK
// constraint without forcing callers predating this field to set it.
func (g *Group) ValidateTargetGroup() error {
	if !IsValidTargetGroupType(g.TargetGroupType) {
		return errors.New("invalid target group type")
	}

	if g.TargetGroupType == "" {
		g.TargetGroupType = TargetGroupTypeNone
	}

	if err := g.validateOfferingSource(); err != nil {
		return err
	}

	switch g.TargetGroupType {
	case TargetGroupTypeJahrgang:
		return g.validateGradeTarget()
	case TargetGroupTypeKlasse:
		return g.validateClassTarget()
	case TargetGroupTypeGruppe:
		return g.validateEducationGroupTarget()
	case TargetGroupTypeAngebot, TargetGroupTypeNone:
		return g.validateValuelessTarget()
	}

	return nil
}

// validateOfferingSource mirrors the DB CHECK
// chk_activities_groups_offering_source plus grade-range sanity: a source
// offering only on 'angebot' templates, a grade filter only with a source,
// filter values within the supported grade bounds and free of duplicates.
func (g *Group) validateOfferingSource() error {
	if g.SourceCareOfferingID == nil {
		if len(g.SourceGradeLevels) > 0 {
			return errors.New("source_grade_levels requires source_care_offering_id")
		}
		g.SourceGradeLevels = nil
		return nil
	}
	if *g.SourceCareOfferingID <= 0 {
		return errors.New("source_care_offering_id must be positive when set")
	}
	if g.TargetGroupType != TargetGroupTypeAngebot {
		return errors.New("source_care_offering_id requires target group type 'angebot'")
	}
	seen := make(map[int]bool, len(g.SourceGradeLevels))
	for _, level := range g.SourceGradeLevels {
		if level < schoolclass.MinGradeLevel || level > schoolclass.MaxGradeLevel {
			return errors.New("source_grade_levels entries must be between 1 and 13")
		}
		if seen[level] {
			return errors.New("source_grade_levels must not contain duplicates")
		}
		seen[level] = true
	}
	if len(g.SourceGradeLevels) == 0 {
		g.SourceGradeLevels = nil
	}
	return nil
}

// MatchesSourceGradeFilter reports whether a child with the given grade level
// (nil = grade not derivable from the school class) passes this template's
// offering grade filter. An empty filter admits every child; a set filter
// never admits a child without grade data — silently planning a child whose
// grade is unknown into a Jahrgang-scoped Termin would hide data problems.
func (g *Group) MatchesSourceGradeFilter(gradeLevel *int16) bool {
	if len(g.SourceGradeLevels) == 0 {
		return true
	}
	if gradeLevel == nil {
		return false
	}
	for _, level := range g.SourceGradeLevels {
		if level == int(*gradeLevel) {
			return true
		}
	}
	return false
}

func (g *Group) validateGradeTarget() error {
	if g.TargetGradeLevel == nil {
		return errors.New("jahrgang target group requires target_grade_level")
	}
	if *g.TargetGradeLevel < schoolclass.MinGradeLevel || *g.TargetGradeLevel > schoolclass.MaxGradeLevel {
		return errors.New("target_grade_level must be between 1 and 13")
	}
	if g.TargetSchoolClass != nil {
		return errors.New("jahrgang target group must not set target_school_class")
	}
	return nil
}

func (g *Group) validateClassTarget() error {
	if g.TargetSchoolClass == nil || strings.TrimSpace(*g.TargetSchoolClass) == "" {
		return errors.New("klasse target group requires target_school_class")
	}
	trimmedClass := strings.TrimSpace(*g.TargetSchoolClass)
	g.TargetSchoolClass = &trimmedClass
	if g.TargetGradeLevel != nil {
		return errors.New("klasse target group must not set target_grade_level")
	}
	return nil
}

func (g *Group) validateEducationGroupTarget() error {
	if g.EducationGroupID == nil {
		return errors.New("gruppe target group requires education_group_id")
	}
	if g.TargetGradeLevel != nil || g.TargetSchoolClass != nil {
		return errors.New("gruppe target group must not set target_grade_level or target_school_class")
	}
	return nil
}

func (g *Group) validateValuelessTarget() error {
	if g.TargetGradeLevel != nil || g.TargetSchoolClass != nil {
		return errors.New("target_grade_level and target_school_class must be empty for this target group type")
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
