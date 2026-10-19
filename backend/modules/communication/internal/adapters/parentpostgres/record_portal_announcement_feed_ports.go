package parentpostgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func (r *announcementRepository) PortalAnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*communication.PortalAnnouncementPollChild, error) {
	return r.AnswerableChildren(ctx, accountID, announcementIDs)
}
func (r *announcementRepository) PortalCountUnreadForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) (int, error) {
	return r.CountUnreadForAccount(ctx, accountID, scope)
}
func (r *announcementRepository) PortalFindByID(ctx context.Context, id int64) (*communication.PortalParentAnnouncement, error) {
	return r.FindByID(ctx, id)
}
func (r *announcementRepository) PortalListFeedForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) ([]*communication.PortalAnnouncementFeedItem, error) {
	return r.ListFeedForAccount(ctx, accountID, scope)
}
func (r *announcementRepository) PortalListOptions(ctx context.Context, announcementID int64) ([]*communication.PortalParentAnnouncementOption, error) {
	return r.ListOptions(ctx, announcementID)
}
func (r *announcementRepository) PortalListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*communication.PortalParentAnnouncementOption, error) {
	return r.ListOptionsForAnnouncements(ctx, announcementIDs)
}
