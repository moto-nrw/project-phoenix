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

// TruncateBytes caps s to at most n bytes and appends suffix when it cut.
// Pass "" for a plain cut. Byte-based: only safe for ASCII-bounded content
// (log/error payloads); use TruncateRunes for user-visible text.
func TruncateBytes(s string, n int, suffix string) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + suffix
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

// JoinUnique splits every value on ";", trims the parts, and joins the
// non-empty ones on "; " with case-insensitive duplicates removed
// (first occurrence wins, original casing kept).
func JoinUnique(values ...string) string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ";") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "; ")
}
