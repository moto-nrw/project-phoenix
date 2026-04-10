package platform

import (
	"strings"
	"testing"
)

func TestBuildUnreadArgs_CountMatchesPlaceholders(t *testing.T) {
	// The full query used by GetUnreadForUser and CountUnread has one ? in the
	// JOIN clause (v.user_id = ?) plus all ?'s in unreadWhereClause. The args
	// returned by buildUnreadArgs must match that total exactly.
	joinPlaceholders := 1 // LEFT JOIN ... ON ... AND v.user_id = ?
	wherePlaceholders := strings.Count(unreadWhereClause, "?")
	expectedArgs := joinPlaceholders + wherePlaceholders

	args := buildUnreadArgs(1, []string{"user"}, 2, 3)

	if len(args) != expectedArgs {
		t.Errorf("buildUnreadArgs returned %d args, but query has %d placeholders (JOIN: %d + WHERE: %d)",
			len(args), expectedArgs, joinPlaceholders, wherePlaceholders)
	}
}
