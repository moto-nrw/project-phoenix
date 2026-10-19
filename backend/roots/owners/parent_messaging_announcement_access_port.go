package owners

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func BindParentAnnouncementAccess(source communication.PortalAnnouncementAccess) parentportal.AnnouncementAccess {
	if source == nil {
		return nil
	}
	return parentportal.NewAnnouncementAccess(parentportal.AnnouncementAccessCallbacks{
		AccountMatchesAnnouncement: func(ctx context.Context, tenantID int64, announcementID int64, accountID int64) (bool, error) {
			result0, result1 := source.AccountMatchesAnnouncement(ctx, tenantID, announcementID, accountID)
			return result0, result1
		},
		AccountMayAnswerForStudent: func(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (bool, error) {
			result0, result1 := source.AccountMayAnswerForStudent(ctx, tenantID, announcementID, accountID, studentID)
			return result0, result1
		},
		AnswerableChildren: func(ctx context.Context, accountID int64, announcementIDs []int64) ([]*parentportal.AnnouncementPollChild, error) {
			result0, result1 := source.PortalAnswerableChildren(ctx, accountID, announcementIDs)
			return ParentMessagingSlice(result0, func(row *communication.PortalAnnouncementPollChild) *parentportal.AnnouncementPollChild {
				return ParentMessagingPointer(row, parentMessagingFromOwnerAnnouncementPollChild)
			}), result1
		},
		CountUnreadForAccount: func(ctx context.Context, accountID int64, scope parentportal.AnnouncementFeedScope) (int, error) {
			ownerScope := parentMessagingToOwnerAnnouncementFeedScope(scope)
			result0, result1 := source.PortalCountUnreadForAccount(ctx, accountID, ownerScope)
			return result0, result1
		},
		FindByID: func(ctx context.Context, id int64) (*parentportal.ParentAnnouncement, error) {
			result0, result1 := source.PortalFindByID(ctx, id)
			return ParentMessagingPointer(result0, parentMessagingFromOwnerParentAnnouncement), result1
		},
		ListFeedForAccount: func(ctx context.Context, accountID int64, scope parentportal.AnnouncementFeedScope) ([]*parentportal.AnnouncementFeedItem, error) {
			ownerScope := parentMessagingToOwnerAnnouncementFeedScope(scope)
			result0, result1 := source.PortalListFeedForAccount(ctx, accountID, ownerScope)
			return ParentMessagingSlice(result0, func(row *communication.PortalAnnouncementFeedItem) *parentportal.AnnouncementFeedItem {
				return ParentMessagingPointer(row, parentMessagingFromOwnerAnnouncementFeedItem)
			}), result1
		},
		ListOptions: func(ctx context.Context, announcementID int64) ([]*parentportal.ParentAnnouncementOption, error) {
			result0, result1 := source.PortalListOptions(ctx, announcementID)
			return ParentMessagingSlice(result0, func(row *communication.PortalParentAnnouncementOption) *parentportal.ParentAnnouncementOption {
				return ParentMessagingPointer(row, parentMessagingFromOwnerParentAnnouncementOption)
			}), result1
		},
		ListOptionsForAnnouncements: func(ctx context.Context, announcementIDs []int64) ([]*parentportal.ParentAnnouncementOption, error) {
			result0, result1 := source.PortalListOptionsForAnnouncements(ctx, announcementIDs)
			return ParentMessagingSlice(result0, func(row *communication.PortalParentAnnouncementOption) *parentportal.ParentAnnouncementOption {
				return ParentMessagingPointer(row, parentMessagingFromOwnerParentAnnouncementOption)
			}), result1
		},
		MarkAcknowledged: func(ctx context.Context, tenantID int64, announcementID int64, accountID int64, expectedPublishedAt time.Time) (bool, error) {
			result0, result1 := source.MarkAcknowledged(ctx, tenantID, announcementID, accountID, expectedPublishedAt)
			return result0, result1
		},
		MarkRead: func(ctx context.Context, tenantID int64, announcementID int64, accountID int64, expectedPublishedAt time.Time) (bool, error) {
			result0, result1 := source.MarkRead(ctx, tenantID, announcementID, accountID, expectedPublishedAt)
			return result0, result1
		},
		SetResponse: func(ctx context.Context, tenantID int64, announcementID int64, studentID int64, accountID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error) {
			result0, result1 := source.SetResponse(ctx, tenantID, announcementID, studentID, accountID, optionIDs, expectedPublishedAt)
			return result0, result1
		},
		DeclarationChildrenForAccount: func(ctx context.Context, accountID int64, announcementIDs []int64) ([]*parentportal.DeclarationChild, error) {
			result0, result1 := source.PortalDeclarationChildrenForAccount(ctx, accountID, announcementIDs)
			return ParentMessagingSlice(result0, func(row *communication.PortalDeclarationChild) *parentportal.DeclarationChild {
				return ParentMessagingPointer(row, parentMessagingFromOwnerDeclarationChild)
			}), result1
		},
		DeclarationSettings: func(ctx context.Context, announcementIDs []int64) (map[int64]parentportal.AnnouncementDeclarationSettings, error) {
			result0, result1 := source.PortalDeclarationSettings(ctx, announcementIDs)
			return parentMessagingMap(result0, func(row communication.PortalAnnouncementDeclarationSettings) parentportal.AnnouncementDeclarationSettings {
				return parentMessagingFromOwnerAnnouncementDeclarationSettings(row)
			}), result1
		},
		DeclarationSignersForStudents: func(ctx context.Context, studentIDs []int64) ([]*parentportal.DeclarationSigner, error) {
			result0, result1 := source.PortalDeclarationSignersForStudents(ctx, studentIDs)
			return ParentMessagingSlice(result0, func(row *communication.PortalDeclarationSigner) *parentportal.DeclarationSigner {
				return ParentMessagingPointer(row, parentMessagingFromOwnerDeclarationSigner)
			}), result1
		},
		LatestDeclarationVersion: func(ctx context.Context, tenantID int64, announcementID int64) (*parentportal.DeclarationVersion, error) {
			result0, result1 := source.PortalLatestDeclarationVersion(ctx, tenantID, announcementID)
			return ParentMessagingPointer(result0, ParentMessagingFromOwnerDeclarationVersion), result1
		},
		LatestDeclarationVersions: func(ctx context.Context, announcementIDs []int64) (map[int64]*parentportal.DeclarationVersion, error) {
			result0, result1 := source.PortalLatestDeclarationVersions(ctx, announcementIDs)
			return parentMessagingMap(result0, func(row *communication.PortalDeclarationVersion) *parentportal.DeclarationVersion {
				return ParentMessagingPointer(row, ParentMessagingFromOwnerDeclarationVersion)
			}), result1
		},
		ListDeclarationSubmissionsForStudents: func(ctx context.Context, announcementIDs []int64, studentIDs []int64) ([]*parentportal.DeclarationSubmission, error) {
			result0, result1 := source.PortalListDeclarationSubmissionsForStudents(ctx, announcementIDs, studentIDs)
			return ParentMessagingSlice(result0, func(row *communication.PortalDeclarationSubmission) *parentportal.DeclarationSubmission {
				return ParentMessagingPointer(row, ParentMessagingFromOwnerDeclarationSubmission)
			}), result1
		},
		ListDeclarationVersions: func(ctx context.Context, tenantID int64, announcementID int64) ([]*parentportal.DeclarationVersion, error) {
			result0, result1 := source.PortalListDeclarationVersions(ctx, tenantID, announcementID)
			return ParentMessagingSlice(result0, func(row *communication.PortalDeclarationVersion) *parentportal.DeclarationVersion {
				return ParentMessagingPointer(row, ParentMessagingFromOwnerDeclarationVersion)
			}), result1
		},
		ReadOpenDeclarations: func(ctx context.Context, accountID int64, tenantIDs []int64) (map[int64]*time.Time, error) {
			result0, result1 := source.ReadOpenDeclarations(ctx, accountID, tenantIDs)
			return result0, result1
		},
		HoldDeclarationSigner: func(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*parentportal.DeclarationSignerContext, error) {
			result0, result1 := source.PortalHoldDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
			return ParentMessagingPointer(result0, parentMessagingFromOwnerDeclarationSignerContext), result1
		},
		HoldHistoricalDeclarationSigner: func(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*parentportal.DeclarationSignerContext, error) {
			result0, result1 := source.PortalHoldHistoricalDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
			return ParentMessagingPointer(result0, parentMessagingFromOwnerDeclarationSignerContext), result1
		},
		InsertDeclarationSubmission: func(ctx context.Context, submission *parentportal.DeclarationSubmission) error {
			ownerSubmission := ParentMessagingPointer(submission, ParentMessagingToOwnerDeclarationSubmission)
			result0 := source.PortalInsertDeclarationSubmission(ctx, ownerSubmission)
			if submission != nil && ownerSubmission != nil {
				*submission = ParentMessagingFromOwnerDeclarationSubmission(*ownerSubmission)
			}
			return result0
		},
		LockDeclarationChild: func(ctx context.Context, announcementID int64, studentID int64) error {
			result0 := source.LockDeclarationChild(ctx, announcementID, studentID)
			return result0
		},
	})
}
