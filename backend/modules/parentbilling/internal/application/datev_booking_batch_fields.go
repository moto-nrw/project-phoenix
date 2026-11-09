package application

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"
)

// The text fields of the DATEV booking batch (#4694) and what each may
// hold. Sources: developer portal, DATEV-Format, "Header" and
// "Buchungsstapel" (field expressions) and "Zeichensatz" (Windows-1252).
// The portal's "\w" is read as ASCII [A-Za-z0-9_] and "." as a printable
// Windows-1252 character; the portal names no regex flavour.
//
// Invariants (PR #6360, design):
//  1. Every text field is written only through its rule in datevTextRules.
//  2. Values from settings and header data are never changed; a value
//     outside its rule is a DatevProblem naming the field.
//  3. Only the Buchungstext, which comes from names of families and
//     children, is brought onto its rule by datevPersonText; it is then
//     checked against the same rule.
//  4. The line writer checks every text field against its rule and the
//     encoder writes strict Windows-1252; both refuse instead of replacing.

// datevTextRule is the allowlist of one text field: the whole value must
// match.
type datevTextRule struct {
	field  string
	allows func(string) bool
}

func datevPattern(field, expression string) datevTextRule {
	pattern := regexp.MustCompile(`^(?:` + expression + `)$`)
	return datevTextRule{field: field, allows: pattern.MatchString}
}

// datevPrintable admits printable Windows-1252 text of at most maxRunes
// characters: the portal's "." in a quoted field. Quotes and semicolons
// are text there; the writer doubles a quote.
func datevPrintable(field string, maxRunes int) datevTextRule {
	return datevTextRule{field: field, allows: func(value string) bool {
		if utf8.RuneCountInString(value) > maxRunes {
			return false
		}
		for _, r := range value {
			if _, ok := charmap.Windows1252.EncodeRune(r); !ok || unicode.IsControl(r) {
				return false
			}
		}
		return true
	}}
}

// Fields of text columns this package never fills.
const (
	datevFieldUnused   = "unused"
	datevFieldConstant = "constant"
)

// datevTextRules is the table of invariant 1: header field or booking
// column → allowlist.
var datevTextRules = struct {
	kennzeichen, formatname, herkunft, exportedBy, importedBy, label, dictation, currency,
	empty, chart, applicationInfo, side, documentNumber, text, costCenter1, costCenter2 datevTextRule
}{
	kennzeichen:     datevPattern(datevFieldConstant, `EXTF`),           // Kopf 1: ^["](EXTF|DTVF)["]$
	formatname:      datevPattern(datevFieldConstant, `Buchungsstapel`), // Kopf 4
	herkunft:        datevPattern(datevFieldConstant, `[A-Za-z0-9_]{0,2}`),
	exportedBy:      datevPattern(datevFieldConstant, `[A-Za-z0-9_]{0,25}`),
	importedBy:      datevPattern(datevFieldConstant, `[A-Za-z0-9_]{0,25}`),
	label:           datevPattern(DatevFieldLabel, `[A-Za-z0-9_./ ]{0,30}`), // Kopf 17: ^["][\w.-/ ]{0,30}["]$
	dictation:       datevPattern(datevFieldConstant, `(?:[A-Z]{2}){0,2}`),  // Kopf 18
	currency:        datevPattern(datevFieldConstant, `[A-Z]{3}`),           // Kopf 22
	empty:           datevPattern(datevFieldUnused, ``),                     // Kopf 24, 30, unused columns
	chart:           datevPattern(DatevFieldChartOfAccounts, `(?:[0-9]{2}){0,2}`),
	applicationInfo: datevPrintable(datevFieldConstant, 16),                               // Kopf 31
	side:            datevPattern(DatevFieldSide, `S|H`),                                  // Buchung 2
	documentNumber:  datevPattern(DatevFieldDocumentNumber, `[A-Za-z0-9_$&%*+\-/]{1,36}`), // Buchung 11
	text:            datevPrintable(DatevFieldText, datevTextLength),                      // Buchung 14: ^(["].{0,60}["])$
	costCenter1:     datevPattern(DatevFieldCostCenter1, `[A-Za-z0-9_ ]{0,36}`),           // Buchung 37
	costCenter2:     datevPattern(DatevFieldCostCenter2, `[A-Za-z0-9_ ]{0,36}`),           // Buchung 38
}

// datevTextLength is the most characters of a Buchungstext.
const datevTextLength = 60

// datevLineWriter writes the fields of one line and records every text
// field outside its rule (invariant 4).
type datevLineWriter struct {
	line     int
	problems []DatevProblem
}

// text checks value against rule and quotes it; a quote inside is doubled.
func (w *datevLineWriter) text(rule datevTextRule, value string) string {
	if !rule.allows(value) {
		w.problems = append(w.problems, DatevProblem{Line: w.line, Field: rule.field, Reason: DatevInvalid})
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

// datevBaseLetters are letters without a decomposition and their fixed
// replacement in a Buchungstext.
var datevBaseLetters = map[rune]string{'ł': "l", 'Ł': "L", 'đ': "d", 'Đ': "D", 'ı': "i", 'ħ': "h", 'Ħ': "H"}

// datevPersonText is the defined replacement of invariant 3 for the
// Buchungstext: a control character becomes a space, a character outside
// Windows-1252 loses its accents (ő becomes o) or takes its fixed letter
// (ł becomes l) or is left out; spaces are joined and the text is cut to
// 60 characters.
func datevPersonText(value string) string {
	var out strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) {
			out.WriteRune(' ')
			continue
		}
		out.WriteString(datevWritable(r))
	}
	runes := []rune(strings.Join(strings.Fields(out.String()), " "))
	if len(runes) > datevTextLength {
		runes = runes[:datevTextLength]
	}
	return strings.TrimSpace(string(runes))
}

// datevWritable is a character as Windows-1252 writes it, or "" when it
// has no form there.
func datevWritable(r rune) string {
	if _, ok := charmap.Windows1252.EncodeRune(r); ok {
		return string(r)
	}
	if base, ok := datevBaseLetters[r]; ok {
		return base
	}
	var out strings.Builder
	for _, part := range norm.NFD.String(string(r)) {
		if unicode.Is(unicode.Mn, part) {
			continue
		}
		if _, ok := charmap.Windows1252.EncodeRune(part); !ok {
			return ""
		}
		out.WriteRune(part)
	}
	return out.String()
}

// datevEncode writes text in strict Windows-1252: a character without a
// code is an error, never a replacement (invariant 4).
func datevEncode(text string) ([]byte, bool) {
	out := make([]byte, 0, len(text))
	for _, r := range text {
		b, ok := charmap.Windows1252.EncodeRune(r)
		if !ok {
			return nil, false
		}
		out = append(out, b)
	}
	return out, true
}
