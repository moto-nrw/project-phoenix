package sepa

import "testing"

func TestValidIBANChecksLengthCountryAndCheckDigits(t *testing.T) {
	t.Parallel()
	for iban, want := range map[string]bool{
		"DE89370400440532013000":      true,
		"AT611904300234573201":        true,
		"SM86U0322509800000000270100": true,
		"VA59001123000012345678":      true,
		"SM86U0322509800000000270101": false,
		"VA59001123000012345679":      false,
		"DE89370400440532013001":      false, // check digits
		"DE8937040044053201300":       false, // length of the country
		"XX89370400440532013000":      false, // unknown country
		"DE89A70400440532013000":      false, // German BBAN has digits only
		"de89370400440532013000":      false, // not compacted
		"DE89 3704 0044 0532 013":     false,
		"":                            false,
	} {
		if got := ValidIBAN(iban); got != want {
			t.Errorf("ValidIBAN(%q) = %v, want %v", iban, got, want)
		}
	}
}

func TestValidCreditorIDChecksShapeAndCheckDigits(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]bool{
		"DE98ZZZ09999999999":  true, // the Bundesbank's test identifier
		"DE98ABC09999999999":  true, // the business code is not checked
		"DE97ZZZ09999999999":  false,
		"DE98ZZZ0999999999":   false, // German national part has eleven digits
		"DE98ZZZ0999999999A":  false,
		"AT61ZZZ01234567890":  true, // another country keeps its own national part
		"AT62ZZZ01234567890":  false,
		"de98zzz09999999999":  false,
		"DE98ZZZ099999999990": false,
		"":                    false,
	} {
		if got := ValidCreditorID(id); got != want {
			t.Errorf("ValidCreditorID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestValidMandateReferenceKeepsTheEPCCharacterSet(t *testing.T) {
	t.Parallel()
	for reference, want := range map[string]bool{
		"SJR-2026-0001":                        true,
		"Mandat 12/3 (alt)":                    true,
		"/SJR-1":                               false,
		"SJR//1":                               false,
		"SJR_1":                                false,
		"Müller-1":                             false,
		"123456789012345678901234567890123456": false, // 36 characters
		"":                                     false,
	} {
		if got := ValidMandateReference(reference); got != want {
			t.Errorf("ValidMandateReference(%q) = %v, want %v", reference, got, want)
		}
	}
}

func TestMaskIBANShowsCountryAndLastFour(t *testing.T) {
	t.Parallel()
	if got, want := MaskIBAN("DE89370400440532013000"), "DE•• •••• •••• •••• ••30 00"; got != want {
		t.Errorf("MaskIBAN = %q, want %q", got, want)
	}
	if got, want := MaskIBAN("NO9386011117947"), "NO•• •••• •••7 947"; got != want {
		t.Errorf("MaskIBAN(NO) = %q, want %q", got, want)
	}
}

func TestCompactRemovesSpacesAndUppercases(t *testing.T) {
	t.Parallel()
	if got := Compact(" de89 3704\t0044 0532 0130 00 "); got != "DE89370400440532013000" {
		t.Errorf("Compact = %q", got)
	}
}

func TestNeedsBICOutsideTheEEA(t *testing.T) {
	t.Parallel()
	if NeedsBIC("DE89370400440532013000") || !NeedsBIC("CH9300762011623852957") || NeedsBIC("") {
		t.Error("NeedsBIC must ask only accounts outside the EEA for a BIC")
	}
}
