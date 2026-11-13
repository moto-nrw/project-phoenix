package application

import (
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain"
)

// ListTemplateTargetInputs resolves the display rules and their members in
// one readset. It retains nothing beyond this call.
func (s *Service) ListTemplateTargetInputs(ctx context.Context, ids []int64) (result timetable.TemplateTargetInputs, err error) {
	if hasInvalidID(ids) {
		return result, s.reject("list_target_student_ids", timetable.ErrInvalidGroupQuery)
	}
	err = s.run(ctx, "list_target_student_ids", func(ctx context.Context, stats *domain.OperationStats) error {
		targets, students, listErr := s.loadTargetStudentCandidates(ctx, ids, stats)
		if listErr != nil {
			return listErr
		}
		result = timetable.TemplateTargetInputs{Targets: targets, StudentIDs: matchTargetStudents(targets, students, s.today())}
		return nil
	})
	return result, err
}

var leadingGrade = regexp.MustCompile(`[0-9]+`)

func matchTargetStudents(targets map[int64][]timetable.GroupTarget, students []domain.TargetStudent, today string) map[int64][]int64 {
	result := make(map[int64][]int64, len(targets))
	for groupID, rules := range targets {
		for _, student := range students {
			if careEndedBefore(student.EnrolledUntil, today) || !matchesAnyTarget(rules, student) {
				continue
			}
			result[groupID] = append(result[groupID], student.ID)
		}
		slices.Sort(result[groupID])
		result[groupID] = slices.Compact(result[groupID])
	}
	return result
}

func matchesAnyTarget(rules []timetable.GroupTarget, student domain.TargetStudent) bool {
	for _, rule := range rules {
		switch rule.TargetGroupType {
		case "jahrgang":
			if rule.TargetGradeLevel != nil && leadingGrade.FindString(student.SchoolClass) == strconv.Itoa(int(*rule.TargetGradeLevel)) {
				return true
			}
		case "klasse":
			if rule.TargetSchoolClass != nil && strings.EqualFold(strings.TrimSpace(student.SchoolClass), strings.TrimSpace(*rule.TargetSchoolClass)) {
				return true
			}
		case "gruppe":
			if rule.EducationGroupID != nil && student.EducationGroupID != nil && *rule.EducationGroupID == *student.EducationGroupID {
				return true
			}
		}
	}
	return false
}

func careEndedBefore(value, today string) bool {
	if value == "" {
		return false
	}
	end, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return false
	}
	todayDate, err := time.Parse(time.DateOnly, today)
	return err == nil && end.Before(todayDate)
}
