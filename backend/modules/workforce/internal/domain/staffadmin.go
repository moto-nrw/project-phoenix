package domain

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// This file holds the rules of the personnel-record administration (#1417,
// #1423, #1424): what a Stammdaten section may contain, what the audit trail
// records about a change, how bank and tax values are masked, and which
// document category needs which authority. It performs no persistence; the
// application layer orders the reads and writes around these rules.

// Failures of the personnel-record administration. The wording is part of the
// HTTP contract: the handlers render it unchanged.
var (
	ErrStaffStammdatenInvalid    = errors.New("invalid stammdaten value")
	ErrStaffPersonnelNumberTaken = errors.New("personnel number already assigned to another staff member")
	ErrPersonnelNumberInvalid    = errors.New("invalid personnel number")
	ErrStaffDocumentInvalid      = errors.New("invalid staff document")
	ErrStaffDocumentForbidden    = errors.New("staff document category not permitted")
)

func invalidStammdaten(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrStaffStammdatenInvalid, fmt.Sprintf(format, args...))
}

// dateFormatError reports a calendar day that is not YYYY-MM-DD. It classifies
// as an invalid Stammdaten value but keeps the bare field wording.
type dateFormatError struct{ field string }

func (e dateFormatError) Error() string { return e.field + " must be YYYY-MM-DD" }

func (dateFormatError) Is(target error) bool { return target == ErrStaffStammdatenInvalid }

// parseOptionalDay validates an optional calendar day; nil stays nil.
func parseOptionalDay(value *string, field string) error {
	if value == nil {
		return nil
	}
	parsed, err := time.Parse(DateLayout, *value)
	if err != nil || parsed.Format(DateLayout) != *value {
		return dateFormatError{field: field}
	}
	return nil
}

// Audit sections of a Stammdaten change (the audit table's section column).
const (
	AuditSectionPerson         = "person"
	AuditSectionKontakt        = "kontakt"
	AuditSectionArbeitsvertrag = "arbeitsvertrag"
	AuditSectionQualifikation  = "qualifikationen"
	AuditSectionBankSteuer     = "bank-steuer"
	AuditSectionDokumente      = "dokumente"
)

// What a data-access log row records.
const (
	DataAccessFinancialView    = "financial_view"
	DataAccessFinancialReveal  = "financial_reveal"
	DataAccessDocumentDownload = "document_download"
)

const (
	unknownActorRole            = "unknown"
	maskedTailPrefix            = "•••• "
	maskedAll                   = "••••••••"
	maskedChangeSuffix          = " (geändert)"
	qualificationAcquiredPrefix = " (erworben "
	qualificationExpiresPrefix  = " (bis "
)

// Employment types a staff member may carry.
const (
	EmploymentTypeFullTime = "full_time"
	EmploymentTypePartTime = "part_time"
	EmploymentTypeMinijob  = "minijob"
)

// StaffSubject is the staff member behind a personnel record, with the person
// it belongs to when the row was read with it. Birthday and
// RotationAnchorDate are calendar days or empty.
type StaffSubject struct {
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

// FieldChange is one field diff destined for the audit trail.
type FieldChange struct {
	Field    string
	OldValue string
	NewValue string
}

// MasterDataChange is one Stammdaten audit row.
type MasterDataChange struct {
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

// DataAccess is one data-access log row: who read which sensitive value of
// which staff member.
type DataAccess struct {
	ActorAccountID int64
	ActorRole      string
	Resource       string
	StaffID        int64
	DocumentID     int64
	Category       string
	At             time.Time
}

// StaffDocumentUploadDeadline bounds every step an upload request may still
// perform once its cleanup intent exists: the file write and the metadata
// transaction. The handler enforces it, so no request can persist a document
// row after its queued intent has become eligible for the cleanup scheduler.
const StaffDocumentUploadDeadline = 2 * time.Minute

// StaffDocumentCleanupDelay keeps a queued intent ineligible until well past
// StaffDocumentUploadDeadline. The difference is deliberate slack: it covers a
// metadata transaction whose commit was still in flight when the deadline
// fired, so the cleanup pass only ever sees uploads whose outcome is settled.
// It MUST stay larger than StaffDocumentUploadDeadline.
const StaffDocumentCleanupDelay = 5 * time.Minute

// RecordNotFoundError is a missing personnel record row in the shape the
// repositories always reported it: the HTTP layer recognizes it by the
// RepositoryNotFound marker and renders this wording as the 404 body.
type RecordNotFoundError struct{ Op string }

func (e *RecordNotFoundError) Error() string {
	return "database error during " + e.Op + ": repository: not found\nsql: no rows in result set"
}

// RepositoryNotFound marks the error for the HTTP layer's not-found check.
func (*RecordNotFoundError) RepositoryNotFound() {}

// Is reports the document sentinel this failure stands for.
func (*RecordNotFoundError) Is(target error) bool { return target == ErrStaffDocumentNotFound }

// --- personnel number -------------------------------------------------------

// personnelNumberPattern accepts what both DATEV systems can match on: digits
// only. The length cap is deliberately looser than any single DATEV format —
// the writers enforce their exact per-format bounds; this layer only blocks
// obvious junk.
var personnelNumberPattern = regexp.MustCompile(`^\d{1,9}$`)

// NormalizePersonnelNumber trims the value; empty clears it (nil). A value
// other than digits is invalid.
func NormalizePersonnelNumber(value *string) (*string, error) {
	normalized := trimToNil(value)
	if normalized != nil && !personnelNumberPattern.MatchString(*normalized) {
		return nil, ErrPersonnelNumberInvalid
	}
	return normalized, nil
}

// --- Stammdaten sections ----------------------------------------------------

// PersonSection is the full person section: names and birthday live on the
// person, gender on the master data row.
type PersonSection struct {
	FirstName string
	LastName  string
	Birthday  *string
	Gender    *string
}

// ContactSection is the full contact section; nil clears a field.
type ContactSection struct {
	AddressStreet         *string
	AddressPostalCode     *string
	AddressCity           *string
	Phone                 *string
	Email                 *string
	EmergencyContactName  *string
	EmergencyContactPhone *string
}

// ContractSection is the full contract section. EmploymentType lives on the
// staff row, everything else on the master data row.
type ContractSection struct {
	EntryDate        *string
	ContractEndDate  *string
	ProbationEndDate *string
	WeeklyHours      *float64
	EmploymentType   *string
}

// QualificationInput is one row of the submitted qualification list.
type QualificationInput struct {
	Name       string
	AcquiredOn *string
	ExpiresOn  *string
}

// FinancialSection is the full bank and tax section; nil clears a field.
type FinancialSection struct {
	IBAN                 *string
	TaxID                *string
	SocialSecurityNumber *string
}

// NormalizePersonSection checks the calendar day first, then the names and the
// gender: both names are required, and a gender other than the three known
// values is refused.
func NormalizePersonSection(in PersonSection) (PersonSection, error) {
	if err := parseOptionalDay(in.Birthday, "birthday"); err != nil {
		return PersonSection{}, err
	}
	in.FirstName = strings.TrimSpace(in.FirstName)
	in.LastName = strings.TrimSpace(in.LastName)
	if in.FirstName == "" || in.LastName == "" {
		return PersonSection{}, invalidStammdaten("first and last name are required")
	}
	if in.Gender != nil {
		switch *in.Gender {
		case GenderFemale, GenderMale, GenderDiverse:
		default:
			return PersonSection{}, invalidStammdaten("unknown gender value")
		}
	}
	return in, nil
}

// NormalizeContactSection trims every field (empty clears it) and refuses an
// address without an @ or with whitespace in it.
func NormalizeContactSection(in ContactSection) (ContactSection, error) {
	in.AddressStreet = trimToNil(in.AddressStreet)
	in.AddressPostalCode = trimToNil(in.AddressPostalCode)
	in.AddressCity = trimToNil(in.AddressCity)
	in.Phone = trimToNil(in.Phone)
	in.Email = trimToNil(in.Email)
	in.EmergencyContactName = trimToNil(in.EmergencyContactName)
	in.EmergencyContactPhone = trimToNil(in.EmergencyContactPhone)
	if in.Email != nil && (!strings.Contains(*in.Email, "@") || strings.ContainsAny(*in.Email, " \t")) {
		return ContactSection{}, invalidStammdaten("malformed email address")
	}
	return in, nil
}

// NormalizeContractSection checks the three calendar days in order, then their
// order against the entry date, the weekly hours and the employment type.
func NormalizeContractSection(in ContractSection) (ContractSection, error) {
	for _, day := range []struct {
		value *string
		field string
	}{{in.EntryDate, "entry_date"}, {in.ContractEndDate, "contract_end_date"}, {in.ProbationEndDate, "probation_end_date"}} {
		if err := parseOptionalDay(day.value, day.field); err != nil {
			return ContractSection{}, err
		}
	}
	if in.EntryDate != nil && in.ContractEndDate != nil && *in.ContractEndDate < *in.EntryDate {
		return ContractSection{}, invalidStammdaten("contract end date must not be before entry date")
	}
	if in.EntryDate != nil && in.ProbationEndDate != nil && *in.ProbationEndDate < *in.EntryDate {
		return ContractSection{}, invalidStammdaten("probation end date must not be before entry date")
	}
	if in.WeeklyHours != nil && (*in.WeeklyHours < 0 || *in.WeeklyHours > maxWeeklyHours) {
		return ContractSection{}, invalidStammdaten("weekly hours must be between 0 and 80")
	}
	if in.EmploymentType != nil {
		switch *in.EmploymentType {
		case EmploymentTypeFullTime, EmploymentTypePartTime, EmploymentTypeMinijob:
		default:
			return ContractSection{}, invalidStammdaten("unknown employment type")
		}
	}
	return in, nil
}

// NormalizeQualifications checks every submitted calendar day first, then each
// row's name and date order, and returns the rows the store takes.
func NormalizeQualifications(inputs []QualificationInput) ([]StaffQualification, error) {
	for _, input := range inputs {
		if err := parseOptionalDay(input.AcquiredOn, "acquired_on"); err != nil {
			return nil, err
		}
		if err := parseOptionalDay(input.ExpiresOn, "expires_on"); err != nil {
			return nil, err
		}
	}
	rows := make([]StaffQualification, 0, len(inputs))
	for _, input := range inputs {
		name := strings.TrimSpace(input.Name)
		if name == "" {
			return nil, invalidStammdaten("qualification name is required")
		}
		if input.AcquiredOn != nil && input.ExpiresOn != nil && *input.ExpiresOn < *input.AcquiredOn {
			return nil, invalidStammdaten("qualification expiry before acquisition")
		}
		rows = append(rows, StaffQualification{Name: name, AcquiredOn: deref(input.AcquiredOn), ExpiresOn: deref(input.ExpiresOn)})
	}
	return rows, nil
}

var (
	ibanPattern  = regexp.MustCompile(`^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$`)
	taxIDPattern = regexp.MustCompile(`^\d{11}$`)
	// German Sozialversicherungsnummer: 2-digit area, 6-digit birth date,
	// initial letter, 2-digit serial + check digit.
	socialSecurityPattern = regexp.MustCompile(`^\d{8}[A-Z]\d{3}$`)
)

// NormalizeFinancialSection trims, uppercases and validates the bank and tax
// fields. Empty strings clear a field (nil).
func NormalizeFinancialSection(in FinancialSection) (FinancialSection, error) {
	out := FinancialSection{}
	if value := compactToNil(in.IBAN); value != nil {
		iban := strings.ToUpper(*value)
		if !ibanPattern.MatchString(iban) || !ibanChecksumValid(iban) {
			return out, invalidStammdaten("malformed IBAN")
		}
		out.IBAN = &iban
	}
	if value := compactToNil(in.TaxID); value != nil {
		if !taxIDPattern.MatchString(*value) {
			return out, invalidStammdaten("Steuer-ID must be 11 digits")
		}
		out.TaxID = value
	}
	if value := compactToNil(in.SocialSecurityNumber); value != nil {
		number := strings.ToUpper(*value)
		if !socialSecurityPattern.MatchString(number) {
			return out, invalidStammdaten("malformed SV-Nummer")
		}
		out.SocialSecurityNumber = &number
	}
	return out, nil
}

// ibanChecksumValid runs the ISO 13616 mod-97 check.
func ibanChecksumValid(iban string) bool {
	rearranged := iban[4:] + iban[:4]
	remainder := 0
	for _, r := range rearranged {
		switch {
		case r >= '0' && r <= '9':
			remainder = (remainder*10 + int(r-'0')) % 97
		case r >= 'A' && r <= 'Z':
			remainder = (remainder*100 + int(r-'A') + 10) % 97
		default:
			return false
		}
	}
	return remainder == 1
}

// --- audit diffs ------------------------------------------------------------

// PersonChanges lists what a person section changes. masterData is nil until
// the first section write.
func PersonChanges(subject StaffSubject, masterData *StaffMasterData, in PersonSection) []FieldChange {
	var gender *string
	if masterData != nil {
		gender = masterData.Gender
	}
	var changes []FieldChange
	appendChange(&changes, "first_name", subject.FirstName, in.FirstName)
	appendChange(&changes, "last_name", subject.LastName, in.LastName)
	appendChange(&changes, "birthday", subject.Birthday, deref(in.Birthday))
	appendChange(&changes, "gender", deref(gender), deref(in.Gender))
	return changes
}

// ContactChanges lists what a contact section changes.
func ContactChanges(masterData *StaffMasterData, in ContactSection) []FieldChange {
	current := StaffMasterData{}
	if masterData != nil {
		current = *masterData
	}
	var changes []FieldChange
	appendChange(&changes, "address_street", deref(current.AddressStreet), deref(in.AddressStreet))
	appendChange(&changes, "address_postal_code", deref(current.AddressPostalCode), deref(in.AddressPostalCode))
	appendChange(&changes, "address_city", deref(current.AddressCity), deref(in.AddressCity))
	appendChange(&changes, "phone", deref(current.Phone), deref(in.Phone))
	appendChange(&changes, "email", deref(current.Email), deref(in.Email))
	appendChange(&changes, "emergency_contact_name", deref(current.EmergencyContactName), deref(in.EmergencyContactName))
	appendChange(&changes, "emergency_contact_phone", deref(current.EmergencyContactPhone), deref(in.EmergencyContactPhone))
	return changes
}

// ContractChanges lists what a contract section changes.
func ContractChanges(subject StaffSubject, masterData *StaffMasterData, in ContractSection) []FieldChange {
	current := StaffMasterData{}
	if masterData != nil {
		current = *masterData
	}
	var changes []FieldChange
	appendChange(&changes, "entry_date", current.EntryDate, deref(in.EntryDate))
	appendChange(&changes, "contract_end_date", current.ContractEndDate, deref(in.ContractEndDate))
	appendChange(&changes, "probation_end_date", current.ProbationEndDate, deref(in.ProbationEndDate))
	appendChange(&changes, "weekly_hours", floatAuditValue(current.WeeklyHours), floatAuditValue(in.WeeklyHours))
	appendChange(&changes, "employment_type", deref(subject.EmploymentType), deref(in.EmploymentType))
	return changes
}

// QualificationChanges records the list as one field diff: the editor always
// submits the complete list, so row-level diffs would only obscure what it saw.
func QualificationChanges(existing, submitted []StaffQualification) []FieldChange {
	var changes []FieldChange
	appendChange(&changes, "qualifications", qualificationsAuditValue(existing), qualificationsAuditValue(submitted))
	return changes
}

// FinancialChanges lists what a bank and tax section changes. The no-op
// decision compares plaintext (a masked comparison would miss a change with
// identical last-4), but the rows carry masked values only — the trail must
// not become a second store of bank data outside the staff:financial gate.
func FinancialChanges(current *StaffFinancialData, in FinancialSection) []FieldChange {
	stored := StaffFinancialData{}
	if current != nil {
		stored = *current
	}
	var changes []FieldChange
	if deref(stored.IBAN) != deref(in.IBAN) {
		changes = append(changes, maskedChange("iban", maskTail(stored.IBAN, 4), maskTail(in.IBAN, 4)))
	}
	if deref(stored.TaxID) != deref(in.TaxID) {
		changes = append(changes, maskedChange("tax_id", maskAll(stored.TaxID), maskAll(in.TaxID)))
	}
	if deref(stored.SocialSecurityNumber) != deref(in.SocialSecurityNumber) {
		changes = append(changes, maskedChange("social_security_number", maskAll(stored.SocialSecurityNumber), maskAll(in.SocialSecurityNumber)))
	}
	return changes
}

// maskedChange builds a financial audit diff from masked values. When the
// plaintext changed but both masks render identically (same last-4, or the
// fixed full mask), the new value is suffixed so the audit row still shows a
// change and passes the old != new validation.
func maskedChange(field string, oldMasked, newMasked *string) FieldChange {
	oldValue := deref(oldMasked)
	newValue := deref(newMasked)
	if oldValue == newValue {
		newValue += maskedChangeSuffix
	}
	return FieldChange{Field: field, OldValue: oldValue, NewValue: newValue}
}

func appendChange(changes *[]FieldChange, field, oldValue, newValue string) {
	if oldValue == newValue {
		return
	}
	*changes = append(*changes, FieldChange{Field: field, OldValue: oldValue, NewValue: newValue})
}

func qualificationsAuditValue(rows []StaffQualification) string {
	if len(rows) == 0 {
		return ""
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		entry := row.Name
		if row.AcquiredOn != "" {
			entry += qualificationAcquiredPrefix + row.AcquiredOn + ")"
		}
		if row.ExpiresOn != "" {
			entry += qualificationExpiresPrefix + row.ExpiresOn + ")"
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, "; ")
}

func floatAuditValue(value *float64) string {
	if value == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", *value), "0"), ".")
}

// --- masking ----------------------------------------------------------------

// StaffFinancialMasked is the default financial read: the IBAN shows its last
// characters, tax id and social security number only that a value exists.
type StaffFinancialMasked struct {
	StaffID                    int64
	IBANMasked                 *string
	TaxIDMasked                *string
	SocialSecurityNumberMasked *string
}

// MaskFinancial masks stored bank and tax data; nil data masks to nothing.
func MaskFinancial(staffID int64, data *StaffFinancialData) StaffFinancialMasked {
	masked := StaffFinancialMasked{StaffID: staffID}
	if data != nil {
		masked.IBANMasked = maskTail(data.IBAN, 4)
		masked.TaxIDMasked = maskAll(data.TaxID)
		masked.SocialSecurityNumberMasked = maskAll(data.SocialSecurityNumber)
	}
	return masked
}

// maskTail masks all but the last visible characters: "•••• 1234".
func maskTail(value *string, visible int) *string {
	if value == nil || *value == "" {
		return nil
	}
	text := *value
	if len(text) <= visible {
		masked := strings.Repeat("•", len(text))
		return &masked
	}
	masked := maskedTailPrefix + text[len(text)-visible:]
	return &masked
}

// maskAll returns a fixed-length mask so not even the value length leaks.
func maskAll(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	masked := maskedAll
	return &masked
}

// ActorRoleOrUnknown is the role an access-log row carries.
func ActorRoleOrUnknown(role string) string {
	if strings.TrimSpace(role) == "" {
		return unknownActorRole
	}
	return role
}

// --- documents --------------------------------------------------------------

// DocumentActor identifies the acting account of a document operation.
type DocumentActor struct {
	AccountID   int64
	Role        string
	Permissions []string
}

// DocumentAuthority decides which document category a permission set may
// touch. The category-to-permission mapping is the whole point of the
// permission design, so it stays in one place: AU-Bescheinigungen are Art. 9
// health data, Lohnabrechnungen belong to the payroll tier, everything else
// to the personnel administrators.
type DocumentAuthority struct {
	// Health, Financial and General are the permissions the three tiers need.
	Health, Financial, General string
	// Allows evaluates a required permission against a held set with the
	// shared wildcard rules.
	Allows func(required string, held []string) bool
}

// Required returns the permission a category needs.
func (a DocumentAuthority) Required(category string) string {
	switch category {
	case StaffDocumentCategoryAUBescheinigung:
		return a.Health
	case StaffDocumentCategoryLohnabrechnung:
		return a.Financial
	default:
		return a.General
	}
}

// Visible filters the category enumeration down to what the actor may see.
func (a DocumentAuthority) Visible(actor DocumentActor) []string {
	visible := make([]string, 0, len(StaffDocumentCategories))
	for _, category := range StaffDocumentCategories {
		if a.Allows(a.Required(category), actor.Permissions) {
			visible = append(visible, category)
		}
	}
	return visible
}

// Require refuses a category the actor's permissions do not cover.
func (a DocumentAuthority) Require(category string, actor DocumentActor) error {
	if !a.Allows(a.Required(category), actor.Permissions) {
		return fmt.Errorf("%w: %s", ErrStaffDocumentForbidden, category)
	}
	return nil
}

// PendingFileCleanupFilter selects the documents of one staff member whose
// stored file still has to be removed, soft-deleted ones included.
func PendingFileCleanupFilter(staffID int64) StaffDocumentFilter {
	return StaffDocumentFilter{StaffID: staffID, IncludeDeleted: true, FilePending: true}
}

// OffboardedPendingFileCleanupFilter selects the documents of the given
// retired staff members whose stored file still has to be removed.
func OffboardedPendingFileCleanupFilter(staffIDs []int64) StaffDocumentFilter {
	return StaffDocumentFilter{StaffIDs: staffIDs, IncludeDeleted: true, FilePending: true}
}

// DeletedPendingFileCleanupFilter selects the soft-deleted documents whose
// stored file still has to be removed: every one when staffID is zero and
// categories is nil, otherwise the ones of that staff member in those
// categories.
func DeletedPendingFileCleanupFilter(staffID int64, categories []string) StaffDocumentFilter {
	return StaffDocumentFilter{StaffID: staffID, Categories: categories, DeletedOnly: true, FilePending: true}
}

// IsValidStaffDocumentCategory reports whether the category is one of the
// fixed enumeration.
func IsValidStaffDocumentCategory(category string) bool {
	return slices.Contains(StaffDocumentCategories, category)
}

// SensitiveStaffDocumentCategory reports whether serving the category's file
// content requires a data-access log row.
func SensitiveStaffDocumentCategory(category string) bool {
	return category == StaffDocumentCategoryAUBescheinigung || category == StaffDocumentCategoryLohnabrechnung
}

// StaffDocumentInfo is one document plus its derived retention view.
// RetainUntil is the earliest calendar day the category's statutory retention
// allows disposal; ReviewDue is the BDSG-motivated review date for
// application documents. Both are advisory and empty when they do not apply:
// nothing is ever deleted automatically, and deletes are not blocked either (a
// wrong upload must stay removable).
type StaffDocumentInfo struct {
	Document    StaffDocument
	RetainUntil string
	ReviewDue   string
}

// NewStaffDocumentInfo derives the advisory retention view of one document.
// Lohnabrechnungen 10 years (tax law), AU-Bescheinigungen 4 years (tax audit
// analogy), Arbeitsvertrag contract end + 6 years, Bewerbungsunterlagen get a
// 6-month review date (BDSG), Zeugnis and Sonstiges carry no schedule.
// uploadedOn is the Berlin calendar day of the upload; contractEnd is the
// master data's contract end or empty (unbefristet: retention stays open).
func NewStaffDocumentInfo(document StaffDocument, uploadedOn, contractEnd string) StaffDocumentInfo {
	info := StaffDocumentInfo{Document: document}
	switch document.Category {
	case StaffDocumentCategoryLohnabrechnung:
		info.RetainUntil = addToDay(uploadedOn, 10, 0)
	case StaffDocumentCategoryAUBescheinigung:
		info.RetainUntil = addToDay(uploadedOn, 4, 0)
	case StaffDocumentCategoryArbeitsvertrag:
		if contractEnd != "" {
			info.RetainUntil = addToDay(contractEnd, 6, 0)
		}
	case StaffDocumentCategoryBewerbung:
		info.ReviewDue = addToDay(uploadedOn, 0, 6)
	}
	return info
}

// addToDay shifts a calendar day by whole years and months, normalized the way
// time.Date normalizes out-of-range components (Feb 29 + 1y → Mar 1).
func addToDay(day string, years, months int) string {
	parsed, err := time.Parse(DateLayout, day)
	if err != nil {
		return ""
	}
	return time.Date(parsed.Year()+years, parsed.Month()+time.Month(months), parsed.Day(), 0, 0, 0, 0, time.UTC).Format(DateLayout)
}

// --- helpers ----------------------------------------------------------------

func trimToNil(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func compactToNil(value *string) *string {
	if value == nil {
		return nil
	}
	compact := strings.ReplaceAll(strings.TrimSpace(*value), " ", "")
	if compact == "" {
		return nil
	}
	return &compact
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// Deref renders an optional string as the empty audit value when unset.
func Deref(value *string) string { return deref(value) }
