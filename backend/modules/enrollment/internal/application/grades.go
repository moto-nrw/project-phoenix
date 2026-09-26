package application

import "strings"

// School grade bounds the enrollment form works with. They match the
// School Structure rules the form and the phase validation already apply
// (grades 1..13, a four-grade primary school by default).
const (
	minGradeLevel        = 1
	defaultGradeLevelMax = 4
	maxGradeLevel        = 13
)

// normalizeClass is the comparison form of a class name: trimmed and lower
// case.
func normalizeClass(class string) string {
	return strings.ToLower(strings.TrimSpace(class))
}

// gradePrefix returns the first run of digits in a class name, the grade
// of "3a" or "Klasse 3a", or "" for a class without one ("Bienen").
func gradePrefix(class string) string {
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
