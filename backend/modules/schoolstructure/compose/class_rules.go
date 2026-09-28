package compose

import "github.com/moto-nrw/project-phoenix/internal/schoolclass"

// School Structure's class rules, which the composition root binds into the
// consumer-owned ports of other owners (#3424 slice S1): the supported grade
// range Jahrgang targets are validated against and the class-name identity
// class matching compares with.
const (
	MinGradeLevel = schoolclass.MinGradeLevel
	MaxGradeLevel = schoolclass.MaxGradeLevel
)

// NormalizeClass maps a school class name onto its matching identity:
// trimmed and lowercased.
func NormalizeClass(class string) string {
	return schoolclass.Normalize(class)
}
