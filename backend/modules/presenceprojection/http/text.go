package presenceprojectionhttp

import (
	strings "strings"
)

// containsFold reports whether s contains substr, ignoring case (ToLower on
// both sides, the search and filter semantics of the student lists).
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
