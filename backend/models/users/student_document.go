package users

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/documents"
)

// Student document categories (#777). Fixed enum, mirrored by the CHECK
// constraint on users.student_documents.category — no free-text categories.
//
// The split is not cosmetic: it decides which permission a category needs.
// Attest, Impfnachweis and Medikamentenplan are health data (Art. 9 GDPR);
// Sorgerecht carries custody rulings, which are not Art. 9 but are at least
// as damaging in the wrong hands. Both tiers are gated separately from the
// ordinary paperwork an OGS office handles every day.
const (
	StudentDocumentCategoryBetreuungsvertrag = "betreuungsvertrag"
	StudentDocumentCategoryAbholvollmacht    = "abholvollmacht"
	StudentDocumentCategorySchwimmerlaubnis  = "schwimmerlaubnis"
	StudentDocumentCategoryAttest            = "attest"
	StudentDocumentCategoryImpfnachweis      = "impfnachweis"
	StudentDocumentCategoryMedikamentenplan  = "medikamentenplan"
	StudentDocumentCategorySorgerecht        = "sorgerecht"
	StudentDocumentCategorySonstiges         = "sonstiges"
)

// StudentDocumentCategories lists every valid category in display order.
var StudentDocumentCategories = []string{
	StudentDocumentCategoryBetreuungsvertrag,
	StudentDocumentCategoryAbholvollmacht,
	StudentDocumentCategorySchwimmerlaubnis,
	StudentDocumentCategoryAttest,
	StudentDocumentCategoryImpfnachweis,
	StudentDocumentCategoryMedikamentenplan,
	StudentDocumentCategorySorgerecht,
	StudentDocumentCategorySonstiges,
}

// StudentDocumentCategoryLabels maps each category to its German UI label.
// The office never sees the storage key.
var StudentDocumentCategoryLabels = map[string]string{
	StudentDocumentCategoryBetreuungsvertrag: "Betreuungsvertrag",
	StudentDocumentCategoryAbholvollmacht:    "Abholvollmacht",
	StudentDocumentCategorySchwimmerlaubnis:  "Schwimmerlaubnis",
	StudentDocumentCategoryAttest:            "Ärztliches Attest",
	StudentDocumentCategoryImpfnachweis:      "Impfnachweis",
	StudentDocumentCategoryMedikamentenplan:  "Medikamentenplan",
	StudentDocumentCategorySorgerecht:        "Sorgerechtsnachweis",
	StudentDocumentCategorySonstiges:         "Sonstiges",
}

// IsValidStudentDocumentCategory reports whether v is a known category value.
func IsValidStudentDocumentCategory(v string) bool {
	for _, c := range StudentDocumentCategories {
		if c == v {
			return true
		}
	}
	return false
}

// IsHealthStudentDocumentCategory reports whether the category holds Art. 9
// health data.
func IsHealthStudentDocumentCategory(v string) bool {
	switch v {
	case StudentDocumentCategoryAttest,
		StudentDocumentCategoryImpfnachweis,
		StudentDocumentCategoryMedikamentenplan:
		return true
	default:
		return false
	}
}

// IsLegalStudentDocumentCategory reports whether the category holds custody
// or other court paperwork.
func IsLegalStudentDocumentCategory(v string) bool {
	return v == StudentDocumentCategorySorgerecht
}

// StudentDocument is one uploaded document belonging to a child (#777).
// Only metadata — the bytes live in the storage backend under the stored
// (UUID) name and are served exclusively through the permission-checked
// download handler.
type StudentDocument struct {
	documents.File `bun:"schema:users,table:student_documents"`
	StudentID      int64 `bun:"student_id,notnull" json:"student_id"`
}

// GetOwnerID satisfies documents.Entity.
func (d *StudentDocument) GetOwnerID() int64 { return d.StudentID }

// Validate ensures the document row is storable.
func (d *StudentDocument) Validate() error {
	if d.StudentID <= 0 {
		return errors.New("student_id is required")
	}
	if !IsValidStudentDocumentCategory(d.Category) {
		return errors.New("unknown document category")
	}
	return documents.ValidateFile(&d.File)
}

// StudentDocumentFileCleanup tracks an upload whose metadata could not be
// committed, or whose child was deleted before the bytes were removed.
//
// It has no student foreign key on purpose. Deleting a child cascades the
// document rows away; without a cleanup row that outlives the cascade, the
// stored bytes would stay on disk forever with nothing pointing at them.
type StudentDocumentFileCleanup struct {
	documents.FileCleanup `bun:"schema:users,table:student_document_file_cleanup"`
}

// StudentDocumentRepository persists student document metadata. Soft delete
// is a domain operation (deleted_at + deleted_by in one write) —
// phoenix_tenant has no DELETE grant on the table.
type StudentDocumentRepository interface {
	Create(ctx context.Context, doc *StudentDocument) error
	FindForOwner(ctx context.Context, studentID, documentID int64) (*StudentDocument, error)
	FindForOwnerIncludingDeleted(ctx context.Context, studentID, documentID int64) (*StudentDocument, error)
	ListByOwnerID(ctx context.Context, studentID int64, categories []string) ([]*StudentDocument, error)
	ListPendingFileCleanupByOwnerID(ctx context.Context, studentID int64) ([]*StudentDocument, error)
	ListDeletedPendingFileCleanups(ctx context.Context) ([]*StudentDocument, error)
	ListDeletedPendingFileCleanupByOwnerID(ctx context.Context, studentID int64, categories []string) ([]*StudentDocument, error)
	SoftDelete(ctx context.Context, doc *StudentDocument, deletedBy int64) error
	MarkFileDeleted(ctx context.Context, documentID int64) error
	QueueFileCleanup(ctx context.Context, cleanup *StudentDocumentFileCleanup) error
	ListQueuedFileCleanups(ctx context.Context) ([]*StudentDocumentFileCleanup, error)
	ListQueuedFileCleanupByOwnerID(ctx context.Context, studentID int64) ([]*StudentDocumentFileCleanup, error)
	MarkQueuedFileCleanupComplete(ctx context.Context, cleanupID int64) error
	MarkQueuedFileCleanupCompleteByFilename(ctx context.Context, filename string) error
	ActivateQueuedFileCleanupByFilename(ctx context.Context, filename string) error
}
