package messaging

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/workflows/parentportal/internal/care"
)

// Photos and files in the guardian's conversations (#6685). A guardian
// uploads for a child they may write about and reads a file only through a
// conversation of their own about that child: a message of the 1:1
// conversation, or a group message that reached them for the child.

// MessageAttachment is one file of a message as the guardian sees it.
type MessageAttachment struct {
	ID          int64  `json:"id,string"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

// MessageAttachmentUpload is one validated file on its way into the store.
type MessageAttachmentUpload struct {
	Filename    string
	ContentType string
	Extension   string
	SizeBytes   int64
	Width       int
	Height      int
}

// StoredMessageAttachment locates the bytes of an attachment in its school.
type StoredMessageAttachment struct {
	Attachment MessageAttachment
	TenantID   int64
	StoredName string
}

var (
	// ErrMessageAttachmentNotFound hides a file the guardian may not read
	// behind the answer for one that does not exist.
	ErrMessageAttachmentNotFound = errors.New("message attachment not found")
	// ErrMessageAttachmentUnavailable refuses a send that names an upload
	// the guardian does not hold or another message already carries.
	ErrMessageAttachmentUnavailable = errors.New("message attachment is not available for this message")
	// ErrMessageAttachmentLimit refuses more files than one message or the
	// guardian's unsent uploads may hold.
	ErrMessageAttachmentLimit = errors.New("message attachment limit reached")
)

// MessageAttachments is the consumer-owned port to Communication's
// attachment store. Every method runs in the child's tenant transaction.
type MessageAttachments interface {
	PrepareGuardianAttachment(ctx context.Context, accountID int64, upload MessageAttachmentUpload) (*StoredMessageAttachment, error)
	DiscardGuardianAttachment(ctx context.Context, accountID, attachmentID int64) error
	OpenGuardianAttachment(ctx context.Context, accountID, studentID, attachmentID int64) (*StoredMessageAttachment, error)
}

type messageAttachmentIDsKey struct{}

// WithMessageAttachmentIDs carries the uploads a send names; the store
// claims only unclaimed uploads of the sending guardian.
func WithMessageAttachmentIDs(ctx context.Context, ids []int64) context.Context {
	if len(ids) == 0 {
		return ctx
	}
	return context.WithValue(ctx, messageAttachmentIDsKey{}, append([]int64(nil), ids...))
}

// MessageAttachmentIDs returns the uploads the send in ctx names.
func MessageAttachmentIDs(ctx context.Context) []int64 {
	ids, _ := ctx.Value(messageAttachmentIDsKey{}).([]int64)
	return ids
}

// PrepareChildMessageAttachment stores the row of one upload for a message
// about the child. Uploading is a write about the child, so it needs
// parent_portal.notes.write and runs behind the child write guard; the caller
// writes the bytes after this unit committed.
func (s *Service) PrepareChildMessageAttachment(ctx context.Context, accountID, studentID int64, upload MessageAttachmentUpload) (*StoredMessageAttachment, error) {
	child, err := s.attachmentWriteChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	var stored *StoredMessageAttachment
	err = s.Children.WithChildWrite(ctx, accountID, studentID, child, care.ChildWriteAccess{Permissions: []string{care.GuardianPermissionNotesWrite}}, func(txCtx context.Context) error {
		var prepareErr error
		stored, prepareErr = s.MessageAttachments.PrepareGuardianAttachment(txCtx, accountID, upload)
		return prepareErr
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

// DiscardChildMessageAttachment removes an upload whose bytes could not be
// written.
func (s *Service) DiscardChildMessageAttachment(ctx context.Context, accountID, studentID, attachmentID int64) error {
	child, err := s.attachmentWriteChild(ctx, accountID, studentID)
	if err != nil {
		return err
	}
	return s.Children.WithChildWrite(ctx, accountID, studentID, child, care.ChildWriteAccess{Permissions: []string{care.GuardianPermissionNotesWrite}}, func(txCtx context.Context) error {
		return s.MessageAttachments.DiscardGuardianAttachment(txCtx, accountID, attachmentID)
	})
}

// attachmentWriteChild is the gate of PostChildMessage without a message:
// the write permission for the child and the school's messaging switch.
func (s *Service) attachmentWriteChild(ctx context.Context, accountID, studentID int64) (*Child, error) {
	if s.MessageAttachments == nil {
		return nil, errors.New("parent: message attachments are not configured")
	}
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, care.GuardianPermissionNotesWrite)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if !s.MessagingEnabled(ctx, child.TenantID) {
		return nil, care.ErrNotesDisabled
	}
	return child, nil
}

// OpenChildMessageAttachment locates a file of the guardian's conversations
// about the child. Reading follows the conversation: parent_portal.access is
// enough, as for the messages themselves.
func (s *Service) OpenChildMessageAttachment(ctx context.Context, accountID, studentID, attachmentID int64) (*StoredMessageAttachment, error) {
	if s.MessageAttachments == nil {
		return nil, errors.New("parent: message attachments are not configured")
	}
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	var stored *StoredMessageAttachment
	txErr := s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		var openErr error
		stored, openErr = s.MessageAttachments.OpenGuardianAttachment(txCtx, accountID, studentID, attachmentID)
		return openErr
	})
	if txErr != nil {
		if errors.Is(txErr, ErrMessageAttachmentNotFound) {
			return nil, ErrMessageAttachmentNotFound
		}
		return nil, fmt.Errorf("parent: open message attachment: %w", txErr)
	}
	return stored, nil
}
