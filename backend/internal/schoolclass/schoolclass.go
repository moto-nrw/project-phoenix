// Package schoolclass provides small helpers for deriving structure out of
// the free-text users.students.school_class column (e.g. "3a").
package schoolclass

import "strings"

// Normalize maps a school class name onto the identity used for matching:
// trimmed and lowercased. users.students.school_class is free text, so "1a",
// " 1a " and "1A" must count as the same class everywhere — the DB side uses
// LOWER(BTRIM(school_class)) indexes, and Go-side comparisons must apply the
// same rule or class scoping silently misses students.
func Normalize(class string) string {
	return strings.ToLower(strings.TrimSpace(class))
}
