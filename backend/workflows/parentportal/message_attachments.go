package parentportal

import (
	"context"

	"github.com/moto-ogs/project-phoenix/workflows/parentportal/internal/messaging"
)

// Photos and files in the guardian's conversations (#6685). The root binds
// the port to Communication's attachment store.
type (
	MessageAttachments      = messaging.MessageAttachments
	MessageAttachment       = messaging.MessageAttachment
	MessageAttachmentUpload = messaging.MessageAttachmentUpload
	StoredMessageAttachment = messaging.StoredMessageAttachment
)

var (
	ErrMessageAttachmentNotFound    = messaging.ErrMessageAttachmentNotFound
	ErrMessageAttachmentUnavailable = messaging.ErrMessageAttachmentUnavailable
	ErrMessageAttachmentLimit       = messaging.ErrMessageAttachmentLimit
)

// WithMessageAttachmentIDs names the uploads a guardian send carries without
// granting access to them.
func WithMessageAttachmentIDs(ctx context.Context, ids []int64) context.Context {
	return messaging.WithMessageAttachmentIDs(ctx, ids)
}

// MessageAttachmentIDs returns the uploads the send in ctx names; the root
// hands them to Communication with the message.
func MessageAttachmentIDs(ctx context.Context) []int64 {
	return messaging.MessageAttachmentIDs(ctx)
}

func (p *Portal) PrepareChildMessageAttachment(ctx context.Context, accountID, studentID int64, upload MessageAttachmentUpload) (*StoredMessageAttachment, error) {
	return p.MessagingFlows.PrepareChildMessageAttachment(ctx, accountID, studentID, upload)
}

func (p *Portal) DiscardChildMessageAttachment(ctx context.Context, accountID, studentID, attachmentID int64) error {
	return p.MessagingFlows.DiscardChildMessageAttachment(ctx, accountID, studentID, attachmentID)
}

func (p *Portal) OpenChildMessageAttachment(ctx context.Context, accountID, studentID, attachmentID int64) (*StoredMessageAttachment, error) {
	return p.MessagingFlows.OpenChildMessageAttachment(ctx, accountID, studentID, attachmentID)
}
