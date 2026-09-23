package careplan

import (
	"context"
	"errors"
	"slices"
	"time"
)

// Child documents (#777, the Dokumente tab of a child's profile; #3427).
// Care Plan owns what a document means for the child's care: its category,
// who may see it, and the audit trail of every upload, delete and sensitive
// download. File Storage keeps the bytes and their cleanup; the HTTP adapter
// hands the stored name over.

// The document categories.
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

// StudentDocumentCategories lists every category in display order.
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

// StudentDocumentCategoryLabels maps each category to its German label. The
// office never sees the storage key.
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

// IsValidStudentDocumentCategory reports whether v is a known category.
func IsValidStudentDocumentCategory(v string) bool {
	return slices.Contains(StudentDocumentCategories, v)
}

// IsHealthStudentDocumentCategory reports whether the category holds Art. 9
// GDPR health data.
func IsHealthStudentDocumentCategory(v string) bool {
	switch v {
	case StudentDocumentCategoryAttest, StudentDocumentCategoryImpfnachweis, StudentDocumentCategoryMedikamentenplan:
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

// IsSensitiveStudentDocumentCategory reports whether serving the category's
// content requires a data-access log row.
func IsSensitiveStudentDocumentCategory(v string) bool {
	return IsHealthStudentDocumentCategory(v) || IsLegalStudentDocumentCategory(v)
}

// StudentDocumentUploadDeadline bounds every step an upload request may still
// perform once its cleanup intent exists: the object write and the metadata
// transaction. The handler enforces it, so no request can persist a document
// row after its queued intent became eligible for the cleanup scheduler.
const StudentDocumentUploadDeadline = 2 * time.Minute

var (
	// ErrStudentDocumentInvalid marks a semantically invalid payload.
	ErrStudentDocumentInvalid = errors.New("invalid student document")
	// ErrStudentDocumentForbidden marks a category the caller's permissions
	// do not cover.
	ErrStudentDocumentForbidden = errors.New("student document category not permitted")
	// ErrStudentDocumentNoAccess marks a child the caller may not reach at
	// all: not "which drawer" but "whose file cabinet".
	ErrStudentDocumentNoAccess = errors.New("no access to this child")
)

// StudentDocumentActor identifies the acting account for permission checks
// and audit rows.
type StudentDocumentActor struct {
	AccountID   int64
	Name        string
	Role        string
	Permissions []string
}

// CreateStudentDocumentInput carries the metadata of an already-stored
// upload. The handler owns the bytes; Care Plan owns authority, the metadata
// row and the audit trail.
type CreateStudentDocumentInput struct {
	StudentID       int64
	Category        string
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
}

// StudentDocumentQueries read the Dokumente tab.
type StudentDocumentQueries interface {
	// ListStudentDocuments returns the documents the actor may see, newest
	// first, plus the actor's visible categories.
	ListStudentDocuments(ctx context.Context, studentID int64, category string, actor StudentDocumentActor) ([]CareDocument, []string, error)
	// AuthorizeStudentDocumentUpload answers "may this caller add a document
	// of this category to this child" WITHOUT writing anything.
	AuthorizeStudentDocumentUpload(ctx context.Context, studentID int64, category string, actor StudentDocumentActor) error
	// ResolveStudentDocumentDownload authorizes the download and, for the
	// sensitive categories, writes the data-access log row. No file may be
	// served when it fails.
	ResolveStudentDocumentDownload(ctx context.Context, studentID, documentID int64, actor StudentDocumentActor) (CareDocument, error)
	// ResolveStudentDocumentCleanup authorizes a retry of the storage cleanup
	// for a document that may already be soft-deleted.
	ResolveStudentDocumentCleanup(ctx context.Context, studentID, documentID int64, actor StudentDocumentActor) (CareDocument, error)
	// ListDeletedStudentDocumentsPendingFileCleanup returns the authorized
	// retry candidates whose previous unlink did not complete.
	ListDeletedStudentDocumentsPendingFileCleanup(ctx context.Context, studentID int64, actor StudentDocumentActor) ([]CareDocument, error)
	// CanSeeStudentDocumentCategory reports whether the granted permissions
	// cover a category. Readers of derived data, such as the change history,
	// apply the Dokumente tab's filter instead of becoming a way around it.
	CanSeeStudentDocumentCategory(category string, granted []string) bool
	// CanSeeEveryStudentDocumentCategory reports whether they cover every
	// category; readers use it for rows whose category is unknown.
	CanSeeEveryStudentDocumentCategory(granted []string) bool
}

// StudentDocumentCommands change the Dokumente tab. Each command commits its
// own tenant transaction before it returns, so the caller removes bytes only
// after the metadata change is durable.
type StudentDocumentCommands interface {
	// CreateStudentDocument persists the metadata row and its audit entry.
	CreateStudentDocument(ctx context.Context, input CreateStudentDocumentInput, actor StudentDocumentActor) (CareDocument, error)
	// DeleteStudentDocument soft-deletes the metadata row with its audit entry.
	DeleteStudentDocument(ctx context.Context, studentID, documentID int64, actor StudentDocumentActor) (CareDocument, error)
	// QueueStudentDocumentFileCleanup durably records the cleanup intent
	// before the object is written.
	QueueStudentDocumentFileCleanup(ctx context.Context, studentID int64, storedName string) error
	// SweepStudentDocumentFiles is the scheduler's recovery pass over the
	// tenant: it removes the bytes of soft-deleted documents and of orphaned
	// uploads through remove and settles each row once its object is gone.
	// A failed item stays for the next pass and is reported, not returned;
	// the error covers only a list that could not be read.
	SweepStudentDocumentFiles(ctx context.Context, remove StudentDocumentFileRemover) (StudentDocumentFileSweep, error)
	// The storage coordinator's store half.
	MarkFileDeleted(ctx context.Context, documentID int64) error
	MarkQueuedCleanupCompleteByFilename(ctx context.Context, storedName string) error
	ActivateQueuedCleanup(ctx context.Context, storedName string) error
}

// StudentDocumentFileRemover deletes one stored object of the school.
type StudentDocumentFileRemover func(ctx context.Context, tenantID int64, storedName string) error

// Sweep stages at which one item can fail.
const (
	StudentDocumentSweepRemove = "remove"
	StudentDocumentSweepSettle = "settle"
)

// StudentDocumentFileSweep reports one recovery pass. DeletedListed and
// OrphansListed are the batch sizes read, so a caller can tell a full batch
// (more rows waiting) from a drained queue.
type StudentDocumentFileSweep struct {
	Removed       int
	DeletedListed int
	OrphansListed int
	Failures      []StudentDocumentFileFailure
}

// StudentDocumentFileFailure is one item the pass left for the next one.
// DocumentID names a soft-deleted document, CleanupID an orphaned upload.
type StudentDocumentFileFailure struct {
	StudentID  int64
	DocumentID int64
	CleanupID  int64
	Stage      string
	Err        error
}

// StudentDocuments is the child document capability.
type StudentDocuments interface {
	StudentDocumentQueries
	StudentDocumentCommands
}
