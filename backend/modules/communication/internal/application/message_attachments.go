package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// The attachment vocabulary the messengers share (#6685).
type (
	MessageAttachmentUpload   = communication.MessageAttachmentUpload
	PreparedMessageAttachment = communication.PreparedMessageAttachment
	MessageAttachmentFile     = communication.MessageAttachmentFile
	MessageAttachmentRecord   = communication.MessageAttachmentRecord
)

const (
	MessageAttachmentScopeParent = communication.MessageAttachmentScopeParent
	MessageAttachmentScopeStaff  = communication.MessageAttachmentScopeStaff
)

// ErrMessageAttachmentNotFound hides a file the caller may not open.
var ErrMessageAttachmentNotFound = communication.ErrMessageAttachmentNotFound

// MessageAttachmentStore is the persistence of the photos and files of chat
// messages (#6685). Every method runs in the caller's tenant transaction.
type MessageAttachmentStore interface {
	Insert(ctx context.Context, scope string, accountID int64, upload communication.MessageAttachmentUpload) (*communication.MessageAttachmentRecord, error)
	CountUnclaimed(ctx context.Context, accountID int64) (int, error)
	DeleteUnclaimed(ctx context.Context, accountID, attachmentID int64) error
	Find(ctx context.Context, scope string, attachmentID int64) (*communication.MessageAttachmentRecord, error)
	FindForGuardian(ctx context.Context, accountID, studentID, attachmentID int64) (*communication.MessageAttachmentRecord, error)
}

// ErrMessageAttachmentsNotConfigured reports a messenger built without the
// attachment store.
var ErrMessageAttachmentsNotConfigured = errors.New("communication: message attachments are not configured")

const maxAttachmentFilenameRunes = 120

// PrepareMessageAttachment stores the unclaimed row of one validated upload
// for the account. The caller has authorized the account for the chat family
// and writes the bytes after this transaction commits: a crash in between
// leaves an expiring row, never bytes no row points at.
func PrepareMessageAttachment(ctx context.Context, store MessageAttachmentStore, scope string, accountID int64, upload communication.MessageAttachmentUpload) (*communication.PreparedMessageAttachment, error) {
	if store == nil {
		return nil, ErrMessageAttachmentsNotConfigured
	}
	if accountID <= 0 || upload.SizeBytes <= 0 || upload.SizeBytes > communication.MaxMessageAttachmentBytes {
		return nil, communication.ErrMessageAttachmentUnavailable
	}
	held, err := store.CountUnclaimed(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if held >= communication.MaxUnclaimedMessageAttachments {
		return nil, communication.ErrMessageAttachmentLimit
	}
	if !strings.HasPrefix(upload.Extension, ".") || strings.ContainsAny(upload.Extension, "/\\") {
		return nil, fmt.Errorf("message attachment: unsupported extension %q", upload.Extension)
	}
	upload.Filename = attachmentDisplayName(upload.Filename, upload.Extension)
	record, err := store.Insert(ctx, scope, accountID, upload)
	if err != nil {
		return nil, err
	}
	return &communication.PreparedMessageAttachment{Attachment: record.Attachment, TenantID: record.TenantID, StoredName: record.StoredName}, nil
}

// DiscardMessageAttachment removes an upload the account did not send, for
// example because its bytes could not be written. A sent or foreign upload
// stays.
func DiscardMessageAttachment(ctx context.Context, store MessageAttachmentStore, accountID, attachmentID int64) error {
	if store == nil {
		return ErrMessageAttachmentsNotConfigured
	}
	return store.DeleteUnclaimed(ctx, accountID, attachmentID)
}

// MessageAttachmentFileOf locates the bytes of a record the caller may open.
func MessageAttachmentFileOf(record *communication.MessageAttachmentRecord) *communication.MessageAttachmentFile {
	return &communication.MessageAttachmentFile{Attachment: record.Attachment, TenantID: record.TenantID, StoredName: record.StoredName}
}

// FindMessageAttachment loads an attachment of the chat family for a reader.
// An unsent upload answers its uploader at once (done) and nobody else; for
// a sent file the caller still authorizes the reader for the conversation.
func FindMessageAttachment(ctx context.Context, store MessageAttachmentStore, scope string, accountID, attachmentID int64) (record *communication.MessageAttachmentRecord, done bool, err error) {
	if store == nil {
		return nil, false, ErrMessageAttachmentsNotConfigured
	}
	record, err = store.Find(ctx, scope, attachmentID)
	if err != nil {
		return nil, false, err
	}
	if record == nil || (!record.Claimed && record.UploadedByAccountID != accountID) {
		return nil, false, communication.ErrMessageAttachmentNotFound
	}
	return record, !record.Claimed, nil
}

// attachmentDisplayName keeps the sender's file name for the readers,
// bounded and without path separators.
func attachmentDisplayName(name, extension string) string {
	name = strings.TrimSpace(strings.NewReplacer("/", "_", "\\", "_").Replace(name))
	if runes := []rune(name); len(runes) > maxAttachmentFilenameRunes {
		name = string(runes[:maxAttachmentFilenameRunes])
	}
	if name == "" {
		name = "Datei" + extension
	}
	return name
}

// PortalMessageAttachments is the guardian's side of the attachment store.
type PortalMessageAttachments struct{ Store MessageAttachmentStore }

var _ communication.PortalMessageAttachments = PortalMessageAttachments{}

func (p PortalMessageAttachments) PrepareGuardianAttachment(ctx context.Context, accountID int64, upload communication.MessageAttachmentUpload) (*communication.PreparedMessageAttachment, error) {
	return PrepareMessageAttachment(ctx, p.Store, communication.MessageAttachmentScopeParent, accountID, upload)
}

func (p PortalMessageAttachments) DiscardGuardianAttachment(ctx context.Context, accountID, attachmentID int64) error {
	return DiscardMessageAttachment(ctx, p.Store, accountID, attachmentID)
}

func (p PortalMessageAttachments) OpenGuardianAttachment(ctx context.Context, accountID, studentID, attachmentID int64) (*communication.MessageAttachmentFile, error) {
	return openGuardianAttachment(ctx, p.Store, accountID, studentID, attachmentID)
}

// openGuardianAttachment admits a file only through a conversation of the
// guardian about the child; everything else answers as a missing file.
func openGuardianAttachment(ctx context.Context, store MessageAttachmentStore, accountID, studentID, attachmentID int64) (*communication.MessageAttachmentFile, error) {
	if store == nil {
		return nil, ErrMessageAttachmentsNotConfigured
	}
	record, err := store.FindForGuardian(ctx, accountID, studentID, attachmentID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, communication.ErrMessageAttachmentNotFound
	}
	return MessageAttachmentFileOf(record), nil
}

// MessageAttachmentCleanupStore is the cleanup side of the attachment store.
type MessageAttachmentCleanupStore interface {
	PendingMessageAttachmentCleanups(ctx context.Context, limit int) ([]communication.MessageAttachmentCleanupIntent, error)
	CompleteMessageAttachmentCleanup(ctx context.Context, intentID int64, at time.Time) error
}

const messageAttachmentSweepBatchSize = 200

// MessageAttachmentSweep removes the objects of deleted attachments.
type MessageAttachmentSweep struct {
	Store  MessageAttachmentCleanupStore
	Now    func() time.Time
	Logger *slog.Logger
}

var _ communication.MessageAttachmentCleanup = MessageAttachmentSweep{}

// SweepMessageAttachmentObjects removes the objects of the school's pending
// intents in the caller's tenant transaction. The intents stay locked until
// it commits, so two passes never race on one object.
func (s MessageAttachmentSweep) SweepMessageAttachmentObjects(ctx context.Context, remove func(context.Context, int64, string) error) (int, error) {
	if s.Store == nil || remove == nil {
		return 0, ErrMessageAttachmentsNotConfigured
	}
	intents, err := s.Store.PendingMessageAttachmentCleanups(ctx, messageAttachmentSweepBatchSize)
	if err != nil {
		return 0, err
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	var removed int
	var failures []error
	for _, intent := range intents {
		if err := remove(ctx, intent.TenantID, intent.StoredName); err != nil {
			failures = append(failures, fmt.Errorf("remove message attachment object: %w", err))
			continue
		}
		if err := s.Store.CompleteMessageAttachmentCleanup(ctx, intent.ID, now()); err != nil {
			return removed, errors.Join(append(failures, err)...)
		}
		removed++
	}
	if len(intents) >= messageAttachmentSweepBatchSize && s.Logger != nil {
		s.Logger.Info("message attachment cleanup batch full, more pending",
			slog.Int("batch_size", messageAttachmentSweepBatchSize))
	}
	return removed, errors.Join(failures...)
}
