package workforce

import (
	"context"
	"errors"
	"time"
)

// Gender values of the personnel record. Nil means "keine Angabe".
const (
	GenderFemale  = "female"
	GenderMale    = "male"
	GenderDiverse = "diverse"
)

// Staff document categories (#1424): a fixed enumeration mirrored by the
// CHECK constraint on users.staff_documents.category.
const (
	StaffDocumentCategoryArbeitsvertrag  = "arbeitsvertrag"
	StaffDocumentCategoryZeugnis         = "zeugnis"
	StaffDocumentCategoryLohnabrechnung  = "lohnabrechnung"
	StaffDocumentCategoryBewerbung       = "bewerbung"
	StaffDocumentCategoryAUBescheinigung = "au_bescheinigung"
	StaffDocumentCategorySonstiges       = "sonstiges"
)

var (
	ErrStaffMasterDataNotFound    = errors.New("staff master data not found")
	ErrStaffFinancialDataNotFound = errors.New("staff financial data not found")
	ErrStaffDocumentNotFound      = errors.New("staff document not found")
	ErrInvalidStaffRecord         = errors.New("invalid staff record input")
)

// InvalidStaffRecordError carries the caller-facing validation reason of a
// personnel record row; it unwraps to ErrInvalidStaffRecord.
type InvalidStaffRecordError struct{ Reason string }

func (e *InvalidStaffRecordError) Error() string { return e.Reason }
func (e *InvalidStaffRecordError) Unwrap() error { return ErrInvalidStaffRecord }

// StaffMasterData is the 1:1 Stammdaten extension of a staff member (#1423).
// Dates are calendar days in DateLayout; empty means unset.
type StaffMasterData struct {
	ID                    int64
	TenantID              int64
	StaffID               int64
	Gender                *string
	AddressStreet         *string
	AddressPostalCode     *string
	AddressCity           *string
	Phone                 *string
	Email                 *string
	EmergencyContactName  *string
	EmergencyContactPhone *string
	EntryDate             string
	ContractEndDate       string
	ProbationEndDate      string
	WeeklyHours           *float64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// StaffQualification is one qualification row of a staff member (#1423).
type StaffQualification struct {
	ID       int64
	TenantID int64
	StaffID  int64
	// TypeID names the kind of proof (#4808); nil is "Sonstiges".
	TypeID     *int64
	Name       string
	AcquiredOn string
	ExpiresOn  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// StaffFinancialData is the 1:1 bank and tax record of a staff member. Only
// the staff:financial code path may read it.
type StaffFinancialData struct {
	ID                   int64
	TenantID             int64
	StaffID              int64
	IBAN                 *string
	TaxID                *string
	SocialSecurityNumber *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// StaffDocument is one uploaded personnel file of a staff member (#1424):
// metadata only, soft-deleted so the row survives as the audit trail.
type StaffDocument struct {
	ID              int64
	TenantID        int64
	StaffID         int64
	Category        string
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
	UploadedBy      int64
	DeletedAt       *time.Time
	DeletedBy       *int64
	FileDeletedAt   *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// StaffDocumentFilter narrows a document listing. Categories nil means every
// category; an explicit empty list matches nothing. IncludeDeleted keeps
// soft-deleted rows, DeletedOnly restricts to them, FilePending restricts to
// rows whose stored bytes still exist. Results are newest first.
type StaffDocumentFilter struct {
	StaffID        int64
	StaffIDs       []int64
	Categories     []string
	IncludeDeleted bool
	DeletedOnly    bool
	FilePending    bool
}

// StaffDocumentFileCleanup tracks an upload whose bytes must be removed once
// the metadata transaction has settled.
type StaffDocumentFileCleanup struct {
	ID             int64
	TenantID       int64
	StaffID        int64
	FilenameStored string
	RetryAfter     time.Time
	CleanedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// StaffRecordQuery reads the personnel record rows.
type StaffRecordQuery interface {
	// FindStaffMasterData resolves the Stammdaten row of a staff member;
	// ErrStaffMasterDataNotFound when none was written yet.
	FindStaffMasterData(ctx context.Context, staffID int64) (StaffMasterData, error)
	// ListStaffQualifications returns the staff member's live
	// qualifications, oldest first. Removed ones are retired, not listed.
	ListStaffQualifications(ctx context.Context, staffID int64) ([]StaffQualification, error)
	FindStaffFinancialData(ctx context.Context, staffID int64) (StaffFinancialData, error)
	// FindStaffDocument loads one document by the staff/document pair; the
	// URL names both and they must agree.
	FindStaffDocument(ctx context.Context, staffID, documentID int64, includeDeleted bool) (StaffDocument, error)
	ListStaffDocuments(context.Context, StaffDocumentFilter) ([]StaffDocument, error)
}

// StaffRecordCommand writes the personnel record rows.
type StaffRecordCommand interface {
	StaffPersonnelDataCommand
	StaffDocumentRecordCommand
}

// StaffPersonnelDataCommand writes the Stammdaten, qualification and
// financial rows of a staff member.
type StaffPersonnelDataCommand interface {
	CreateStaffMasterData(context.Context, StaffMasterData) (StaffMasterData, error)
	UpdateStaffMasterData(context.Context, StaffMasterData) (StaffMasterData, error)
	// ReplaceStaffQualifications atomically replaces the qualification list
	// of one staff member and returns the stored rows. Removed rows are
	// retired as history (ADR 0021); unchanged rows keep their identity.
	ReplaceStaffQualifications(ctx context.Context, staffID int64, values []StaffQualification) ([]StaffQualification, error)
	CreateStaffFinancialData(context.Context, StaffFinancialData) (StaffFinancialData, error)
	UpdateStaffFinancialData(context.Context, StaffFinancialData) (StaffFinancialData, error)
}

// StaffDocumentRecordCommand writes the staff document rows and the cleanup
// intents of their stored files.
type StaffDocumentRecordCommand interface {
	CreateStaffDocument(context.Context, StaffDocument) (StaffDocument, error)
	// SoftDeleteStaffDocument stamps deleted_at and deleted_by on a live row
	// and reports how many rows changed.
	SoftDeleteStaffDocument(ctx context.Context, id, deletedBy int64, at time.Time) (int64, error)
	MarkStaffDocumentFileDeleted(ctx context.Context, id int64, at time.Time) error
	// QueueStaffDocumentFileCleanup records the intent once per stored name.
	QueueStaffDocumentFileCleanup(context.Context, StaffDocumentFileCleanup) error
	// ListQueuedStaffDocumentFileCleanups returns and row-locks the eligible
	// intents of the tenant; a positive staffID narrows them.
	ListQueuedStaffDocumentFileCleanups(ctx context.Context, staffID int64) ([]StaffDocumentFileCleanup, error)
	CompleteStaffDocumentFileCleanup(ctx context.Context, id int64) error
	CompleteStaffDocumentFileCleanupByFilename(ctx context.Context, filename string) error
	// ActivateStaffDocumentFileCleanup makes an intent eligible right away.
	ActivateStaffDocumentFileCleanup(ctx context.Context, filename string) error
}
