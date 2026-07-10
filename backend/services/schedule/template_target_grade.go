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
	if err := validateTemplateGradeLevelMax(gradeLevelMax); err != nil {
		return err
	}
	if targetGroupType != activitiesModel.TargetGroupTypeJahrgang || targetGradeLevel == nil ||
		int(*targetGradeLevel) <= gradeLevelMax {
		return nil
	}
	if existing != nil && existing.TargetGroupType == activitiesModel.TargetGroupTypeJahrgang &&
		existing.TargetGradeLevel != nil && *existing.TargetGradeLevel == *targetGradeLevel {
		return nil
	}
	return fmt.Errorf(
		"%w: target_grade_level %d exceeds tenant maximum %d",
		ErrTemplateTargetGradeExceedsLimit,
		*targetGradeLevel,
		gradeLevelMax,
	)
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
