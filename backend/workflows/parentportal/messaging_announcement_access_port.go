package parentportal

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

type AnnouncementAccess interface{ messaging.AnnouncementAccess }
type AnnouncementAccessCallbacks struct {
	AccountMatchesAnnouncement            func(ctx context.Context, tenantID int64, announcementID int64, accountID int64) (bool, error)
	AccountMayAnswerForStudent            func(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (bool, error)
	AnswerableChildren                    func(ctx context.Context, accountID int64, announcementIDs []int64) ([]*AnnouncementPollChild, error)
	CountUnreadForAccount                 func(ctx context.Context, accountID int64, scope AnnouncementFeedScope) (int, error)
	FindByID                              func(ctx context.Context, id int64) (*ParentAnnouncement, error)
	ListFeedForAccount                    func(ctx context.Context, accountID int64, scope AnnouncementFeedScope) ([]*AnnouncementFeedItem, error)
	ListOptions                           func(ctx context.Context, announcementID int64) ([]*ParentAnnouncementOption, error)
	ListOptionsForAnnouncements           func(ctx context.Context, announcementIDs []int64) ([]*ParentAnnouncementOption, error)
	MarkAcknowledged                      func(ctx context.Context, tenantID int64, announcementID int64, accountID int64, expectedPublishedAt time.Time) (bool, error)
	MarkRead                              func(ctx context.Context, tenantID int64, announcementID int64, accountID int64, expectedPublishedAt time.Time) (bool, error)
	SetResponse                           func(ctx context.Context, tenantID int64, announcementID int64, studentID int64, accountID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error)
	DeclarationChildrenForAccount         func(ctx context.Context, accountID int64, announcementIDs []int64) ([]*DeclarationChild, error)
	DeclarationSettings                   func(ctx context.Context, announcementIDs []int64) (map[int64]AnnouncementDeclarationSettings, error)
	DeclarationSignersForStudents         func(ctx context.Context, studentIDs []int64) ([]*DeclarationSigner, error)
	LatestDeclarationVersion              func(ctx context.Context, tenantID int64, announcementID int64) (*DeclarationVersion, error)
	LatestDeclarationVersions             func(ctx context.Context, announcementIDs []int64) (map[int64]*DeclarationVersion, error)
	ListDeclarationSubmissionsForStudents func(ctx context.Context, announcementIDs []int64, studentIDs []int64) ([]*DeclarationSubmission, error)
	ListDeclarationVersions               func(ctx context.Context, tenantID int64, announcementID int64) ([]*DeclarationVersion, error)
	ReadOpenDeclarations                  func(ctx context.Context, accountID int64, tenantIDs []int64) (map[int64]*time.Time, error)
	HoldDeclarationSigner                 func(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*DeclarationSignerContext, error)
	HoldHistoricalDeclarationSigner       func(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*DeclarationSignerContext, error)
	InsertDeclarationSubmission           func(ctx context.Context, submission *DeclarationSubmission) error
	LockDeclarationChild                  func(ctx context.Context, announcementID int64, studentID int64) error
}

func NewAnnouncementAccess(c AnnouncementAccessCallbacks) AnnouncementAccess {
	return announcementAccessBinding{c}
}

type announcementAccessBinding struct{ callbacks AnnouncementAccessCallbacks }

func (p announcementAccessBinding) AccountMatchesAnnouncement(ctx context.Context, tenantID int64, announcementID int64, accountID int64) (bool, error) {
	result0, result1 := p.callbacks.AccountMatchesAnnouncement(ctx, tenantID, announcementID, accountID)
	return result0, result1
}
func (p announcementAccessBinding) AccountMayAnswerForStudent(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (bool, error) {
	result0, result1 := p.callbacks.AccountMayAnswerForStudent(ctx, tenantID, announcementID, accountID, studentID)
	return result0, result1
}
func (p announcementAccessBinding) AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*messaging.AnnouncementPollChild, error) {
	result0, result1 := p.callbacks.AnswerableChildren(ctx, accountID, announcementIDs)
	return messagingValueSlice(result0, func(row *AnnouncementPollChild) *messaging.AnnouncementPollChild {
		return messagingValuePointer(row, messagingToAppAnnouncementPollChild)
	}), result1
}
func (p announcementAccessBinding) CountUnreadForAccount(ctx context.Context, accountID int64, scope messaging.AnnouncementFeedScope) (int, error) {
	nativeScope := messagingFromAppAnnouncementFeedScope(scope)
	result0, result1 := p.callbacks.CountUnreadForAccount(ctx, accountID, nativeScope)
	return result0, result1
}
func (p announcementAccessBinding) FindByID(ctx context.Context, id int64) (*messaging.ParentAnnouncement, error) {
	result0, result1 := p.callbacks.FindByID(ctx, id)
	return messagingValuePointer(result0, messagingToAppParentAnnouncement), result1
}
func (p announcementAccessBinding) ListFeedForAccount(ctx context.Context, accountID int64, scope messaging.AnnouncementFeedScope) ([]*messaging.AnnouncementFeedItem, error) {
	nativeScope := messagingFromAppAnnouncementFeedScope(scope)
	result0, result1 := p.callbacks.ListFeedForAccount(ctx, accountID, nativeScope)
	return messagingValueSlice(result0, func(row *AnnouncementFeedItem) *messaging.AnnouncementFeedItem {
		return messagingValuePointer(row, messagingToAppAnnouncementFeedItem)
	}), result1
}
func (p announcementAccessBinding) ListOptions(ctx context.Context, announcementID int64) ([]*messaging.ParentAnnouncementOption, error) {
	result0, result1 := p.callbacks.ListOptions(ctx, announcementID)
	return messagingValueSlice(result0, func(row *ParentAnnouncementOption) *messaging.ParentAnnouncementOption {
		return messagingValuePointer(row, messagingToAppParentAnnouncementOption)
	}), result1
}
func (p announcementAccessBinding) ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*messaging.ParentAnnouncementOption, error) {
	result0, result1 := p.callbacks.ListOptionsForAnnouncements(ctx, announcementIDs)
	return messagingValueSlice(result0, func(row *ParentAnnouncementOption) *messaging.ParentAnnouncementOption {
		return messagingValuePointer(row, messagingToAppParentAnnouncementOption)
	}), result1
}
func (p announcementAccessBinding) MarkAcknowledged(ctx context.Context, tenantID int64, announcementID int64, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	result0, result1 := p.callbacks.MarkAcknowledged(ctx, tenantID, announcementID, accountID, expectedPublishedAt)
	return result0, result1
}
func (p announcementAccessBinding) MarkRead(ctx context.Context, tenantID int64, announcementID int64, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	result0, result1 := p.callbacks.MarkRead(ctx, tenantID, announcementID, accountID, expectedPublishedAt)
	return result0, result1
}
func (p announcementAccessBinding) SetResponse(ctx context.Context, tenantID int64, announcementID int64, studentID int64, accountID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error) {
	result0, result1 := p.callbacks.SetResponse(ctx, tenantID, announcementID, studentID, accountID, optionIDs, expectedPublishedAt)
	return result0, result1
}
func (p announcementAccessBinding) DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64) ([]*messaging.DeclarationChild, error) {
	result0, result1 := p.callbacks.DeclarationChildrenForAccount(ctx, accountID, announcementIDs)
	return messagingValueSlice(result0, func(row *DeclarationChild) *messaging.DeclarationChild {
		return messagingValuePointer(row, messagingToAppDeclarationChild)
	}), result1
}
func (p announcementAccessBinding) DeclarationSettings(ctx context.Context, announcementIDs []int64) (map[int64]messaging.AnnouncementDeclarationSettings, error) {
	result0, result1 := p.callbacks.DeclarationSettings(ctx, announcementIDs)
	return messagingValueMap(result0, func(row AnnouncementDeclarationSettings) messaging.AnnouncementDeclarationSettings {
		return messagingToAppAnnouncementDeclarationSettings(row)
	}), result1
}
func (p announcementAccessBinding) DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*messaging.DeclarationSigner, error) {
	result0, result1 := p.callbacks.DeclarationSignersForStudents(ctx, studentIDs)
	return messagingValueSlice(result0, func(row *DeclarationSigner) *messaging.DeclarationSigner {
		return messagingValuePointer(row, messagingToAppDeclarationSigner)
	}), result1
}
func (p announcementAccessBinding) LatestDeclarationVersion(ctx context.Context, tenantID int64, announcementID int64) (*messaging.DeclarationVersion, error) {
	result0, result1 := p.callbacks.LatestDeclarationVersion(ctx, tenantID, announcementID)
	return messagingValuePointer(result0, messagingToAppDeclarationVersion), result1
}
func (p announcementAccessBinding) LatestDeclarationVersions(ctx context.Context, announcementIDs []int64) (map[int64]*messaging.DeclarationVersion, error) {
	result0, result1 := p.callbacks.LatestDeclarationVersions(ctx, announcementIDs)
	return messagingValueMap(result0, func(row *DeclarationVersion) *messaging.DeclarationVersion {
		return messagingValuePointer(row, messagingToAppDeclarationVersion)
	}), result1
}
func (p announcementAccessBinding) ListDeclarationSubmissionsForStudents(ctx context.Context, announcementIDs []int64, studentIDs []int64) ([]*messaging.DeclarationSubmission, error) {
	result0, result1 := p.callbacks.ListDeclarationSubmissionsForStudents(ctx, announcementIDs, studentIDs)
	return messagingValueSlice(result0, func(row *DeclarationSubmission) *messaging.DeclarationSubmission {
		return messagingValuePointer(row, messagingToAppDeclarationSubmission)
	}), result1
}
func (p announcementAccessBinding) ListDeclarationVersions(ctx context.Context, tenantID int64, announcementID int64) ([]*messaging.DeclarationVersion, error) {
	result0, result1 := p.callbacks.ListDeclarationVersions(ctx, tenantID, announcementID)
	return messagingValueSlice(result0, func(row *DeclarationVersion) *messaging.DeclarationVersion {
		return messagingValuePointer(row, messagingToAppDeclarationVersion)
	}), result1
}
func (p announcementAccessBinding) ReadOpenDeclarations(ctx context.Context, accountID int64, tenantIDs []int64) (map[int64]*time.Time, error) {
	result0, result1 := p.callbacks.ReadOpenDeclarations(ctx, accountID, tenantIDs)
	return result0, result1
}
func (p announcementAccessBinding) HoldDeclarationSigner(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*messaging.DeclarationSignerContext, error) {
	result0, result1 := p.callbacks.HoldDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
	return messagingValuePointer(result0, messagingToAppDeclarationSignerContext), result1
}
func (p announcementAccessBinding) HoldHistoricalDeclarationSigner(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*messaging.DeclarationSignerContext, error) {
	result0, result1 := p.callbacks.HoldHistoricalDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
	return messagingValuePointer(result0, messagingToAppDeclarationSignerContext), result1
}
func (p announcementAccessBinding) InsertDeclarationSubmission(ctx context.Context, submission *messaging.DeclarationSubmission) error {
	nativeSubmission := messagingValuePointer(submission, messagingFromAppDeclarationSubmission)
	result0 := p.callbacks.InsertDeclarationSubmission(ctx, nativeSubmission)
	if submission != nil && nativeSubmission != nil {
		*submission = messagingToAppDeclarationSubmission(*nativeSubmission)
	}
	return result0
}
func (p announcementAccessBinding) LockDeclarationChild(ctx context.Context, announcementID int64, studentID int64) error {
	result0 := p.callbacks.LockDeclarationChild(ctx, announcementID, studentID)
	return result0
}
