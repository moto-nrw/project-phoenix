package owners

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/communication"
	parentportal "github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

// BindParentMessageAttachments binds the parents portal's attachment port to
// Communication's guardian side of the photos and files of messages (#6685).
func BindParentMessageAttachments(source communication.PortalMessageAttachments) parentportal.MessageAttachments {
	if source == nil {
		return nil
	}
	return parentMessageAttachments{source: source}
}

type parentMessageAttachments struct {
	source communication.PortalMessageAttachments
}

func (p parentMessageAttachments) PrepareGuardianAttachment(ctx context.Context, accountID int64, upload parentportal.MessageAttachmentUpload) (*parentportal.StoredMessageAttachment, error) {
	prepared, err := p.source.PrepareGuardianAttachment(ctx, accountID, communication.MessageAttachmentUpload(upload))
	if err != nil {
		return nil, parentMessageAttachmentError(err)
	}
	return &parentportal.StoredMessageAttachment{
		Attachment: parentportal.MessageAttachment(prepared.Attachment), TenantID: prepared.TenantID, StoredName: prepared.StoredName,
	}, nil
}

func (p parentMessageAttachments) DiscardGuardianAttachment(ctx context.Context, accountID, attachmentID int64) error {
	return parentMessageAttachmentError(p.source.DiscardGuardianAttachment(ctx, accountID, attachmentID))
}

func (p parentMessageAttachments) OpenGuardianAttachment(ctx context.Context, accountID, studentID, attachmentID int64) (*parentportal.StoredMessageAttachment, error) {
	file, err := p.source.OpenGuardianAttachment(ctx, accountID, studentID, attachmentID)
	if err != nil {
		return nil, parentMessageAttachmentError(err)
	}
	return &parentportal.StoredMessageAttachment{
		Attachment: parentportal.MessageAttachment(file.Attachment), TenantID: file.TenantID, StoredName: file.StoredName,
	}, nil
}

// parentMessageAttachmentError translates the store's refusals into the
// parents portal's own, so its routes never name a Communication error.
func parentMessageAttachmentError(err error) error {
	switch {
	case errors.Is(err, communication.ErrMessageAttachmentNotFound):
		return parentportal.ErrMessageAttachmentNotFound
	case errors.Is(err, communication.ErrMessageAttachmentUnavailable):
		return parentportal.ErrMessageAttachmentUnavailable
	case errors.Is(err, communication.ErrMessageAttachmentLimit):
		return parentportal.ErrMessageAttachmentLimit
	default:
		return err
	}
}
