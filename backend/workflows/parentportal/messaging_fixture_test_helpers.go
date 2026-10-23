//go:build testhooks

package parentportal

import "context"

// AppendConversationMessageFixture appends through the same composed native
// conversation; the owner's mutation lands on the fixture value itself.
func AppendConversationMessageFixture(ctx context.Context, core ConversationCore, value *ParentMessage) error {
	return core.AppendMessage(ctx, value)
}
