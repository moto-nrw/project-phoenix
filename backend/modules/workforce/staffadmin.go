package workforce

import (
	"context"
	"errors"
	"time"
)

// This file is the public personnel-record contract of the staff
// administration surface (#2690): the staff member behind an account, the
// payroll number, the Stammdaten sections, and the Dokumente tab. The
// retained people-directory services serve it through the composition root;
// the workforce module owns the master-data, qualification, financial-data
// and document tables themselves (see StaffRecordQuery/StaffRecordCommand).

// Sentinel failures of the personnel-record surface. Adapters report the
// retained service's sentinel as one of these kinds with the original wording
// preserved (see StaffAdminError).
var (
	ErrStaffDocumentForbidden = errors.New("staff document access forbidden")
	ErrStaffDocumentInvalid   = errors.New("staff document input is invalid")
	ErrPersonnelNumberTaken   = errors.New("personnel number is already taken")
	ErrPersonnelNumberInvalid = errors.New("personnel number is invalid")
	ErrStaffStammdatenInvalid = errors.New("staff master data input is invalid")
)

// StaffAdminError is a classified personnel-record failure: Kind is one of
// the sentinels above and is what errors.Is reports, Cause keeps the wording
// of the retained service, which is what the HTTP layer renders.
type StaffAdminError struct {
	Kind  error
	Cause error
}

func (e *StaffAdminError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Kind.Error()
}

func (e *StaffAdminError) Is(target error) bool { return target == e.Kind }
func (e *StaffAdminError) Unwrap() error        { return e.Cause }

// StaffDocumentUploadDeadline bounds one document upload request. Cleanup
// intents queued before the file is written become eligible for retry only
// after this deadline has passed, so a retry can never race a running upload.
const StaffDocumentUploadDeadline = 2 * time.Minute

// Person is the directory identity behind a staff member.
type Person struct {
	ID        int64
	FirstName string
	LastName  string
	// Birthday is a calendar day or empty.
	Birthday  string
	AccountID *int64
}

// StaffProfile is the staff record with the directory identity it belongs
// to. Calendar days are YYYY-MM-DD or empty.
type StaffProfile struct {
	ID                 int64
	PersonID           int64
	FirstName          string
	LastName           string
	Birthday           string
	AccountID          *int64
	EmploymentType     *string
	WorkTimeModelID    *int64
	RotationAnchorDate string
	PersonnelNumber    *string
}

// StaffStammdaten aggregates the non-sensitive master-data sections of one
// staff member. Financial data has its own permission-gated read path.
type StaffStammdaten struct {
	Staff          StaffProfile
	MasterData     *StaffMasterData
	Qualifications []StaffQualification
}

// StammdatenPersonInput is the full person section.
type StammdatenPersonInput struct {
	FirstName string
	LastName  string
	Birthday  *string
	Gender    *string
}

// StammdatenKontaktInput is the full contact section; nil clears a field.
type StammdatenKontaktInput struct {
	AddressStreet         *string
	AddressPostalCode     *string
	AddressCity           *string
	Phone                 *string
	Email                 *string
	EmergencyContactName  *string
	EmergencyContactPhone *string
}

// StammdatenArbeitsvertragInput is the full contract section.
type StammdatenArbeitsvertragInput struct {
	EntryDate        *string
	ContractEndDate  *string
	ProbationEndDate *string
	WeeklyHours      *float64
	EmploymentType   *string
}

// StammdatenQualificationInput is one row of the submitted qualification
// list.
type StammdatenQualificationInput struct {
	Name       string
	AcquiredOn *string
	ExpiresOn  *string
}

// StammdatenFinancialInput is the full bank & tax section; nil clears a
// field.
type StammdatenFinancialInput struct {
	IBAN                 *string
	TaxID                *string
	SocialSecurityNumber *string
}

// StaffFinancialMasked is the default financial read: the IBAN shows its last
// characters, tax id and social security number only that a value exists.
type StaffFinancialMasked struct {
	StaffID                    int64
	IBANMasked                 *string
	TaxIDMasked                *string
	SocialSecurityNumberMasked *string
}

// StaffFinancialPlain carries the unmasked values after an audited reveal.
type StaffFinancialPlain struct {
	StaffID              int64
	IBAN                 *string
	TaxID                *string
	SocialSecurityNumber *string
}

// StaffDirectory resolves staff members and administers their personnel
// record. The lookups return the retained not-found failure unchanged so the
// HTTP layer classifies it the way it always did.
type StaffDirectory interface {
	PersonByAccountID(ctx context.Context, accountID int64) (*Person, error)
	StaffByPersonID(ctx context.Context, personID int64) (*StaffProfile, error)
	StaffByID(ctx context.Context, staffID int64) (*StaffProfile, error)
	ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error)
	UpdatePersonnelNumber(ctx context.Context, staffID int64, value *string, changedByStaffID int64, note string) (*StaffProfile, error)
	StaffStammdaten(ctx context.Context, staffID int64) (*StaffStammdaten, error)
	UpdateStaffStammdatenPerson(ctx context.Context, staffID int64, input StammdatenPersonInput, changedByStaffID int64, note string) error
	UpdateStaffStammdatenKontakt(ctx context.Context, staffID int64, input StammdatenKontaktInput, changedByStaffID int64, note string) error
	UpdateStaffStammdatenArbeitsvertrag(ctx context.Context, staffID int64, input StammdatenArbeitsvertragInput, changedByStaffID int64, note string) error
	ReplaceStaffQualificationList(ctx context.Context, staffID int64, inputs []StammdatenQualificationInput, changedByStaffID int64, note string) error
	StaffFinancialMasked(ctx context.Context, staffID, actorAccountID int64, actorRole string) (*StaffFinancialMasked, error)
	RevealStaffFinancial(ctx context.Context, staffID, actorAccountID int64, actorRole string) (*StaffFinancialPlain, error)
	UpdateStaffFinancial(ctx context.Context, staffID int64, input StammdatenFinancialInput, changedByAccountID int64, note string) error
}

// StaffDocumentActor identifies the acting account of a document operation:
// account id for audit rows, comma-joined roles for the access log, the
// permission set for the per-category checks.
type StaffDocumentActor struct {
	AccountID   int64
	Role        string
	Permissions []string
}

// StaffDocumentInfo is one document plus its advisory retention view; the
// dates are calendar days or empty.
type StaffDocumentInfo struct {
	Document    StaffDocument
	RetainUntil string
	ReviewDue   string
}

// CreateStaffDocumentInput carries the metadata of an already-stored upload.
type CreateStaffDocumentInput struct {
	StaffID         int64
	Category        string
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
}

// StaffDocuments is the Dokumente tab: authority, metadata rows, audit trail
// and the durable cleanup intents of stored files.
type StaffDocuments interface {
	ListStaffDocuments(ctx context.Context, staffID int64, category string, actor StaffDocumentActor) ([]StaffDocumentInfo, []string, error)
	CreateStaffDocument(ctx context.Context, input CreateStaffDocumentInput, actor StaffDocumentActor) (*StaffDocumentInfo, error)
	ResolveStaffDocumentDownload(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*StaffDocument, error)
	DeleteStaffDocument(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*StaffDocument, error)
	ResolveStaffDocumentCleanup(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*StaffDocument, error)
	ListStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64) ([]StaffDocument, error)
	ListOffboardedStaffDocumentsPendingFileCleanup(ctx context.Context) ([]StaffDocument, error)
	ListDeletedStaffDocumentsPendingFileCleanups(ctx context.Context) ([]StaffDocument, error)
	ListDeletedStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64, actor StaffDocumentActor) ([]StaffDocument, error)
	MarkStaffDocumentFileDeleted(ctx context.Context, documentID int64) error
	QueueStaffDocumentFileCleanup(ctx context.Context, staffID int64, storedName string) error
	ListQueuedStaffDocumentFileCleanup(ctx context.Context, staffID int64) ([]StaffDocumentFileCleanup, error)
	ListQueuedStaffDocumentFileCleanups(ctx context.Context) ([]StaffDocumentFileCleanup, error)
	MarkQueuedStaffDocumentFileCleanupComplete(ctx context.Context, cleanupID int64) error
	MarkQueuedStaffDocumentFileCleanupCompleteByFilename(ctx context.Context, storedName string) error
	ActivateQueuedStaffDocumentFileCleanup(ctx context.Context, storedName string) error
}
