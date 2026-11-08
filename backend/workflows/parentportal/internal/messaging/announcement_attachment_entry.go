package messaging

import (
	"context"
	"errors"
)

// ErrAttachmentNotFound is every refusal of the attachment entry: outside the
// audience, no such announcement or attachment, or a changed school. The
// root maps it onto File Storage's not-found answer.
var ErrAttachmentNotFound = errors.New("parent: announcement attachment not found")

// AttachmentFiles is File Storage's tenant-scoped attachment reads. They check
// nothing but the school they are named; the entry below decides the audience
// before each read and again before any bytes are opened (#2890, #4825). The
// attachment A and the opened content C are File Storage's values; the entry
// hands them out unchanged.
type AttachmentFiles[A, C any] interface {
	ListAnnouncementAttachmentsInTenant(ctx context.Context, tenantID, announcementID int64) ([]A, error)
	FindAnnouncementAttachmentInTenant(ctx context.Context, tenantID, announcementID, attachmentID int64) (A, error)
	OpenAnnouncementAttachmentInTenant(ctx context.Context, tenantID, announcementID, attachmentID int64) (C, error)
}

// AttachmentAudience is the guardian-side gate: the school of a live
// announcement whose audience includes the account, or of an attachment
// frozen in the account's declaration proof. Tenant id 0 with no error means
// "not for you". WithinGuardianAnnouncementTenant is the same announcement
// gate whose school switch shares one tenant transaction with read (ADR 0161
// step 3); read runs only when the gate admits the account.
type AttachmentAudience interface {
	GuardianAnnouncementTenant(ctx context.Context, accountID, announcementID int64) (int64, error)
	WithinGuardianAnnouncementTenant(ctx context.Context, accountID, announcementID int64, read func(ctx context.Context, tenantID int64) error) (int64, error)
	GuardianDeclarationProofAttachmentTenant(ctx context.Context, accountID, announcementID, studentID, attachmentID int64) (int64, error)
}

// AnnouncementAttachments is the guardian's attachment entry. The first
// audience check and the row read run in separate transactions (only the
// school switch shares the row read's), and the announcement may be withdrawn
// in between. The audience is therefore asked a
// second time after the rows were read and before anything is handed out,
// and a different school counts as "not visible". Every refusal is
// ErrAttachmentNotFound, so "outside the audience" and "no such
// announcement" stay indistinguishable.
type AnnouncementAttachments[A, C any] struct {
	audience AttachmentAudience
	files    AttachmentFiles[A, C]
}

// NewAnnouncementAttachments binds the audience gate to File Storage's reads.
func NewAnnouncementAttachments[A, C any](audience AttachmentAudience, files AttachmentFiles[A, C]) (*AnnouncementAttachments[A, C], error) {
	if audience == nil || files == nil {
		return nil, errors.New("parent: attachment audience and file storage are required")
	}
	return &AnnouncementAttachments[A, C]{audience: audience, files: files}, nil
}

// ListGuardianAttachments returns the attachments a guardian account may see.
func (a *AnnouncementAttachments[A, C]) ListGuardianAttachments(ctx context.Context, accountID, announcementID int64) ([]A, error) {
	gate := func(ctx context.Context) (int64, error) { return a.announcementTenant(ctx, accountID, announcementID) }
	var attachments []A
	tenantID, err := a.withinAnnouncementTenant(ctx, accountID, announcementID, func(txCtx context.Context, tenantID int64) error {
		var listErr error
		attachments, listErr = a.files.ListAnnouncementAttachmentsInTenant(txCtx, tenantID, announcementID)
		return listErr
	})
	if err != nil {
		return nil, err
	}
	if err := revalidate(ctx, gate, tenantID); err != nil {
		return nil, err
	}
	return attachments, nil
}

// OpenGuardianAttachment opens one attachment of a live announcement for a
// guardian in its audience, under the announcement's school.
func (a *AnnouncementAttachments[A, C]) OpenGuardianAttachment(ctx context.Context, accountID, announcementID, attachmentID int64) (C, error) {
	var none C
	tenantID, err := a.withinAnnouncementTenant(ctx, accountID, announcementID, func(txCtx context.Context, tenantID int64) error {
		_, findErr := a.files.FindAnnouncementAttachmentInTenant(txCtx, tenantID, announcementID, attachmentID)
		return findErr
	})
	if err != nil {
		return none, err
	}
	return a.handOut(ctx, func(ctx context.Context) (int64, error) {
		return a.announcementTenant(ctx, accountID, announcementID)
	}, tenantID, announcementID, attachmentID)
}

// OpenGuardianDeclarationProofAttachment opens an attachment frozen in the
// account's own declaration proof for the child. Unlike the live-announcement
// path, it stays available after expiry or withdrawal; the proof gate checks
// the relationship, the submission and the frozen attachment together.
func (a *AnnouncementAttachments[A, C]) OpenGuardianDeclarationProofAttachment(ctx context.Context, accountID, announcementID, studentID, attachmentID int64) (C, error) {
	return a.open(ctx, func(ctx context.Context) (int64, error) {
		if accountID <= 0 || announcementID <= 0 || studentID <= 0 || attachmentID <= 0 {
			return 0, ErrAttachmentNotFound
		}
		return visibleTenant(a.audience.GuardianDeclarationProofAttachmentTenant(ctx, accountID, announcementID, studentID, attachmentID))
	}, announcementID, attachmentID)
}

// open reads the attachment row under the gate's school, asks the gate again
// and only then opens the bytes.
func (a *AnnouncementAttachments[A, C]) open(ctx context.Context, gate func(context.Context) (int64, error), announcementID, attachmentID int64) (C, error) {
	var none C
	tenantID, err := gate(ctx)
	if err != nil {
		return none, err
	}
	if _, err := a.files.FindAnnouncementAttachmentInTenant(ctx, tenantID, announcementID, attachmentID); err != nil {
		return none, err
	}
	return a.handOut(ctx, gate, tenantID, announcementID, attachmentID)
}

// handOut asks the gate again and only then opens the bytes.
func (a *AnnouncementAttachments[A, C]) handOut(ctx context.Context, gate func(context.Context) (int64, error), tenantID, announcementID, attachmentID int64) (C, error) {
	var none C
	if err := revalidate(ctx, gate, tenantID); err != nil {
		return none, err
	}
	return a.files.OpenAnnouncementAttachmentInTenant(ctx, tenantID, announcementID, attachmentID)
}

func (a *AnnouncementAttachments[A, C]) announcementTenant(ctx context.Context, accountID, announcementID int64) (int64, error) {
	if accountID <= 0 || announcementID <= 0 {
		return 0, ErrAttachmentNotFound
	}
	return visibleTenant(a.audience.GuardianAnnouncementTenant(ctx, accountID, announcementID))
}

// withinAnnouncementTenant is the announcement gate's first pass: read runs
// in the unit of the school switch, for the school the gate admits.
func (a *AnnouncementAttachments[A, C]) withinAnnouncementTenant(ctx context.Context, accountID, announcementID int64, read func(context.Context, int64) error) (int64, error) {
	if accountID <= 0 || announcementID <= 0 {
		return 0, ErrAttachmentNotFound
	}
	return visibleTenant(a.audience.WithinGuardianAnnouncementTenant(ctx, accountID, announcementID, read))
}

// visibleTenant turns the gate's "0, no error" into the not-found refusal.
func visibleTenant(tenantID int64, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	if tenantID <= 0 {
		return 0, ErrAttachmentNotFound
	}
	return tenantID, nil
}

// revalidate asks the gate a second time; a different school is "not visible".
func revalidate(ctx context.Context, gate func(context.Context) (int64, error), expectedTenantID int64) error {
	tenantID, err := gate(ctx)
	if err != nil {
		return err
	}
	if tenantID != expectedTenantID {
		return ErrAttachmentNotFound
	}
	return nil
}
