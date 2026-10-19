package activitycontract

import (
	"errors"
	"strings"
	"time"
)

// GroupTarget stores one dynamic cohort referenced by a timetable template.
// All targets of one template use the same TargetGroupType.
type GroupTarget struct {
	ID                 int64     `json:"id"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	TenantID           int64     `json:"tenant_id"`
	ActivityGroupID    int64     `json:"activity_group_id"`
	TargetGroupType    string    `json:"target_group_type"`
	TargetGradeLevel   *int16    `json:"target_grade_level,omitempty"`
	TargetSchoolClass  *string   `json:"target_school_class,omitempty"`
	EducationGroupID   *int64    `json:"education_group_id,omitempty"`
	EducationGroupName string    `json:"education_group_name,omitempty"`
}

func (t *GroupTarget) Validate() error {
	switch t.TargetGroupType {
	case TargetGroupTypeJahrgang:
		if t.TargetGradeLevel == nil || *t.TargetGradeLevel < minTargetGradeLevel || *t.TargetGradeLevel > maxTargetGradeLevel {
			return errors.New("target_grade_level must be between 1 and 13")
		}
		if t.TargetSchoolClass != nil || t.EducationGroupID != nil {
			return errors.New("jahrgang target accepts only target_grade_level")
		}
	case TargetGroupTypeKlasse:
		return t.validateClassTarget()
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

func (v *GroupTarget) GetTenantID() int64   { return v.TenantID }
func (v *GroupTarget) SetTenantID(id int64) { v.TenantID = id }

func (t *GroupTarget) validateClassTarget() error {
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
