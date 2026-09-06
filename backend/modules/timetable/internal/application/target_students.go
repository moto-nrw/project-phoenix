package application

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

var leadingGrade = regexp.MustCompile(`[0-9]+`)

func matchTargetStudents(targets map[int64][]domain.GroupTarget, students []domain.TargetStudent, today string) map[int64][]int64 {
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

func matchesAnyTarget(rules []domain.GroupTarget, student domain.TargetStudent) bool {
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
