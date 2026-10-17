package parentstore

import (
	"context"

	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func (r *announcementRepository) PortalAnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*communication.PortalAnnouncementPollChild, error) {
	result0, result1 := r.AnswerableChildren(ctx, accountID, announcementIDs)
	return portalRecordSlice(result0, func(row *models.AnnouncementPollChild) *communication.PortalAnnouncementPollChild {
		return portalRecordPointer(row, portalFromRetainedAnnouncementPollChild)
	}), result1
}
func (r *announcementRepository) PortalCountUnreadForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) (int, error) {
	retainedScope := portalToRetainedAnnouncementFeedScope(scope)
	result0, result1 := r.CountUnreadForAccount(ctx, accountID, retainedScope)
	return result0, result1
}
func (r *announcementRepository) PortalFindByID(ctx context.Context, id int64) (*communication.PortalParentAnnouncement, error) {
	result0, result1 := r.FindByID(ctx, id)
	return portalRecordPointer(result0, portalFromRetainedParentAnnouncement), result1
}
func (r *announcementRepository) PortalListFeedForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) ([]*communication.PortalAnnouncementFeedItem, error) {
	retainedScope := portalToRetainedAnnouncementFeedScope(scope)
	result0, result1 := r.ListFeedForAccount(ctx, accountID, retainedScope)
	return portalRecordSlice(result0, func(row *models.AnnouncementFeedItem) *communication.PortalAnnouncementFeedItem {
		return portalRecordPointer(row, portalFromRetainedAnnouncementFeedItem)
	}), result1
}
func (r *announcementRepository) PortalListOptions(ctx context.Context, announcementID int64) ([]*communication.PortalParentAnnouncementOption, error) {
	result0, result1 := r.ListOptions(ctx, announcementID)
	return portalRecordSlice(result0, func(row *models.ParentAnnouncementOption) *communication.PortalParentAnnouncementOption {
		return portalRecordPointer(row, portalFromRetainedParentAnnouncementOption)
	}), result1
}
func (r *announcementRepository) PortalListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*communication.PortalParentAnnouncementOption, error) {
	result0, result1 := r.ListOptionsForAnnouncements(ctx, announcementIDs)
	return portalRecordSlice(result0, func(row *models.ParentAnnouncementOption) *communication.PortalParentAnnouncementOption {
		return portalRecordPointer(row, portalFromRetainedParentAnnouncementOption)
	}), result1
}
func (r schoolParentAnnouncementRepository) PortalListFeedForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) ([]*communication.PortalAnnouncementFeedItem, error) {
	retainedScope := portalToRetainedAnnouncementFeedScope(scope)
	result0, result1 := r.ListFeedForAccount(ctx, accountID, retainedScope)
	return portalRecordSlice(result0, func(row *models.AnnouncementFeedItem) *communication.PortalAnnouncementFeedItem {
		return portalRecordPointer(row, portalFromRetainedAnnouncementFeedItem)
	}), result1
}
