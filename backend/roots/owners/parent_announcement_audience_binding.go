package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentaudience "github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"
)

type projectionAnnouncementAudienceReads interface {
	projectionDeclarationAudienceReads
	projectionDeliveryAudienceReads
	projectionFeedAudienceReads
	projectionPollAudienceReads
}

type projectionDeclarationAudienceReads interface {
	DeclarationChildren(ctx context.Context, schoolID, announcementID int64, history []parentaudience.DeclarationHistoryLink) ([]*parentaudience.DeclarationChild, error)
	DeclarationSigners(ctx context.Context, schoolID int64, studentIDs []int64) ([]*parentaudience.DeclarationSigner, error)
	DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*parentaudience.DeclarationSigner, error)
	DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64, history []parentaudience.DeclarationHistoryLink) ([]*parentaudience.DeclarationChild, error)
	HoldDeclarationSigner(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (*parentaudience.DeclarationSignerContext, error)
	HoldHistoricalDeclarationSigner(ctx context.Context, schoolID, accountID, studentID int64) (*parentaudience.DeclarationSignerContext, error)
	HistoricalDeclarationAccess(ctx context.Context, schoolID, accountID int64, history []parentaudience.DeclarationHistoryLink) (bool, error)
	HistoricalDeclarationFeed(ctx context.Context, accountID int64, scope parentaudience.ParentAnnouncementFeedScope, history []parentaudience.DeclarationHistoryLink) ([]*parentaudience.ParentAnnouncementFeedItem, error)
}

type projectionDeliveryAudienceReads interface {
	CountAudience(ctx context.Context, schoolID, announcementID int64) (int, error)
	AccountMatchesAnnouncement(ctx context.Context, schoolID, announcementID, accountID int64) (bool, error)
	ResolveAudienceEmails(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementRecipient, error)
	LetterChildStatuses(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementLetterChildStatus, error)
	ResolveDeliveryRecipients(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementDeliveryRecipient, error)
	AudienceRecipients(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementRecipientStatus, error)
	CountReachableGuardiansForStudents(ctx context.Context, schoolID int64, studentIDs []int64) (int, error)
}

type projectionFeedAudienceReads interface {
	ListFeedForAccount(ctx context.Context, accountID int64, scope parentaudience.ParentAnnouncementFeedScope) ([]*parentaudience.ParentAnnouncementFeedItem, error)
	CountOutstandingForAccount(ctx context.Context, accountID int64, scope parentaudience.ParentAnnouncementFeedScope) (int, error)
	Stats(ctx context.Context, schoolID, announcementID int64) (*parentaudience.ParentAnnouncementStats, error)
}

type projectionPollAudienceReads interface {
	AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*parentaudience.ParentAnnouncementPollChild, error)
	AccountMayAnswerForStudent(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (bool, error)
	HoldAnswerPermission(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (bool, error)
	PollResults(ctx context.Context, schoolID, announcementID int64) (*parentaudience.ParentAnnouncementPollResults, error)
	PollChildren(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementPollChildStatus, error)
	UnansweredReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementReminderRecipient, error)
	UnacknowledgedReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementReminderRecipient, error)
}

type communicationAnnouncementAudience struct {
	source projectionAnnouncementAudienceReads
}

func mapProjectionRows[A, B any](rows []*A, convert func(*A) *B) []*B {
	if rows == nil {
		return nil
	}
	out := make([]*B, 0, len(rows))
	for _, row := range rows {
		out = append(out, convert(row))
	}
	return out
}
func projectionDeclarationHistory(rows []*communication.PortalDeclarationSubmission) []parentaudience.DeclarationHistoryLink {
	links := make([]parentaudience.DeclarationHistoryLink, 0, len(rows))
	seen := make(map[parentaudience.DeclarationHistoryLink]struct{}, len(rows))
	for _, row := range rows {
		link := parentaudience.DeclarationHistoryLink{TenantID: row.TenantID, AnnouncementID: row.AnnouncementID, StudentID: row.StudentID}
		if _, exists := seen[link]; exists {
			continue
		}
		seen[link] = struct{}{}
		links = append(links, link)
	}
	return links
}
func projectionPollResults(row *parentaudience.ParentAnnouncementPollResults) *communication.PortalAnnouncementPollResults {
	if row == nil {
		return nil
	}
	return &communication.PortalAnnouncementPollResults{ChildCount: row.ChildCount, TargetChildCount: row.TargetChildCount, AnsweredCount: row.AnsweredCount, Options: mapProjectionRows(row.Options, func(option *parentaudience.ParentAnnouncementPollOptionResult) *communication.PortalAnnouncementPollOptionResult {
		return (*communication.PortalAnnouncementPollOptionResult)(option)
	})}
}
func (r communicationAnnouncementAudience) DeclarationChildren(ctx context.Context, schoolID, announcementID int64, history []*communication.PortalDeclarationSubmission) ([]*communication.PortalDeclarationChild, error) {
	rows, err := r.source.DeclarationChildren(ctx, schoolID, announcementID, projectionDeclarationHistory(history))
	return mapProjectionRows(rows, func(row *parentaudience.DeclarationChild) *communication.PortalDeclarationChild {
		return (*communication.PortalDeclarationChild)(row)
	}), err
}
func (r communicationAnnouncementAudience) DeclarationSigners(ctx context.Context, schoolID int64, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error) {
	rows, err := r.source.DeclarationSigners(ctx, schoolID, studentIDs)
	return mapProjectionRows(rows, func(row *parentaudience.DeclarationSigner) *communication.PortalDeclarationSigner {
		return (*communication.PortalDeclarationSigner)(row)
	}), err
}
func (r communicationAnnouncementAudience) DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error) {
	rows, err := r.source.DeclarationSignersForStudents(ctx, studentIDs)
	return mapProjectionRows(rows, func(row *parentaudience.DeclarationSigner) *communication.PortalDeclarationSigner {
		return (*communication.PortalDeclarationSigner)(row)
	}), err
}
func (r communicationAnnouncementAudience) DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64, history []*communication.PortalDeclarationSubmission) ([]*communication.PortalDeclarationChild, error) {
	rows, err := r.source.DeclarationChildrenForAccount(ctx, accountID, announcementIDs, projectionDeclarationHistory(history))
	return mapProjectionRows(rows, func(row *parentaudience.DeclarationChild) *communication.PortalDeclarationChild {
		return (*communication.PortalDeclarationChild)(row)
	}), err
}
func (r communicationAnnouncementAudience) HoldDeclarationSigner(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (*communication.PortalDeclarationSignerContext, error) {
	rows, err := r.source.HoldDeclarationSigner(ctx, schoolID, announcementID, accountID, studentID)
	return (*communication.PortalDeclarationSignerContext)(rows), err
}
func (r communicationAnnouncementAudience) HoldHistoricalDeclarationSigner(ctx context.Context, schoolID, accountID, studentID int64) (*communication.PortalDeclarationSignerContext, error) {
	rows, err := r.source.HoldHistoricalDeclarationSigner(ctx, schoolID, accountID, studentID)
	return (*communication.PortalDeclarationSignerContext)(rows), err
}
func (r communicationAnnouncementAudience) HistoricalDeclarationAccess(ctx context.Context, schoolID, accountID int64, history []*communication.PortalDeclarationSubmission) (bool, error) {
	return r.source.HistoricalDeclarationAccess(ctx, schoolID, accountID, projectionDeclarationHistory(history))
}
func (r communicationAnnouncementAudience) HistoricalDeclarationFeed(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope, history []*communication.PortalDeclarationSubmission) ([]*communication.PortalAnnouncementFeedItem, error) {
	rows, err := r.source.HistoricalDeclarationFeed(ctx, accountID, parentaudience.ParentAnnouncementFeedScope{TenantIDs: scope.TenantIDs, SystemOnlyTenantIDs: scope.SystemOnlyTenantIDs}, projectionDeclarationHistory(history))
	return mapProjectionRows(rows, func(row *parentaudience.ParentAnnouncementFeedItem) *communication.PortalAnnouncementFeedItem {
		if row == nil {
			return nil
		}
		return &communication.PortalAnnouncementFeedItem{
			ID: row.ID, TenantID: row.TenantID, Title: row.Title, Body: row.Body, Priority: row.Priority,
			LinkURL: row.LinkURL, RequiresAcknowledgement: row.RequiresAcknowledgement,
			PublishedAt: row.PublishedAt, ExpiresAt: row.ExpiresAt, ResponseType: row.ResponseType,
			DeliveryMode: row.DeliveryMode, ResponseDeadline: row.ResponseDeadline, SystemKind: row.SystemKind,
			ReminderSentAt: row.ReminderSentAt, ReminderText: row.ReminderText, ReadAt: row.ReadAt, AcknowledgedAt: row.AcknowledgedAt,
		}
	}), err
}
func (r communicationAnnouncementAudience) CountAudience(ctx context.Context, schoolID, announcementID int64) (int, error) {
	return r.source.CountAudience(ctx, schoolID, announcementID)
}
func (r communicationAnnouncementAudience) AccountMatchesAnnouncement(ctx context.Context, schoolID, announcementID, accountID int64) (bool, error) {
	return r.source.AccountMatchesAnnouncement(ctx, schoolID, announcementID, accountID)
}
func (r communicationAnnouncementAudience) ResolveAudienceEmails(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementRecipient, error) {
	rows, err := r.source.ResolveAudienceEmails(ctx, schoolID, announcementID)
	return mapProjectionRows(rows, func(row *parentaudience.ParentAnnouncementRecipient) *communication.PortalAnnouncementRecipient {
		return (*communication.PortalAnnouncementRecipient)(row)
	}), err
}
func (r communicationAnnouncementAudience) LetterChildStatuses(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementLetterChildStatus, error) {
	rows, err := r.source.LetterChildStatuses(ctx, schoolID, announcementID)
	return mapProjectionRows(rows, func(row *parentaudience.ParentAnnouncementLetterChildStatus) *communication.PortalAnnouncementLetterChildStatus {
		return (*communication.PortalAnnouncementLetterChildStatus)(row)
	}), err
}
func (r communicationAnnouncementAudience) ResolveDeliveryRecipients(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementDeliveryRecipient, error) {
	rows, err := r.source.ResolveDeliveryRecipients(ctx, schoolID, announcementID)
	return mapProjectionRows(rows, func(row *parentaudience.ParentAnnouncementDeliveryRecipient) *communication.PortalAnnouncementDeliveryRecipient {
		return (*communication.PortalAnnouncementDeliveryRecipient)(row)
	}), err
}
func (r communicationAnnouncementAudience) AudienceRecipients(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementRecipientStatus, error) {
	rows, err := r.source.AudienceRecipients(ctx, schoolID, announcementID)
	return mapProjectionRows(rows, func(row *parentaudience.ParentAnnouncementRecipientStatus) *communication.PortalAnnouncementRecipientStatus {
		return (*communication.PortalAnnouncementRecipientStatus)(row)
	}), err
}
func (r communicationAnnouncementAudience) CountReachableGuardiansForStudents(ctx context.Context, schoolID int64, studentIDs []int64) (int, error) {
	return r.source.CountReachableGuardiansForStudents(ctx, schoolID, studentIDs)
}
func (r communicationAnnouncementAudience) ListFeedForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) ([]*communication.PortalAnnouncementFeedItem, error) {
	rows, err := r.source.ListFeedForAccount(ctx, accountID, parentaudience.ParentAnnouncementFeedScope{TenantIDs: scope.TenantIDs, SystemOnlyTenantIDs: scope.SystemOnlyTenantIDs})
	return mapProjectionRows(rows, func(row *parentaudience.ParentAnnouncementFeedItem) *communication.PortalAnnouncementFeedItem {
		if row == nil {
			return nil
		}
		return &communication.PortalAnnouncementFeedItem{
			ID: row.ID, TenantID: row.TenantID, Title: row.Title, Body: row.Body, Priority: row.Priority,
			LinkURL: row.LinkURL, RequiresAcknowledgement: row.RequiresAcknowledgement,
			PublishedAt: row.PublishedAt, ExpiresAt: row.ExpiresAt, ResponseType: row.ResponseType,
			DeliveryMode: row.DeliveryMode, ResponseDeadline: row.ResponseDeadline, SystemKind: row.SystemKind,
			ReminderSentAt: row.ReminderSentAt, ReminderText: row.ReminderText, ReadAt: row.ReadAt, AcknowledgedAt: row.AcknowledgedAt,
		}
	}), err
}
func (r communicationAnnouncementAudience) CountOutstandingForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) (int, error) {
	return r.source.CountOutstandingForAccount(ctx, accountID, parentaudience.ParentAnnouncementFeedScope{TenantIDs: scope.TenantIDs, SystemOnlyTenantIDs: scope.SystemOnlyTenantIDs})
}
func (r communicationAnnouncementAudience) Stats(ctx context.Context, schoolID, announcementID int64) (*communication.PortalAnnouncementStats, error) {
	rows, err := r.source.Stats(ctx, schoolID, announcementID)
	return (*communication.PortalAnnouncementStats)(rows), err
}
func (r communicationAnnouncementAudience) AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*communication.PortalAnnouncementPollChild, error) {
	rows, err := r.source.AnswerableChildren(ctx, accountID, announcementIDs)
	return mapProjectionRows(rows, func(row *parentaudience.ParentAnnouncementPollChild) *communication.PortalAnnouncementPollChild {
		return (*communication.PortalAnnouncementPollChild)(row)
	}), err
}
func (r communicationAnnouncementAudience) AccountMayAnswerForStudent(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (bool, error) {
	return r.source.AccountMayAnswerForStudent(ctx, schoolID, announcementID, accountID, studentID)
}
func (r communicationAnnouncementAudience) HoldAnswerPermission(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (bool, error) {
	return r.source.HoldAnswerPermission(ctx, schoolID, announcementID, accountID, studentID)
}
func (r communicationAnnouncementAudience) PollResults(ctx context.Context, schoolID, announcementID int64) (*communication.PortalAnnouncementPollResults, error) {
	rows, err := r.source.PollResults(ctx, schoolID, announcementID)
	return projectionPollResults(rows), err
}
func (r communicationAnnouncementAudience) PollChildren(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementPollChildStatus, error) {
	rows, err := r.source.PollChildren(ctx, schoolID, announcementID)
	return mapProjectionRows(rows, func(row *parentaudience.ParentAnnouncementPollChildStatus) *communication.PortalAnnouncementPollChildStatus {
		if row == nil {
			return nil
		}
		return &communication.PortalAnnouncementPollChildStatus{StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass, AnswerLabels: row.AnswerLabels, RespondedAt: row.RespondedAt, CanAnswer: row.CanAnswer}
	}), err
}
func (r communicationAnnouncementAudience) UnansweredReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error) {
	rows, err := r.source.UnansweredReminderRecipients(ctx, schoolID, announcementID)
	return mapProjectionRows(rows, func(row *parentaudience.ParentAnnouncementReminderRecipient) *communication.PortalAnnouncementPollReminderRecipient {
		return (*communication.PortalAnnouncementPollReminderRecipient)(row)
	}), err
}
func (r communicationAnnouncementAudience) UnacknowledgedReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error) {
	rows, err := r.source.UnacknowledgedReminderRecipients(ctx, schoolID, announcementID)
	return mapProjectionRows(rows, func(row *parentaudience.ParentAnnouncementReminderRecipient) *communication.PortalAnnouncementPollReminderRecipient {
		return (*communication.PortalAnnouncementPollReminderRecipient)(row)
	}), err
}
