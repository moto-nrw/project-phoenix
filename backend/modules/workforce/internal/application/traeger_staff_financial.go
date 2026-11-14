package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
)

// Financial serves the bank and tax section of a record: masked, or with
// reveal the values as entered. The read is logged at every Einsatzort
// first; nothing leaves without the entries (ADR 0089 section 6).
func (t *TraegerStaff) Financial(ctx context.Context, actor workforce.TraegerStaffActor, recordID int64, reveal bool) (workforce.TraegerStaffFinancial, error) {
	var record domain.TraegerStaffRecord
	err := t.released(ctx, actor, func(ctx context.Context) error {
		var err error
		record, err = t.store.FindStaffRecord(ctx, actor.OrganizationID, recordID, false)
		return err
	})
	if err != nil {
		return workforce.TraegerStaffFinancial{}, publicTraegerStaffError(err)
	}
	view := workforce.TraegerStaffViewFinancial
	if reveal {
		view = workforce.TraegerStaffViewFinancialReveal
	}
	if _, err := t.readSchools(ctx, actor, staffView{name: view, recordID: record.ID}, linkSchools(record), linkWanted(record), false); err != nil {
		return workforce.TraegerStaffFinancial{}, err
	}
	if !reveal {
		return maskedRecordFinancial(record), nil
	}
	values, err := t.openRecordFinancial(record)
	if err != nil {
		return workforce.TraegerStaffFinancial{}, err
	}
	return workforce.TraegerStaffFinancial{
		IBAN: optional(values[domain.StaffFieldIBAN]), TaxID: optional(values[domain.StaffFieldTaxID]),
		SocialSecurityNumber: optional(values[domain.StaffFieldSocialSecurityNumber]),
	}, nil
}

// UpdateFinancial replaces the bank and tax section of a record. The trail
// keeps each replaced value, sealed.
func (t *TraegerStaff) UpdateFinancial(ctx context.Context, actor workforce.TraegerStaffActor, recordID int64, input workforce.StammdatenFinancialInput) error {
	section, err := domain.NormalizeFinancialSection(domain.FinancialSection{IBAN: input.IBAN, TaxID: input.TaxID, SocialSecurityNumber: input.SocialSecurityNumber})
	if err != nil {
		return publicTraegerStaffError(err)
	}
	after := map[string]string{
		domain.StaffFieldIBAN: domain.Deref(section.IBAN), domain.StaffFieldTaxID: domain.Deref(section.TaxID),
		domain.StaffFieldSocialSecurityNumber: domain.Deref(section.SocialSecurityNumber),
	}
	var saved domain.TraegerStaffRecord
	err = t.released(ctx, actor, func(ctx context.Context) error {
		record, err := t.store.FindStaffRecord(ctx, actor.OrganizationID, recordID, true)
		if err != nil {
			return err
		}
		before, err := t.openRecordFinancial(record)
		if err != nil {
			return err
		}
		changes, err := t.replacedValues(actor, recordID, before, after)
		if err != nil || len(changes) == 0 {
			return err
		}
		if err := t.sealRecordFinancial(&record, after); err != nil {
			return err
		}
		if err := t.store.SealStaffRecord(ctx, record); err != nil {
			return err
		}
		saved = record
		return t.store.AppendStaffChanges(ctx, actor.OrganizationID, changes)
	})
	if err != nil {
		return publicTraegerStaffError(err)
	}
	if saved.ID == 0 {
		return nil
	}
	return t.handOver(ctx, actor, saved)
}

// Dissolve ends a record: every live Einsatzort gets the bank and tax values
// back as its own row, then the record goes with its links and trail. The
// schools' personnel numbers stay. A school that kept a row of its own
// meanwhile keeps it.
func (t *TraegerStaff) Dissolve(ctx context.Context, actor workforce.TraegerStaffActor, recordID int64) error {
	var record domain.TraegerStaffRecord
	var values map[string]string
	err := t.released(ctx, actor, func(ctx context.Context) error {
		var err error
		if record, err = t.store.FindStaffRecord(ctx, actor.OrganizationID, recordID, false); err != nil {
			return err
		}
		values, err = t.openRecordFinancial(record)
		return err
	})
	if err != nil {
		return publicTraegerStaffError(err)
	}
	for _, link := range record.Links {
		err := t.ports.Runtime.WithinSchool(ctx, link.SchoolID, func(ctx context.Context) error {
			return t.returnToSchool(ctx, actor, link, values)
		})
		if err != nil {
			return publicTraegerStaffError(fmt.Errorf("return to school %d: %w", link.SchoolID, err))
		}
	}
	return publicTraegerStaffError(t.released(ctx, actor, func(ctx context.Context) error {
		return t.store.DeleteStaffRecord(ctx, actor.OrganizationID, recordID)
	}))
}

func (t *TraegerStaff) returnToSchool(ctx context.Context, actor workforce.TraegerStaffActor, link domain.TraegerStaffLink, values map[string]string) error {
	live, err := t.schoolStaff(ctx, []int64{link.StaffID})
	if err != nil || len(live) == 0 || len(values) == 0 {
		return err
	}
	existing, err := t.store.SchoolFinancialRows(ctx, []int64{link.StaffID})
	if err != nil || len(existing) > 0 {
		return err
	}
	row, err := t.schoolFinancialRow(link.SchoolID, link.StaffID, values)
	if err != nil {
		return err
	}
	for _, field := range []string{domain.StaffFieldIBAN, domain.StaffFieldTaxID, domain.StaffFieldSocialSecurityNumber} {
		if values[field] == "" {
			continue
		}
		if err := t.audit.RecordMasterDataChange(ctx, domain.MasterDataChange{
			StaffID: link.StaffID, ChangedBy: actor.AccountID, Section: domain.AuditSectionBankSteuer, Field: field,
			OldValue: "", NewValue: domain.MaskStaffValue(field, values[field]), Note: noteReturnedByTraeger,
		}); err != nil {
			return fmt.Errorf("write bank return audit: %w", err)
		}
	}
	return t.store.InsertSchoolFinancialRow(ctx, row)
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
