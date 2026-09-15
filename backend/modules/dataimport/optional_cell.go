package dataimport

import "strings"

// OptionalCell converts an optional upload cell to an owner-command value.
// Blank cells mean unset; surrounding whitespace is not part of the value.
func OptionalCell(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
