package parentportal

import (
	"context"
)

func (p *Portal) ListMessageThreads(ctx context.Context, accountID int64) ([]*InboxThread, error) {
	return p.MessagingFlows.ListMessageThreads(ctx, accountID)
}
func (p *Portal) ListChildThreads(ctx context.Context, accountID, studentID int64) ([]*InboxThread, error) {
	return p.MessagingFlows.ListChildThreads(ctx, accountID, studentID)
}
func (p *Portal) GetChildConversation(ctx context.Context, accountID, studentID int64) (*MessageThreadView, error) {
	return p.MessagingFlows.GetChildConversation(ctx, accountID, studentID)
}
func (p *Portal) PostChildMessage(ctx context.Context, accountID, studentID int64, body string) (*MessageThreadView, error) {
	return p.MessagingFlows.PostChildMessage(ctx, accountID, studentID, body, 0)
}
func (p *Portal) ListAnnouncements(ctx context.Context, accountID int64) ([]*AnnouncementFeedItem, error) {
	return p.MessagingFlows.ListAnnouncements(ctx, accountID)
}
func (p *Portal) SubmitDeclaration(ctx context.Context, accountID, announcementID int64, input DeclarationInput, confirm PasswordConfirmer) (*DeclarationSubmission, bool, error) {
	v, created, err := p.MessagingFlows.SubmitDeclaration(ctx, accountID, announcementID, input, confirm)
	return v, created, err
}
func (p *Portal) DeclarationProof(ctx context.Context, accountID, announcementID, studentID int64) (*DeclarationProof, error) {
	return p.MessagingFlows.DeclarationProof(ctx, accountID, announcementID, studentID)
}
