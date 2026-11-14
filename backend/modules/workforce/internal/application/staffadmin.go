package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/ports"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// StaffAdmin administers the personnel record of a staff member (#1417,
// #1423, #1424): the payroll number, the Stammdaten sections, the audited
// bank and tax reads and the Dokumente tab. Two hard rules hold for every path:
// no change without an audit row in the same tenant transaction (the section,
// payroll number and upload paths write it before they apply; a delete writes
// it after the soft delete, and either failing rolls both back), and no
// sensitive read without a data-access log row — a path refuses to serve when
// the audit write fails.
type StaffAdmin struct {
	records     *Service
	transaction ports.Transaction
	subjects    ports.StaffSubjects
	audit       ports.StaffAdminAudit
	authority   domain.DocumentAuthority
	clock       ports.Clock
	warn        func(message string, args ...any)
	// bank seals and opens IBAN, tax ID and social security number with the
	// bank ring (#6332, ADR 0183).
	bank workforce.BankDataCipher
	// traeger is the Personalstamm of the Träger (#4705): which employments
	// it keeps, and the store the Träger-Portal composes it over.
	traeger ports.TraegerStaffStore
}

// NewStaffAdmin composes the administration over the Workforce-owned record
// rows (records) and the consumer-owned ports of the rows it does not own.
func NewStaffAdmin(
	records *Service,
	transaction ports.Transaction,
	subjects ports.StaffSubjects,
	audit ports.StaffAdminAudit,
	authority domain.DocumentAuthority,
	clock ports.Clock,
	warn func(message string, args ...any),
	bank workforce.BankDataCipher,
	traeger ports.TraegerStaffStore,
) *StaffAdmin {
	if records == nil || transaction == nil || subjects == nil || audit == nil || clock == nil || warn == nil || bank == nil || traeger == nil ||
		authority.Allows == nil || authority.Health == "" || authority.Financial == "" || authority.General == "" {
		panic("workforce staff admin: all dependencies are required")
	}
	return &StaffAdmin{records: records, transaction: transaction, subjects: subjects, audit: audit, authority: authority, clock: clock, warn: warn, bank: bank, traeger: traeger}
}

// --- personnel number -------------------------------------------------------

// UpdatePersonnelNumber sets or clears (value nil / empty) the staff member's
// Personalnummer. The payroll system's number is what DATEV bills against, so
// a change is unique per tenant and never happens without an audit row. A
// no-op change writes no audit row.
func (a *StaffAdmin) UpdatePersonnelNumber(ctx context.Context, staffID int64, value *string, changedByStaffID int64, note string) (domain.StaffSubject, error) {
	if changedByStaffID <= 0 {
		return domain.StaffSubject{}, errors.New("changed-by staff id is required")
	}
	normalized, err := domain.NormalizePersonnelNumber(value)
	if err != nil {
		return domain.StaffSubject{}, err
	}

	var updated domain.StaffSubject
	err = a.transaction.RunWrite(ctx, func(ctx context.Context) error {
		subject, err := a.subjects.LockStaff(ctx, staffID, false)
		if err != nil {
			return err
		}
		if err := a.refuseKeptByTraeger(ctx, staffID); err != nil {
			return err
		}
		oldValue, newValue := domain.Deref(subject.PersonnelNumber), domain.Deref(normalized)
		if oldValue == newValue {
			updated = subject
			return nil
		}
		if err := a.audit.RecordPersonnelNumberChange(ctx, domain.PersonnelNumberChange{
			StaffID: subject.ID, ChangedBy: changedByStaffID, OldValue: oldValue, NewValue: newValue, Note: strings.TrimSpace(note),
		}); err != nil {
			return fmt.Errorf("write personnel number audit: %w", err)
		}
		if err := a.subjects.SetPersonnelNumber(ctx, staffID, normalized); err != nil {
			return err
		}
		subject.PersonnelNumber = normalized
		updated = subject
		return nil
	})
	if err != nil {
		return domain.StaffSubject{}, err
	}
	return updated, nil
}

// --- Stammdaten -------------------------------------------------------------

// StaffStammdaten aggregates the non-sensitive sections of one staff member.
// MasterData is nil until the first section write.
type StaffStammdaten struct {
	Staff          domain.StaffSubject
	MasterData     *domain.StaffMasterData
	Qualifications []domain.StaffQualification
}

// StaffStammdaten returns the non-sensitive Stammdaten sections. Financial
// data is deliberately absent — it has its own permission-gated read path.
func (a *StaffAdmin) StaffStammdaten(ctx context.Context, staffID int64) (StaffStammdaten, error) {
	subject, err := a.subjects.StaffWithPerson(ctx, staffID)
	if err != nil {
		return StaffStammdaten{}, err
	}
	masterData, err := a.masterData(ctx, staffID)
	if err != nil {
		return StaffStammdaten{}, err
	}
	qualifications, err := a.records.ListStaffQualifications(ctx, staffID)
	if err != nil {
		return StaffStammdaten{}, err
	}
	return StaffStammdaten{Staff: subject, MasterData: masterData, Qualifications: qualificationsToDomain(qualifications)}, nil
}

// UpdateStaffStammdatenPerson replaces the person section. A no-op submit
// writes no audit row.
func (a *StaffAdmin) UpdateStaffStammdatenPerson(ctx context.Context, staffID int64, in domain.PersonSection, changedBy int64, note string) error {
	section, err := domain.NormalizePersonSection(in)
	if err != nil {
		return err
	}
	return a.writeSection(ctx, staffID, changedBy, domain.AuditSectionPerson, note, func(ctx context.Context, subject domain.StaffSubject) ([]domain.FieldChange, func() error, error) {
		masterData, err := a.masterData(ctx, staffID)
		if err != nil {
			return nil, nil, err
		}
		apply := func() error {
			if err := a.subjects.UpdatePerson(ctx, subject.PersonID, section.FirstName, section.LastName, section.Birthday); err != nil {
				return err
			}
			return a.upsertMasterData(ctx, staffID, masterData, func(row *domain.StaffMasterData) { row.Gender = section.Gender })
		}
		return domain.PersonChanges(subject, masterData, section), apply, nil
	})
}

// UpdateStaffStammdatenKontakt replaces the contact section.
func (a *StaffAdmin) UpdateStaffStammdatenKontakt(ctx context.Context, staffID int64, in domain.ContactSection, changedBy int64, note string) error {
	section, err := domain.NormalizeContactSection(in)
	if err != nil {
		return err
	}
	return a.writeSection(ctx, staffID, changedBy, domain.AuditSectionKontakt, note, func(ctx context.Context, _ domain.StaffSubject) ([]domain.FieldChange, func() error, error) {
		masterData, err := a.masterData(ctx, staffID)
		if err != nil {
			return nil, nil, err
		}
		apply := func() error {
			return a.upsertMasterData(ctx, staffID, masterData, func(row *domain.StaffMasterData) {
				row.AddressStreet = section.AddressStreet
				row.AddressPostalCode = section.AddressPostalCode
				row.AddressCity = section.AddressCity
				row.Phone = section.Phone
				row.Email = section.Email
				row.EmergencyContactName = section.EmergencyContactName
				row.EmergencyContactPhone = section.EmergencyContactPhone
			})
		}
		return domain.ContactChanges(masterData, section), apply, nil
	})
}

// UpdateStaffStammdatenArbeitsvertrag replaces the contract section.
func (a *StaffAdmin) UpdateStaffStammdatenArbeitsvertrag(ctx context.Context, staffID int64, in domain.ContractSection, changedBy int64, note string) error {
	section, err := domain.NormalizeContractSection(in)
	if err != nil {
		return err
	}
	return a.writeSection(ctx, staffID, changedBy, domain.AuditSectionArbeitsvertrag, note, func(ctx context.Context, subject domain.StaffSubject) ([]domain.FieldChange, func() error, error) {
		masterData, err := a.masterData(ctx, staffID)
		if err != nil {
			return nil, nil, err
		}
		apply := func() error {
			if domain.Deref(subject.EmploymentType) != domain.Deref(section.EmploymentType) {
				if err := a.subjects.SetEmploymentType(ctx, staffID, section.EmploymentType); err != nil {
					return err
				}
			}
			return a.upsertMasterData(ctx, staffID, masterData, func(row *domain.StaffMasterData) {
				row.EntryDate = domain.Deref(section.EntryDate)
				row.ContractEndDate = domain.Deref(section.ContractEndDate)
				row.ProbationEndDate = domain.Deref(section.ProbationEndDate)
				row.WeeklyHours = section.WeeklyHours
			})
		}
		return domain.ContractChanges(subject, masterData, section), apply, nil
	})
}

// ReplaceStaffQualifications replaces the full qualification list. The audit
// trail records the list as one field diff.
func (a *StaffAdmin) ReplaceStaffQualifications(ctx context.Context, staffID int64, inputs []domain.QualificationInput, changedBy int64, note string) error {
	rows, err := domain.NormalizeQualifications(inputs)
	if err != nil {
		return err
	}
	return a.writeSection(ctx, staffID, changedBy, domain.AuditSectionQualifikation, note, func(ctx context.Context, _ domain.StaffSubject) ([]domain.FieldChange, func() error, error) {
		existing, err := a.records.ListStaffQualifications(ctx, staffID)
		if err != nil {
			return nil, nil, err
		}
		apply := func() error {
			_, err := a.records.ReplaceStaffQualifications(ctx, staffID, qualificationsToPublic(rows))
			if errors.Is(err, workforce.ErrInvalidStaffRecord) {
				return fmt.Errorf("%w: %s", domain.ErrStaffStammdatenInvalid, err)
			}
			return err
		}
		return domain.QualificationChanges(qualificationsToDomain(existing), rows), apply, nil
	})
}

// StaffFinancialMasked serves the masked bank and tax data and logs the read.
// No data leaves without a successful audit row.
func (a *StaffAdmin) StaffFinancialMasked(ctx context.Context, staffID, actorAccountID int64, actorRole string) (domain.StaffFinancialMasked, error) {
	var kept bool
	data, err := a.loadFinancialAudited(ctx, staffID, actorAccountID, actorRole, domain.DataAccessFinancialView, func(ctx context.Context) error {
		var err error
		kept, err = a.keptByTraeger(ctx, staffID)
		return err
	})
	if err != nil {
		return domain.StaffFinancialMasked{}, err
	}
	masked := domain.MaskFinancial(staffID, data)
	masked.KeptByTraeger = kept
	return masked, nil
}

// RevealStaffFinancial serves the full bank and tax values after the explicit
// reveal toggle and logs the reveal.
func (a *StaffAdmin) RevealStaffFinancial(ctx context.Context, staffID, actorAccountID int64, actorRole string) (domain.StaffFinancialData, error) {
	data, err := a.loadFinancialAudited(ctx, staffID, actorAccountID, actorRole, domain.DataAccessFinancialReveal, func(ctx context.Context) error {
		return a.refuseKeptByTraeger(ctx, staffID)
	})
	if err != nil {
		return domain.StaffFinancialData{}, err
	}
	if data == nil {
		return domain.StaffFinancialData{StaffID: staffID}, nil
	}
	// Opening follows the committed access log row, so a value that fails to open
	// still leaves the attempt in the trail.
	if err := openStaffFinancial(a.bank, data); err != nil {
		return domain.StaffFinancialData{}, err
	}
	return *data, nil
}

// UpdateStaffFinancial replaces the bank and tax section. Audit rows carry
// masked values only and record the authenticated payroll account as actor.
func (a *StaffAdmin) UpdateStaffFinancial(ctx context.Context, staffID int64, in domain.FinancialSection, changedByAccountID int64, note string) error {
	section, err := domain.NormalizeFinancialSection(in)
	if err != nil {
		return err
	}
	return a.writeSection(ctx, staffID, changedByAccountID, domain.AuditSectionBankSteuer, note, func(ctx context.Context, _ domain.StaffSubject) ([]domain.FieldChange, func() error, error) {
		return a.financialSectionChanges(ctx, staffID, section)
	})
}

// financialSectionChanges compares the stored bank and tax section with the
// new one and returns the write. A section the Träger keeps is refused.
func (a *StaffAdmin) financialSectionChanges(ctx context.Context, staffID int64, section domain.FinancialSection) ([]domain.FieldChange, func() error, error) {
	if err := a.refuseKeptByTraeger(ctx, staffID); err != nil {
		return nil, nil, err
	}
	data, err := a.financial(ctx, staffID)
	if err != nil {
		return nil, nil, err
	}
	// The diff compares plaintext; opening also proves the stored values
	// belong to this staff member before they are replaced.
	if err := openStaffFinancial(a.bank, data); err != nil {
		return nil, nil, err
	}
	apply := func() error {
		if data == nil {
			fresh := domain.StaffFinancialData{StaffID: staffID}
			if err := sealStaffFinancial(a.bank, &fresh, tenant.FromContext(ctx), section); err != nil {
				return err
			}
			_, err := a.records.CreateStaffFinancialData(ctx, financialToPublic(fresh))
			return err
		}
		row := *data
		if err := sealStaffFinancial(a.bank, &row, row.TenantID, section); err != nil {
			return err
		}
		_, err := a.records.UpdateStaffFinancialData(ctx, financialToPublic(row))
		return err
	}
	return domain.FinancialChanges(data, section), apply, nil
}

// writeSection is the shared write shape: lock the staff row, let the section
// compute its field diffs and an apply closure, then write the audit rows
// BEFORE applying, all in one tenant transaction. No diffs, no write.
func (a *StaffAdmin) writeSection(
	ctx context.Context,
	staffID, changedBy int64,
	section, note string,
	build func(ctx context.Context, subject domain.StaffSubject) ([]domain.FieldChange, func() error, error),
) error {
	if changedBy <= 0 {
		return errors.New("changed-by actor id is required")
	}
	return a.transaction.RunWrite(ctx, func(ctx context.Context) error {
		subject, err := a.subjects.LockStaff(ctx, staffID, true)
		if err != nil {
			return err
		}
		changes, apply, err := build(ctx, subject)
		if err != nil {
			return err
		}
		if len(changes) == 0 {
			return nil
		}
		trimmedNote := strings.TrimSpace(note)
		for _, change := range changes {
			if err := a.audit.RecordMasterDataChange(ctx, domain.MasterDataChange{
				StaffID: subject.ID, ChangedBy: changedBy, Section: section,
				Field: change.Field, OldValue: change.OldValue, NewValue: change.NewValue, Note: trimmedNote,
			}); err != nil {
				return fmt.Errorf("write stammdaten audit: %w", err)
			}
		}
		return apply()
	})
}

// upsertMasterData creates or updates the 1:1 master data row, mutating it
// through set before persisting.
func (a *StaffAdmin) upsertMasterData(ctx context.Context, staffID int64, existing *domain.StaffMasterData, set func(*domain.StaffMasterData)) error {
	if existing == nil {
		fresh := domain.StaffMasterData{StaffID: staffID}
		set(&fresh)
		if err := fresh.Validate(); err != nil {
			return fmt.Errorf("%w: %s", domain.ErrStaffStammdatenInvalid, err.Error())
		}
		_, err := a.records.CreateStaffMasterData(ctx, masterDataToPublic(fresh))
		return err
	}
	row := *existing
	set(&row)
	if err := row.Validate(); err != nil {
		return fmt.Errorf("%w: %s", domain.ErrStaffStammdatenInvalid, err.Error())
	}
	_, err := a.records.UpdateStaffMasterData(ctx, masterDataToPublic(row))
	return err
}

// loadFinancialAudited resolves the staff member, loads the financial row (nil
// when none exists) and writes the data-access log entry. Callers must not
// serve any value when the returned error is non-nil.
func (a *StaffAdmin) loadFinancialAudited(ctx context.Context, staffID, actorAccountID int64, actorRole, resource string, check func(context.Context) error) (*domain.StaffFinancialData, error) {
	// Own the audited read through the same RLS query boundary as the record
	// reads. A later decryption failure must not roll back its access trail.
	auditCtx := tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
	var data *domain.StaffFinancialData
	err := a.records.run(auditCtx, "load_financial_audited", func(txCtx context.Context, _ *domain.OperationStats) error {
		var err error
		if err := check(txCtx); err != nil {
			return err
		}
		data, err = a.loadFinancialAuditedInTx(txCtx, staffID, actorAccountID, actorRole, resource)
		return err
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (a *StaffAdmin) loadFinancialAuditedInTx(ctx context.Context, staffID, actorAccountID int64, actorRole, resource string) (*domain.StaffFinancialData, error) {
	if actorAccountID <= 0 {
		return nil, errors.New("actor account id is required for financial reads")
	}
	// Existence (and tenant scope) check before anything is disclosed.
	if _, err := a.subjects.StaffWithPerson(ctx, staffID); err != nil {
		return nil, err
	}
	data, err := a.financial(ctx, staffID)
	if err != nil {
		return nil, err
	}
	if err := a.audit.RecordDataAccess(ctx, domain.DataAccess{
		ActorAccountID: actorAccountID, ActorRole: domain.ActorRoleOrUnknown(actorRole),
		Resource: resource, StaffID: staffID, At: a.clock.Now(),
	}); err != nil {
		return nil, fmt.Errorf("write financial access audit: %w", err)
	}
	return data, nil
}

// masterData reads the master data row; nil when the tab was never written.
func (a *StaffAdmin) masterData(ctx context.Context, staffID int64) (*domain.StaffMasterData, error) {
	row, err := a.records.FindStaffMasterData(ctx, staffID)
	if errors.Is(err, workforce.ErrStaffMasterDataNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return new(domain.StaffMasterData(row)), nil
}

// financial reads the bank and tax row; nil when none was written.
func (a *StaffAdmin) financial(ctx context.Context, staffID int64) (*domain.StaffFinancialData, error) {
	row, err := a.records.FindStaffFinancialData(ctx, staffID)
	if errors.Is(err, workforce.ErrStaffFinancialDataNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return new(domain.StaffFinancialData(row)), nil
}

// --- Personalstamm of the Träger (#4705) --------------------------------------

// StaffKeptByTraeger reports whether a record of the school's Träger holds
// the employment: the Träger then keeps its personnel number and its bank
// and tax values (ADR 0089 section 6).
// It joins the caller's tenant transaction (the route's request
// transaction).
func (a *StaffAdmin) StaffKeptByTraeger(ctx context.Context, staffID int64) (bool, error) {
	if _, err := a.subjects.StaffWithPerson(ctx, staffID); err != nil {
		return false, MapStaffAdminError(err)
	}
	kept, err := a.traeger.KeptStaffIDs(ctx, []int64{staffID})
	return len(kept) > 0, MapStaffAdminError(err)
}

// TraegerStaffRecords composes the Personalstamm of the Träger-Portal over
// the same staff subjects, audit trail and bank ring as the personnel
// record: a link writes the school's number and bank hand-over into the
// school's own trail.
func (a *StaffAdmin) TraegerStaffRecords(collaborators workforce.TraegerStaffPorts) workforce.TraegerStaffRecords {
	return NewTraegerStaff(a.traeger, collaborators, a.subjects, a.audit, a.bank)
}

func (a *StaffAdmin) keptByTraeger(ctx context.Context, staffID int64) (bool, error) {
	kept, err := a.traeger.KeptStaffIDs(ctx, []int64{staffID})
	return len(kept) > 0, err
}

// refuseKeptByTraeger refuses a change of a value the Träger keeps.
func (a *StaffAdmin) refuseKeptByTraeger(ctx context.Context, staffID int64) error {
	kept, err := a.keptByTraeger(ctx, staffID)
	if err != nil {
		return err
	}
	if kept {
		return domain.ErrStaffKeptByTraeger
	}
	return nil
}
