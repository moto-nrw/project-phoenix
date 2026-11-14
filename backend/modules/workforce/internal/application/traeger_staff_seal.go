package application

import (
	"strconv"

	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
)

// The Personalstamm seals its bank and tax values with the bank ring like
// the school does (#6332, ADR 0183), bound to field, Träger and record: a
// value copied into another record, field or Träger does not open. A value
// of the trail is bound to its field, Träger and record as well.

func recordBankContext(field string, organizationID, recordID int64) []byte {
	return []byte("workforce.staff_records." + field + "|" + strconv.FormatInt(organizationID, 10) + "|" + strconv.FormatInt(recordID, 10))
}

func changeBankContext(field string, organizationID, recordID int64) []byte {
	return []byte("workforce.staff_record_changes." + field + "|" + strconv.FormatInt(organizationID, 10) + "|" + strconv.FormatInt(recordID, 10))
}

// schoolBankField maps a record field onto the field name of the school's
// sealed row.
var schoolBankField = map[string]string{
	domain.StaffFieldIBAN:                 staffIBANField,
	domain.StaffFieldTaxID:                staffTaxIDField,
	domain.StaffFieldSocialSecurityNumber: staffSocialField,
}

// recordSealed returns the pointer to the sealed value of field on record.
func recordSealed(record *domain.TraegerStaffRecord, field string) *[]byte {
	switch field {
	case domain.StaffFieldIBAN:
		return &record.IBANSealed
	case domain.StaffFieldTaxID:
		return &record.TaxIDSealed
	default:
		return &record.SocialSecurityNumberSealed
	}
}

// openRecordFinancial returns the opened bank and tax values of a record by
// field; a field without a value is absent.
func (t *TraegerStaff) openRecordFinancial(record domain.TraegerStaffRecord) (map[string]string, error) {
	values := map[string]string{}
	for field := range schoolBankField {
		sealed := *recordSealed(&record, field)
		if len(sealed) == 0 {
			continue
		}
		plain, err := t.bank.Open(sealed, recordBankContext(field, record.OrganizationID, record.ID))
		if err != nil {
			return nil, errStaffBankDataOpen
		}
		values[field] = string(plain)
	}
	return values, nil
}

// sealRecordFinancial seals values onto a saved record and sets the IBAN
// mask; an empty value clears its field.
func (t *TraegerStaff) sealRecordFinancial(record *domain.TraegerStaffRecord, values map[string]string) error {
	for field := range schoolBankField {
		target := recordSealed(record, field)
		value := values[field]
		if value == "" {
			*target = nil
			continue
		}
		sealed, err := t.bank.Seal([]byte(value), recordBankContext(field, record.OrganizationID, record.ID))
		if err != nil {
			return errStaffBankDataSeal
		}
		*target = sealed
	}
	record.IBANMasked = nil
	if iban := values[domain.StaffFieldIBAN]; iban != "" {
		record.IBANMasked = domain.MaskIBAN(&iban)
	}
	return nil
}

// openSchoolFinancial opens a school's sealed row by record field.
func (t *TraegerStaff) openSchoolFinancial(row domain.StaffFinancialData) (map[string]string, error) {
	opened := row
	if err := openStaffFinancial(t.bank, &opened); err != nil {
		return nil, err
	}
	values := map[string]string{}
	for field, value := range map[string]*string{
		domain.StaffFieldIBAN: opened.IBAN, domain.StaffFieldTaxID: opened.TaxID, domain.StaffFieldSocialSecurityNumber: opened.SocialSecurityNumber,
	} {
		if value != nil && *value != "" {
			values[field] = *value
		}
	}
	return values, nil
}

// schoolFinancialRow seals values for a school's row of one employment.
func (t *TraegerStaff) schoolFinancialRow(schoolID, staffID int64, values map[string]string) (domain.StaffFinancialData, error) {
	row := domain.StaffFinancialData{StaffID: staffID}
	section := domain.FinancialSection{}
	for field, target := range map[string]**string{
		domain.StaffFieldIBAN: &section.IBAN, domain.StaffFieldTaxID: &section.TaxID, domain.StaffFieldSocialSecurityNumber: &section.SocialSecurityNumber,
	} {
		if value := values[field]; value != "" {
			*target = &value
		}
	}
	if err := sealStaffFinancial(t.bank, &row, schoolID, section); err != nil {
		return domain.StaffFinancialData{}, err
	}
	return row, nil
}

// sealedChange is a trail entry of a bank or tax value: sealed, with its mask.
func (t *TraegerStaff) sealedChange(change domain.TraegerStaffChange, organizationID int64, value string) (domain.TraegerStaffChange, error) {
	sealed, err := t.bank.Seal([]byte(value), changeBankContext(change.Field, organizationID, change.RecordID))
	if err != nil {
		return domain.TraegerStaffChange{}, errStaffBankDataSeal
	}
	change.ValueSealed = sealed
	change.ValueMasked = new(domain.MaskStaffValue(change.Field, value))
	return change, nil
}

// trailChange builds a trail entry of field with value, sealed for a bank or
// tax value.
func (t *TraegerStaff) trailChange(organizationID int64, change domain.TraegerStaffChange, value string) (domain.TraegerStaffChange, error) {
	if domain.IsFinancialStaffField(change.Field) {
		if value == "" {
			change.ValueMasked = new("")
			return change, nil
		}
		return t.sealedChange(change, organizationID, value)
	}
	change.ValueText = new(value)
	return change, nil
}
