// Package schoolclass provides small helpers for deriving structure out of
// the free-text users.students.school_class column (e.g. "3a").
package schoolclass

import "strings"

// Supported grade bounds are shared by target-group validation, tenant
// metadata, and enrollment settings. Keeping them here prevents the public
// form and timetable APIs from drifting apart again.
const (
	MinGradeLevel        = 1
	DefaultGradeLevelMax = 4
	MaxGradeLevel        = 13
)

// Normalize maps a school class name onto the identity used for matching:
// trimmed and lowercased. users.students.school_class is free text, so "1a",
// " 1a " and "1A" must count as the same class everywhere — the DB side uses
// LOWER(BTRIM(school_class)) indexes, and Go-side comparisons must apply the
// same rule or class scoping silently misses students.
func Normalize(class string) string {
	return strings.ToLower(strings.TrimSpace(class))
}

// GradePrefix returns the first run of ASCII digits in a school class name
// ("2a" -> "2", "Klasse 12b" -> "12"), or "" when the class contains no
// grade number ("Bienen"). Despite the historical name, the number need not
// be a literal prefix: frontend class parsing has long supported labels such
// as "Klasse 3a", and every backend filter must use the same grammar.
func GradePrefix(class string) string {
	class = strings.TrimSpace(class)
	start := 0
	for start < len(class) && (class[start] < '0' || class[start] > '9') {
		start++
	}
	end := start
	for end < len(class) && class[end] >= '0' && class[end] <= '9' {
		end++
	}
	return class[start:end]
}
