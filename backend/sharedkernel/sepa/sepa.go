// Package sepa checks the identifiers of a SEPA payment: the IBAN and BIC of
// an account, the creditor identifier of a creditor and the reference of a
// mandate (#4695). The rules are the ISO and EPC formats, so every owner that
// keeps bank data checks them the same way (Organisation & Tenancy for the
// Träger's own accounts, the Elternabrechnung for the families' mandates).
package sepa

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	ibanPattern = regexp.MustCompile(`^[A-Z]{2}[0-9]{2}[A-Z0-9]{11,30}$`)
	bicPattern  = regexp.MustCompile(`^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$`)
	// germanIBANPattern is the German BBAN: an eight-digit bank code and a
	// ten-digit account number.
	germanIBANPattern = regexp.MustCompile(`^DE[0-9]{20}$`)
	// creditorIDPattern is the EPC creditor identifier: country, two check
	// digits, a creditor business code of three characters and the national
	// identifier, at most 35 characters.
	creditorIDPattern = regexp.MustCompile(`^[A-Z]{2}[0-9]{2}[A-Z0-9]{3}[A-Z0-9]{1,28}$`)
	// germanCreditorIDPattern is the identifier the Bundesbank assigns: an
	// eleven-digit national part, 18 characters in all.
	germanCreditorIDPattern = regexp.MustCompile(`^DE[0-9]{2}[A-Z0-9]{3}[0-9]{11}$`)
	// mandateReferencePattern is the character set of the EPC rulebook for
	// a mandate reference (the Latin subset of the SEPA messages).
	mandateReferencePattern = regexp.MustCompile(`^[A-Za-z0-9+?/\-:().,' ]{1,35}$`)
)

// MaxMandateReferenceLength is the longest mandate reference the EPC
// rulebook allows.
const MaxMandateReferenceLength = 35

// ibanLengths are the ISO 13616 lengths of the SEPA countries. The check
// digits alone accept a truncated IBAN that happens to satisfy mod 97.
var ibanLengths = map[string]int{
	"AD": 24, "AT": 20, "BE": 16, "BG": 22, "CH": 21, "CY": 28,
	"CZ": 24, "DE": 22, "DK": 18, "EE": 20, "ES": 24, "FI": 18,
	"FR": 27, "GB": 22, "GI": 23, "GR": 27, "HR": 21, "HU": 28,
	"IE": 22, "IS": 26, "IT": 27, "LI": 21, "LT": 20, "LU": 20,
	"LV": 21, "MC": 27, "MT": 31, "NL": 18, "NO": 15, "PL": 28,
	"PT": 25, "RO": 24, "SE": 24, "SI": 19, "SK": 24,
}

// Compact removes white space and upper-cases an IBAN, a BIC or a creditor
// identifier as people type it.
func Compact(value string) string {
	return strings.ToUpper(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value))
}

// ValidIBAN reports whether a compacted IBAN has the ISO 13616 shape, the
// length of its country and its check digits; a German one also has the
// German BBAN.
func ValidIBAN(iban string) bool {
	if !ibanPattern.MatchString(iban) {
		return false
	}
	if length, known := ibanLengths[iban[:2]]; !known || len(iban) != length {
		return false
	}
	if iban[:2] == "DE" && !germanIBANPattern.MatchString(iban) {
		return false
	}
	return mod97(iban[4:]+iban[:4]) == 1
}

// bicCountries are the SEPA countries outside the EEA: a debit from an
// account there needs the BIC of its bank (EPC rulebook, AT-13).
var bicCountries = map[string]bool{"AD": true, "CH": true, "GB": true, "GI": true, "MC": true, "SM": true, "VA": true}

// NeedsBIC reports whether a debit from the account of a compacted IBAN
// needs its BIC.
func NeedsBIC(iban string) bool { return len(iban) >= 2 && bicCountries[iban[:2]] }

// ValidBIC reports whether a compacted BIC has the ISO 9362 shape.
func ValidBIC(bic string) bool { return bicPattern.MatchString(bic) }

// ValidCreditorID reports whether a compacted creditor identifier has the
// EPC shape and its check digits. The check digits cover the national
// identifier and the country, never the business code (EPC 262-08); a
// German identifier has the eleven-digit national part.
func ValidCreditorID(id string) bool {
	if !creditorIDPattern.MatchString(id) {
		return false
	}
	if id[:2] == "DE" && !germanCreditorIDPattern.MatchString(id) {
		return false
	}
	return mod97(id[7:]+id[:4]) == 1
}

// ValidMandateReference reports whether a trimmed mandate reference uses
// only the characters of the EPC rulebook, has at most 35 of them, and
// neither starts with nor contains a double slash.
func ValidMandateReference(reference string) bool {
	return mandateReferencePattern.MatchString(reference) && !strings.HasPrefix(reference, "/") &&
		!strings.Contains(reference, "//")
}

// MaskIBAN shows the country and the last four characters of an IBAN and
// hides the rest, in groups of four: "DE•• •••• •••• •••• ••30 00". The
// mask shows nothing a statement of the family's own bank does not show.
func MaskIBAN(iban string) string {
	var masked strings.Builder
	for i, r := range iban {
		if i > 0 && i%4 == 0 {
			masked.WriteByte(' ')
		}
		if i < 2 || i >= len(iban)-4 {
			masked.WriteRune(r)
			continue
		}
		masked.WriteString("•")
	}
	return masked.String()
}

// mod97 is the ISO 7064 MOD 97-10 remainder of an alphanumeric value, each
// letter counted as two digits (A = 10).
func mod97(value string) int {
	remainder := 0
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			remainder = (remainder*10 + int(r-'0')) % 97
		default:
			remainder = (remainder*100 + int(r-'A') + 10) % 97
		}
	}
	return remainder
}
