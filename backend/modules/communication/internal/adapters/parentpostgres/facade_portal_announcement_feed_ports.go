package parentpostgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func (r schoolParentAnnouncementRepository) PortalListFeedForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) ([]*communication.PortalAnnouncementFeedItem, error) {
	return r.ListFeedForAccount(ctx, accountID, scope)
}
