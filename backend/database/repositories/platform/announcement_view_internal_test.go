package platform

import (
	"strings"
	"testing"
)

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestBuildUnreadArgs_CountMatchesPlaceholders(t *testing.T) {
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
