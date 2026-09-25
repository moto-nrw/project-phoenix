package enrollment

import (
	"strconv"
	"strings"
)

// Supported school grades, the bounds School Structure applies to target
// groups and the enrollment settings.
const (
	minSchoolGrade = 1
	maxSchoolGrade = 13
)

// SchoolClassGradeLevel derives the numeric Jahrgang from a free-text school
// class ("3a" -> 3), the grade Care Plan's Jahrgang filters of offering-sourced
// Regeltermine match on (#2137). Classes without a supported grade number
// ("Bienen") yield nil, so a set grade filter never matches them.
func SchoolClassGradeLevel(schoolClass string) *int16 {
	prefix := classGrade(schoolClass)
	if prefix == "" {
		return nil
	}
	parsed, err := strconv.Atoi(prefix)
	if err != nil || parsed < minSchoolGrade || parsed > maxSchoolGrade {
		return nil
	}
	grade := int16(parsed)
	return &grade
}

// CollectsGrade1Class reports whether a grade-1 child's concrete class is
// collected for this phase. Grade 1 is opt-in — the #1833 default keeps it
// grade-level-only — and there are exactly two triggers (#1663):
//
//   - No class restriction: the phase offers a concrete grade-1 class ("1a").
//     Prefixless classes ("Bienen") deliberately do not trigger it here.
//   - Class restriction active: the eligible list decides, because it is the
//     only world the submit gate accepts. A grade-1 child can satisfy it when
//     an eligible class is grade-1-prefixed or prefixless, so the class must
//     be collected.
//
// The form loads present the same decision the submission makes; a field the
// form hides and the validator then demands would be a dead end.
func CollectsGrade1Class(phase *Phase) bool {
	if phase == nil {
		return false
	}
	if RestrictsEligibleClasses(phase.EligibleSchoolClasses) {
		return classListSelectableByGrade(phase.EligibleSchoolClasses, 1)
	}
	return classListHasGradePrefixedClass(phase.AvailableSchoolClasses, 1)
}

// classListHasGradePrefixedClass reports whether the list holds a class whose
// numeric prefix equals the grade. Prefixless classes are ignored on purpose.
func classListHasGradePrefixedClass(classes []string, grade int) bool {
	want := strconv.Itoa(grade)
	for _, class := range classes {
		if classGrade(strings.TrimSpace(class)) == want {
			return true
		}
	}
	return false
}

// classListSelectableByGrade reports whether a child in the given grade can
// pick at least one class of the list: a matching numeric prefix, or a
// prefixless class, which belongs to every grade.
func classListSelectableByGrade(classes []string, grade int) bool {
	want := strconv.Itoa(grade)
	for _, class := range classes {
		trimmed := strings.TrimSpace(class)
		if trimmed == "" {
			continue
		}
		if prefix := classGrade(trimmed); prefix == "" || prefix == want {
			return true
		}
	}
	return false
}
