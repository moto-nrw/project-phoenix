//go:build testhooks

package parentportal

import "context"

// AppendConversationMessageFixture uses the same composed native conversation
// and copies the owner's complete mutation back into the public fixture value.
func AppendConversationMessageFixture(ctx context.Context, core ConversationCore, value *ParentMessage) error {
	row := messagingValuePointer(value, messagingToAppParentMessage)
	err := core.AppendMessage(ctx, row)
	if value != nil && row != nil {
		*value = messagingFromAppParentMessage(*row)
	}
	return err
}
