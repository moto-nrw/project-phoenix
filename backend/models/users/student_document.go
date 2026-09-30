package users

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
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

// StudentDocument is one uploaded document belonging to a child (#777).
// Only metadata — the bytes live in the storage backend under the stored
// (UUID) name and are served exclusively through the permission-checked
// download handler.
type StudentDocument struct {
	base.Model `bun:"schema:users,table:student_documents"`
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
	StudentID       int64      `bun:"student_id,notnull" json:"student_id"`
}

// StudentDocumentFileCleanup tracks an upload whose metadata could not be
// committed, or whose child was deleted before the bytes were removed.
//
// It has no student foreign key on purpose. Deleting a child cascades the
// document rows away; without a cleanup row that outlives the cascade, the
// stored bytes would stay on disk forever with nothing pointing at them.
type StudentDocumentFileCleanup struct {
	base.Model `bun:"schema:users,table:student_document_file_cleanup"`
	base.TenantModel
	OwnerID        int64      `bun:"owner_id,notnull"`
	FilenameStored string     `bun:"filename_stored,notnull"`
	RetryAfter     time.Time  `bun:"retry_after,notnull"`
	CleanedAt      *time.Time `bun:"cleaned_at"`
}
