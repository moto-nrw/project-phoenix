package application

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSEPAExportFieldsOpenOnlyTheirOwnRow(t *testing.T) {
	t.Parallel()
	cipher := prefixCipher{}
	iban, _ := cipher.Seal([]byte("DE89370400440532013000"), ibanContext(7, "M-1"))
	holder, _ := cipher.Seal([]byte("Erika Muster"), holderContext(7, "M-1"))
	fields := NewSEPAExportFields(cipher)
	mandate := map[string]json.RawMessage{"organization_id": json.RawMessage("7"), "reference": json.RawMessage(`"M-1"`)}
	debit := map[string]json.RawMessage{"organization_id": json.RawMessage("7"), "mandate_reference": json.RawMessage(`"M-1"`)}

	value, err := fields.OpenExportField("parent_billing.sepa_mandates", "iban_sealed", mandate, iban)
	require.NoError(t, err)
	require.Equal(t, "DE89370400440532013000", value)
	value, err = fields.OpenExportField("parent_billing.sepa_debits", "account_holder_sealed", debit, holder)
	require.NoError(t, err)
	require.Equal(t, "Erika Muster", value)

	otherTraeger := map[string]json.RawMessage{"organization_id": json.RawMessage("8"), "reference": json.RawMessage(`"M-1"`)}
	otherReference := map[string]json.RawMessage{"organization_id": json.RawMessage("7"), "reference": json.RawMessage(`"M-2"`)}
	for name, call := range map[string]func() (string, error){
		"other Träger": func() (string, error) {
			return fields.OpenExportField("parent_billing.sepa_mandates", "iban_sealed", otherTraeger, iban)
		},
		"other reference": func() (string, error) {
			return fields.OpenExportField("parent_billing.sepa_mandates", "iban_sealed", otherReference, iban)
		},
		"holder as IBAN": func() (string, error) {
			return fields.OpenExportField("parent_billing.sepa_mandates", "iban_sealed", mandate, holder)
		},
		"foreign table": func() (string, error) { return fields.OpenExportField("users.persons", "iban_sealed", mandate, iban) },
		"unknown column": func() (string, error) {
			return fields.OpenExportField("parent_billing.sepa_mandates", "bic", mandate, iban)
		},
		"row without Träger": func() (string, error) {
			return fields.OpenExportField("parent_billing.sepa_mandates", "iban_sealed", map[string]json.RawMessage{"reference": json.RawMessage(`"M-1"`)}, iban)
		},
		"debit without mandate": func() (string, error) {
			return fields.OpenExportField("parent_billing.sepa_debits", "iban_sealed", mandate, iban)
		},
	} {
		value, err := call()
		require.Error(t, err, name)
		require.Empty(t, value, name)
		require.NotContains(t, err.Error(), "DE89", name)
	}
}
