// Package strutil provides small string helpers shared across the backend.
// It replaces the per-package trim-to-nil, truncate, and join-unique copies
// consolidated in the 2026-07 code-reduction audit (B7).
package strutil

import "strings"

// TrimToNil returns the whitespace-trimmed value, or nil when the result is
// empty. Used to normalize optional free-text fields before persistence.
func TrimToNil(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// TrimPtrToNil is TrimToNil for optional inputs: nil stays nil.
func TrimPtrToNil(p *string) *string {
	if p == nil {
		return nil
	}
	return TrimToNil(*p)
}

// TruncateRunes caps s to at most n runes (multibyte-safe) and appends suffix
// when it cut. Pass "" for a plain cut.
func TruncateRunes(s string, n int, suffix string) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + suffix
}

// ContainsFold reports whether s contains substr, ignoring case (ToLower on
// both sides — the search/filter semantics of the staff and student lists).
func ContainsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
