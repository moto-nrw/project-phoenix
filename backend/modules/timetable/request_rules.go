package timetable

import (
	"errors"
	"strings"
)

// The HTTP adapter validates planner requests before it calls the owner
// (#3424 slice S1). These rules and values mirror the retained timetable
// models (models/activities, models/schedule) and School Structure's grade
// range, so the adapter no longer names those packages. The parity and
// vocabulary tests in modules/timetable/compose pin every value and every
// error message to its source.

const (
	// MaxTimetableReadRangeDays caps the inclusive day span of one planner
	// read (day, week, student week).
	MaxTimetableReadRangeDays = 14

	// MinSchoolGradeLevel and MaxSchoolGradeLevel are the grade range School
	// Structure supports; a tenant's grade-level maximum must lie within it.
	MinSchoolGradeLevel = 1
	MaxSchoolGradeLevel = 13

	// Kinds of a TemplateWeekdayRosterRow. An empty row marks a weekday that
	// deliberately has nobody assigned; a protected student is an
	// Enrollment-owned child the editor must not drop.
	TemplateWeekdayRosterKindEmpty            = "empty"
	TemplateWeekdayRosterKindStaff            = "staff"
	TemplateWeekdayRosterKindStudent          = "student"
	TemplateWeekdayRosterKindProtectedStudent = "protected_student"
)

const (
	minTargetGradeLevel = 1
	maxTargetGradeLevel = 13
)

// IsValidWeekday reports an ISO 8601 weekday (Monday = 1, Sunday = 7).
func IsValidWeekday(weekday int) bool {
	return weekday >= WeekdayMonday && weekday <= WeekdaySunday
}

// IsValidListKind reports a known list kind; empty means "no list kind".
func IsValidListKind(kind string) bool {
	switch kind {
	case "", ListKindEdgeHours, ListKindLearningTime, ListKindActivity, ListKindMensa:
		return true
	default:
		return false
	}
}

// IsValidTargetGroupType reports a permitted Zielgruppe type; empty is the
// alias of TargetGroupTypeNone.
func IsValidTargetGroupType(targetType string) bool {
	switch targetType {
	case "", TargetGroupTypeGrade, TargetGroupTypeSchoolClass, TargetGroupTypeEducationGroup, TargetGroupTypeOffering, TargetGroupTypeNone:
		return true
	}
	return false
}

// ParticipantLimitPtr converts a stored participant cap into its public
// form: nil for an unlimited group.
func ParticipantLimitPtr(limit int) *int {
	if limit <= 0 {
		return nil
	}
	return &limit
}

// NormalizeSourceSchoolClasses trims the class filter entries, rejects
// blanks and case-insensitive duplicates, and returns nil for an empty
// filter. The stored spelling stays the school's own.
func NormalizeSourceSchoolClasses(classes []string) ([]string, error) {
	if len(classes) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(classes))
	seen := make(map[string]bool, len(classes))
	for _, class := range classes {
		trimmed := strings.TrimSpace(class)
		if trimmed == "" {
			return nil, errors.New("source_school_classes entries must not be empty")
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			return nil, errors.New("source_school_classes must not contain duplicates")
		}
		seen[key] = true
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}

// ValidateDynamicTarget checks one entry of a template's multi-target list
// and trims its school class in place. Only grade, school class and
// education group targets are dynamic.
func (t *GroupTargetInput) ValidateDynamicTarget() error {
	switch t.TargetGroupType {
	case TargetGroupTypeGrade:
		return t.validateDynamicGrade()
	case TargetGroupTypeSchoolClass:
		return t.validateDynamicClass()
	case TargetGroupTypeEducationGroup:
		return t.validateDynamicEducationGroup()
	default:
		return errors.New("dynamic target type must be jahrgang, klasse, or gruppe")
	}
}

func (t *GroupTargetInput) validateDynamicGrade() error {
	if t.TargetGradeLevel == nil || *t.TargetGradeLevel < minTargetGradeLevel || *t.TargetGradeLevel > maxTargetGradeLevel {
		return errors.New("target_grade_level must be between 1 and 13")
	}
	if t.TargetSchoolClass != nil || t.EducationGroupID != nil {
		return errors.New("jahrgang target accepts only target_grade_level")
	}
	return nil
}

func (t *GroupTargetInput) validateDynamicClass() error {
	if t.TargetSchoolClass == nil || strings.TrimSpace(*t.TargetSchoolClass) == "" {
		return errors.New("target_school_class is required for klasse target")
	}
	trimmed := strings.TrimSpace(*t.TargetSchoolClass)
	t.TargetSchoolClass = &trimmed
	if t.TargetGradeLevel != nil || t.EducationGroupID != nil {
		return errors.New("klasse target accepts only target_school_class")
	}
	return nil
}

func (t *GroupTargetInput) validateDynamicEducationGroup() error {
	if t.EducationGroupID == nil || *t.EducationGroupID <= 0 {
		return errors.New("education_group_id is required for gruppe target")
	}
	if t.TargetGradeLevel != nil || t.TargetSchoolClass != nil {
		return errors.New("gruppe target accepts only education_group_id")
	}
	return nil
}

// TemplateTargetGroup is the single-target Zielgruppe of a template request
// with its optional offering source (#2137, #2482).
type TemplateTargetGroup struct {
	TargetGroupType       string
	TargetGradeLevel      *int16
	TargetSchoolClass     *string
	EducationGroupID      *int64
	SourceCareOfferingIDs []int64
	SourceGradeLevels     []int
	SourceSchoolClasses   []string
}

// Validate enforces that the target type and its value fields agree and
// canonicalizes the value in place: an empty type becomes
// TargetGroupTypeNone, an empty source becomes nil, the school class and
// the class filter are trimmed. A failed check may leave the value
// partially canonicalized.
func (g *TemplateTargetGroup) Validate() error {
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
	case TargetGroupTypeGrade:
		return g.validateGradeTarget()
	case TargetGroupTypeSchoolClass:
		return g.validateClassTarget()
	case TargetGroupTypeEducationGroup:
		return g.validateEducationGroupTarget()
	case TargetGroupTypeOffering, TargetGroupTypeNone:
		return g.validateValuelessTarget()
	}
	return nil
}

func (g *TemplateTargetGroup) validateOfferingSource() error {
	if len(g.SourceCareOfferingIDs) == 0 {
		return g.clearOfferingSource()
	}
	if g.TargetGroupType != TargetGroupTypeOffering {
		return errors.New("source_care_offering_ids requires target group type 'angebot'")
	}
	if err := validateSourceCareOfferingIDs(g.SourceCareOfferingIDs); err != nil {
		return err
	}
	if err := validateSourceGradeLevels(g.SourceGradeLevels); err != nil {
		return err
	}
	if len(g.SourceGradeLevels) == 0 {
		g.SourceGradeLevels = nil
	}
	classes, err := NormalizeSourceSchoolClasses(g.SourceSchoolClasses)
	if err != nil {
		return err
	}
	g.SourceSchoolClasses = classes
	if len(g.SourceGradeLevels) > 0 && len(g.SourceSchoolClasses) > 0 {
		return errors.New("source_school_classes and source_grade_levels cannot be combined")
	}
	return nil
}

// clearOfferingSource canonicalizes "no source" to nil and refuses a filter
// without a source.
func (g *TemplateTargetGroup) clearOfferingSource() error {
	g.SourceCareOfferingIDs = nil
	if len(g.SourceGradeLevels) > 0 {
		return errors.New("source_grade_levels requires source_care_offering_ids")
	}
	if len(g.SourceSchoolClasses) > 0 {
		return errors.New("source_school_classes requires source_care_offering_ids")
	}
	g.SourceGradeLevels = nil
	g.SourceSchoolClasses = nil
	return nil
}

func validateSourceCareOfferingIDs(offeringIDs []int64) error {
	seen := make(map[int64]bool, len(offeringIDs))
	for _, offeringID := range offeringIDs {
		if offeringID <= 0 {
			return errors.New("source_care_offering_ids entries must be positive")
		}
		if seen[offeringID] {
			return errors.New("source_care_offering_ids must not contain duplicates")
		}
		seen[offeringID] = true
	}
	return nil
}

func validateSourceGradeLevels(levels []int) error {
	seen := make(map[int]bool, len(levels))
	for _, level := range levels {
		if level < minTargetGradeLevel || level > maxTargetGradeLevel {
			return errors.New("source_grade_levels entries must be between 1 and 13")
		}
		if seen[level] {
			return errors.New("source_grade_levels must not contain duplicates")
		}
		seen[level] = true
	}
	return nil
}

func (g *TemplateTargetGroup) validateGradeTarget() error {
	if g.TargetGradeLevel == nil {
		return errors.New("jahrgang target group requires target_grade_level")
	}
	if *g.TargetGradeLevel < minTargetGradeLevel || *g.TargetGradeLevel > maxTargetGradeLevel {
		return errors.New("target_grade_level must be between 1 and 13")
	}
	if g.TargetSchoolClass != nil {
		return errors.New("jahrgang target group must not set target_school_class")
	}
	return nil
}

func (g *TemplateTargetGroup) validateClassTarget() error {
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

func (g *TemplateTargetGroup) validateEducationGroupTarget() error {
	if g.EducationGroupID == nil {
		return errors.New("gruppe target group requires education_group_id")
	}
	if g.TargetGradeLevel != nil || g.TargetSchoolClass != nil {
		return errors.New("gruppe target group must not set target_grade_level or target_school_class")
	}
	return nil
}

func (g *TemplateTargetGroup) validateValuelessTarget() error {
	if g.TargetGradeLevel != nil || g.TargetSchoolClass != nil {
		return errors.New("target_grade_level and target_school_class must be empty for this target group type")
	}
	return nil
}
