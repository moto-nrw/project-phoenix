package messaging

import (
	"context"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

// Photos and files in the conversations with guardians (#6685). An upload
// stays with the staff account that made it until a message claims it; a
// sent file is read by whoever reads its conversation, under the same rules
// as the text beside it.

// PrepareMessageAttachment stores the row of one upload for the caller. Every
// account that may write to guardians may upload; which conversation the file
// goes to is decided when the message is sent.
func (s *Service) PrepareMessageAttachment(ctx context.Context, upload deliveryPorts.MessageAttachmentUpload) (*deliveryPorts.PreparedMessageAttachment, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return nil, deliveryPorts.MapParentMessagingError(err, NativeFacadeErrors())
	}
	return deliveryPorts.PrepareMessageAttachment(ctx, s.Attachments, deliveryPorts.MessageAttachmentScopeParent, s.accountID(ctx), upload)
}

// DiscardMessageAttachment removes an upload the caller did not send.
func (s *Service) DiscardMessageAttachment(ctx context.Context, attachmentID int64) error {
	return deliveryPorts.DiscardMessageAttachment(ctx, s.Attachments, s.accountID(ctx), attachmentID)
}

// OpenMessageAttachment locates a file for a staff reader. Every refusal
// answers as a missing file, so the route tells nobody which conversations
// exist.
func (s *Service) OpenMessageAttachment(ctx context.Context, attachmentID int64) (*deliveryPorts.MessageAttachmentFile, error) {
	record, done, err := deliveryPorts.FindMessageAttachment(ctx, s.Attachments, deliveryPorts.MessageAttachmentScopeParent, s.accountID(ctx), attachmentID)
	if err != nil {
		return nil, err
	}
	if !done {
		if record.GroupThread {
			_, err = s.loadGroupThread(ctx, record.ThreadID, false)
		} else {
			_, err = s.loadAuthorizedThread(ctx, record.ThreadID)
		}
		if err != nil {
			return nil, deliveryPorts.ErrMessageAttachmentNotFound
		}
	}
	return deliveryPorts.MessageAttachmentFileOf(record), nil
}
