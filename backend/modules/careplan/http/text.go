package careplanhttp

import (
	strings "strings"
)

// trimToNil returns the whitespace-trimmed value, or nil when the result is
// empty. It normalizes optional free-text fields before persistence.
func trimToNil(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// trimPtrToNil is trimToNil for optional inputs: nil stays nil.
func trimPtrToNil(p *string) *string {
	if p == nil {
		return nil
	}
	return trimToNil(*p)
}
