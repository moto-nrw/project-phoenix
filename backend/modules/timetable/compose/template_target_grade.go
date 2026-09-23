package compose

import (
	"fmt"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// validateTemplateTargetsGradeLimit permits every unchanged legacy grade,
// including secondary targets that are not represented by the group mirror.
func validateTemplateTargetsGradeLimit(
	rules SchoolClassRules,
	gradeLevelMax int,
	existing *activitiesModel.Group,
	existingTargets []*activitiesModel.GroupTarget,
	requestedTargets []*activitiesModel.GroupTarget,
) error {
	if err := validateTemplateGradeLevelMax(rules, gradeLevelMax); err != nil {
		return err
	}
	existingGrades := existingJahrgangGrades(existing, existingTargets)
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
			timetable.ErrTemplateTargetGradeExceedsLimit,
			*target.TargetGradeLevel,
			gradeLevelMax,
		)
	}
	return nil
}

func existingJahrgangGrades(existing *activitiesModel.Group, existingTargets []*activitiesModel.GroupTarget) map[int16]struct{} {
	grades := make(map[int16]struct{}, len(existingTargets)+1)
	if existing != nil && existing.TargetGroupType == activitiesModel.TargetGroupTypeJahrgang && existing.TargetGradeLevel != nil {
		grades[*existing.TargetGradeLevel] = struct{}{}
	}
	for _, target := range existingTargets {
		if target != nil && target.TargetGroupType == activitiesModel.TargetGroupTypeJahrgang && target.TargetGradeLevel != nil {
			grades[*target.TargetGradeLevel] = struct{}{}
		}
	}
	return grades
}

func validateTemplateGradeLevelMax(rules SchoolClassRules, value int) error {
	if value >= rules.MinGradeLevel && value <= rules.MaxGradeLevel {
		return nil
	}
	return fmt.Errorf(
		"grade level max must be between %d and %d",
		rules.MinGradeLevel,
		rules.MaxGradeLevel,
	)
}
