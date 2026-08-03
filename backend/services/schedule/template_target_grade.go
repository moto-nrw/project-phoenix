package schedule

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
)

// ErrTemplateTargetGradeExceedsLimit identifies a Jahrgang target that is
// above the tenant's enrollment.grade_level_max setting. API handlers expose
// it as a client-correctable 400 response.
var ErrTemplateTargetGradeExceedsLimit = errors.New("template target grade exceeds tenant limit")

// ValidateTemplateTargetGradeLimit applies the tenant cap to a requested
// target. Existing above-cap Jahrgang targets remain editable when their grade
// is unchanged, so lowering the setting does not strand legacy series.
func ValidateTemplateTargetGradeLimit(
	gradeLevelMax int,
	existing *activitiesModel.Group,
	targetGroupType string,
	targetGradeLevel *int16,
) error {
	target := &activitiesModel.GroupTarget{
		TargetGroupType:  targetGroupType,
		TargetGradeLevel: targetGradeLevel,
	}
	return ValidateTemplateTargetsGradeLimit(gradeLevelMax, existing, nil, []*activitiesModel.GroupTarget{target})
}

// ValidateTemplateTargetsGradeLimit permits every unchanged legacy grade,
// including secondary targets that are not represented by the group mirror.
func ValidateTemplateTargetsGradeLimit(
	gradeLevelMax int,
	existing *activitiesModel.Group,
	existingTargets []*activitiesModel.GroupTarget,
	requestedTargets []*activitiesModel.GroupTarget,
) error {
	if err := validateTemplateGradeLevelMax(gradeLevelMax); err != nil {
		return err
	}
	existingGrades := make(map[int16]struct{}, len(existingTargets)+1)
	if existing != nil && existing.TargetGroupType == activitiesModel.TargetGroupTypeJahrgang && existing.TargetGradeLevel != nil {
		existingGrades[*existing.TargetGradeLevel] = struct{}{}
	}
	for _, target := range existingTargets {
		if target != nil && target.TargetGroupType == activitiesModel.TargetGroupTypeJahrgang && target.TargetGradeLevel != nil {
			existingGrades[*target.TargetGradeLevel] = struct{}{}
		}
	}
	for _, target := range requestedTargets {
		if target == nil || target.TargetGroupType != activitiesModel.TargetGroupTypeJahrgang || target.TargetGradeLevel == nil ||
			int(*target.TargetGradeLevel) <= gradeLevelMax {
			continue
		}
		if _, unchanged := existingGrades[*target.TargetGradeLevel]; unchanged {
			continue
		}
		return fmt.Errorf(
			"%w: target_grade_level %d exceeds tenant maximum %d",
			ErrTemplateTargetGradeExceedsLimit,
			*target.TargetGradeLevel,
			gradeLevelMax,
		)
	}
	return nil
}

func validateTemplateGradeLevelMax(value int) error {
	if value >= schoolclass.MinGradeLevel && value <= schoolclass.MaxGradeLevel {
		return nil
	}
	return fmt.Errorf(
		"grade level max must be between %d and %d",
		schoolclass.MinGradeLevel,
		schoolclass.MaxGradeLevel,
	)
}
