package schoolstructure

import "strings"

// NormalizeClass maps a school class name onto the identity class matching
// compares with: trimmed and lowercased. users.students.school_class is free
// text, so "1a", " 1a " and "1A" are the same class; the database side
// matches LOWER(BTRIM(school_class)). It is School Structure's class-name
// rule (internal/schoolclass.Normalize) for the owners and workflows that
// match classes; class_name_internal_test.go pins the two together.
func NormalizeClass(class string) string {
	return strings.ToLower(strings.TrimSpace(class))
}

// Supported grade bounds are shared by target-group validation, tenant
// metadata, and enrollment settings. Keeping them here prevents the public
// form and timetable APIs from drifting apart again.
const (
	MinGradeLevel        = 1
	DefaultGradeLevelMax = 4
	MaxGradeLevel        = 13
)
