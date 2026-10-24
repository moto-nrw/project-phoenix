package communication

import (
	"context"
	"time"
)

// PortalAnnouncementAccess composes feed, declaration reads and submission commands.
type PortalAnnouncementAccess interface {
	PortalAnnouncementFeed
	PortalDeclarationReads
	PortalDeclarationCommands
}

type PortalAnnouncementFeed interface {
	AccountMatchesAnnouncement(ctx context.Context, tenantID int64, announcementID int64, accountID int64) (bool, error)
	AccountMayAnswerForStudent(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (bool, error)
	PortalAnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*PortalAnnouncementPollChild, error)
	PortalCountUnreadForAccount(ctx context.Context, accountID int64, scope PortalAnnouncementFeedScope) (int, error)
	PortalFindByID(ctx context.Context, id int64) (*PortalParentAnnouncement, error)
	PortalListFeedForAccount(ctx context.Context, accountID int64, scope PortalAnnouncementFeedScope) ([]*PortalAnnouncementFeedItem, error)
	PortalListOptions(ctx context.Context, announcementID int64) ([]*PortalParentAnnouncementOption, error)
	PortalListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*PortalParentAnnouncementOption, error)
	MarkAcknowledged(ctx context.Context, tenantID int64, announcementID int64, accountID int64, expectedPublishedAt time.Time) (bool, error)
	MarkRead(ctx context.Context, tenantID int64, announcementID int64, accountID int64, expectedPublishedAt time.Time) (bool, error)
	SetResponse(ctx context.Context, tenantID int64, announcementID int64, studentID int64, accountID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error)
}

type PortalDeclarationReads interface {
	PortalDeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64) ([]*PortalDeclarationChild, error)
	PortalDeclarationSettings(ctx context.Context, announcementIDs []int64) (map[int64]PortalAnnouncementDeclarationSettings, error)
	PortalDeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*PortalDeclarationSigner, error)
	PortalLatestDeclarationVersion(ctx context.Context, tenantID int64, announcementID int64) (*PortalDeclarationVersion, error)
	PortalLatestDeclarationVersions(ctx context.Context, announcementIDs []int64) (map[int64]*PortalDeclarationVersion, error)
	PortalListDeclarationSubmissionsForStudents(ctx context.Context, announcementIDs []int64, studentIDs []int64) ([]*PortalDeclarationSubmission, error)
	PortalListDeclarationVersions(ctx context.Context, tenantID int64, announcementID int64) ([]*PortalDeclarationVersion, error)
	ReadOpenDeclarations(ctx context.Context, accountID int64, tenantIDs []int64) (map[int64]*time.Time, error)
}

type PortalDeclarationCommands interface {
	PortalHoldDeclarationSigner(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*PortalDeclarationSignerContext, error)
	PortalHoldHistoricalDeclarationSigner(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*PortalDeclarationSignerContext, error)
	PortalInsertDeclarationSubmission(ctx context.Context, submission *PortalDeclarationSubmission) error
	LockDeclarationChild(ctx context.Context, announcementID int64, studentID int64) error
}
