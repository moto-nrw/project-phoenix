package messaging

import (
	"context"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
)

// AnnouncementAccess combines the feed and declaration capabilities consumed
// by the parent workflow. It excludes staff authoring, delivery and reminders.
type AnnouncementAccess interface {
	AnnouncementFeedAccess
	DeclarationReads
	DeclarationCommands
}

// AnnouncementFeedAccess reads the guardian's feed and poll choices and records
// their engagement. The workflow checks the audience and relationship first.
type AnnouncementFeedAccess interface {
	AccountMatchesAnnouncement(ctx context.Context, tenantID, announcementID, accountID int64) (bool, error)
	AccountMayAnswerForStudent(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (bool, error)
	AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*usersModels.AnnouncementPollChild, error)
	CountUnreadForAccount(ctx context.Context, accountID int64, scope usersModels.AnnouncementFeedScope) (int, error)
	FindByID(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error)
	ListFeedForAccount(ctx context.Context, accountID int64, scope usersModels.AnnouncementFeedScope) ([]*usersModels.AnnouncementFeedItem, error)
	ListOptions(ctx context.Context, announcementID int64) ([]*usersModels.ParentAnnouncementOption, error)
	ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*usersModels.ParentAnnouncementOption, error)
	MarkAcknowledged(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error)
	MarkRead(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error)
	SetResponse(ctx context.Context, tenantID, announcementID, studentID, accountID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error)
}

// DeclarationReads loads versions, signer facts and history for the guardian's
// declaration feed and proof. It cannot publish or change declaration content.
type DeclarationReads interface {
	DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64) ([]*usersModels.DeclarationChild, error)
	DeclarationSettings(ctx context.Context, announcementIDs []int64) (map[int64]usersModels.AnnouncementDeclarationSettings, error)
	DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*usersModels.DeclarationSigner, error)
	LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*usersModels.DeclarationVersion, error)
	LatestDeclarationVersions(ctx context.Context, announcementIDs []int64) (map[int64]*usersModels.DeclarationVersion, error)
	ListDeclarationSubmissionsForStudents(ctx context.Context, announcementIDs, studentIDs []int64) ([]*usersModels.DeclarationSubmission, error)
	ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.DeclarationVersion, error)
	ReadOpenDeclarations(ctx context.Context, accountID int64, tenantIDs []int64) (map[int64]*time.Time, error)
}

// DeclarationCommands serializes declarations, holds the authorizing relationship
// and appends a submission in the caller's tenant unit of work.
type DeclarationCommands interface {
	HoldDeclarationSigner(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (*usersModels.DeclarationSignerContext, error)
	HoldHistoricalDeclarationSigner(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (*usersModels.DeclarationSignerContext, error)
	InsertDeclarationSubmission(ctx context.Context, submission *usersModels.DeclarationSubmission) error
	LockDeclarationChild(ctx context.Context, announcementID, studentID int64) error
}

// MessageThreadAccess resolves or atomically creates the guardian's conversation
// inside the child's authorized tenant transaction. It exposes no staff update commands.
type MessageThreadAccess interface {
	FindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64) (*usersModels.ParentMessageThread, error)
	GetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64) (*usersModels.ParentMessageThread, error)
}

// MessageReads loads the conversation snapshot. Communication's shared core
// remains the only append and receipt policy used by the workflow.
type MessageReads interface {
	ListByThread(ctx context.Context, threadID int64, limit int) ([]*usersModels.ParentMessage, error)
}

// MessageInboxReads reads guardian inboxes and counts. Cross-tenant methods
// receive the schools already resolved by the workflow and use its admin unit of work.
type MessageInboxReads interface {
	ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64) ([]*usersModels.InboxThread, error)
	ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*usersModels.InboxThread, error)
	UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error)
}

// RequestShareRecords appends and reads the guardian's request-sharing ledger
// inside the caller's tenant transaction.
type RequestShareRecords interface {
	Create(context.Context, *usersModels.ParentRequestShareEvent) error
	CurrentForStudent(context.Context, int64) ([]*usersModels.ParentRequestShareEvent, error)
}

// RequestEvents is the request history surface used by sharing: append a change
// atomically and read its history. Listing every event for a child is not needed.
type RequestEvents interface {
	care.RequestEventWriter
	ListForRequest(ctx context.Context, requestType string, requestID int64) ([]*care.RequestEvent, error)
}
