package application

import (
	"strconv"
	"strings"
)

// gradeToClass renders the optional grade level into the student's
// school_class field. The student schema uses free-form text; we land
// "1", "2", … when the grade is provided and the open placeholder otherwise.
// Admins can rename via the student profile UI later.
func gradeToClass(grade *int16) string {
	if grade == nil || *grade == 0 {
		// users.students.school_class is required even when the enrollment
		// tenant deliberately does not collect a grade. Persist a neutral,
		// non-grade placeholder while request_children keeps the grade NULL.
		return openSchoolClassPlaceholder
	}
	return strconv.Itoa(int(*grade))
}

// isBareGradePlaceholderClass reports whether a student's school_class is an
// un-customized grade placeholder — empty or all digits ("", "1", "2"), as
// produced by gradeToClass — rather than a concrete class an admin assigned
// ("2a"). Bare placeholders may be safely re-derived from a changed grade;
// concrete classes must be preserved. Issue #1833.
func isBareGradePlaceholderClass(class string) bool {
	class = strings.TrimSpace(class)
	if strings.EqualFold(class, openSchoolClassPlaceholder) {
		return true
	}
	for _, r := range class {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// concreteSchoolClass returns the trimmed concrete class the parent
// chose at enrollment (e.g. "2a"), or "" when none was collected
// ("Klasse offen"). Issue #1833.
func concreteSchoolClass(child *RequestChild) string {
	if child.TargetSchoolClass == nil {
		return ""
	}
	return strings.TrimSpace(*child.TargetSchoolClass)
}

// resolveSchoolClass is what a freshly-created student's school_class
// should be: the concrete class when the parent picked one, otherwise
// the bare grade number as a placeholder the admin renames later. Used
// only for brand-new student rows — never to overwrite an existing
// student, where clobbering a concrete "2a" with a bare grade number
// would lose information (see the rollover/adjustment paths). Issue #1833.
func resolveSchoolClass(child *RequestChild) string {
	if concrete := concreteSchoolClass(child); concrete != "" {
		return concrete
	}
	return gradeToClass(child.TargetGradeLevel)
}

// resolveRolloverSchoolClass decides what an EXISTING student's
// school_class becomes when a rollover approval updates the row in place.
// Pure (no DB) so it is unit-testable. Issue #1833.
//
// Rules, in order:
//   - rollover carries a concrete class (e.g. "3a") -> use it.
//   - no concrete class and no target grade -> keep the existing class. The
//     tenant deliberately did not collect a grade, so there is no replacement
//     class information to apply.
//   - no concrete class, but the current class is still an un-customized
//     bare grade placeholder (empty or all digits, e.g. "1") -> re-derive
//     the new grade number so grade bumps still track ("1" -> "2"). Also
//     covers legacy/admin-reviewed rows whose source grade is nil.
//   - no concrete class and the current class is a customised concrete
//     class whose grade still matches the rollover's target grade ("3b"
//     rolling into grade 3, or any half-year rollover that keeps the
//     grade) -> keep it. The letter is still valid for the new grade.
//   - no concrete class and the current class is a concrete class from a
//     DIFFERENT grade than the rollover target ("2a" while the grade
//     bumps to 3) -> the letter is stale for the new grade, so fall back
//     to the bare grade placeholder ("3"). Keeping "2a" would strand a
//     grade-3 student in a grade-2 class on every class-based view
//     (rosters, filters) until someone noticed and fixed it by hand; the
//     admin instead reassigns the concrete class in the new grade
//     ("manuell zuordnen"). Classes with no numeric prefix ("Bienen")
//     carry no derivable grade and are left untouched.
//
// Mirrors the bare-placeholder check of the approved-child sync (#1833).
func resolveRolloverSchoolClass(child *RequestChild, existingClass string) string {
	if concrete := concreteSchoolClass(child); concrete != "" {
		return concrete
	}
	if child.TargetGradeLevel == nil || *child.TargetGradeLevel == 0 {
		return existingClass
	}
	if isBareGradePlaceholderClass(existingClass) {
		return gradeToClass(child.TargetGradeLevel)
	}
	if newGradeClass := gradeToClass(child.TargetGradeLevel); newGradeClass != "" {
		if prefix := gradePrefix(existingClass); prefix != "" && prefix != newGradeClass {
			return newGradeClass
		}
	}
	return existingClass
}
