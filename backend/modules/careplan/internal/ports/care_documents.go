package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// StudentDocumentAccess answers the two authority questions of the Dokumente
// tab with the security runtime's semantics.
type StudentDocumentAccess interface {
	// HasPermission reports whether the granted permissions cover required,
	// wildcards included.
	HasPermission(required string, granted []string) bool
	// CanAccessStudent reports whether the caller may reach the child at all:
	// an administrator, or verified staff of the tenant. A child that does
	// not exist is the People Directory's not-found error.
	CanAccessStudent(ctx context.Context, studentID int64, granted []string) (bool, error)
}

// StudentDocumentPermissions names the permissions guarding the category
// classes: health documents (Art. 9 GDPR), custody paperwork, everything
// else. Composition binds the security runtime's permission names.
type StudentDocumentPermissions struct {
	Health  string
	Legal   string
	Default string
}

// StudentDocumentAudit writes the document trail: one change-history row per
// upload or delete, and one data-access row per sensitive download.
type StudentDocumentAudit interface {
	RecordDocumentChange(ctx context.Context, studentID int64, actor careplan.StudentDocumentActor, category, oldValue, newValue string) error
	RecordDocumentDownload(ctx context.Context, studentID int64, actor careplan.StudentDocumentActor, document careplan.CareDocument, at time.Time) error
}
