package activities

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// GroupTarget stores one dynamic cohort referenced by a timetable template.
// All targets of one template use the same TargetGroupType.
type GroupTarget struct {
	base.Model `bun:"schema:activities,table:group_targets"`
	base.TenantModel
	ActivityGroupID    int64   `bun:"activity_group_id,notnull" json:"activity_group_id"`
	TargetGroupType    string  `bun:"target_group_type,notnull" json:"target_group_type"`
	TargetGradeLevel   *int16  `bun:"target_grade_level" json:"target_grade_level,omitempty"`
	TargetSchoolClass  *string `bun:"target_school_class" json:"target_school_class,omitempty"`
	EducationGroupID   *int64  `bun:"education_group_id" json:"education_group_id,omitempty"`
	EducationGroupName string  `bun:"education_group_name,scanonly" json:"education_group_name,omitempty"`
}

func (t *GroupTarget) Validate() error {
	switch t.TargetGroupType {
	case TargetGroupTypeJahrgang:
		if t.TargetGradeLevel == nil || *t.TargetGradeLevel < schoolclass.MinGradeLevel || *t.TargetGradeLevel > schoolclass.MaxGradeLevel {
			return errors.New("target_grade_level must be between 1 and 13")
		}
		if t.TargetSchoolClass != nil || t.EducationGroupID != nil {
			return errors.New("jahrgang target accepts only target_grade_level")
		}
	case TargetGroupTypeKlasse:
		if t.TargetSchoolClass == nil || strings.TrimSpace(*t.TargetSchoolClass) == "" {
			return errors.New("target_school_class is required for klasse target")
		}
		trimmed := strings.TrimSpace(*t.TargetSchoolClass)
		t.TargetSchoolClass = &trimmed
		if t.TargetGradeLevel != nil || t.EducationGroupID != nil {
			return errors.New("klasse target accepts only target_school_class")
		}
	case TargetGroupTypeGruppe:
		if t.EducationGroupID == nil || *t.EducationGroupID <= 0 {
			return errors.New("education_group_id is required for gruppe target")
		}
		if t.TargetGradeLevel != nil || t.TargetSchoolClass != nil {
			return errors.New("gruppe target accepts only education_group_id")
		}
	default:
		return errors.New("dynamic target type must be jahrgang, klasse, or gruppe")
	}
	return nil
}
