package users

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Staff document categories (#1424). Fixed enum, mirrored by the CHECK
// constraint on users.staff_documents.category — no free-text categories.
const (
	StaffDocumentCategoryArbeitsvertrag  = "arbeitsvertrag"
	StaffDocumentCategoryZeugnis         = "zeugnis"
	StaffDocumentCategoryLohnabrechnung  = "lohnabrechnung"
	StaffDocumentCategoryBewerbung       = "bewerbung"
	StaffDocumentCategoryAUBescheinigung = "au_bescheinigung"
	StaffDocumentCategorySonstiges       = "sonstiges"
)

// StaffDocumentCategories lists every valid category in display order.
var StaffDocumentCategories = []string{
	StaffDocumentCategoryArbeitsvertrag,
	StaffDocumentCategoryZeugnis,
	StaffDocumentCategoryLohnabrechnung,
	StaffDocumentCategoryBewerbung,
	StaffDocumentCategoryAUBescheinigung,
	StaffDocumentCategorySonstiges,
}

// IsValidStaffDocumentCategory reports whether v is a known category value.
func IsValidStaffDocumentCategory(v string) bool {
	for _, c := range StaffDocumentCategories {
		if c == v {
			return true
		}
	}
	return false
}

// StaffDocument is one uploaded personnel file of a staff member (#1424).
// Only metadata — the bytes live on the filesystem under the stored (UUID)
// filename and are served exclusively through the permission-checked
// download handler. The row is soft-deleted so the metadata survives as an
// audit trail after the file itself is removed.
type StaffDocument struct {
	base.Model `bun:"schema:users,table:staff_documents"`
	base.TenantModel
	StaffID         int64      `bun:"staff_id,notnull" json:"staff_id"`
	Category        string     `bun:"category,notnull" json:"category"`
	FilenameDisplay string     `bun:"filename_display,notnull" json:"filename_display"`
	FilenameStored  string     `bun:"filename_stored,notnull" json:"-"`
	SizeBytes       int64      `bun:"size_bytes,notnull" json:"size_bytes"`
	ContentType     string     `bun:"content_type,notnull" json:"content_type"`
	UploadedBy      int64      `bun:"uploaded_by,notnull" json:"uploaded_by"`
	DeletedAt       *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"-"`
	DeletedBy       *int64     `bun:"deleted_by" json:"-"`
	FileDeletedAt   *time.Time `bun:"file_deleted_at" json:"-"`
}

// StaffDocumentFileCleanup tracks an upload whose metadata could not be
// committed and whose file could not be removed immediately. It intentionally
// has no staff foreign key: the failed request may name a staff row that was
// concurrently offboarded, but the sensitive file still must remain queued.
type StaffDocumentFileCleanup struct {
	base.Model `bun:"schema:users,table:staff_document_file_cleanup"`
	base.TenantModel
	StaffID        int64      `bun:"staff_id,notnull"`
	FilenameStored string     `bun:"filename_stored,notnull"`
	CleanedAt      *time.Time `bun:"cleaned_at"`
}

// Validate ensures the document row is storable.
func (d *StaffDocument) Validate() error {
	if d.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if !IsValidStaffDocumentCategory(d.Category) {
		return errors.New("unknown document category")
	}
	if d.FilenameDisplay == "" {
		return errors.New("filename_display is required")
	}
	if d.FilenameStored == "" {
		return errors.New("filename_stored is required")
	}
	if d.SizeBytes < 0 {
		return errors.New("size_bytes must not be negative")
	}
	if d.ContentType == "" {
		return errors.New("content_type is required")
	}
	if d.UploadedBy <= 0 {
		return errors.New("uploaded_by is required")
	}
	return nil
}

// StaffDocumentRepository persists staff document metadata. Soft delete is a
// domain operation (deleted_at + deleted_by in one write) — phoenix_tenant
// has no DELETE grant on the table.
type StaffDocumentRepository interface {
	Create(ctx context.Context, doc *StaffDocument) error
	// FindForStaff loads one non-deleted document and enforces that it
	// belongs to the given staff member — the URL names both IDs and they
	// must agree.
	FindForStaff(ctx context.Context, staffID, documentID int64) (*StaffDocument, error)
	// ListByStaffID returns the staff member's non-deleted documents,
	// newest first, optionally restricted to the given categories. An empty
	// category list returns nothing (the caller may see no category at all).
	ListByStaffID(ctx context.Context, staffID int64, categories []string) ([]*StaffDocument, error)
	// ListPendingFileCleanupByStaffID returns documents whose stored bytes have
	// not yet been removed, including soft-deleted rows for offboarding.
	ListPendingFileCleanupByStaffID(ctx context.Context, staffID int64) ([]*StaffDocument, error)
	// ListDeletedPendingFileCleanupByStaffID limits retry candidates to
	// soft-deleted documents in the caller's visible categories.
	ListDeletedPendingFileCleanupByStaffID(ctx context.Context, staffID int64, categories []string) ([]*StaffDocument, error)
	// FindForStaffIncludingDeleted loads a document by the staff/document URL
	// pair even after its metadata was soft-deleted, so filesystem cleanup can
	// be retried without exposing unrelated records.
	FindForStaffIncludingDeleted(ctx context.Context, staffID, documentID int64) (*StaffDocument, error)
	SoftDelete(ctx context.Context, doc *StaffDocument, deletedBy int64) error
	MarkFileDeleted(ctx context.Context, documentID int64) error
	QueueFileCleanup(ctx context.Context, cleanup *StaffDocumentFileCleanup) error
	ListQueuedFileCleanupByStaffID(ctx context.Context, staffID int64) ([]*StaffDocumentFileCleanup, error)
	MarkQueuedFileCleanupComplete(ctx context.Context, cleanupID int64) error
}
