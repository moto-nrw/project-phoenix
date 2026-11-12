package application

import (
	"encoding/json"
	"errors"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// GuardianBankExportFields opens the sealed IBAN and account holder of a
// guardian for the return export of a school (#6412): a new provider can
// only go on collecting with the real account, not with its mask.
type GuardianBankExportFields struct {
	cipher peopleValues.BankDataCipher
}

func NewGuardianBankExportFields(cipher peopleValues.BankDataCipher) *GuardianBankExportFields {
	return &GuardianBankExportFields{cipher: cipher}
}

var errGuardianBankExportField = errors.New("people directory: not a sealed guardian bank export field")

// OpenExportField binds the value to field, school and guardian of its row,
// as sealing did; a value copied into another row or field does not open.
// The error never names the value.
func (f *GuardianBankExportFields) OpenExportField(table, column string, row map[string]json.RawMessage, sealed []byte) (string, error) {
	field := map[string]string{
		"iban_sealed":           guardianIBANField,
		"account_holder_sealed": guardianHolderField,
	}[column]
	if table != "users.guardian_financial_data" || field == "" || f.cipher == nil {
		return "", errGuardianBankExportField
	}
	var tenantID, guardianProfileID int64
	if json.Unmarshal(row["tenant_id"], &tenantID) != nil || tenantID <= 0 ||
		json.Unmarshal(row["guardian_profile_id"], &guardianProfileID) != nil || guardianProfileID <= 0 {
		return "", errGuardianBankExportField
	}
	plain, err := f.cipher.Open(sealed, guardianBankContext(field, tenantID, guardianProfileID))
	if err != nil {
		return "", errGuardianBankDataOpen
	}
	return string(plain), nil
}
