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
// open. The error never names the value.
func (f *StaffBankExportFields) OpenExportField(table, column string, row map[string]json.RawMessage, sealed []byte) (string, error) {
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
