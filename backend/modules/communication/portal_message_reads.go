package communication

import (
	"context"
)

type PortalMessageReads interface {
	PortalListByThread(ctx context.Context, threadID int64, limit int) ([]*PortalParentMessage, error)
}
