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
	// AccidentRead and AccidentManage guard the accident reports (#4771):
	// seeing and downloading them, and every write.
	AccidentRead   string
	AccidentManage string
}

// StudentAccidentSignatures is the shared signature store as the accident
// reports use it (#4785, #4771). The binding fingerprints the statement as
// the signature's document checksum and refuses an invalid image with
// careplan.ErrAccidentSignatureInvalid. Both run in the caller's tenant unit
// of work.
type StudentAccidentSignatures interface {
	RecordAccidentSignature(ctx context.Context, entry careplan.AccidentSignatureEntry) (int64, error)
	// AccidentSignature reads one signature of the child and checks it
	// against the statement; nil when the school holds no such signature.
	AccidentSignature(ctx context.Context, studentID, signatureID int64, statement string) (*careplan.AccidentSignature, error)
}

// StudentDocumentAudit writes the document trail: one change-history row per
// upload or delete, and one data-access row per sensitive download.
type StudentDocumentAudit interface {
	RecordDocumentChange(ctx context.Context, studentID int64, actor careplan.StudentDocumentActor, category, oldValue, newValue string) error
	RecordDocumentDownload(ctx context.Context, studentID int64, actor careplan.StudentDocumentActor, document careplan.CareDocument, at time.Time) error
}
