package application

import (
	"encoding/json"
	"strings"
	"testing"
)

// #6412: the return export opens a guardian's IBAN and account holder only
// in their own row and field; a copied value does not open and no error
// names the value.
func TestGuardianBankExportFieldsOpenOnlyTheirOwnRow(t *testing.T) {
	t.Parallel()
	cipher := contextCipher{}
	iban, _ := cipher.Seal([]byte("DE89370400440532013000"), guardianBankContext(guardianIBANField, 5, 7))
	holder, _ := cipher.Seal([]byte("Synthetic holder"), guardianBankContext(guardianHolderField, 5, 7))
	fields := NewGuardianBankExportFields(cipher)
	row := map[string]json.RawMessage{"tenant_id": json.RawMessage("5"), "guardian_profile_id": json.RawMessage("7")}
	const table = "users.guardian_financial_data"

	for column, want := range map[string]struct {
		sealed []byte
		plain  string
	}{"iban_sealed": {iban, "DE89370400440532013000"}, "account_holder_sealed": {holder, "Synthetic holder"}} {
		value, err := fields.OpenExportField(table, column, row, want.sealed)
		if err != nil || value != want.plain {
			t.Fatalf("%s: got %q, %v", column, value, err)
		}
	}

	for name, call := range map[string]func() (string, error){
		"other school": func() (string, error) {
			return fields.OpenExportField(table, "iban_sealed", map[string]json.RawMessage{"tenant_id": json.RawMessage("6"), "guardian_profile_id": json.RawMessage("7")}, iban)
		},
		"other guardian": func() (string, error) {
			return fields.OpenExportField(table, "iban_sealed", map[string]json.RawMessage{"tenant_id": json.RawMessage("5"), "guardian_profile_id": json.RawMessage("8")}, iban)
		},
		"holder as IBAN": func() (string, error) { return fields.OpenExportField(table, "iban_sealed", row, holder) },
		"foreign table": func() (string, error) {
			return fields.OpenExportField("users.staff_financial_data", "iban_sealed", row, iban)
		},
		"unknown column": func() (string, error) { return fields.OpenExportField(table, "iban_masked", row, iban) },
		"row without guardian": func() (string, error) {
			return fields.OpenExportField(table, "iban_sealed", map[string]json.RawMessage{"tenant_id": json.RawMessage("5")}, iban)
		},
	} {
		value, err := call()
		if err == nil || value != "" || strings.Contains(err.Error(), "DE89") {
			t.Fatalf("%s: got %q, %v", name, value, err)
		}
	}
}
