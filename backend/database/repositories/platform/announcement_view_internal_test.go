package platform

import (
	"strings"
	"testing"
)

func TestBuildUnreadArgs_CountMatchesPlaceholders(t *testing.T) {
	t.Parallel()
	// The full query used by GetUnreadForUser and CountUnread has all ?'s in the
	// shared FROM clause plus all ?'s in unreadWhereClause. The args
	// returned by buildUnreadArgs must match that total exactly.
	fromPlaceholders := strings.Count(unreadFromClause, "?")
	wherePlaceholders := strings.Count(unreadWhereClause, "?")
	expectedArgs := fromPlaceholders + wherePlaceholders

	args := buildUnreadArgs(1, []string{"user"}, 2, 3)

	if len(args) != expectedArgs {
		t.Errorf("buildUnreadArgs returned %d args, but query has %d placeholders (FROM: %d + WHERE: %d)",
			len(args), expectedArgs, fromPlaceholders, wherePlaceholders)
	}
}

func TestUnreadWhereClauseUsesDatabaseClockForActivityWindow(t *testing.T) {
	t.Parallel()

	if !strings.Contains(unreadWhereClause, "a.published_at <= CURRENT_TIMESTAMP") {
		t.Error("published_at activity check must use the database clock")
	}
	if !strings.Contains(unreadWhereClause, "a.expires_at > CURRENT_TIMESTAMP") {
		t.Error("expires_at activity check must use the database clock")
	}
}
