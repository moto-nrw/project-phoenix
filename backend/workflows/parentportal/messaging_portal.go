package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

func (p *Portal) ListMessageThreads(ctx context.Context, accountID int64) ([]*InboxThread, error) {
	v, err := p.MessagingFlows.ListMessageThreads(ctx, accountID)
	return messagingValueSlice(v, func(row *messaging.InboxThread) *InboxThread {
		return messagingValuePointer(row, messagingFromAppInboxThread)
	}), err
}
func (p *Portal) ListChildThreads(ctx context.Context, accountID, studentID int64) ([]*InboxThread, error) {
	v, err := p.MessagingFlows.ListChildThreads(ctx, accountID, studentID)
	return messagingValueSlice(v, func(row *messaging.InboxThread) *InboxThread {
		return messagingValuePointer(row, messagingFromAppInboxThread)
	}), err
}
func (p *Portal) GetChildConversation(ctx context.Context, accountID, studentID int64) (*MessageThreadView, error) {
	v, err := p.MessagingFlows.GetChildConversation(ctx, accountID, studentID)
	return messagingValuePointer(v, messagingFromAppMessageThreadView), err
}
func (p *Portal) PostChildMessage(ctx context.Context, accountID, studentID int64, body string) (*MessageThreadView, error) {
	v, err := p.MessagingFlows.PostChildMessage(ctx, accountID, studentID, body)
	return messagingValuePointer(v, messagingFromAppMessageThreadView), err
}
func (p *Portal) ListAnnouncements(ctx context.Context, accountID int64) ([]*AnnouncementFeedItem, error) {
	v, err := p.MessagingFlows.ListAnnouncements(ctx, accountID)
	return messagingValueSlice(v, func(row *messaging.AnnouncementFeedItem) *AnnouncementFeedItem {
		return messagingValuePointer(row, messagingFromAppAnnouncementFeedItem)
	}), err
}
func (p *Portal) SubmitDeclaration(ctx context.Context, accountID, announcementID int64, input DeclarationInput, confirm PasswordConfirmer) (*DeclarationSubmission, bool, error) {
	v, created, err := p.MessagingFlows.SubmitDeclaration(ctx, accountID, announcementID, input, confirm)
	return messagingValuePointer(v, messagingFromAppDeclarationSubmission), created, err
}
func (p *Portal) DeclarationProof(ctx context.Context, accountID, announcementID, studentID int64) (*DeclarationProof, error) {
	v, err := p.MessagingFlows.DeclarationProof(ctx, accountID, announcementID, studentID)
	return messagingValuePointer(v, messagingFromAppDeclarationProof), err
}
