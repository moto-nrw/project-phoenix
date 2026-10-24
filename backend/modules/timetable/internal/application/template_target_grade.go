package application

import (
	"fmt"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// validateTemplateTargetsGradeLimit permits every unchanged legacy grade,
// including secondary targets that are not represented by the group mirror.
func validateTemplateTargetsGradeLimit(
	rules ports.SchoolClassRules,
	gradeLevelMax int,
	existing *activityContract.Group,
	existingTargets []*activityContract.GroupTarget,
	requestedTargets []*activityContract.GroupTarget,
) error {
	if err := validateTemplateGradeLevelMax(rules, gradeLevelMax); err != nil {
		return err
	}
	existingGrades := existingJahrgangGrades(existing, existingTargets)
	for _, target := range requestedTargets {
		if target == nil || target.TargetGroupType != activityContract.TargetGroupTypeJahrgang || target.TargetGradeLevel == nil ||
			int(*target.TargetGradeLevel) <= gradeLevelMax {
			continue
		}
		if _, unchanged := existingGrades[*target.TargetGradeLevel]; unchanged {
			continue
		}
		return timetable.WithCode(fmt.Errorf(
			"%w: target_grade_level %d exceeds tenant maximum %d",
			timetable.ErrTemplateTargetGradeExceedsLimit,
			*target.TargetGradeLevel,
			gradeLevelMax,
		), timetable.CodeTemplateGradeAboveMax, timetable.RefusalValues{Grade: int(*target.TargetGradeLevel), Max: gradeLevelMax})
	}
	return nil
}

func existingJahrgangGrades(existing *activityContract.Group, existingTargets []*activityContract.GroupTarget) map[int16]struct{} {
	grades := make(map[int16]struct{}, len(existingTargets)+1)
	if existing != nil && existing.TargetGroupType == activityContract.TargetGroupTypeJahrgang && existing.TargetGradeLevel != nil {
		grades[*existing.TargetGradeLevel] = struct{}{}
	}
	for _, target := range existingTargets {
		if target != nil && target.TargetGroupType == activityContract.TargetGroupTypeJahrgang && target.TargetGradeLevel != nil {
			grades[*target.TargetGradeLevel] = struct{}{}
		}
	}
	return grades
}

func validateTemplateGradeLevelMax(rules ports.SchoolClassRules, value int) error {
	if value >= rules.MinGradeLevel && value <= rules.MaxGradeLevel {
		return nil
	}
	return fmt.Errorf(
		"grade level max must be between %d and %d",
		rules.MinGradeLevel,
		rules.MaxGradeLevel,
	)
}
