package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
)

// Notes the schools' audit trail carries for a change of the Träger.
const (
	noteHandedToTraeger   = "An die Personalakte des Trägers übergeben"
	noteNumberOfTraeger   = "Personalnummer aus der Personalakte des Trägers"
	noteReturnedByTraeger = "Personalakte des Trägers aufgelöst, Werte an die Schule zurückgegeben"
)

// writeLink saves the record, its new Einsatzorte and the trail in the
// Träger transaction. An existing record is locked first, so two links of
// one record wait for each other.
func (t *TraegerStaff) writeLink(ctx context.Context, actor workforce.TraegerStaffActor, plan linkPlan, resolved domain.ResolvedStaffFields) (domain.TraegerStaffRecord, error) {
	record := domain.TraegerStaffRecord{OrganizationID: actor.OrganizationID}
	var before map[string]string
	if plan.record != nil {
		locked, err := t.store.FindStaffRecord(ctx, actor.OrganizationID, plan.record.ID, true)
		if err != nil {
			return domain.TraegerStaffRecord{}, err
		}
		financial, err := t.openRecordFinancial(locked)
		if err != nil {
			return domain.TraegerStaffRecord{}, err
		}
		record, before = locked, recordValues(locked, financial)
	}
	values := resolved.Values
	record.FirstName, record.LastName = strings.TrimSpace(values[domain.StaffFieldFirstName]), strings.TrimSpace(values[domain.StaffFieldLastName])
	if record.FirstName == "" || record.LastName == "" {
		return domain.TraegerStaffRecord{}, &workforce.TraegerStaffLinkError{Field: "members", Reason: "a record needs a first and a last name"}
	}
	record.Birthday = values[domain.StaffFieldBirthday]
	record.PersonnelNumber = nil
	if number := values[domain.StaffFieldPersonnelNumber]; number != "" {
		record.PersonnelNumber = &number
	}
	id, err := t.store.SaveStaffRecord(ctx, record)
	if err != nil {
		return domain.TraegerStaffRecord{}, err
	}
	record.ID = id
	if err := t.sealRecordFinancial(&record, values); err != nil {
		return domain.TraegerStaffRecord{}, err
	}
	if err := t.store.SealStaffRecord(ctx, record); err != nil {
		return domain.TraegerStaffRecord{}, err
	}
	links := make([]domain.TraegerStaffLink, 0, len(plan.members))
	for _, member := range plan.members {
		links = append(links, domain.TraegerStaffLink{SchoolID: member.ref.SchoolID, StaffID: member.ref.StaffID})
	}
	if err := t.store.InsertStaffLinks(ctx, actor.OrganizationID, record.ID, actor.AccountID, links); err != nil {
		return domain.TraegerStaffRecord{}, err
	}
	record.Links = append(record.Links, links...)
	changes, err := t.linkChanges(actor, record.ID, links, resolved, before)
	if err != nil {
		return domain.TraegerStaffRecord{}, err
	}
	return record, t.store.AppendStaffChanges(ctx, actor.OrganizationID, changes)
}

// linkChanges lists the trail of a link: each new Einsatzort, each value
// not taken with its school and, for an existing record, each value it
// replaced.
func (t *TraegerStaff) linkChanges(actor workforce.TraegerStaffActor, recordID int64, links []domain.TraegerStaffLink, resolved domain.ResolvedStaffFields, before map[string]string) ([]domain.TraegerStaffChange, error) {
	changes := make([]domain.TraegerStaffChange, 0, len(links)+len(resolved.NotChosen))
	for _, link := range links {
		changes = append(changes, domain.TraegerStaffChange{
			RecordID: recordID, Kind: domain.StaffChangeLinked, Field: domain.StaffChangeFieldSchool,
			SchoolID: new(link.SchoolID), AccountID: actor.AccountID,
		})
	}
	for _, notChosen := range resolved.NotChosen {
		change, err := t.trailChange(actor.OrganizationID, domain.TraegerStaffChange{
			RecordID: recordID, Kind: domain.StaffChangeNotChosen, Field: notChosen.Field,
			SchoolID: new(notChosen.Source.SchoolID), AccountID: actor.AccountID,
		}, notChosen.Value)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	if before == nil {
		return changes, nil
	}
	replaced, err := t.replacedValues(actor, recordID, before, resolved.Values)
	return append(changes, replaced...), err
}

// replacedValues lists a changed entry with the old value of every field a
// write changed.
func (t *TraegerStaff) replacedValues(actor workforce.TraegerStaffActor, recordID int64, before, after map[string]string) ([]domain.TraegerStaffChange, error) {
	var changes []domain.TraegerStaffChange
	for _, field := range domain.StaffRecordFields {
		if strings.TrimSpace(before[field]) == strings.TrimSpace(after[field]) {
			continue
		}
		change, err := t.trailChange(actor.OrganizationID, domain.TraegerStaffChange{
			RecordID: recordID, Kind: domain.StaffChangeChanged, Field: field, AccountID: actor.AccountID,
		}, before[field])
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, nil
}

// handOver brings every live Einsatzort of the record up to date in its own
// tenant transaction: the record's personnel number on the employment, and
// no bank row of the school, because the record keeps the values (decision
// E1). Every write of the record repeats it, so a school that failed before
// catches up (ADR 0232 section 6).
func (t *TraegerStaff) handOver(ctx context.Context, actor workforce.TraegerStaffActor, record domain.TraegerStaffRecord) error {
	for _, link := range record.Links {
		err := t.ports.Runtime.WithinSchool(ctx, link.SchoolID, func(ctx context.Context) error {
			return t.handOverSchool(ctx, actor, link.StaffID, record.PersonnelNumber)
		})
		if err != nil {
			return publicTraegerStaffError(fmt.Errorf("hand over to school %d: %w", link.SchoolID, err))
		}
	}
	return nil
}

func (t *TraegerStaff) handOverSchool(ctx context.Context, actor workforce.TraegerStaffActor, staffID int64, number *string) error {
	live, err := t.schoolStaff(ctx, []int64{staffID})
	if err != nil || len(live) == 0 {
		return err
	}
	subject, err := t.subjects.LockStaff(ctx, staffID, false)
	if err != nil {
		return err
	}
	if oldNumber, newNumber := domain.Deref(subject.PersonnelNumber), domain.Deref(number); oldNumber != newNumber {
		if err := t.audit.RecordMasterDataChange(ctx, domain.MasterDataChange{
			StaffID: staffID, ChangedBy: actor.AccountID, Section: domain.AuditSectionArbeitsvertrag,
			Field: domain.StaffFieldPersonnelNumber, OldValue: oldNumber, NewValue: newNumber, Note: noteNumberOfTraeger,
		}); err != nil {
			return fmt.Errorf("write personnel number audit: %w", err)
		}
		if err := t.subjects.SetPersonnelNumber(ctx, staffID, number); err != nil {
			return err
		}
	}
	return t.removeSchoolBankRow(ctx, actor, staffID)
}

// removeSchoolBankRow deletes the school's bank row of an employment the
// record holds, with an audit entry per value the school had. Only the masks
// go into the school's trail; nothing is opened.
func (t *TraegerStaff) removeSchoolBankRow(ctx context.Context, actor workforce.TraegerStaffActor, staffID int64) error {
	rows, err := t.store.SchoolFinancialRows(ctx, []int64{staffID})
	if err != nil || len(rows) == 0 {
		return err
	}
	masked := domain.MaskFinancial(staffID, &rows[0])
	for field, value := range map[string]*string{
		domain.StaffFieldIBAN: masked.IBANMasked, domain.StaffFieldTaxID: masked.TaxIDMasked,
		domain.StaffFieldSocialSecurityNumber: masked.SocialSecurityNumberMasked,
	} {
		if value == nil || *value == "" {
			continue
		}
		if err := t.audit.RecordMasterDataChange(ctx, domain.MasterDataChange{
			StaffID: staffID, ChangedBy: actor.AccountID, Section: domain.AuditSectionBankSteuer, Field: field,
			OldValue: *value, NewValue: "", Note: noteHandedToTraeger,
		}); err != nil {
			return fmt.Errorf("write bank hand-over audit: %w", err)
		}
	}
	return t.store.DeleteSchoolFinancialRow(ctx, staffID)
}

// UpdatePersonnelNumber sets the Träger's number on the record and on every
// live Einsatzort. A number another record of the Träger or another
// employment of one of its schools holds is refused before anything changes.
func (t *TraegerStaff) UpdatePersonnelNumber(ctx context.Context, actor workforce.TraegerStaffActor, recordID int64, value *string) (workforce.TraegerStaffRecordDetail, error) {
	number, err := domain.NormalizePersonnelNumber(value)
	if err != nil {
		return workforce.TraegerStaffRecordDetail{}, publicTraegerStaffError(err)
	}
	var record domain.TraegerStaffRecord
	err = t.released(ctx, actor, func(ctx context.Context) error {
		record, err = t.store.FindStaffRecord(ctx, actor.OrganizationID, recordID, false)
		return err
	})
	if err != nil {
		return workforce.TraegerStaffRecordDetail{}, publicTraegerStaffError(err)
	}
	if number != nil {
		if err := t.numberFreeAtSchools(ctx, record.Links, *number); err != nil {
			return workforce.TraegerStaffRecordDetail{}, err
		}
	}
	err = t.released(ctx, actor, func(ctx context.Context) error {
		locked, err := t.store.FindStaffRecord(ctx, actor.OrganizationID, recordID, true)
		if err != nil || domain.Deref(locked.PersonnelNumber) == domain.Deref(number) {
			return err
		}
		change, err := t.trailChange(actor.OrganizationID, domain.TraegerStaffChange{
			RecordID: recordID, Kind: domain.StaffChangeChanged, Field: domain.StaffFieldPersonnelNumber, AccountID: actor.AccountID,
		}, domain.Deref(locked.PersonnelNumber))
		if err != nil {
			return err
		}
		locked.PersonnelNumber = number
		if _, err := t.store.SaveStaffRecord(ctx, locked); err != nil {
			return err
		}
		record = locked
		return t.store.AppendStaffChanges(ctx, actor.OrganizationID, []domain.TraegerStaffChange{change})
	})
	if err != nil {
		return workforce.TraegerStaffRecordDetail{}, publicTraegerStaffError(err)
	}
	if err := t.handOver(ctx, actor, record); err != nil {
		return workforce.TraegerStaffRecordDetail{}, err
	}
	return t.Record(ctx, actor, recordID)
}

// numberFreeAtSchools refuses a number another live employment of one of the
// record's schools holds, before anything changes (ADR 0089 section 5, ADR
// 0232 section 5): the school keeps one number per employment. It reads only
// the employments' numbers, no person.
func (t *TraegerStaff) numberFreeAtSchools(ctx context.Context, links []domain.TraegerStaffLink, number string) error {
	for _, link := range links {
		err := t.ports.Runtime.WithinSchool(ctx, link.SchoolID, func(ctx context.Context) error {
			employments, err := t.ports.Employments.SchoolEmployments(ctx, nil)
			if err != nil {
				return err
			}
			for _, employment := range employments {
				if employment.StaffID != link.StaffID && domain.Deref(employment.PersonnelNumber) == number {
					return workforce.ErrPersonnelNumberTaken
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
