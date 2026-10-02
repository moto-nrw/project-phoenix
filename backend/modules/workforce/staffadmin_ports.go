package workforce

import (
	"context"
	"time"
)

// Audit sections of a Stammdaten change: the values of the audit table's
// section column.
const (
	StaffAuditSectionPerson         = "person"
	StaffAuditSectionKontakt        = "kontakt"
	StaffAuditSectionArbeitsvertrag = "arbeitsvertrag"
	StaffAuditSectionQualifikation  = "qualifikationen"
	StaffAuditSectionBankSteuer     = "bank-steuer"
	StaffAuditSectionDokumente      = "dokumente"
)

// What a data-access log row of the personnel record records.
const (
	StaffDataAccessFinancialView    = "financial_view"
	StaffDataAccessFinancialReveal  = "financial_reveal"
	StaffDataAccessDocumentDownload = "document_download"
)

// StaffMasterDataChange is one field-level Stammdaten audit row.
type StaffMasterDataChange struct {
	StaffID   int64
	ChangedBy int64
	Section   string
	Field     string
	OldValue  string
	NewValue  string
	Note      string
}

// PersonnelNumberChange is one Personalnummer audit row.
type PersonnelNumberChange struct {
	StaffID   int64
	ChangedBy int64
	OldValue  string
	NewValue  string
	Note      string
}

// StaffDataAccess is one data-access log row: which account read which
// sensitive value of which staff member. DocumentID and Category are set for
// a document download only.
type StaffDataAccess struct {
	ActorAccountID int64
	ActorRole      string
	Resource       string
	StaffID        int64
	DocumentID     int64
	Category       string
	At             time.Time
}

// StaffAdminSubjects is the consumer-owned port over the staff and person rows
// behind a personnel record. School Membership and People Directory own them;
// Workforce never reads or writes them itself. Every failure passes through
// unchanged, so a missing row keeps the repository not-found shape the HTTP
// layer classifies as 404.
type StaffAdminSubjects interface {
	// StaffWithPerson reads a live staff member together with its person.
	StaffWithPerson(ctx context.Context, staffID int64) (StaffProfile, error)
	// StaffExists proves the live staff member is visible in the tenant.
	StaffExists(ctx context.Context, staffID int64) error
	// LockStaff locks the staff row for the caller's transaction and, with
	// withPerson, the person row too, so a person-section diff reads the last
	// committed values.
	LockStaff(ctx context.Context, staffID int64, withPerson bool) (StaffProfile, error)
	// UpdatePerson writes the names and the birthday (a calendar day or nil)
	// of a locked person.
	UpdatePerson(ctx context.Context, personID int64, firstName, lastName string, birthday *string) error
	// SetEmploymentType writes the employment type of a locked staff row.
	SetEmploymentType(ctx context.Context, staffID int64, value *string) error
	// SetPersonnelNumber writes the personnel number of a locked staff row and
	// reports ErrPersonnelNumberTaken when another staff member of the tenant
	// already holds it.
	SetPersonnelNumber(ctx context.Context, staffID int64, value *string) error
	// OffboardedStaffIDs returns every staff member School Membership has
	// retired, so their leftover document files can be cleaned up.
	OffboardedStaffIDs(ctx context.Context) ([]int64, error)
}

// StaffAdminAudit is the consumer-owned port over the audit trail of the
// personnel record. Each call joins the caller's tenant transaction.
type StaffAdminAudit interface {
	RecordMasterDataChange(ctx context.Context, change StaffMasterDataChange) error
	RecordPersonnelNumberChange(ctx context.Context, change PersonnelNumberChange) error
	RecordDataAccess(ctx context.Context, access StaffDataAccess) error
}

// staffAdminEngine is the composed personnel-record administration.
type staffAdminEngine interface {
	StaffRecordAdmin
	StaffDocuments
}

// StaffAdmin is the public facade of the personnel-record administration: the
// Stammdaten sections, the payroll number, the audited financial reads and
// the Dokumente tab. It serves StaffRecordAdmin and StaffDocuments.
type StaffAdmin struct{ engine staffAdminEngine }

// NewStaffAdmin wraps the composed engine.
func NewStaffAdmin(engine staffAdminEngine) *StaffAdmin {
	if engine == nil {
		panic("workforce: staff admin engine is required")
	}
	return &StaffAdmin{engine: engine}
}

func (a *StaffAdmin) UpdatePersonnelNumber(ctx context.Context, staffID int64, value *string, changedByStaffID int64, note string) (*StaffProfile, error) {
	return a.engine.UpdatePersonnelNumber(ctx, staffID, value, changedByStaffID, note)
}

func (a *StaffAdmin) StaffStammdaten(ctx context.Context, staffID int64) (*StaffStammdaten, error) {
	return a.engine.StaffStammdaten(ctx, staffID)
}

func (a *StaffAdmin) UpdateStaffStammdatenPerson(ctx context.Context, staffID int64, input StammdatenPersonInput, changedByStaffID int64, note string) error {
	return a.engine.UpdateStaffStammdatenPerson(ctx, staffID, input, changedByStaffID, note)
}

func (a *StaffAdmin) UpdateStaffStammdatenKontakt(ctx context.Context, staffID int64, input StammdatenKontaktInput, changedByStaffID int64, note string) error {
	return a.engine.UpdateStaffStammdatenKontakt(ctx, staffID, input, changedByStaffID, note)
}

func (a *StaffAdmin) UpdateStaffStammdatenArbeitsvertrag(ctx context.Context, staffID int64, input StammdatenArbeitsvertragInput, changedByStaffID int64, note string) error {
	return a.engine.UpdateStaffStammdatenArbeitsvertrag(ctx, staffID, input, changedByStaffID, note)
}

func (a *StaffAdmin) ReplaceStaffQualificationList(ctx context.Context, staffID int64, inputs []StammdatenQualificationInput, changedByStaffID int64, note string) error {
	return a.engine.ReplaceStaffQualificationList(ctx, staffID, inputs, changedByStaffID, note)
}

func (a *StaffAdmin) StaffFinancialMasked(ctx context.Context, staffID, actorAccountID int64, actorRole string) (*StaffFinancialMasked, error) {
	return a.engine.StaffFinancialMasked(ctx, staffID, actorAccountID, actorRole)
}

func (a *StaffAdmin) RevealStaffFinancial(ctx context.Context, staffID, actorAccountID int64, actorRole string) (*StaffFinancialPlain, error) {
	return a.engine.RevealStaffFinancial(ctx, staffID, actorAccountID, actorRole)
}

func (a *StaffAdmin) UpdateStaffFinancial(ctx context.Context, staffID int64, input StammdatenFinancialInput, changedByAccountID int64, note string) error {
	return a.engine.UpdateStaffFinancial(ctx, staffID, input, changedByAccountID, note)
}

func (a *StaffAdmin) ListStaffDocuments(ctx context.Context, staffID int64, category string, actor StaffDocumentActor) ([]StaffDocumentInfo, []string, error) {
	return a.engine.ListStaffDocuments(ctx, staffID, category, actor)
}

func (a *StaffAdmin) CreateStaffDocument(ctx context.Context, input CreateStaffDocumentInput, actor StaffDocumentActor) (*StaffDocumentInfo, error) {
	return a.engine.CreateStaffDocument(ctx, input, actor)
}

func (a *StaffAdmin) ResolveStaffDocumentDownload(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*StaffDocument, error) {
	return a.engine.ResolveStaffDocumentDownload(ctx, staffID, documentID, actor)
}

func (a *StaffAdmin) DeleteStaffDocument(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*StaffDocument, error) {
	return a.engine.DeleteStaffDocument(ctx, staffID, documentID, actor)
}

func (a *StaffAdmin) ResolveStaffDocumentCleanup(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*StaffDocument, error) {
	return a.engine.ResolveStaffDocumentCleanup(ctx, staffID, documentID, actor)
}

func (a *StaffAdmin) ListStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64) ([]StaffDocument, error) {
	return a.engine.ListStaffDocumentsPendingFileCleanup(ctx, staffID)
}

func (a *StaffAdmin) ListOffboardedStaffDocumentsPendingFileCleanup(ctx context.Context) ([]StaffDocument, error) {
	return a.engine.ListOffboardedStaffDocumentsPendingFileCleanup(ctx)
}

func (a *StaffAdmin) ListDeletedStaffDocumentsPendingFileCleanups(ctx context.Context) ([]StaffDocument, error) {
	return a.engine.ListDeletedStaffDocumentsPendingFileCleanups(ctx)
}

func (a *StaffAdmin) ListDeletedStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64, actor StaffDocumentActor) ([]StaffDocument, error) {
	return a.engine.ListDeletedStaffDocumentsPendingFileCleanup(ctx, staffID, actor)
}

func (a *StaffAdmin) MarkStaffDocumentFileDeleted(ctx context.Context, documentID int64) error {
	return a.engine.MarkStaffDocumentFileDeleted(ctx, documentID)
}

func (a *StaffAdmin) QueueStaffDocumentFileCleanup(ctx context.Context, staffID int64, storedName string) error {
	return a.engine.QueueStaffDocumentFileCleanup(ctx, staffID, storedName)
}

func (a *StaffAdmin) ListQueuedStaffDocumentFileCleanup(ctx context.Context, staffID int64) ([]StaffDocumentFileCleanup, error) {
	return a.engine.ListQueuedStaffDocumentFileCleanup(ctx, staffID)
}

func (a *StaffAdmin) ListQueuedStaffDocumentFileCleanups(ctx context.Context) ([]StaffDocumentFileCleanup, error) {
	return a.engine.ListQueuedStaffDocumentFileCleanups(ctx)
}

func (a *StaffAdmin) MarkQueuedStaffDocumentFileCleanupComplete(ctx context.Context, cleanupID int64) error {
	return a.engine.MarkQueuedStaffDocumentFileCleanupComplete(ctx, cleanupID)
}

func (a *StaffAdmin) MarkQueuedStaffDocumentFileCleanupCompleteByFilename(ctx context.Context, storedName string) error {
	return a.engine.MarkQueuedStaffDocumentFileCleanupCompleteByFilename(ctx, storedName)
}

func (a *StaffAdmin) ActivateQueuedStaffDocumentFileCleanup(ctx context.Context, storedName string) error {
	return a.engine.ActivateQueuedStaffDocumentFileCleanup(ctx, storedName)
}
