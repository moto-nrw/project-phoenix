package application

import (
	"encoding/json"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
)

// StaffBankExportFields opens the sealed IBAN, tax ID and social security
// number of a staff member for the return export of a school (#6412): the
// payroll of a new provider needs the values as entered, not their mask.
type StaffBankExportFields struct {
	cipher workforce.BankDataCipher
}

func NewStaffBankExportFields(cipher workforce.BankDataCipher) *StaffBankExportFields {
	return &StaffBankExportFields{cipher: cipher}
}

var errStaffBankExportField = errors.New("workforce: not a sealed staff bank export field")

// OpenExportField binds the value to field, school and staff member of its
// row, as sealing did; a value copied into another row or field does not
// open. The error never names the value. The staff records of a Träger and
// their trail open with the context of their record (#4705).
func (f *StaffBankExportFields) OpenExportField(table, column string, row map[string]json.RawMessage, sealed []byte) (string, error) {
	if table == "workforce.staff_records" || table == "workforce.staff_record_changes" {
		return f.openTraegerField(table, column, row, sealed)
	}
	field := map[string]string{
		"iban_sealed":                   staffIBANField,
		"tax_id_sealed":                 staffTaxIDField,
		"social_security_number_sealed": staffSocialField,
	}[column]
	if table != "users.staff_financial_data" || field == "" || f.cipher == nil {
		return "", errStaffBankExportField
	}
	var tenantID, staffID int64
	if json.Unmarshal(row["tenant_id"], &tenantID) != nil || tenantID <= 0 ||
		json.Unmarshal(row["staff_id"], &staffID) != nil || staffID <= 0 {
		return "", errStaffBankExportField
	}
	plain, err := f.cipher.Open(sealed, staffBankContext(field, tenantID, staffID))
	if err != nil {
		return "", errStaffBankDataOpen
	}
	return string(plain), nil
}

// openTraegerField opens a sealed value of a staff record of a Träger or of
// its trail for the Träger's return export.
func (f *StaffBankExportFields) openTraegerField(table, column string, row map[string]json.RawMessage, sealed []byte) (string, error) {
	var organizationID, recordID int64
	if f.cipher == nil || json.Unmarshal(row["organization_id"], &organizationID) != nil || organizationID <= 0 {
		return "", errStaffBankExportField
	}
	var aad []byte
	switch {
	case table == "workforce.staff_records" && recordColumns[column] != "":
		if json.Unmarshal(row["id"], &recordID) != nil || recordID <= 0 {
			return "", errStaffBankExportField
		}
		aad = recordBankContext(recordColumns[column], organizationID, recordID)
	case table == "workforce.staff_record_changes" && column == "value_sealed":
		var field string
		if json.Unmarshal(row["staff_record_id"], &recordID) != nil || recordID <= 0 || json.Unmarshal(row["field"], &field) != nil || field == "" {
			return "", errStaffBankExportField
		}
		aad = changeBankContext(field, organizationID, recordID)
	default:
		return "", errStaffBankExportField
	}
	plain, err := f.cipher.Open(sealed, aad)
	if err != nil {
		return "", errStaffBankDataOpen
	}
	return string(plain), nil
}

// recordColumns maps a sealed column of a staff record onto its field.
var recordColumns = map[string]string{
	"iban_sealed": "iban", "tax_id_sealed": "tax_id", "social_security_number_sealed": "social_security_number",
}
