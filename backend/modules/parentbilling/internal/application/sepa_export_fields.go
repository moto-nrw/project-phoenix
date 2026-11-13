package application

import (
	"encoding/json"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// SEPAExportFields opens the sealed IBAN and account holder of a mandate or
// a debit row for the return export of a Träger (#3838), and the confirmed
// text of an online mandate (#6412). A new provider can only go on
// collecting with the real account, not with its mask; the creditor bears
// the burden of proof for the mandate text.
type SEPAExportFields struct {
	cipher parentbilling.BankDataCipher
}

func NewSEPAExportFields(cipher parentbilling.BankDataCipher) *SEPAExportFields {
	return &SEPAExportFields{cipher: cipher}
}

var errSEPAExportField = errors.New("parent billing: not a sealed SEPA export field")

// OpenExportField binds the value to the Träger and mandate reference of its
// row, as sealing did; a value copied into another row does not open. The
// error never names the value.
func (f *SEPAExportFields) OpenExportField(table, column string, row map[string]json.RawMessage, sealed []byte) (string, error) {
	// A consent row names its mandate only by ID; the export adds the
	// reference of that mandate from the same snapshot (sealedBindings).
	referenceColumn := map[string]string{
		"parent_billing.sepa_mandates":         "reference",
		"parent_billing.sepa_debits":           "mandate_reference",
		"parent_billing.sepa_mandate_consents": "mandate_reference",
	}[table]
	bind := map[string]func(int64, string) []byte{
		"iban_sealed":           ibanContext,
		"account_holder_sealed": holderContext,
	}[column]
	if table == "parent_billing.sepa_mandate_consents" {
		bind = map[string]func(int64, string) []byte{"text_sealed": consentTextContext}[column]
	}
	if referenceColumn == "" || bind == nil {
		return "", errSEPAExportField
	}
	var organizationID int64
	var reference string
	if json.Unmarshal(row["organization_id"], &organizationID) != nil || organizationID <= 0 ||
		json.Unmarshal(row[referenceColumn], &reference) != nil || reference == "" {
		return "", errSEPAExportField
	}
	plain, err := f.cipher.Open(sealed, bind(organizationID, reference))
	if err != nil {
		return "", errors.New("parent billing: a sealed SEPA value does not open")
	}
	return string(plain), nil
}
