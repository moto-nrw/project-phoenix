// Package documents holds the shape every uploaded document row shares,
// independent of the entity it hangs off (a staff member, a child, later a
// feedback post).
//
// Two things are deliberately NOT generic here. The owner reference stays on
// the concrete type, because a document belongs to its owner through a real
// composite foreign key (tenant_id, owner_id) that the database enforces; a
// polymorphic entity_type/entity_id pair would trade that enforcement for a
// column we can never trust. And authority stays with the owning domain,
// because "who may see this category" is a different question for a payslip
// than for a medical certificate.
package documents

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// CleanupBatchSize caps one storage-cleanup sweep pass. The sweep runs inside
// a single tenant transaction, so an uncapped pass after a large deletion
// would hold a pool connection through thousands of unlink-and-mark pairs —
// and a context deadline firing mid-pass would roll back every completion mark
// that pass had written, so the next one would start from zero. Bounded passes
// commit their progress and resume on the next tick.
const CleanupBatchSize = 200

// File is the metadata of one stored document. The bytes live in the storage
// backend under FilenameStored; only the owning domain's download handler can
// reach them.
//
// DeletedAt is a soft delete on purpose: deleting a document removes the
// bytes immediately (GDPR wants the content gone now) while the row survives
// as the record that the document once existed. FileDeletedAt records that
// the bytes are actually gone, so a failed unlink can be retried without
// guessing.
type File struct {
	base.Model
	base.TenantModel
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

// GetCategory returns the document's category.
func (f *File) GetCategory() string { return f.Category }

// GetFilenameStored returns the on-disk (storage backend) name.
func (f *File) GetFilenameStored() string { return f.FilenameStored }

// GetFilenameDisplay returns the name the uploader saw.
func (f *File) GetFilenameDisplay() string { return f.FilenameDisplay }

// IsFileDeleted reports whether the stored bytes are already gone.
func (f *File) IsFileDeleted() bool { return f.FileDeletedAt != nil }

// MarkSoftDeleted stamps the in-memory row after a successful soft delete so
// the caller can respond without re-reading.
func (f *File) MarkSoftDeleted(at time.Time, by int64) {
	f.DeletedAt = &at
	f.DeletedBy = &by
}

// ValidateFile checks the parts of a document row that are identical across
// domains. The owner reference and the category vocabulary belong to the
// concrete type and are validated there.
func ValidateFile(f *File) error {
	if strings.TrimSpace(f.Category) == "" {
		return errors.New("category is required")
	}
	if strings.TrimSpace(f.FilenameDisplay) == "" {
		return errors.New("filename_display is required")
	}
	if strings.TrimSpace(f.FilenameStored) == "" {
		return errors.New("filename_stored is required")
	}
	if f.SizeBytes < 0 {
		return errors.New("size_bytes must not be negative")
	}
	if strings.TrimSpace(f.ContentType) == "" {
		return errors.New("content_type is required")
	}
	if f.UploadedBy <= 0 {
		return errors.New("uploaded_by is required")
	}
	return nil
}

// FileCleanup is a durable intent to remove stored bytes. It is written
// BEFORE the file is saved, so a process that dies between the write and the
// metadata commit still leaves a record the cleanup scheduler can act on.
//
// It intentionally carries no foreign key to the owner: the failed request
// may name an owner that was concurrently deleted, and the orphaned bytes
// still have to go.
type FileCleanup struct {
	base.Model
	base.TenantModel
	OwnerID        int64      `bun:"owner_id,notnull"`
	FilenameStored string     `bun:"filename_stored,notnull"`
	RetryAfter     time.Time  `bun:"retry_after,notnull"`
	CleanedAt      *time.Time `bun:"cleaned_at"`
}

// GetOwnerID returns the owner the failed upload was meant for.
func (c *FileCleanup) GetOwnerID() int64 { return c.OwnerID }

// GetFilenameStored returns the orphaned object's storage name.
func (c *FileCleanup) GetFilenameStored() string { return c.FilenameStored }

// Entity is what the generic document repository needs from a concrete
// document type.
type Entity interface {
	base.Entity
	base.TenantScoped
	GetOwnerID() int64
	GetCategory() string
	GetFilenameStored() string
	GetFilenameDisplay() string
	IsFileDeleted() bool
	MarkSoftDeleted(at time.Time, by int64)
	Validate() error
}

// CleanupEntity is what the generic repository needs from a concrete cleanup
// row.
type CleanupEntity interface {
	base.Entity
	base.TenantScoped
	GetOwnerID() int64
	GetFilenameStored() string
}
