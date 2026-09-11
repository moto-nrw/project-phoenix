package domain

import (
	"errors"
	"slices"
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

var StaffDocumentCategories = []string{
	StaffDocumentCategoryArbeitsvertrag, StaffDocumentCategoryZeugnis, StaffDocumentCategoryLohnabrechnung,
	StaffDocumentCategoryBewerbung, StaffDocumentCategoryAUBescheinigung, StaffDocumentCategorySonstiges,
}

const maxWeeklyHours = 80

var (
	ErrStaffMasterDataNotFound    = errors.New("staff master data not found")
	ErrStaffFinancialDataNotFound = errors.New("staff financial data not found")
	ErrStaffDocumentNotFound      = errors.New("staff document not found")
	ErrInvalidStaffRecord         = errors.New("invalid staff record input")
)

// InvalidStaffRecordError carries the caller-facing validation reason of a
// personnel record row; the wording is the established model contract.
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

func (m StaffMasterData) Validate() error {
	if m.StaffID <= 0 {
		return invalidStaffRecord("staff_id is required")
	}
	if m.Gender != nil {
		switch *m.Gender {
		case GenderFemale, GenderMale, GenderDiverse:
		default:
			return invalidStaffRecord("gender must be 'female', 'male', or 'diverse'")
		}
	}
	if m.WeeklyHours != nil && (*m.WeeklyHours < 0 || *m.WeeklyHours > maxWeeklyHours) {
		return invalidStaffRecord("weekly_hours must be between 0 and 80")
	}
	for _, pair := range []struct{ value, field string }{
		{m.EntryDate, "entry_date"}, {m.ContractEndDate, "contract_end_date"}, {m.ProbationEndDate, "probation_end_date"},
	} {
		if pair.value == "" {
			continue
		}
		if err := ValidateDate(pair.value, pair.field); err != nil {
			return invalidStaffRecord(err.Error())
		}
	}
	if m.EntryDate != "" && m.ContractEndDate != "" && m.ContractEndDate < m.EntryDate {
		return invalidStaffRecord("contract_end_date must not be before entry_date")
	}
	if m.EntryDate != "" && m.ProbationEndDate != "" && m.ProbationEndDate < m.EntryDate {
		return invalidStaffRecord("probation_end_date must not be before entry_date")
	}
	return nil
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

func (q StaffQualification) Validate() error {
	if q.StaffID <= 0 {
		return invalidStaffRecord("staff_id is required")
	}
	if q.Name == "" {
		return invalidStaffRecord("name is required")
	}
	for _, pair := range []struct{ value, field string }{{q.AcquiredOn, "acquired_on"}, {q.ExpiresOn, "expires_on"}} {
		if pair.value == "" {
			continue
		}
		if err := ValidateDate(pair.value, pair.field); err != nil {
			return invalidStaffRecord(err.Error())
		}
	}
	if q.AcquiredOn != "" && q.ExpiresOn != "" && q.ExpiresOn < q.AcquiredOn {
		return invalidStaffRecord("expires_on must not be before acquired_on")
	}
	return nil
}

// StaffFinancialData is the 1:1 bank and tax record of a staff member. Field
// syntax is validated by the personnel service; the store only requires the
// owner.
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

func (f StaffFinancialData) Validate() error {
	if f.StaffID <= 0 {
		return invalidStaffRecord("staff_id is required")
	}
	return nil
}

// StaffDocument is one uploaded personnel file of a staff member (#1424):
// metadata only, soft-deleted so the row survives as the audit trail after
// the file itself is removed.
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

func (d StaffDocument) Validate() error {
	if d.StaffID <= 0 {
		return invalidStaffRecord("staff_id is required")
	}
	if !slices.Contains(StaffDocumentCategories, d.Category) {
		return invalidStaffRecord("unknown document category")
	}
	if d.FilenameDisplay == "" {
		return invalidStaffRecord("filename_display is required")
	}
	if d.FilenameStored == "" {
		return invalidStaffRecord("filename_stored is required")
	}
	if d.SizeBytes < 0 {
		return invalidStaffRecord("size_bytes must not be negative")
	}
	if d.ContentType == "" {
		return invalidStaffRecord("content_type is required")
	}
	if d.UploadedBy <= 0 {
		return invalidStaffRecord("uploaded_by is required")
	}
	return nil
}

// StaffDocumentFilter narrows a document listing. Categories nil means every
// category; an explicit empty list matches nothing. IncludeDeleted keeps
// soft-deleted rows, DeletedOnly restricts to them, FilePending restricts to
// rows whose stored bytes still exist.
type StaffDocumentFilter struct {
	StaffID        int64
	StaffIDs       []int64
	Categories     []string
	IncludeDeleted bool
	DeletedOnly    bool
	FilePending    bool
}

// StaffDocumentFileCleanup tracks an upload whose bytes must be removed once
// the metadata transaction has settled (#1424).
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

func (c StaffDocumentFileCleanup) Validate() error {
	if c.StaffID <= 0 {
		return invalidStaffRecord("staff_id is required")
	}
	if c.FilenameStored == "" {
		return invalidStaffRecord("filename_stored is required")
	}
	if c.RetryAfter.IsZero() {
		return invalidStaffRecord("retry_after is required")
	}
	return nil
}
