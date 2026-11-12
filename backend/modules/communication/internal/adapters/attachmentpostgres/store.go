// Package attachmentpostgres holds Communication's persistence for the
// photos and files of chat messages (#6685): users.message_attachments and
// its cleanup intents in users.message_attachment_cleanup.
//
// The message stores claim the uploads a send names inside their own insert
// (claimable rows in parentpostgres and staffpostgres), so a message and its
// files commit together.
package attachmentpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/communication"
	"github.com/uptrace/bun"
)

// Database resolves the caller's transaction and school.
type Database func(context.Context) (bun.IDB, int64, error)

// Store reads and writes the attachment rows of one school.
type Store struct{ database Database }

// New builds the store over the ambient transaction runtime.
func New(database Database) *Store {
	if database == nil {
		panic("communication attachment store: database runtime is required")
	}
	return &Store{database: database}
}

type attachmentRow struct {
	bun.BaseModel        `bun:"table:users.message_attachments,alias:attachment"`
	ID                   int64      `bun:"id,pk,autoincrement"`
	TenantID             int64      `bun:"tenant_id,notnull"`
	Scope                string     `bun:"scope,notnull"`
	UploadedByAccountID  *int64     `bun:"uploaded_by_account_id"`
	ParentMessageID      *int64     `bun:"parent_message_id"`
	ParentGroupMessageID *int64     `bun:"parent_group_message_id"`
	StaffMessageID       *int64     `bun:"staff_message_id"`
	FilenameDisplay      string     `bun:"filename_display,notnull"`
	FilenameStored       string     `bun:"filename_stored,notnull"`
	SizeBytes            int64      `bun:"size_bytes,notnull"`
	ContentType          string     `bun:"content_type,notnull"`
	Width                *int       `bun:"width"`
	Height               *int       `bun:"height"`
	CreatedAt            time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	ClaimedAt            *time.Time `bun:"claimed_at"`
}

func (r *attachmentRow) attachment() communication.MessageAttachment {
	value := communication.MessageAttachment{ID: r.ID, Filename: r.FilenameDisplay, SizeBytes: r.SizeBytes, ContentType: r.ContentType}
	if r.Width != nil && r.Height != nil {
		value.Width, value.Height = *r.Width, *r.Height
	}
	return value
}

// Insert stores one unclaimed upload under a fresh object name and returns
// its row.
func (s *Store) Insert(ctx context.Context, scope string, accountID int64, upload communication.MessageAttachmentUpload) (*communication.MessageAttachmentRecord, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := &attachmentRow{
		TenantID: tenantID, Scope: scope, UploadedByAccountID: &accountID,
		FilenameDisplay: upload.Filename,
		SizeBytes:       upload.SizeBytes, ContentType: upload.ContentType,
	}
	if upload.Width > 0 && upload.Height > 0 {
		row.Width, row.Height = &upload.Width, &upload.Height
	}
	// The object name is random hex with the detected extension: it cannot
	// collide in practice, carries no traversal payload and leaks nothing of
	// the display name.
	if _, err := db.NewInsert().Model(row).ModelTableExpr("users.message_attachments").
		Value("filename_stored", "replace(gen_random_uuid()::text, '-', '') || ?", upload.Extension).
		Returning("*").Exec(ctx); err != nil {
		return nil, fmt.Errorf("store message attachment: %w", err)
	}
	return &communication.MessageAttachmentRecord{Attachment: row.attachment(), TenantID: row.TenantID, Scope: row.Scope, UploadedByAccountID: accountID, StoredName: row.FilenameStored}, nil
}

// CountUnclaimed counts the uploads the account holds without a message.
func (s *Store) CountUnclaimed(ctx context.Context, accountID int64) (int, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	count, err := db.NewSelect().TableExpr("users.message_attachments AS attachment").
		Where("attachment.tenant_id = ? AND attachment.uploaded_by_account_id = ? AND attachment.claimed_at IS NULL", tenantID, accountID).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count unclaimed message attachments: %w", err)
	}
	return int(count), nil
}

// DeleteUnclaimed removes an upload its account never sent. Its object is
// queued by the delete trigger. A claimed or foreign upload stays.
func (s *Store) DeleteUnclaimed(ctx context.Context, accountID, attachmentID int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewDelete().TableExpr("users.message_attachments AS attachment").
		Where("attachment.tenant_id = ? AND attachment.id = ? AND attachment.uploaded_by_account_id = ? AND attachment.claimed_at IS NULL", tenantID, attachmentID, accountID).
		Exec(ctx); err != nil {
		return fmt.Errorf("discard message attachment: %w", err)
	}
	return nil
}

type recordRow struct {
	attachmentRow    `bun:",extend"`
	ThreadID         *int64     `bun:"thread_id"`
	MessageCreatedAt *time.Time `bun:"message_created_at"`
}

const findRecordSQL = `SELECT attachment.*,
	COALESCE(parent_message.thread_id, group_message.thread_id, staff_message.thread_id) AS thread_id,
	COALESCE(parent_message.created_at, group_message.created_at, staff_message.created_at) AS message_created_at
FROM users.message_attachments AS attachment
LEFT JOIN users.parent_messages AS parent_message
	ON parent_message.id = attachment.parent_message_id AND parent_message.tenant_id = attachment.tenant_id
LEFT JOIN users.parent_group_messages AS group_message
	ON group_message.id = attachment.parent_group_message_id AND group_message.tenant_id = attachment.tenant_id
LEFT JOIN users.staff_messages AS staff_message
	ON staff_message.id = attachment.staff_message_id AND staff_message.tenant_id = attachment.tenant_id
WHERE attachment.tenant_id = ? AND attachment.id = ? AND attachment.scope = ?`

// Find returns the attachment of the chat family with its conversation, or
// nil when the school holds none under the id.
func (s *Store) Find(ctx context.Context, scope string, attachmentID int64) (*communication.MessageAttachmentRecord, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := new(recordRow)
	if err := db.NewRaw(findRecordSQL, tenantID, attachmentID, scope).Scan(ctx, row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find message attachment: %w", err)
	}
	return row.record(), nil
}

func (r *recordRow) record() *communication.MessageAttachmentRecord {
	record := &communication.MessageAttachmentRecord{
		Attachment: r.attachment(), TenantID: r.TenantID, Scope: r.Scope, StoredName: r.FilenameStored,
		Claimed: r.ClaimedAt != nil, GroupThread: r.ParentGroupMessageID != nil,
	}
	if r.UploadedByAccountID != nil {
		record.UploadedByAccountID = *r.UploadedByAccountID
	}
	if r.ThreadID != nil {
		record.ThreadID = *r.ThreadID
	}
	if r.MessageCreatedAt != nil {
		record.MessageCreatedAt = *r.MessageCreatedAt
	}
	return record
}

// findForGuardianSQL admits an attachment only through a conversation of
// this guardian: a message of their 1:1 conversation about this child, a
// group message that reached them, or their own unsent upload. A group
// message reaches a guardian for one or several of their children; which
// one the caller names does not matter, because the caller has authorized
// the guardian for that child and the school is the child's.
const findForGuardianSQL = `SELECT attachment.*
FROM users.message_attachments AS attachment
WHERE attachment.tenant_id = ? AND attachment.id = ? AND attachment.scope = 'parent'
  AND (
	(attachment.claimed_at IS NULL AND attachment.uploaded_by_account_id = ?)
	OR EXISTS (
		SELECT 1 FROM users.parent_messages AS message
		JOIN users.parent_message_threads AS thread
			ON thread.id = message.thread_id AND thread.tenant_id = message.tenant_id
		WHERE message.id = attachment.parent_message_id AND message.tenant_id = attachment.tenant_id
		  AND thread.guardian_account_id = ? AND thread.student_id = ?)
	OR EXISTS (
		SELECT 1 FROM users.parent_group_message_recipients AS recipient
		WHERE recipient.message_id = attachment.parent_group_message_id AND recipient.tenant_id = attachment.tenant_id
		  AND recipient.guardian_account_id = ?)
  )`

// FindForGuardian returns the attachment when the guardian may read it for
// the child, or nil.
func (s *Store) FindForGuardian(ctx context.Context, accountID, studentID, attachmentID int64) (*communication.MessageAttachmentRecord, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := new(recordRow)
	if err := db.NewRaw(findForGuardianSQL, tenantID, attachmentID, accountID, accountID, studentID, accountID).Scan(ctx, row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find guardian message attachment: %w", err)
	}
	return row.record(), nil
}

type cleanupRow struct {
	bun.BaseModel  `bun:"table:users.message_attachment_cleanup,alias:intent"`
	ID             int64  `bun:"id,pk,autoincrement"`
	TenantID       int64  `bun:"tenant_id,notnull"`
	FilenameStored string `bun:"filename_stored,notnull"`
}

// PendingMessageAttachmentCleanups locks a batch of queued object removals. CALLER CONTRACT:
// remove the objects and complete the intents in the same transaction.
func (s *Store) PendingMessageAttachmentCleanups(ctx context.Context, limit int) ([]communication.MessageAttachmentCleanupIntent, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []cleanupRow
	if err := db.NewSelect().Model(&rows).
		Where("intent.tenant_id = ? AND intent.cleaned_at IS NULL", tenantID).
		OrderExpr("intent.id ASC").Limit(int64(limit)).
		For("UPDATE SKIP LOCKED").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list message attachment cleanups: %w", err)
	}
	out := make([]communication.MessageAttachmentCleanupIntent, 0, len(rows))
	for _, row := range rows {
		out = append(out, communication.MessageAttachmentCleanupIntent{ID: row.ID, TenantID: row.TenantID, StoredName: row.FilenameStored})
	}
	return out, nil
}

// CompleteMessageAttachmentCleanup settles one intent.
func (s *Store) CompleteMessageAttachmentCleanup(ctx context.Context, intentID int64, at time.Time) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewUpdate().TableExpr("users.message_attachment_cleanup AS intent").
		Set("cleaned_at = ?", at).
		Where("intent.tenant_id = ? AND intent.id = ?", tenantID, intentID).Exec(ctx); err != nil {
		return fmt.Errorf("complete message attachment cleanup: %w", err)
	}
	return nil
}
