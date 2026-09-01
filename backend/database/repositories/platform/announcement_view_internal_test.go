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

func TestUnreadQueriesUseDatabaseClock(t *testing.T) {
	t.Parallel()
	if got := strings.Count(unreadWhereClause, "CURRENT_TIMESTAMP"); got != 2 {
		t.Fatalf("unread WHERE clause contains CURRENT_TIMESTAMP %d times, want 2", got)
	}
}
