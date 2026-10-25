package domain

import "testing"

func TestValidIBANChecksLengthCountryAndCheckDigits(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"DE89370400440532013000": true,
		"GB82WEST12345698765432": true,
		"DE89370400440532013001": false, // wrong check digits
		"DE5112345678901":        false, // check digits fit, but too short for Germany
		"XX82WEST12345698765432": false, // no SEPA country
		"DE89370400440532A13000": false, // letter in a German account number
	}
	for iban, want := range cases {
		if got := ValidIBAN(iban); got != want {
			t.Errorf("ValidIBAN(%q) = %v, want %v", iban, got, want)
		}
	}
}
