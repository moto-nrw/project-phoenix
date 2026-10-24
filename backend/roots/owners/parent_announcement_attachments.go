package owners

import (
	"context"
	"errors"

	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// ParentAttachmentReads are File Storage's tenant-scoped attachment reads the
// parents portal's attachment entry goes through.
type ParentAttachmentReads = parentportal.AttachmentFiles[filestorageModule.Attachment, filestorageModule.Content]

// ParentGuardianAttachments is the parents portal's attachment entry for File
// Storage's parent download routes (#2890, #4825): the workflow decides the
// audience and reads through File Storage's tenant-scoped capability; this
// binding only answers the entry's refusal with File Storage's not-found.
type ParentGuardianAttachments struct {
	entry *parentportal.AnnouncementAttachments[filestorageModule.Attachment, filestorageModule.Content]
}

// NewParentGuardianAttachments binds the portal's audience gate to the file
// store.
func NewParentGuardianAttachments(audience parentportal.AttachmentAudience, files ParentAttachmentReads) (ParentGuardianAttachments, error) {
	entry, err := parentportal.NewAnnouncementAttachments(audience, files)
	if err != nil {
		return ParentGuardianAttachments{}, err
	}
	return ParentGuardianAttachments{entry: entry}, nil
}

func (a ParentGuardianAttachments) ListGuardianAttachments(ctx context.Context, accountID, announcementID int64) ([]filestorageModule.Attachment, error) {
	attachments, err := a.entry.ListGuardianAttachments(ctx, accountID, announcementID)
	return attachments, fileAttachmentError(err)
}

func (a ParentGuardianAttachments) OpenGuardianAttachment(ctx context.Context, accountID, announcementID, attachmentID int64) (filestorageModule.Content, error) {
	content, err := a.entry.OpenGuardianAttachment(ctx, accountID, announcementID, attachmentID)
	return content, fileAttachmentError(err)
}

func (a ParentGuardianAttachments) OpenGuardianDeclarationProofAttachment(ctx context.Context, accountID, announcementID, studentID, attachmentID int64) (filestorageModule.Content, error) {
	content, err := a.entry.OpenGuardianDeclarationProofAttachment(ctx, accountID, announcementID, studentID, attachmentID)
	return content, fileAttachmentError(err)
}

// fileAttachmentError answers the entry's refusal with File Storage's
// not-found, which the download routes render as 404.
func fileAttachmentError(err error) error {
	if errors.Is(err, parentportal.ErrAttachmentNotFound) {
		return filestorageModule.ErrAttachmentNotFound
	}
	return err
}
