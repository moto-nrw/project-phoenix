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

func invalidStaffRecord(reason string) error { return &InvalidStaffRecordError{Reason: reason} }

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
	ID         int64
	TenantID   int64
	StaffID    int64
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
	// ListStaffQualifications returns the staff member's qualifications,
	// oldest first.
	ListStaffQualifications(ctx context.Context, staffID int64) ([]StaffQualification, error)
	FindStaffFinancialData(ctx context.Context, staffID int64) (StaffFinancialData, error)
	// FindStaffDocument loads one document by the staff/document pair; the
	// URL names both and they must agree.
	FindStaffDocument(ctx context.Context, staffID, documentID int64, includeDeleted bool) (StaffDocument, error)
	ListStaffDocuments(context.Context, StaffDocumentFilter) ([]StaffDocument, error)
}

// StaffRecordCommand writes the personnel record rows.
type StaffRecordCommand interface {
	CreateStaffMasterData(context.Context, StaffMasterData) (StaffMasterData, error)
	UpdateStaffMasterData(context.Context, StaffMasterData) (StaffMasterData, error)
	// ReplaceStaffQualifications atomically replaces the qualification list
	// of one staff member and returns the stored rows.
	ReplaceStaffQualifications(ctx context.Context, staffID int64, values []StaffQualification) ([]StaffQualification, error)
	CreateStaffFinancialData(context.Context, StaffFinancialData) (StaffFinancialData, error)
	UpdateStaffFinancialData(context.Context, StaffFinancialData) (StaffFinancialData, error)
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

type staffRecordEngine interface {
	FindStaffMasterData(ctx context.Context, staffID int64) (StaffMasterData, error)
	CreateStaffMasterData(context.Context, StaffMasterData) (StaffMasterData, error)
	UpdateStaffMasterData(context.Context, StaffMasterData) (StaffMasterData, error)
	ListStaffQualifications(ctx context.Context, staffID int64) ([]StaffQualification, error)
	ReplaceStaffQualifications(ctx context.Context, staffID int64, values []StaffQualification) ([]StaffQualification, error)
	FindStaffFinancialData(ctx context.Context, staffID int64) (StaffFinancialData, error)
	CreateStaffFinancialData(context.Context, StaffFinancialData) (StaffFinancialData, error)
	UpdateStaffFinancialData(context.Context, StaffFinancialData) (StaffFinancialData, error)
	CreateStaffDocument(context.Context, StaffDocument) (StaffDocument, error)
	FindStaffDocument(ctx context.Context, staffID, documentID int64, includeDeleted bool) (StaffDocument, error)
	ListStaffDocuments(context.Context, StaffDocumentFilter) ([]StaffDocument, error)
	SoftDeleteStaffDocument(ctx context.Context, id, deletedBy int64, at time.Time) (int64, error)
	MarkStaffDocumentFileDeleted(ctx context.Context, id int64, at time.Time) error
	QueueStaffDocumentFileCleanup(context.Context, StaffDocumentFileCleanup) error
	ListQueuedStaffDocumentFileCleanups(ctx context.Context, staffID int64) ([]StaffDocumentFileCleanup, error)
	CompleteStaffDocumentFileCleanup(ctx context.Context, id int64) error
	CompleteStaffDocumentFileCleanupByFilename(ctx context.Context, filename string) error
	ActivateStaffDocumentFileCleanup(ctx context.Context, filename string) error
}

func (m *Module) FindStaffMasterData(ctx context.Context, staffID int64) (StaffMasterData, error) {
	if staffID <= 0 {
		return StaffMasterData{}, invalidStaffRecord("staff ID is required")
	}
	return m.engine.FindStaffMasterData(ctx, staffID)
}

func (m *Module) CreateStaffMasterData(ctx context.Context, value StaffMasterData) (StaffMasterData, error) {
	return m.engine.CreateStaffMasterData(ctx, value)
}

func (m *Module) UpdateStaffMasterData(ctx context.Context, value StaffMasterData) (StaffMasterData, error) {
	if value.ID <= 0 {
		return StaffMasterData{}, invalidStaffRecord("staff master data ID is required")
	}
	return m.engine.UpdateStaffMasterData(ctx, value)
}

func (m *Module) ListStaffQualifications(ctx context.Context, staffID int64) ([]StaffQualification, error) {
	if staffID <= 0 {
		return nil, invalidStaffRecord("staff ID is required")
	}
	return m.engine.ListStaffQualifications(ctx, staffID)
}

func (m *Module) ReplaceStaffQualifications(ctx context.Context, staffID int64, values []StaffQualification) ([]StaffQualification, error) {
	if staffID <= 0 {
		return nil, invalidStaffRecord("staff ID is required")
	}
	return m.engine.ReplaceStaffQualifications(ctx, staffID, values)
}

func (m *Module) FindStaffFinancialData(ctx context.Context, staffID int64) (StaffFinancialData, error) {
	if staffID <= 0 {
		return StaffFinancialData{}, invalidStaffRecord("staff ID is required")
	}
	return m.engine.FindStaffFinancialData(ctx, staffID)
}

func (m *Module) CreateStaffFinancialData(ctx context.Context, value StaffFinancialData) (StaffFinancialData, error) {
	return m.engine.CreateStaffFinancialData(ctx, value)
}

func (m *Module) UpdateStaffFinancialData(ctx context.Context, value StaffFinancialData) (StaffFinancialData, error) {
	if value.ID <= 0 {
		return StaffFinancialData{}, invalidStaffRecord("staff financial data ID is required")
	}
	return m.engine.UpdateStaffFinancialData(ctx, value)
}

func (m *Module) CreateStaffDocument(ctx context.Context, value StaffDocument) (StaffDocument, error) {
	return m.engine.CreateStaffDocument(ctx, value)
}

func (m *Module) FindStaffDocument(ctx context.Context, staffID, documentID int64, includeDeleted bool) (StaffDocument, error) {
	if staffID <= 0 || documentID <= 0 {
		return StaffDocument{}, invalidStaffRecord("staff ID and document ID are required")
	}
	return m.engine.FindStaffDocument(ctx, staffID, documentID, includeDeleted)
}

func (m *Module) ListStaffDocuments(ctx context.Context, filter StaffDocumentFilter) ([]StaffDocument, error) {
	return m.engine.ListStaffDocuments(ctx, filter)
}

func (m *Module) SoftDeleteStaffDocument(ctx context.Context, id, deletedBy int64, at time.Time) (int64, error) {
	if id <= 0 {
		return 0, invalidStaffRecord("staff document ID is required")
	}
	return m.engine.SoftDeleteStaffDocument(ctx, id, deletedBy, at)
}

func (m *Module) MarkStaffDocumentFileDeleted(ctx context.Context, id int64, at time.Time) error {
	if id <= 0 {
		return invalidStaffRecord("staff document ID is required")
	}
	return m.engine.MarkStaffDocumentFileDeleted(ctx, id, at)
}

func (m *Module) QueueStaffDocumentFileCleanup(ctx context.Context, value StaffDocumentFileCleanup) error {
	return m.engine.QueueStaffDocumentFileCleanup(ctx, value)
}

func (m *Module) ListQueuedStaffDocumentFileCleanups(ctx context.Context, staffID int64) ([]StaffDocumentFileCleanup, error) {
	return m.engine.ListQueuedStaffDocumentFileCleanups(ctx, staffID)
}

func (m *Module) CompleteStaffDocumentFileCleanup(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidStaffRecord("staff document file cleanup ID is required")
	}
	return m.engine.CompleteStaffDocumentFileCleanup(ctx, id)
}

func (m *Module) CompleteStaffDocumentFileCleanupByFilename(ctx context.Context, filename string) error {
	return m.engine.CompleteStaffDocumentFileCleanupByFilename(ctx, filename)
}

func (m *Module) ActivateStaffDocumentFileCleanup(ctx context.Context, filename string) error {
	return m.engine.ActivateStaffDocumentFileCleanup(ctx, filename)
}
