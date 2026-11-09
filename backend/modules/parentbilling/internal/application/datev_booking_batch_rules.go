package application

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The checks of the DATEV booking batch (#4694). They follow the regular
// expressions of the DATEV developer portal for the header and the
// Buchungsstapel; where a value is a key (account, invoice number, cost
// centre) the batch refuses it instead of changing it.

var (
	// Bezeichnung: ^["][\w.-/ ]{0,30}["]$ — letters, digits, underscore,
	// space, dot, slash and hyphen.
	datevLabelPattern = regexp.MustCompile(`^[\p{L}\p{N}_ ./-]*$`)
	// Belegfeld 1: ^(["][\w$&%*+\-\/]{0,36}["])$; the portal names
	// umlauts, spaces, dots, commas, semicolons and colons as invalid.
	datevDocumentNumberPattern = regexp.MustCompile(`^[A-Za-z0-9_$&%*+\-/]{1,36}$`)
	// KOST1, KOST2: ^(["][\w ]{0,36}["])$.
	datevCostCenterPattern = regexp.MustCompile(`^[A-Za-z0-9_ ]{0,36}$`)
	// Sachkontenrahmen: ^["](\d{2}){0,2}["]$.
	datevChartPattern = regexp.MustCompile(`^(\d{2}){0,2}$`)
)

// Limits of the portal: Beraternummer 1001 to 9999999, Mandantennummer 1 to
// 99999, Sachkontenlänge 4 to 8, Umsatz at most ten digits before the comma.
const (
	datevMinConsultant    = 1001
	datevMaxConsultant    = 9999999
	datevMaxClient        = 99999
	datevMinAccountLength = 4
	datevMaxAccountLength = 8
	datevMaxAmountCents   = 999999999999
)

type datevProblems struct {
	line int
	list []DatevProblem
}

func (p *datevProblems) add(field, reason string) {
	p.list = append(p.list, DatevProblem{Line: p.line, Field: field, Reason: reason})
}

// datevHeaderProblems checks the header.
func datevHeaderProblems(h DatevBatchHeader) []DatevProblem {
	p := &datevProblems{}
	datevRange(p, DatevFieldConsultantNumber, h.ConsultantNumber, datevMinConsultant, datevMaxConsultant)
	datevRange(p, DatevFieldClientNumber, h.ClientNumber, 1, datevMaxClient)
	datevRange(p, DatevFieldAccountLength, h.AccountLength, datevMinAccountLength, datevMaxAccountLength)
	if !datevChartPattern.MatchString(h.ChartOfAccounts) {
		p.add(DatevFieldChartOfAccounts, DatevInvalid)
	}
	if h.FiscalYearStart.IsZero() {
		p.add(DatevFieldFiscalYearStart, DatevMissing)
	}
	datevPeriodProblems(p, h)
	// The label is the Träger's own text: a character Windows-1252 cannot
	// write is refused, never replaced (see datevWritable).
	if label := datevCut(h.Label, datevLabelLength); !datevLabelPattern.MatchString(label) || !datevWritableText(label) {
		p.add(DatevFieldLabel, DatevInvalid)
	}
	if h.FileNumber < 1 {
		p.add(DatevFieldFileNumber, DatevMissing)
	}
	if h.CreatedAt.IsZero() {
		p.add(DatevFieldCreatedAt, DatevMissing)
	}
	return p.list
}

// datevRange reports a number that is unset (zero) or outside its range.
func datevRange(p *datevProblems, field string, value, low, high int) {
	switch {
	case value == 0:
		p.add(field, DatevMissing)
	case value < low || value > high:
		p.add(field, DatevInvalid)
	}
}

// datevPeriodProblems checks that the period lies in one fiscal year:
// DATEV takes the year of every TTMM document date from the fiscal year.
func datevPeriodProblems(p *datevProblems, h DatevBatchHeader) {
	if h.From.IsZero() || h.To.IsZero() {
		p.add(DatevFieldPeriod, DatevMissing)
		return
	}
	if h.To.Before(h.From) {
		p.add(DatevFieldPeriod, DatevInvalid)
		return
	}
	if h.FiscalYearStart.IsZero() {
		return
	}
	start := h.FiscalYearStart
	next := calendar.NewDate(start.Year()+1, start.Month(), start.Day())
	if h.From.Before(start) || !h.To.Before(next) {
		p.add(DatevFieldPeriod, DatevOutOfPeriod)
	}
}

// datevBookingProblems checks one booking against the header.
func datevBookingProblems(line int, b DatevBooking, h DatevBatchHeader) []DatevProblem {
	p := &datevProblems{line: line}
	if b.AmountCents <= 0 || b.AmountCents > datevMaxAmountCents {
		p.add(DatevFieldAmount, DatevInvalid)
	}
	if b.Side != DatevDebit && b.Side != DatevCredit {
		p.add(DatevFieldSide, DatevInvalid)
	}
	datevAccountProblem(p, DatevFieldAccount, b.Account, datevDebtorAccount(h.AccountLength))
	datevAccountProblem(p, DatevFieldCounterAccount, b.CounterAccount, datevGeneralAccount(h.AccountLength))
	datevDateProblem(p, b.DocumentDate, h)
	switch {
	case b.DocumentNumber == "":
		p.add(DatevFieldDocumentNumber, DatevMissing)
	case !datevDocumentNumberPattern.MatchString(b.DocumentNumber):
		p.add(DatevFieldDocumentNumber, DatevInvalid)
	}
	if !datevCostCenterPattern.MatchString(b.CostCenter1) {
		p.add(DatevFieldCostCenter1, DatevInvalid)
	}
	if !datevCostCenterPattern.MatchString(b.CostCenter2) {
		p.add(DatevFieldCostCenter2, DatevInvalid)
	}
	return p.list
}

func datevAccountProblem(p *datevProblems, field, account string, valid func(string) bool) {
	switch {
	case account == "":
		p.add(field, DatevMissing)
	case !valid(account):
		p.add(field, DatevInvalid)
	}
}

func datevDateProblem(p *datevProblems, date calendar.Date, h DatevBatchHeader) {
	switch {
	case date.IsZero():
		p.add(DatevFieldDocumentDate, DatevMissing)
	case !h.From.IsZero() && date.Before(h.From), !h.To.IsZero() && date.After(h.To):
		p.add(DatevFieldDocumentDate, DatevOutOfPeriod)
	}
}

// datevDebtorAccount accepts a debtor: one digit more than the Sachkonten
// and in the debtor range, which DATEV fixes at 10000 to 69999 for length
// 4 (first digit 1 to 6; 7 to 9 are creditors). With an invalid length no
// debtor fits; the header already names the length.
func datevDebtorAccount(length int) func(string) bool {
	return func(account string) bool {
		return length >= datevMinAccountLength && length <= datevMaxAccountLength &&
			len(account) == length+1 && datevDigits(account) && account[0] >= '1' && account[0] <= '6'
	}
}

// datevGeneralAccount accepts a Sachkonto (revenue account): at most
// AccountLength digits and not zero. The DATEV sample books Sachkonten
// shorter than the length (85, 320).
func datevGeneralAccount(length int) func(string) bool {
	return func(account string) bool {
		return length >= datevMinAccountLength && length <= datevMaxAccountLength &&
			len(account) <= length && datevDigits(account) && strings.Trim(account, "0") != ""
	}
}

func datevDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Characters outside Windows-1252 (#4694). The file has no "?" for a
// character it cannot write: the label of the batch is the Träger's own text
// and is refused with such a character; the Buchungstext comes from names of
// families and children and is written by a defined replacement instead, so
// a name in another script does not block the export. A character without
// a code loses its accents (ő becomes o); a few letters without such a base
// letter have a fixed replacement (ł becomes l); any other is left out.
var datevBaseLetters = map[rune]string{'ł': "l", 'Ł': "L", 'đ': "d", 'Đ': "D", 'ı': "i", 'ħ': "h", 'Ħ': "H"}

// datevWritable is how a character is written in Windows-1252, and false
// when it cannot be written.
func datevWritable(r rune) (string, bool) {
	if _, ok := charmap.Windows1252.EncodeRune(r); ok {
		return string(r), true
	}
	if base, ok := datevBaseLetters[r]; ok {
		return base, true
	}
	var out strings.Builder
	for _, part := range norm.NFD.String(string(r)) {
		if unicode.Is(unicode.Mn, part) {
			continue
		}
		if _, ok := charmap.Windows1252.EncodeRune(part); !ok {
			return "", false
		}
		out.WriteRune(part)
	}
	return out.String(), out.Len() > 0
}

// datevWritableText reports whether every character of text can be written.
func datevWritableText(text string) bool {
	for _, r := range text {
		if _, ok := datevWritable(r); !ok {
			return false
		}
	}
	return true
}

// datevTransliterate writes every character that can be written, leaves out
// the others and joins the remaining words with single spaces.
func datevTransliterate(text string) string {
	var out strings.Builder
	for _, r := range text {
		if written, ok := datevWritable(r); ok {
			out.WriteString(written)
		}
	}
	return strings.Join(strings.Fields(out.String()), " ")
}

// datevEncode writes text in Windows-1252. Every free text passed the label
// check or datevTransliterate before, so a character it cannot write is a
// defect of the formatter, never a silent "?".
func datevEncode(text string) ([]byte, error) {
	out := make([]byte, 0, len(text))
	for _, r := range text {
		written, ok := datevWritable(r)
		if !ok {
			return nil, fmt.Errorf("datev booking batch: %U has no code in Windows-1252", r)
		}
		for _, part := range written {
			b, _ := charmap.Windows1252.EncodeRune(part)
			out = append(out, b)
		}
	}
	return out, nil
}

// datevColumns is the column line of format version 13 as the DATEV sample
// file EXTF_Buchungsstapel.csv writes it (developer portal, "Musterdaten").
var datevColumns = [125]string{
	"Umsatz (ohne Soll/Haben-Kz)", "Soll/Haben-Kennzeichen", "WKZ Umsatz", "Kurs", "Basis-Umsatz",
	"WKZ Basis-Umsatz", "Konto", "Gegenkonto (ohne BU-Schlüssel)", "BU-Schlüssel", "Belegdatum",
	"Belegfeld 1", "Belegfeld 2", "Skonto", "Buchungstext", "Postensperre",
	"Diverse Adressnummer", "Geschäftspartnerbank", "Sachverhalt", "Zinssperre", "Beleglink",
	"Beleginfo - Art 1", "Beleginfo - Inhalt 1", "Beleginfo - Art 2", "Beleginfo - Inhalt 2", "Beleginfo - Art 3",
	"Beleginfo - Inhalt 3", "Beleginfo - Art 4", "Beleginfo - Inhalt 4", "Beleginfo - Art 5", "Beleginfo - Inhalt 5",
	"Beleginfo - Art 6", "Beleginfo - Inhalt 6", "Beleginfo - Art 7", "Beleginfo - Inhalt 7", "Beleginfo - Art 8",
	"Beleginfo - Inhalt 8", "KOST1 - Kostenstelle", "KOST2 - Kostenstelle", "Kost-Menge", "EU-Land u. UStID (Bestimmung)",
	"EU-Steuersatz (Bestimmung)", "Abw. Versteuerungsart", "Sachverhalt L+L", "Funktionsergänzung L+L", "BU 49 Hauptfunktionstyp",
	"BU 49 Hauptfunktionsnummer", "BU 49 Funktionsergänzung",
	"Zusatzinformation - Art 1", "Zusatzinformation- Inhalt 1", "Zusatzinformation - Art 2", "Zusatzinformation- Inhalt 2",
	"Zusatzinformation - Art 3", "Zusatzinformation- Inhalt 3", "Zusatzinformation - Art 4", "Zusatzinformation- Inhalt 4",
	"Zusatzinformation - Art 5", "Zusatzinformation- Inhalt 5", "Zusatzinformation - Art 6", "Zusatzinformation- Inhalt 6",
	"Zusatzinformation - Art 7", "Zusatzinformation- Inhalt 7", "Zusatzinformation - Art 8", "Zusatzinformation- Inhalt 8",
	"Zusatzinformation - Art 9", "Zusatzinformation- Inhalt 9", "Zusatzinformation - Art 10", "Zusatzinformation- Inhalt 10",
	"Zusatzinformation - Art 11", "Zusatzinformation- Inhalt 11", "Zusatzinformation - Art 12", "Zusatzinformation- Inhalt 12",
	"Zusatzinformation - Art 13", "Zusatzinformation- Inhalt 13", "Zusatzinformation - Art 14", "Zusatzinformation- Inhalt 14",
	"Zusatzinformation - Art 15", "Zusatzinformation- Inhalt 15", "Zusatzinformation - Art 16", "Zusatzinformation- Inhalt 16",
	"Zusatzinformation - Art 17", "Zusatzinformation- Inhalt 17", "Zusatzinformation - Art 18", "Zusatzinformation- Inhalt 18",
	"Zusatzinformation - Art 19", "Zusatzinformation- Inhalt 19", "Zusatzinformation - Art 20", "Zusatzinformation- Inhalt 20",
	"Stück", "Gewicht", "Zahlweise", "Forderungsart", "Veranlagungsjahr",
	"Zugeordnete Fälligkeit", "Skontotyp", "Auftragsnummer", "Buchungstyp", "USt-Schlüssel (Anzahlungen)",
	"EU-Land (Anzahlungen)", "Sachverhalt L+L (Anzahlungen)", "EU-Steuersatz (Anzahlungen)", "Erlöskonto (Anzahlungen)", "Herkunft-Kz",
	"Buchungs GUID", "KOST-Datum", "SEPA-Mandatsreferenz", "Skontosperre", "Gesellschaftername",
	"Beteiligtennummer", "Identifikationsnummer", "Zeichnernummer", "Postensperre bis", "Bezeichnung SoBil-Sachverhalt",
	"Kennzeichen SoBil-Buchung", "Festschreibung", "Leistungsdatum", "Datum Zuord. Steuerperiode", "Fälligkeit",
	"Generalumkehr (GU)", "Steuersatz", "Land", "Abrechnungsreferenz", "BVV-Position",
	"EU-Land u. UStID (Ursprung)", "EU-Steuersatz (Ursprung)", "Abw. Skontokonto",
}

// datevTextColumns are the fields (1-based) whose portal expression is a
// quoted text, plus 6 (WKZ Basisumsatz) and 20 (Beleglink), which the
// sample file quotes; every other field is a number or a date.
var datevTextColumns = func() map[int]bool {
	set := map[int]bool{}
	for _, n := range []int{2, 3, 6, 9, 11, 12, 14, 16, 20, 40, 42, 91, 95, 96, 98, 102, 103, 105, 107, 109, 110, 112, 118, 120, 121, 123} {
		set[n] = true
	}
	for n := 21; n <= 38; n++ { // Beleginfo 1 to 8, KOST1, KOST2
		set[n] = true
	}
	for n := 48; n <= 87; n++ { // Zusatzinformation 1 to 20
		set[n] = true
	}
	return set
}()
