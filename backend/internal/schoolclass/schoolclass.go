// Package schoolclass provides small helpers for deriving structure out of
// the free-text users.students.school_class column (e.g. "3a").
package schoolclass

import "strings"

// GradePrefix returns the leading run of ASCII digits in a school class name
// ("2a" -> "2", "12b" -> "12"), or "" when the class has no numeric prefix
// ("Bienen"). Mirrors services/enrollment/request_service.go's
// schoolClassGradePrefix (not exported from that package, so duplicated here
// rather than creating a cross-domain import) and the frontend's
// getSchoolYear (frontend/src/lib/student-helpers.ts) — all three must derive
// the same grade from the same class string.
func GradePrefix(class string) string {
	class = strings.TrimSpace(class)
	end := 0
	for end < len(class) && class[end] >= '0' && class[end] <= '9' {
		end++
	}
	return class[:end]
}
