package owners

import (
	"context"
	"errors"

	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// ParentGuardianAttachments is the parents portal's attachment entry in File
// Storage's HTTP vocabulary (#2890, #4825): the workflow decides the
// audience and reads through File Storage's tenant-scoped capability; this
// binding only translates the values and the refusal.
type ParentGuardianAttachments struct {
	entry *parentportal.AnnouncementAttachments
}

// ParentAttachmentReads are File Storage's tenant-scoped attachment reads the
// entry goes through.
type ParentAttachmentReads interface {
	ListAnnouncementAttachmentsInTenant(ctx context.Context, tenantID, announcementID int64) ([]filestorageModule.Attachment, error)
	FindAnnouncementAttachmentInTenant(ctx context.Context, tenantID, announcementID, attachmentID int64) (filestorageModule.Attachment, error)
	OpenAnnouncementAttachmentInTenant(ctx context.Context, tenantID, announcementID, attachmentID int64) (filestorageModule.Content, error)
}

// NewParentGuardianAttachments binds the portal's audience gate to the file
// store.
func NewParentGuardianAttachments(audience parentportal.AttachmentAudience, files ParentAttachmentReads) (ParentGuardianAttachments, error) {
	if files == nil {
		return ParentGuardianAttachments{}, errors.New("parent attachments: file storage is required")
	}
	entry, err := parentportal.NewAnnouncementAttachments(audience, parentAttachmentFiles{files: files})
	if err != nil {
		return ParentGuardianAttachments{}, err
	}
	return ParentGuardianAttachments{entry: entry}, nil
}

func (a ParentGuardianAttachments) ListGuardianAttachments(ctx context.Context, accountID, announcementID int64) ([]filestorageModule.Attachment, error) {
	values, err := a.entry.ListGuardianAttachments(ctx, accountID, announcementID)
	if err != nil {
		return nil, fileAttachmentError(err)
	}
	out := make([]filestorageModule.Attachment, 0, len(values))
	for _, value := range values {
		out = append(out, filestorageModule.Attachment(value))
	}
	return out, nil
}

func (a ParentGuardianAttachments) OpenGuardianAttachment(ctx context.Context, accountID, announcementID, attachmentID int64) (filestorageModule.Content, error) {
	return fileAttachmentContent(a.entry.OpenGuardianAttachment(ctx, accountID, announcementID, attachmentID))
}

func (a ParentGuardianAttachments) OpenGuardianDeclarationProofAttachment(ctx context.Context, accountID, announcementID, studentID, attachmentID int64) (filestorageModule.Content, error) {
	return fileAttachmentContent(a.entry.OpenGuardianDeclarationProofAttachment(ctx, accountID, announcementID, studentID, attachmentID))
}

func fileAttachmentContent(value parentportal.AttachmentContent, err error) (filestorageModule.Content, error) {
	if err != nil {
		return filestorageModule.Content{}, fileAttachmentError(err)
	}
	return filestorageModule.Content{Object: value.Object, ModTime: value.ModTime, Filename: value.Filename, ContentType: value.ContentType}, nil
}

// fileAttachmentError answers the entry's refusal with File Storage's
// not-found, which the download routes render as 404.
func fileAttachmentError(err error) error {
	if errors.Is(err, parentportal.ErrAttachmentNotFound) {
		return filestorageModule.ErrAttachmentNotFound
	}
	return err
}

// parentAttachmentFiles serves the entry's file port from File Storage.
type parentAttachmentFiles struct {
	files ParentAttachmentReads
}

func (f parentAttachmentFiles) ListAnnouncementAttachmentsInTenant(ctx context.Context, tenantID, announcementID int64) ([]parentportal.Attachment, error) {
	values, err := f.files.ListAnnouncementAttachmentsInTenant(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	out := make([]parentportal.Attachment, 0, len(values))
	for _, value := range values {
		out = append(out, parentportal.Attachment(value))
	}
	return out, nil
}

func (f parentAttachmentFiles) FindAnnouncementAttachmentInTenant(ctx context.Context, tenantID, announcementID, attachmentID int64) (parentportal.Attachment, error) {
	value, err := f.files.FindAnnouncementAttachmentInTenant(ctx, tenantID, announcementID, attachmentID)
	return parentportal.Attachment(value), err
}

func (f parentAttachmentFiles) OpenAnnouncementAttachmentInTenant(ctx context.Context, tenantID, announcementID, attachmentID int64) (parentportal.AttachmentContent, error) {
	value, err := f.files.OpenAnnouncementAttachmentInTenant(ctx, tenantID, announcementID, attachmentID)
	if err != nil {
		return parentportal.AttachmentContent{}, err
	}
	return parentportal.AttachmentContent{Object: value.Object, ModTime: value.ModTime, Filename: value.Filename, ContentType: value.ContentType}, nil
}
