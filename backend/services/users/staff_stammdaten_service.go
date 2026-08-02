package users

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Staff Stammdaten (#1423). Section-scoped reads and full-section writes for
// the staff master data tab. Two hard rules carried over from the
// Personalnummer path (#1417): no change without an audit row in the same
// tenant transaction, and no financial read without a data-access log row —
// the service refuses to serve IBAN / Steuer-ID / SV-Nummer when the audit
// write fails. Symbols are prefixed StaffStammdaten* because this package
// already owns the *student* master-data review flow (MasterDataReview*).

// ErrStaffStammdatenInvalid marks a semantically invalid section payload —
// HTTP 400. Wrapped with a field-specific message.
var ErrStaffStammdatenInvalid = errors.New("invalid stammdaten value")

var (
	ibanPattern  = regexp.MustCompile(`^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$`)
	taxIDPattern = regexp.MustCompile(`^\d{11}$`)
	// German Sozialversicherungsnummer: 2-digit area, 6-digit birth date,
	// initial letter, 2-digit serial + check digit.
	socialSecurityPattern = regexp.MustCompile(`^\d{8}[A-Z]\d{3}$`)
)

// StaffStammdaten aggregates the non-sensitive sections for the tab's GET.
// Financial data is deliberately absent — it has its own permission-gated
// read path.
type StaffStammdaten struct {
	Staff          *userModels.Staff
	MasterData     *userModels.StaffMasterData
	Qualifications []*userModels.StaffQualification
}

// StammdatenPersonInput is the full person section: names and birthday live
// on users.persons, gender on users.staff_master_data.
type StammdatenPersonInput struct {
	FirstName string
	LastName  string
	Birthday  *timezone.Date
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
// EmploymentType lives on users.staff, everything else on staff_master_data.
type StammdatenArbeitsvertragInput struct {
	EntryDate        *timezone.Date
	ContractEndDate  *timezone.Date
	ProbationEndDate *timezone.Date
	WeeklyHours      *float64
	EmploymentType   *string
}

// StammdatenQualificationInput is one qualification row of the submitted
// full list.
type StammdatenQualificationInput struct {
	Name       string
	AcquiredOn *timezone.Date
	ExpiresOn  *timezone.Date
}

// StammdatenFinancialInput is the full bank & tax section; nil clears a
// field.
type StammdatenFinancialInput struct {
	IBAN                 *string
	TaxID                *string
	SocialSecurityNumber *string
}

// StaffFinancialMasked is the default (masked) financial read: IBAN shows
// its last four characters, Steuer-ID and SV-Nummer only that a value is
// stored. Full values require RevealStaffFinancial.
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

// stammdatenChange is one field diff destined for the audit trail.
type stammdatenChange struct {
	field    string
	oldValue string
	newValue string
}

// GetStaffStammdaten returns the non-sensitive Stammdaten sections.
// MasterData is nil until the first section write.
func (s *personService) GetStaffStammdaten(ctx context.Context, staffID int64) (*StaffStammdaten, error) {
	staff, err := s.GetStaffWithPerson(ctx, staffID)
	if err != nil {
		return nil, err
	}
	masterData, err := s.StaffMasterDataRepo.FindByStaffID(ctx, staffID)
	if err != nil {
		return nil, err
	}
	qualifications, err := s.StaffQualificationRepo.ListByStaffID(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return &StaffStammdaten{Staff: staff, MasterData: masterData, Qualifications: qualifications}, nil
}

// UpdateStaffStammdatenPerson replaces the person section. First and last
// name are required; a no-op submit writes no audit row.
func (s *personService) UpdateStaffStammdatenPerson(ctx context.Context, staffID int64, input StammdatenPersonInput, changedByStaffID int64, note string) error {
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	if input.FirstName == "" || input.LastName == "" {
		return fmt.Errorf("%w: first and last name are required", ErrStaffStammdatenInvalid)
	}
	if input.Gender != nil {
		switch *input.Gender {
		case userModels.GenderFemale, userModels.GenderMale, userModels.GenderDiverse:
		default:
			return fmt.Errorf("%w: unknown gender value", ErrStaffStammdatenInvalid)
		}
	}

	return s.runStammdatenTx(ctx, staffID, changedByStaffID, func(ctx context.Context, staff *userModels.Staff) ([]stammdatenChange, func() error, error) {
		person := staff.Person
		if person == nil {
			return nil, nil, fmt.Errorf("staff %d has no person record", staffID)
		}
		masterData, err := s.StaffMasterDataRepo.FindByStaffID(ctx, staffID)
		if err != nil {
			return nil, nil, err
		}

		var changes []stammdatenChange
		appendChange(&changes, "first_name", person.FirstName, input.FirstName)
		appendChange(&changes, "last_name", person.LastName, input.LastName)
		appendChange(&changes, "birthday", dateAuditValue(person.Birthday), dateAuditValue(input.Birthday))
		appendChange(&changes, "gender", strPtrAuditValue(masterDataGender(masterData)), strPtrAuditValue(input.Gender))

		apply := func() error {
			person.FirstName = input.FirstName
			person.LastName = input.LastName
			person.Birthday = input.Birthday
			if err := s.PersonRepo.Update(ctx, person); err != nil {
				return err
			}
			return s.upsertMasterData(ctx, staffID, masterData, func(md *userModels.StaffMasterData) {
				md.Gender = input.Gender
			})
		}
		return changes, apply, nil
	}, auditModels.StammdatenSectionPerson, note)
}

// UpdateStaffStammdatenKontakt replaces the contact section.
func (s *personService) UpdateStaffStammdatenKontakt(ctx context.Context, staffID int64, input StammdatenKontaktInput, changedByStaffID int64, note string) error {
	input.AddressStreet = normalizeOptional(input.AddressStreet)
	input.AddressPostalCode = normalizeOptional(input.AddressPostalCode)
	input.AddressCity = normalizeOptional(input.AddressCity)
	input.Phone = normalizeOptional(input.Phone)
	input.Email = normalizeOptional(input.Email)
	input.EmergencyContactName = normalizeOptional(input.EmergencyContactName)
	input.EmergencyContactPhone = normalizeOptional(input.EmergencyContactPhone)

	if input.Email != nil && (!strings.Contains(*input.Email, "@") || strings.ContainsAny(*input.Email, " \t")) {
		return fmt.Errorf("%w: malformed email address", ErrStaffStammdatenInvalid)
	}

	return s.runStammdatenTx(ctx, staffID, changedByStaffID, func(ctx context.Context, _ *userModels.Staff) ([]stammdatenChange, func() error, error) {
		masterData, err := s.StaffMasterDataRepo.FindByStaffID(ctx, staffID)
		if err != nil {
			return nil, nil, err
		}

		var changes []stammdatenChange
		appendChange(&changes, "address_street", masterDataStr(masterData, func(m *userModels.StaffMasterData) *string { return m.AddressStreet }), strPtrAuditValue(input.AddressStreet))
		appendChange(&changes, "address_postal_code", masterDataStr(masterData, func(m *userModels.StaffMasterData) *string { return m.AddressPostalCode }), strPtrAuditValue(input.AddressPostalCode))
		appendChange(&changes, "address_city", masterDataStr(masterData, func(m *userModels.StaffMasterData) *string { return m.AddressCity }), strPtrAuditValue(input.AddressCity))
		appendChange(&changes, "phone", masterDataStr(masterData, func(m *userModels.StaffMasterData) *string { return m.Phone }), strPtrAuditValue(input.Phone))
		appendChange(&changes, "email", masterDataStr(masterData, func(m *userModels.StaffMasterData) *string { return m.Email }), strPtrAuditValue(input.Email))
		appendChange(&changes, "emergency_contact_name", masterDataStr(masterData, func(m *userModels.StaffMasterData) *string { return m.EmergencyContactName }), strPtrAuditValue(input.EmergencyContactName))
		appendChange(&changes, "emergency_contact_phone", masterDataStr(masterData, func(m *userModels.StaffMasterData) *string { return m.EmergencyContactPhone }), strPtrAuditValue(input.EmergencyContactPhone))

		apply := func() error {
			return s.upsertMasterData(ctx, staffID, masterData, func(md *userModels.StaffMasterData) {
				md.AddressStreet = input.AddressStreet
				md.AddressPostalCode = input.AddressPostalCode
				md.AddressCity = input.AddressCity
				md.Phone = input.Phone
				md.Email = input.Email
				md.EmergencyContactName = input.EmergencyContactName
				md.EmergencyContactPhone = input.EmergencyContactPhone
			})
		}
		return changes, apply, nil
	}, auditModels.StammdatenSectionKontakt, note)
}

// UpdateStaffStammdatenArbeitsvertrag replaces the contract section.
func (s *personService) UpdateStaffStammdatenArbeitsvertrag(ctx context.Context, staffID int64, input StammdatenArbeitsvertragInput, changedByStaffID int64, note string) error {
	if input.WeeklyHours != nil && (*input.WeeklyHours < 0 || *input.WeeklyHours > 80) {
		return fmt.Errorf("%w: weekly hours must be between 0 and 80", ErrStaffStammdatenInvalid)
	}
	if input.EmploymentType != nil {
		switch *input.EmploymentType {
		case userModels.EmploymentTypeFullTime, userModels.EmploymentTypePartTime, userModels.EmploymentTypeMinijob:
		default:
			return fmt.Errorf("%w: unknown employment type", ErrStaffStammdatenInvalid)
		}
	}

	return s.runStammdatenTx(ctx, staffID, changedByStaffID, func(ctx context.Context, staff *userModels.Staff) ([]stammdatenChange, func() error, error) {
		masterData, err := s.StaffMasterDataRepo.FindByStaffID(ctx, staffID)
		if err != nil {
			return nil, nil, err
		}

		var changes []stammdatenChange
		appendChange(&changes, "entry_date", masterDataDate(masterData, func(m *userModels.StaffMasterData) *timezone.Date { return m.EntryDate }), dateAuditValue(input.EntryDate))
		appendChange(&changes, "contract_end_date", masterDataDate(masterData, func(m *userModels.StaffMasterData) *timezone.Date { return m.ContractEndDate }), dateAuditValue(input.ContractEndDate))
		appendChange(&changes, "probation_end_date", masterDataDate(masterData, func(m *userModels.StaffMasterData) *timezone.Date { return m.ProbationEndDate }), dateAuditValue(input.ProbationEndDate))
		appendChange(&changes, "weekly_hours", floatAuditValue(masterDataFloat(masterData)), floatAuditValue(input.WeeklyHours))
		appendChange(&changes, "employment_type", strPtrAuditValue(staff.EmploymentType), strPtrAuditValue(input.EmploymentType))

		apply := func() error {
			if strPtrAuditValue(staff.EmploymentType) != strPtrAuditValue(input.EmploymentType) {
				staff.EmploymentType = input.EmploymentType
				if err := s.StaffRepo.Update(ctx, staff); err != nil {
					return err
				}
			}
			return s.upsertMasterData(ctx, staffID, masterData, func(md *userModels.StaffMasterData) {
				md.EntryDate = input.EntryDate
				md.ContractEndDate = input.ContractEndDate
				md.ProbationEndDate = input.ProbationEndDate
				md.WeeklyHours = input.WeeklyHours
			})
		}
		return changes, apply, nil
	}, auditModels.StammdatenSectionArbeitsvertrag, note)
}

// ReplaceStaffQualifications replaces the full qualification list. The audit
// trail records the list as one field diff — the modal always submits the
// complete list, so row-level diffs would only obscure what the editor saw.
func (s *personService) ReplaceStaffQualifications(ctx context.Context, staffID int64, inputs []StammdatenQualificationInput, changedByStaffID int64, note string) error {
	rows := make([]*userModels.StaffQualification, 0, len(inputs))
	for _, in := range inputs {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return fmt.Errorf("%w: qualification name is required", ErrStaffStammdatenInvalid)
		}
		if in.AcquiredOn != nil && in.ExpiresOn != nil && in.ExpiresOn.Before(*in.AcquiredOn) {
			return fmt.Errorf("%w: qualification expiry before acquisition", ErrStaffStammdatenInvalid)
		}
		rows = append(rows, &userModels.StaffQualification{
			StaffID:    staffID,
			Name:       name,
			AcquiredOn: in.AcquiredOn,
			ExpiresOn:  in.ExpiresOn,
		})
	}

	return s.runStammdatenTx(ctx, staffID, changedByStaffID, func(ctx context.Context, _ *userModels.Staff) ([]stammdatenChange, func() error, error) {
		existing, err := s.StaffQualificationRepo.ListByStaffID(ctx, staffID)
		if err != nil {
			return nil, nil, err
		}

		var changes []stammdatenChange
		appendChange(&changes, "qualifications", qualificationsAuditValue(existing), qualificationsAuditValue(rows))

		apply := func() error {
			return s.StaffQualificationRepo.ReplaceForStaff(ctx, staffID, rows)
		}
		return changes, apply, nil
	}, auditModels.StammdatenSectionQualifikation, note)
}

// GetStaffFinancialMasked serves the masked bank & tax data and logs the
// read. No data leaves without a successful audit row.
func (s *personService) GetStaffFinancialMasked(ctx context.Context, staffID int64, actorAccountID int64, actorRole string) (*StaffFinancialMasked, error) {
	data, err := s.loadFinancialAudited(ctx, staffID, actorAccountID, actorRole, auditModels.ResourceTypeStaffFinancialView)
	if err != nil {
		return nil, err
	}
	masked := &StaffFinancialMasked{StaffID: staffID}
	if data != nil {
		masked.IBANMasked = maskTailPtr(data.IBAN, 4)
		masked.TaxIDMasked = maskAllPtr(data.TaxID)
		masked.SocialSecurityNumberMasked = maskAllPtr(data.SocialSecurityNumber)
	}
	return masked, nil
}

// RevealStaffFinancial serves the full bank & tax values after the explicit
// "Anzeigen" toggle and logs the reveal.
func (s *personService) RevealStaffFinancial(ctx context.Context, staffID int64, actorAccountID int64, actorRole string) (*StaffFinancialPlain, error) {
	data, err := s.loadFinancialAudited(ctx, staffID, actorAccountID, actorRole, auditModels.ResourceTypeStaffFinancialReveal)
	if err != nil {
		return nil, err
	}
	plain := &StaffFinancialPlain{StaffID: staffID}
	if data != nil {
		plain.IBAN = data.IBAN
		plain.TaxID = data.TaxID
		plain.SocialSecurityNumber = data.SocialSecurityNumber
	}
	return plain, nil
}

// UpdateStaffFinancial replaces the bank & tax section. Audit rows carry
// masked values only.
func (s *personService) UpdateStaffFinancial(ctx context.Context, staffID int64, input StammdatenFinancialInput, changedByStaffID int64, note string) error {
	normalized, err := normalizeFinancialInput(input)
	if err != nil {
		return err
	}

	return s.runStammdatenTx(ctx, staffID, changedByStaffID, func(ctx context.Context, _ *userModels.Staff) ([]stammdatenChange, func() error, error) {
		data, err := s.StaffFinancialRepo.FindByStaffID(ctx, staffID)
		if err != nil {
			return nil, nil, err
		}

		var current userModels.StaffFinancialData
		if data != nil {
			current = *data
		}

		// The no-op decision compares plaintext (a masked comparison would
		// miss a change with identical last-4), but the audit rows carry
		// masked values only — the trail must not become a second store of
		// bank data outside the staff:financial gate.
		var changes []stammdatenChange
		if strPtrAuditValue(current.IBAN) != strPtrAuditValue(normalized.IBAN) {
			changes = append(changes, maskedChange("iban", maskTailPtr(current.IBAN, 4), maskTailPtr(normalized.IBAN, 4)))
		}
		if strPtrAuditValue(current.TaxID) != strPtrAuditValue(normalized.TaxID) {
			changes = append(changes, maskedChange("tax_id", maskAllPtr(current.TaxID), maskAllPtr(normalized.TaxID)))
		}
		if strPtrAuditValue(current.SocialSecurityNumber) != strPtrAuditValue(normalized.SocialSecurityNumber) {
			changes = append(changes, maskedChange("social_security_number", maskAllPtr(current.SocialSecurityNumber), maskAllPtr(normalized.SocialSecurityNumber)))
		}

		apply := func() error {
			if data == nil {
				fresh := &userModels.StaffFinancialData{StaffID: staffID}
				fresh.IBAN = normalized.IBAN
				fresh.TaxID = normalized.TaxID
				fresh.SocialSecurityNumber = normalized.SocialSecurityNumber
				return s.StaffFinancialRepo.Create(ctx, fresh)
			}
			data.IBAN = normalized.IBAN
			data.TaxID = normalized.TaxID
			data.SocialSecurityNumber = normalized.SocialSecurityNumber
			return s.StaffFinancialRepo.Update(ctx, data)
		}
		return changes, apply, nil
	}, auditModels.StammdatenSectionBankSteuer, note)
}

// --- internals -------------------------------------------------------------

// runStammdatenTx is the shared write shape: lock the staff row, let the
// section compute its field diffs and an apply closure, then write the audit
// rows BEFORE applying, all in one tenant transaction. No diffs → no-op.
func (s *personService) runStammdatenTx(
	ctx context.Context,
	staffID int64,
	changedByStaffID int64,
	build func(ctx context.Context, staff *userModels.Staff) ([]stammdatenChange, func() error, error),
	section string,
	note string,
) error {
	if s.StammdatenAudit == nil {
		return fmt.Errorf("stammdaten audit repository is not wired; refusing unaudited change")
	}
	if changedByStaffID <= 0 {
		return fmt.Errorf("changed-by staff id is required")
	}

	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		staff, err := s.StaffRepo.FindByIDForUpdate(ctx, staffID)
		if err != nil {
			return err
		}
		if staff.Person == nil {
			// FindByIDForUpdate does not preload; lock the person row too so
			// person-section diffs read the last committed values.
			person, err := s.PersonRepo.FindByIDForUpdate(ctx, staff.PersonID)
			if err != nil {
				return err
			}
			staff.Person = person
		}

		changes, apply, err := build(ctx, staff)
		if err != nil {
			return err
		}
		if len(changes) == 0 {
			return nil
		}

		trimmedNote := strings.TrimSpace(note)
		for _, change := range changes {
			if err := s.StammdatenAudit.Create(ctx, &auditModels.StaffMasterDataChange{
				StaffID:   staff.ID,
				ChangedBy: changedByStaffID,
				Section:   section,
				FieldName: change.field,
				OldValue:  change.oldValue,
				NewValue:  change.newValue,
				Note:      trimmedNote,
			}); err != nil {
				return fmt.Errorf("write stammdaten audit: %w", err)
			}
		}
		return apply()
	})
}

// upsertMasterData creates or updates the 1:1 master-data row, mutating it
// through set before persisting.
func (s *personService) upsertMasterData(ctx context.Context, staffID int64, existing *userModels.StaffMasterData, set func(*userModels.StaffMasterData)) error {
	if existing == nil {
		fresh := &userModels.StaffMasterData{StaffID: staffID}
		set(fresh)
		if err := fresh.Validate(); err != nil {
			return fmt.Errorf("%w: %s", ErrStaffStammdatenInvalid, err.Error())
		}
		return s.StaffMasterDataRepo.Create(ctx, fresh)
	}
	set(existing)
	if err := existing.Validate(); err != nil {
		return fmt.Errorf("%w: %s", ErrStaffStammdatenInvalid, err.Error())
	}
	return s.StaffMasterDataRepo.Update(ctx, existing)
}

// loadFinancialAudited resolves the staff member, loads the financial row
// (nil when none exists) and writes the data-access log entry. Callers must
// not serve any value when the returned error is non-nil.
func (s *personService) loadFinancialAudited(ctx context.Context, staffID int64, actorAccountID int64, actorRole string, resourceType string) (*userModels.StaffFinancialData, error) {
	if s.DataAccessLog == nil {
		return nil, fmt.Errorf("data access log repository is not wired; refusing unaudited financial read")
	}
	if actorAccountID <= 0 {
		return nil, fmt.Errorf("actor account id is required for financial reads")
	}
	if strings.TrimSpace(actorRole) == "" {
		actorRole = "unknown"
	}

	// Existence (and tenant scope) check before anything is disclosed.
	if _, err := s.GetStaffByID(ctx, staffID); err != nil {
		return nil, err
	}

	data, err := s.StaffFinancialRepo.FindByStaffID(ctx, staffID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := s.DataAccessLog.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: actorAccountID,
		ActorRole:      actorRole,
		ResourceType:   resourceType,
		RangeStart:     now,
		RangeEnd:       now,
		AccessedAt:     now,
		Metadata: map[string]interface{}{
			"staff_id": staffID,
		},
	}); err != nil {
		return nil, fmt.Errorf("write financial access audit: %w", err)
	}
	return data, nil
}

// normalizeFinancialInput trims, uppercases and validates the financial
// fields. Empty strings clear a field (nil).
func normalizeFinancialInput(input StammdatenFinancialInput) (StammdatenFinancialInput, error) {
	out := StammdatenFinancialInput{}

	if v := normalizeCompact(input.IBAN); v != nil {
		iban := strings.ToUpper(*v)
		if !ibanPattern.MatchString(iban) || !ibanChecksumValid(iban) {
			return out, fmt.Errorf("%w: malformed IBAN", ErrStaffStammdatenInvalid)
		}
		out.IBAN = &iban
	}
	if v := normalizeCompact(input.TaxID); v != nil {
		if !taxIDPattern.MatchString(*v) {
			return out, fmt.Errorf("%w: Steuer-ID must be 11 digits", ErrStaffStammdatenInvalid)
		}
		out.TaxID = v
	}
	if v := normalizeCompact(input.SocialSecurityNumber); v != nil {
		sv := strings.ToUpper(*v)
		if !socialSecurityPattern.MatchString(sv) {
			return out, fmt.Errorf("%w: malformed SV-Nummer", ErrStaffStammdatenInvalid)
		}
		out.SocialSecurityNumber = &sv
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

// --- audit-value helpers ---------------------------------------------------

func appendChange(changes *[]stammdatenChange, field, oldValue, newValue string) {
	if oldValue == newValue {
		return
	}
	*changes = append(*changes, stammdatenChange{field: field, oldValue: oldValue, newValue: newValue})
}

func normalizeOptional(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeCompact(v *string) *string {
	if v == nil {
		return nil
	}
	compact := strings.ReplaceAll(strings.TrimSpace(*v), " ", "")
	if compact == "" {
		return nil
	}
	return &compact
}

func strPtrAuditValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func dateAuditValue(d *timezone.Date) string {
	if d == nil {
		return ""
	}
	return d.String()
}

func floatAuditValue(v *float64) string {
	if v == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", *v), "0"), ".")
}

func maskedAuditValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// maskedChange builds a financial audit diff from masked values. When the
// plaintext changed but both masks render identically (same last-4, or the
// fixed full mask), the new value is suffixed so the audit row still shows a
// change and passes the old!=new validation.
func maskedChange(field string, oldMasked, newMasked *string) stammdatenChange {
	oldValue := maskedAuditValue(oldMasked)
	newValue := maskedAuditValue(newMasked)
	if oldValue == newValue {
		newValue += " (geändert)"
	}
	return stammdatenChange{field: field, oldValue: oldValue, newValue: newValue}
}

func masterDataGender(m *userModels.StaffMasterData) *string {
	if m == nil {
		return nil
	}
	return m.Gender
}

func masterDataStr(m *userModels.StaffMasterData, get func(*userModels.StaffMasterData) *string) string {
	if m == nil {
		return ""
	}
	return strPtrAuditValue(get(m))
}

func masterDataDate(m *userModels.StaffMasterData, get func(*userModels.StaffMasterData) *timezone.Date) string {
	if m == nil {
		return ""
	}
	return dateAuditValue(get(m))
}

func masterDataFloat(m *userModels.StaffMasterData) *float64 {
	if m == nil {
		return nil
	}
	return m.WeeklyHours
}

func qualificationsAuditValue(rows []*userModels.StaffQualification) string {
	if len(rows) == 0 {
		return ""
	}
	parts := make([]string, 0, len(rows))
	for _, q := range rows {
		entry := q.Name
		if q.AcquiredOn != nil {
			entry += " (erworben " + q.AcquiredOn.String() + ")"
		}
		if q.ExpiresOn != nil {
			entry += " (bis " + q.ExpiresOn.String() + ")"
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, "; ")
}

// maskTailPtr masks all but the last visible characters: "•••• 1234".
func maskTailPtr(v *string, visible int) *string {
	if v == nil || *v == "" {
		return nil
	}
	value := *v
	if len(value) <= visible {
		masked := strings.Repeat("•", len(value))
		return &masked
	}
	masked := "•••• " + value[len(value)-visible:]
	return &masked
}

// maskAllPtr returns a fixed-length mask so not even the value length leaks.
func maskAllPtr(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	masked := "••••••••"
	return &masked
}
