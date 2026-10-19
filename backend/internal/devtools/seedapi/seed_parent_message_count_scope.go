package api

import (
	"context"
	"fmt"
)

// seedParentMessageCountScopeStep gives one staff member a personal count
// scope for parent messages (#3673): the counter at "Nachrichten" counts only
// children of their own groups. It fills users.parent_message_count_preferences
// so the setting shows a stored choice on a dev machine. The first account,
// the shared developer login, keeps the default and still counts every
// message.
type seedParentMessageCountScopeStep struct{}

func (seedParentMessageCountScopeStep) Name() string { return "Seeding parent message count scope" }

func (seedParentMessageCountScopeStep) Run(ctx context.Context, rt *Runtime) error {
	if rt == nil || rt.FixedSeeder == nil || rt.Adapter == nil || rt.Client == nil {
		return fmt.Errorf("parent message count scope prerequisites not available")
	}
	staff, _ := buildStaffOrder(rt.FixedSeeder)
	// The second account chooses; the first, the shared developer login,
	// keeps the default.
	if len(staff) < 2 {
		return fmt.Errorf("parent message count scope requires at least two staff accounts, got %d", len(staff))
	}
	member := staff[1]
	auth, err := rt.Adapter.LoginTenant(ctx, member.Email, member.Password, "")
	if err != nil {
		return fmt.Errorf("login %s for parent message count scope: %w", member.Email, err)
	}
	if _, err := rt.Client.PutWithAuth(auth, "/api/messages/count-scope", map[string]any{"scope": "own_groups"}); err != nil {
		return fmt.Errorf("seed parent message count scope: %w", err)
	}
	fmt.Printf("  %s counts parent messages of their own groups only\n", member.Name)
	return nil
}
