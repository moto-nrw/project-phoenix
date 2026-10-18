package students

import (
	strings "strings"
)

// schoolClassKey is the matching identity of a class name, trimmed and
// lowercased, as School Structure compares classes
// (internal/schoolclass.Normalize).
func schoolClassKey(class string) string {
	return strings.ToLower(strings.TrimSpace(class))
}
