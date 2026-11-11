package application

import (
	"encoding/json"
	"strings"
	"testing"
)

// #6412: the return export opens a staff member's IBAN, tax ID and social
// security number only in their own row and field; a copied value does not
// open and no error names the value.
func TestStaffBankExportFieldsOpenOnlyTheirOwnRow(t *testing.T) {
	t.Parallel()
	cipher := fakeBankCipher{}
	seal := func(field, plain string) []byte {
		sealed, _ := cipher.Seal([]byte(plain), staffBankContext(field, 5, 7))
		return sealed
	}
	iban := seal(staffIBANField, "DE89370400440532013000")
	taxID := seal(staffTaxIDField, "12345678995")
	social := seal(staffSocialField, "65170839J003")
	fields := NewStaffBankExportFields(cipher)
	row := map[string]json.RawMessage{"tenant_id": json.RawMessage("5"), "staff_id": json.RawMessage("7")}
	const table = "users.staff_financial_data"

	for column, want := range map[string]struct {
		sealed []byte
		plain  string
	}{
		"iban_sealed":                   {iban, "DE89370400440532013000"},
		"tax_id_sealed":                 {taxID, "12345678995"},
		"social_security_number_sealed": {social, "65170839J003"},
	} {
		value, err := fields.OpenExportField(table, column, row, want.sealed)
		if err != nil || value != want.plain {
			t.Fatalf("%s: got %q, %v", column, value, err)
		}
	}

	for name, call := range map[string]func() (string, error){
		"other school": func() (string, error) {
			return fields.OpenExportField(table, "tax_id_sealed", map[string]json.RawMessage{"tenant_id": json.RawMessage("6"), "staff_id": json.RawMessage("7")}, taxID)
		},
		"other staff member": func() (string, error) {
			return fields.OpenExportField(table, "tax_id_sealed", map[string]json.RawMessage{"tenant_id": json.RawMessage("5"), "staff_id": json.RawMessage("8")}, taxID)
		},
		"tax ID as social security number": func() (string, error) {
			return fields.OpenExportField(table, "social_security_number_sealed", row, taxID)
		},
		"foreign table": func() (string, error) {
			return fields.OpenExportField("users.guardian_financial_data", "iban_sealed", row, iban)
		},
		"unknown column": func() (string, error) { return fields.OpenExportField(table, "iban_masked", row, iban) },
		"row without staff member": func() (string, error) {
			return fields.OpenExportField(table, "iban_sealed", map[string]json.RawMessage{"tenant_id": json.RawMessage("5")}, iban)
		},
	} {
		value, err := call()
		if err == nil || value != "" || strings.Contains(err.Error(), "1234") || strings.Contains(err.Error(), "DE89") {
			t.Fatalf("%s: got %q, %v", name, value, err)
		}
	}
}
