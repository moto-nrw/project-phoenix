package application

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The DATEV booking batch of the Elternabrechnung (#4694): one EXTF file in
// format category 21 ("Buchungsstapel", format version 13, header version
// 700) as described by the DATEV developer portal
// (https://developer.datev.de/de/file-format/details/datev-format/format-description/booking-batch
// and .../format-description/header). The function is pure: the caller
// brings the header, which the Träger's settings fill, and the bookings of
// its issued invoices; the file number comes from the export protocol of
// #4693. Nothing here reads the database.
//
// File layout (portal, "Einstieg" and "Zeichensatz"):
//   - line 1 the header, line 2 the column names, then one line per booking;
//   - fields separated by a semicolon, every line ends in CR/LF;
//   - text fields in double quotes, a quote inside doubled, no control
//     characters; numbers with a decimal comma and no thousands separator;
//   - Windows-1252 ("ANSI"), the default character set of DATEV
//     Rechnungswesen; Unicode only with a BOM and not in every import path.

// DatevSide is the debit/credit mark of a booking (field 2). It refers to
// the account in field 7.
type DatevSide string

const (
	// DatevDebit ("S", Soll) charges the account: an invoice on the debtor.
	DatevDebit DatevSide = "S"
	// DatevCredit ("H", Haben) relieves the account: a credit note or the
	// cancellation of an invoice on the debtor.
	DatevCredit DatevSide = "H"
)

// DatevBatchHeader is the data of the header line that the caller supplies.
type DatevBatchHeader struct {
	// ConsultantNumber is the Beraternummer (field 11, 1001 to 9999999).
	ConsultantNumber int
	// ClientNumber is the Mandantennummer (field 12, 1 to 99999).
	ClientNumber int
	// FiscalYearStart is the first day of the fiscal year (field 13). DATEV
	// takes the year of every document date from it.
	FiscalYearStart calendar.Date
	// AccountLength is the Sachkontenlänge of the client (field 14, 4 to 8).
	// A personal account (debtor) has one digit more.
	AccountLength int
	// ChartOfAccounts is the Sachkontenrahmen (field 27): "03", "04" or
	// empty when the Träger did not set it.
	ChartOfAccounts string
	// From and To are the period of the batch (fields 15 and 16). They lie
	// in one fiscal year and every document date lies in them.
	From, To calendar.Date
	// Label is the Bezeichnung of the batch (field 17), at most 30
	// characters; a longer one is cut.
	Label string
	// FileNumber is the number of the file from the export protocol; it
	// names the file. The format has no header field for it.
	FileNumber int
	// CreatedAt is the time of the export (field 6, Berlin time).
	CreatedAt time.Time
	// Locked is the Festschreibung (field 21). Off leaves the bookings open
	// for the tax office of the Träger to correct.
	Locked bool
}

// DatevBooking is one line of the batch.
type DatevBooking struct {
	// AmountCents is the positive amount (field 1); its direction is Side.
	AmountCents int64
	// Side is the debit/credit mark for Account (field 2).
	Side DatevSide
	// Account is the debtor of the family (field 7): AccountLength + 1
	// digits in the debtor range.
	Account string
	// CounterAccount is the revenue account (field 8): at most
	// AccountLength digits.
	CounterAccount string
	// DocumentDate is the date of the invoice (field 10, TTMM).
	DocumentDate calendar.Date
	// DocumentNumber is the invoice number (Belegfeld 1, field 11), the key
	// DATEV matches open items by; it is never cut.
	DocumentNumber string
	// Text is the Buchungstext (field 14), at most 60 characters; a longer
	// one is cut.
	Text string
	// CostCenter1 and CostCenter2 are KOST1 and KOST2 (fields 37 and 38),
	// optional.
	CostCenter1, CostCenter2 string
}

// DatevBatchFile is the produced file.
type DatevBatchFile struct {
	FileName string
	Content  []byte
}

// Fields a DatevProblem names.
const (
	DatevFieldConsultantNumber = "consultant_number"
	DatevFieldClientNumber     = "client_number"
	DatevFieldFiscalYearStart  = "fiscal_year_start"
	DatevFieldAccountLength    = "account_length"
	DatevFieldChartOfAccounts  = "chart_of_accounts"
	DatevFieldPeriod           = "period"
	DatevFieldLabel            = "label"
	DatevFieldFileNumber       = "file_number"
	DatevFieldCreatedAt        = "created_at"
	DatevFieldAmount           = "amount"
	DatevFieldSide             = "side"
	DatevFieldAccount          = "account"
	DatevFieldCounterAccount   = "counter_account"
	DatevFieldDocumentDate     = "document_date"
	DatevFieldDocumentNumber   = "document_number"
	DatevFieldCostCenter1      = "cost_center_1"
	DatevFieldCostCenter2      = "cost_center_2"
)

// Reasons a DatevProblem gives.
const (
	DatevMissing     = "missing"
	DatevInvalid     = "invalid"
	DatevOutOfPeriod = "out_of_period"
)

// DatevProblem is one value the format refuses. Line 0 is the header,
// line n the n-th booking.
type DatevProblem struct {
	Line   int
	Field  string
	Reason string
}

// DatevFormatError lists every refused value of a batch; the file is not
// produced.
type DatevFormatError struct {
	Problems []DatevProblem
}

func (e *DatevFormatError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = fmt.Sprintf("line %d %s %s", p.Line, p.Field, p.Reason)
	}
	return "datev booking batch: " + strings.Join(parts, "; ")
}

// FormatDatevBookingBatch writes the batch, or a *DatevFormatError with
// every refused value. A batch without bookings is a header and the column
// line; the readiness check uses it to test the settings of a Träger.
func FormatDatevBookingBatch(header DatevBatchHeader, bookings []DatevBooking) (DatevBatchFile, error) {
	problems := datevHeaderProblems(header)
	for i, booking := range bookings {
		problems = append(problems, datevBookingProblems(i+1, booking, header)...)
	}
	if len(problems) > 0 {
		return DatevBatchFile{}, &DatevFormatError{Problems: problems}
	}
	var text strings.Builder
	text.WriteString(datevHeaderLine(header))
	text.WriteString(datevLineEnd)
	text.WriteString(strings.Join(datevColumns[:], ";"))
	text.WriteString(datevLineEnd)
	for _, booking := range bookings {
		text.WriteString(datevBookingLine(booking))
		text.WriteString(datevLineEnd)
	}
	return DatevBatchFile{
		FileName: "EXTF_Buchungsstapel_" + strconv.Itoa(header.FileNumber) + ".csv",
		Content:  datevEncode(text.String()),
	}, nil
}

const datevLineEnd = "\r\n"

// datevHeaderLine is the 31 fields of the header (portal, "Header").
func datevHeaderLine(h DatevBatchHeader) string {
	festschreibung := "0"
	if h.Locked {
		festschreibung = "1"
	}
	created := h.CreatedAt.In(calendar.Berlin)
	fields := []string{
		datevQuote("EXTF"), "700", "21", datevQuote("Buchungsstapel"), "13",
		created.Format("20060102150405") + fmt.Sprintf("%03d", created.Nanosecond()/int(time.Millisecond)),
		"", datevQuote(""), datevQuote(""), datevQuote(""),
		strconv.Itoa(h.ConsultantNumber), strconv.Itoa(h.ClientNumber),
		h.FiscalYearStart.Format("20060102"), strconv.Itoa(h.AccountLength),
		h.From.Format("20060102"), h.To.Format("20060102"),
		datevQuote(datevCut(h.Label, datevLabelLength)), datevQuote(""),
		"1", "0", festschreibung, datevQuote("EUR"),
		"", datevQuote(""), "", "", datevQuote(h.ChartOfAccounts), "", "", datevQuote(""), datevQuote(""),
	}
	return strings.Join(fields, ";")
}

// datevBookingLine is the 125 fields of one booking; the unused text
// fields carry an empty quoted value like the DATEV sample file.
func datevBookingLine(b DatevBooking) string {
	values := map[int]string{
		1:  datevAmount(b.AmountCents),
		2:  string(b.Side),
		7:  b.Account,
		8:  b.CounterAccount,
		10: b.DocumentDate.Format("0201"),
		11: b.DocumentNumber,
		14: datevCut(b.Text, datevTextLength),
		37: b.CostCenter1,
		38: b.CostCenter2,
	}
	fields := make([]string, len(datevColumns))
	for i := range fields {
		value := values[i+1]
		if datevTextColumns[i+1] {
			value = datevQuote(value)
		}
		fields[i] = value
	}
	return strings.Join(fields, ";")
}

// datevAmount writes cents with a decimal comma: 12345 is "123,45".
func datevAmount(cents int64) string {
	return strconv.FormatInt(cents/100, 10) + "," + fmt.Sprintf("%02d", cents%100)
}

// datevQuote encloses a text field in quotes and doubles a quote inside.
func datevQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

// Lengths after which a free text is cut (portal: Bezeichnung 30,
// Buchungstext 60 characters).
const (
	datevLabelLength = 30
	datevTextLength  = 60
)

// datevCut replaces control characters by a space and cuts the text to its
// first max characters.
func datevCut(value string, maxRunes int) string {
	runes := []rune(strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, value))
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	return strings.TrimSpace(string(runes))
}
