package messaging

import (
	"context"

	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

// BindPortalConversationCore exposes the same bound native conversation rules.
func BindPortalConversationCore(core *ConversationCore) values.ParentConversationSource {
	if core == nil {
		return nil
	}
	return values.BindParentConversationCore(func(ctx context.Context, row *values.ParentMessage) error {
		return core.AppendMessage(ctx, (*ParentMessage)(row))
	}, func(ctx context.Context, a, b, c int64, staff bool, rows []*values.ParentMessage) (bool, error) {
		return core.MarkReadToNewest(ctx, a, b, c, staff, parentMessagesFromApplication(rows))
	}, func(ctx context.Context, a, b int64, rows []*values.ParentMessage) {
		core.DecorateReadReceipts(ctx, a, b, parentMessagesFromApplication(rows))
	}, core.Broadcast, core.BroadcastRead)
}
