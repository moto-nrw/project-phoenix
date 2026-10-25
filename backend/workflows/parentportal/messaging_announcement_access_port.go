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
	return p.callbacks.AccountMatchesAnnouncement(ctx, tenantID, announcementID, accountID)
}
func (p announcementAccessBinding) AccountMayAnswerForStudent(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (bool, error) {
	return p.callbacks.AccountMayAnswerForStudent(ctx, tenantID, announcementID, accountID, studentID)
}
func (p announcementAccessBinding) AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*messaging.AnnouncementPollChild, error) {
	return p.callbacks.AnswerableChildren(ctx, accountID, announcementIDs)
}
func (p announcementAccessBinding) CountUnreadForAccount(ctx context.Context, accountID int64, scope messaging.AnnouncementFeedScope) (int, error) {
	return p.callbacks.CountUnreadForAccount(ctx, accountID, scope)
}
func (p announcementAccessBinding) FindByID(ctx context.Context, id int64) (*messaging.ParentAnnouncement, error) {
	return p.callbacks.FindByID(ctx, id)
}
func (p announcementAccessBinding) ListFeedForAccount(ctx context.Context, accountID int64, scope messaging.AnnouncementFeedScope) ([]*messaging.AnnouncementFeedItem, error) {
	return p.callbacks.ListFeedForAccount(ctx, accountID, scope)
}
func (p announcementAccessBinding) ListOptions(ctx context.Context, announcementID int64) ([]*messaging.ParentAnnouncementOption, error) {
	return p.callbacks.ListOptions(ctx, announcementID)
}
func (p announcementAccessBinding) ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*messaging.ParentAnnouncementOption, error) {
	return p.callbacks.ListOptionsForAnnouncements(ctx, announcementIDs)
}
func (p announcementAccessBinding) MarkAcknowledged(ctx context.Context, tenantID int64, announcementID int64, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	return p.callbacks.MarkAcknowledged(ctx, tenantID, announcementID, accountID, expectedPublishedAt)
}
func (p announcementAccessBinding) MarkRead(ctx context.Context, tenantID int64, announcementID int64, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	return p.callbacks.MarkRead(ctx, tenantID, announcementID, accountID, expectedPublishedAt)
}
func (p announcementAccessBinding) SetResponse(ctx context.Context, tenantID int64, announcementID int64, studentID int64, accountID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error) {
	return p.callbacks.SetResponse(ctx, tenantID, announcementID, studentID, accountID, optionIDs, expectedPublishedAt)
}
func (p announcementAccessBinding) DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64) ([]*messaging.DeclarationChild, error) {
	return p.callbacks.DeclarationChildrenForAccount(ctx, accountID, announcementIDs)
}
func (p announcementAccessBinding) DeclarationSettings(ctx context.Context, announcementIDs []int64) (map[int64]messaging.AnnouncementDeclarationSettings, error) {
	return p.callbacks.DeclarationSettings(ctx, announcementIDs)
}
func (p announcementAccessBinding) DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*messaging.DeclarationSigner, error) {
	return p.callbacks.DeclarationSignersForStudents(ctx, studentIDs)
}
func (p announcementAccessBinding) LatestDeclarationVersion(ctx context.Context, tenantID int64, announcementID int64) (*messaging.DeclarationVersion, error) {
	return p.callbacks.LatestDeclarationVersion(ctx, tenantID, announcementID)
}
func (p announcementAccessBinding) LatestDeclarationVersions(ctx context.Context, announcementIDs []int64) (map[int64]*messaging.DeclarationVersion, error) {
	return p.callbacks.LatestDeclarationVersions(ctx, announcementIDs)
}
func (p announcementAccessBinding) ListDeclarationSubmissionsForStudents(ctx context.Context, announcementIDs []int64, studentIDs []int64) ([]*messaging.DeclarationSubmission, error) {
	return p.callbacks.ListDeclarationSubmissionsForStudents(ctx, announcementIDs, studentIDs)
}
func (p announcementAccessBinding) ListDeclarationVersions(ctx context.Context, tenantID int64, announcementID int64) ([]*messaging.DeclarationVersion, error) {
	return p.callbacks.ListDeclarationVersions(ctx, tenantID, announcementID)
}
func (p announcementAccessBinding) ReadOpenDeclarations(ctx context.Context, accountID int64, tenantIDs []int64) (map[int64]*time.Time, error) {
	return p.callbacks.ReadOpenDeclarations(ctx, accountID, tenantIDs)
}
func (p announcementAccessBinding) HoldDeclarationSigner(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*messaging.DeclarationSignerContext, error) {
	return p.callbacks.HoldDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
}
func (p announcementAccessBinding) HoldHistoricalDeclarationSigner(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*messaging.DeclarationSignerContext, error) {
	return p.callbacks.HoldHistoricalDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
}
func (p announcementAccessBinding) InsertDeclarationSubmission(ctx context.Context, submission *messaging.DeclarationSubmission) error {
	return p.callbacks.InsertDeclarationSubmission(ctx, submission)
}
func (p announcementAccessBinding) LockDeclarationChild(ctx context.Context, announcementID int64, studentID int64) error {
	result0 := p.callbacks.LockDeclarationChild(ctx, announcementID, studentID)
	return result0
}
