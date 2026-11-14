package application

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	validation "github.com/go-ozzo/ozzo-validation"
	"golang.org/x/text/encoding/charmap"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The Sollstellung (#4693): a general CSV for the finance system of a
// Kämmerei, one row per invoice, Storno or credit of a run, with the
// columns, separators, date format and encoding the Träger maps. The file
// is a pure function of the mapping, the invoices and the file number; a
// golden test pins it.

// sollFileNamePattern is a file name with the file number: letters, digits
// and . _ - around exactly one SollFileNumberToken.
var sollFileNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]*\{NR\}[A-Za-z0-9._-]*$`)

var (
	sollDelimiters = []string{";", ",", "\t", "|"}
	sollDecimals   = []string{",", "."}
	sollDates      = []string{parentbilling.SollDateGerman, parentbilling.SollDateISO}
	sollEncodings  = []string{parentbilling.SollEncodingUTF8BOM, parentbilling.SollEncodingUTF8, parentbilling.SollEncodingWindows1252}
)

// sollTextFields are the fields with text or identifiers; a value that a
// spreadsheet would read as a formula gets a leading apostrophe there.
var sollTextFields = []string{parentbilling.SollFieldInvoiceNumber, parentbilling.SollFieldRecipientName, parentbilling.SollFieldRecipientStreet,
	parentbilling.SollFieldRecipientCity, parentbilling.SollFieldChildName, parentbilling.SollFieldSchoolName,
	parentbilling.SollFieldMandateReference, parentbilling.SollFieldDebtorNumber, parentbilling.SollFieldConstant,
	parentbilling.SollFieldCostCenter, parentbilling.SollFieldCostCenter2, parentbilling.SollFieldBookingText}

// sollPlaceholderPattern finds what a booking text names in braces.
var sollPlaceholderPattern = regexp.MustCompile(`\{[^{}]*\}`)

// normalizeSollMapping trims a mapping and checks it; every problem is a
// validation error on its field ("columns.2.header", "delimiter", …).
func normalizeSollMapping(mapping parentbilling.SollstellungMapping) (parentbilling.SollstellungMapping, error) {
	errs := validation.Errors{}
	mapping.FileNamePattern = strings.TrimSpace(mapping.FileNamePattern)
	switch {
	case len(mapping.Columns) == 0:
		errs["columns"] = errors.New("needs at least one column")
	case len(mapping.Columns) > parentbilling.MaxSollColumns:
		errs["columns"] = errors.New("has too many columns")
	}
	for i := range mapping.Columns {
		checkSollColumn(errs, "columns."+strconv.Itoa(i), &mapping.Columns[i], mapping.HeaderRow)
	}
	if !slices.Contains(sollDelimiters, mapping.Delimiter) {
		errs["delimiter"] = errors.New("must be ; , tab or |")
	}
	if !slices.Contains(sollDecimals, mapping.DecimalSeparator) {
		errs["decimal_separator"] = errors.New("must be a comma or a point")
	} else if mapping.DecimalSeparator == mapping.Delimiter {
		errs["decimal_separator"] = errors.New("must differ from the delimiter")
	}
	if !slices.Contains(sollDates, mapping.DateFormat) {
		errs["date_format"] = errors.New("must be dd.mm.yyyy or yyyy-mm-dd")
	}
	if !slices.Contains(sollEncodings, mapping.Encoding) {
		errs["encoding"] = errors.New("must be utf-8-bom, utf-8 or windows-1252")
	}
	if utf8.RuneCountInString(mapping.FileNamePattern) > parentbilling.MaxSollFileNameLength || !sollFileNamePattern.MatchString(mapping.FileNamePattern) {
		errs["file_name_pattern"] = errors.New("needs {NR} once and only letters, digits, . _ -")
	}
	if len(errs) > 0 {
		return mapping, errs
	}
	return mapping, nil
}

// checkSollColumn trims one column and checks its field, header and value.
func checkSollColumn(errs validation.Errors, key string, column *parentbilling.SollstellungColumn, headerRow bool) {
	column.Header, column.Value = strings.TrimSpace(column.Header), strings.TrimSpace(column.Value)
	switch {
	case !slices.Contains(parentbilling.SollstellungFields, column.Field):
		errs[key+".field"] = errors.New("is no field of the Sollstellung")
	case utf8.RuneCountInString(column.Header) > parentbilling.MaxSollHeaderLength || strings.ContainsAny(column.Header, "\r\n"):
		errs[key+".header"] = errors.New("is too long or has a line break")
	case headerRow && column.Header == "":
		errs[key+".header"] = errors.New("is required with a header row")
	case sollKeepsValue(column.Field) && (utf8.RuneCountInString(column.Value) > parentbilling.MaxSollConstantLength ||
		strings.ContainsAny(column.Value, "\r\n")):
		errs[key+".value"] = errors.New("is too long or has a line break")
	case column.Field == parentbilling.SollFieldBookingText && unknownSollPlaceholder(column.Value) != "":
		errs[key+".value"] = fmt.Errorf("names the unknown placeholder %s", unknownSollPlaceholder(column.Value))
	}
	if !sollKeepsValue(column.Field) {
		column.Value = ""
	}
}

// sollKeepsValue says whether a column of the field carries a value: the
// fixed value of a constant or the pattern of a booking text, both with the
// same limits.
func sollKeepsValue(field string) bool {
	return field == parentbilling.SollFieldConstant || field == parentbilling.SollFieldBookingText
}

// unknownSollPlaceholder is the first text in braces of a pattern that is
// no placeholder, empty when there is none: a misspelt placeholder would
// otherwise stand in every row of the file.
func unknownSollPlaceholder(pattern string) string {
	for _, named := range sollPlaceholderPattern.FindAllString(pattern, -1) {
		if !slices.Contains(parentbilling.SollBookingTextPlaceholders, named) {
			return named
		}
	}
	return ""
}

// sollBookingText fills the placeholders of a pattern for one document.
func sollBookingText(pattern string, invoice parentbilling.ExportInvoice) string {
	month := ""
	if !invoice.ServiceFrom.IsZero() {
		month = invoice.ServiceFrom.Format("01/2006")
	}
	return strings.NewReplacer(parentbilling.SollPlaceholderMonth, month, parentbilling.SollPlaceholderChild, invoice.ChildName,
		parentbilling.SollPlaceholderRecipient, invoice.RecipientName, parentbilling.SollPlaceholderInvoiceNumber, invoice.Number,
	).Replace(pattern)
}

// sollFileName writes the file number into the pattern.
func sollFileName(pattern string, fileNumber int64) string {
	return strings.Replace(pattern, parentbilling.SollFileNumberToken, strconv.FormatInt(fileNumber, 10), 1)
}

// sollRow is one row of the file: a document, or one line of it when the
// mapping names where it books (#4821).
type sollRow struct {
	invoice parentbilling.ExportInvoice
	line    *bookingLine
}

// sollSplitsLines says whether the mapping names where a row books; the
// file then writes one row per line of a document.
func sollSplitsLines(mapping parentbilling.SollstellungMapping) bool {
	return slices.ContainsFunc(mapping.Columns, func(column parentbilling.SollstellungColumn) bool {
		return slices.Contains(parentbilling.SollBookingFields, column.Field)
	})
}

// sollRows are the rows of the invoices: one per document, or one per line
// of where it books (booking_lines.go) when the mapping names an account
// field, with the positions resolved at export and the documents a line of
// which has no revenue account although the mapping writes it.
func sollRows(mapping parentbilling.SollstellungMapping, invoices []parentbilling.ExportInvoice, resolver bookingResolver,
) (rows []sollRow, resolved int, missing []string) {
	if !sollSplitsLines(mapping) {
		return invoiceSollRows(invoices), 0, nil
	}
	writesAccount := slices.ContainsFunc(mapping.Columns, func(column parentbilling.SollstellungColumn) bool {
		return column.Field == parentbilling.SollFieldRevenueAccount
	})
	for _, invoice := range invoices {
		lines, fromToday := resolver.documentLines(invoice)
		resolved += fromToday
		if len(lines) == 0 {
			rows = append(rows, sollRow{invoice: invoice})
			continue
		}
		for i := range lines {
			if writesAccount && lines[i].accounts.RevenueAccount == "" && !slices.Contains(missing, invoice.Number) {
				missing = append(missing, invoice.Number)
			}
			rows = append(rows, sollRow{invoice: invoice, line: &lines[i]})
		}
	}
	return rows, resolved, missing
}

func invoiceSollRows(invoices []parentbilling.ExportInvoice) []sollRow {
	rows := make([]sollRow, len(invoices))
	for i, invoice := range invoices {
		rows[i] = sollRow{invoice: invoice}
	}
	return rows
}

// renderSollRows writes the file of the rows with the mapping.
func renderSollRows(mapping parentbilling.SollstellungMapping, rows []sollRow, fileNumber int64) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	writer.Comma, _ = utf8.DecodeRuneInString(mapping.Delimiter)
	writer.UseCRLF = true
	if mapping.HeaderRow {
		header := make([]string, len(mapping.Columns))
		for i, column := range mapping.Columns {
			header[i] = sollFormulaSafe(column.Header)
		}
		if err := writer.Write(header); err != nil {
			return nil, err
		}
	}
	for _, row := range rows {
		record := make([]string, len(mapping.Columns))
		for i, column := range mapping.Columns {
			record[i] = sollCell(mapping, column, row, fileNumber)
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return encodeSoll(mapping.Encoding, buffer.Bytes())
}

// sollCell is the value of one column for one invoice.
func sollCell(mapping parentbilling.SollstellungMapping, column parentbilling.SollstellungColumn, row sollRow, fileNumber int64) string {
	value := sollRowValue(mapping, column, row, fileNumber)
	if slices.Contains(sollTextFields, column.Field) {
		return sollFormulaSafe(value)
	}
	return value
}

// sollFormulaSafe keeps a spreadsheet from reading a text as a formula: a
// text that starts like one gets a leading apostrophe. Quoting the CSV field
// does not stop Excel, so names, free columns and the column headers of the
// Träger's mapping all pass here.
func sollFormulaSafe(value string) string {
	if value != "" && strings.ContainsRune("=+-@\t\r", rune(value[0])) {
		return "'" + value
	}
	return value
}

// sollRowValue is the value of a column in a row: where its line books and
// the line's amount, the document's values otherwise.
func sollRowValue(mapping parentbilling.SollstellungMapping, column parentbilling.SollstellungColumn, row sollRow, fileNumber int64) string {
	switch {
	case slices.Contains(parentbilling.SollBookingFields, column.Field):
		return sollLineValue(column, row)
	case column.Field == parentbilling.SollFieldAmount:
		if sollIsCredit(row.invoice) && sollHasCreditColumn(mapping) {
			return ""
		}
		return sollAmount(sollRowCents(row), mapping.DecimalSeparator)
	case column.Field == parentbilling.SollFieldCreditAmount:
		if !sollIsCredit(row.invoice) {
			return ""
		}
		cents := sollRowCents(row)
		if cents < 0 {
			cents = -cents
		}
		return sollAmount(cents, mapping.DecimalSeparator)
	}
	return sollValue(mapping, column, row.invoice, fileNumber)
}

// sollRowCents is the amount of a row: its line's, else its document's.
func sollRowCents(row sollRow) int64 {
	if row.line != nil {
		return row.line.amountCents
	}
	return row.invoice.TotalCents
}

// sollIsCredit says whether a document is a Storno or a credit, the rows a
// credit column takes (#7008).
func sollIsCredit(invoice parentbilling.ExportInvoice) bool {
	return invoice.Kind == parentbilling.DocumentCancellation || invoice.Kind == parentbilling.DocumentCredit
}

// sollHasCreditColumn says whether the mapping writes the credits in a
// column of their own; without one, the amount column keeps them.
func sollHasCreditColumn(mapping parentbilling.SollstellungMapping) bool {
	return slices.ContainsFunc(mapping.Columns, func(column parentbilling.SollstellungColumn) bool {
		return column.Field == parentbilling.SollFieldCreditAmount
	})
}

// sollLineValue is where the line of a row books; a row of a whole
// document has no line.
func sollLineValue(column parentbilling.SollstellungColumn, row sollRow) string {
	if row.line == nil {
		return ""
	}
	switch column.Field {
	case parentbilling.SollFieldRevenueAccount:
		return row.line.accounts.RevenueAccount
	case parentbilling.SollFieldCostCenter:
		return row.line.accounts.CostCenter
	case parentbilling.SollFieldCostCenter2:
		return row.line.accounts.CostCenter2
	}
	return ""
}

func sollValue(mapping parentbilling.SollstellungMapping, column parentbilling.SollstellungColumn, invoice parentbilling.ExportInvoice, fileNumber int64) string {
	switch column.Field {
	case parentbilling.SollFieldInvoiceNumber:
		return invoice.Number
	case parentbilling.SollFieldInvoiceDate:
		return sollDate(mapping.DateFormat, invoice.IssuedOn)
	case parentbilling.SollFieldDueOn:
		if invoice.DueOn == nil {
			return ""
		}
		return sollDate(mapping.DateFormat, *invoice.DueOn)
	case parentbilling.SollFieldDebtorNumber:
		return invoice.DebtorNumber
	case parentbilling.SollFieldRecipientName:
		return invoice.RecipientName
	case parentbilling.SollFieldRecipientStreet:
		return invoice.Street
	case parentbilling.SollFieldRecipientPostal:
		return invoice.PostalCode
	case parentbilling.SollFieldRecipientCity:
		return invoice.City
	case parentbilling.SollFieldChildName:
		return invoice.ChildName
	case parentbilling.SollFieldSchoolName:
		return invoice.SchoolName
	case parentbilling.SollFieldServiceFrom:
		return sollDate(mapping.DateFormat, invoice.ServiceFrom)
	case parentbilling.SollFieldServiceUntil:
		return sollDate(mapping.DateFormat, invoice.ServiceUntil)
	case parentbilling.SollFieldPaymentMethod:
		return sollPaymentMethod(invoice.PaymentMethod)
	case parentbilling.SollFieldMandateReference:
		return invoice.MandateReference
	case parentbilling.SollFieldDocumentKind:
		return sollDocumentKind(invoice.Kind)
	case parentbilling.SollFieldFileNumber:
		return strconv.FormatInt(fileNumber, 10)
	case parentbilling.SollFieldConstant:
		return column.Value
	case parentbilling.SollFieldBookingText:
		return sollBookingText(column.Value, invoice)
	}
	return ""
}

func sollDate(format string, date calendar.Date) string {
	if date.IsZero() {
		return ""
	}
	if format == parentbilling.SollDateISO {
		return date.String()
	}
	return date.Format("02.01.2006")
}

// sollAmount writes cents as an amount without thousands separators:
// "-1234,50" or "-1234.50".
func sollAmount(cents int64, decimal string) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d%s%02d", sign, cents/100, decimal, cents%100)
}

func sollPaymentMethod(method string) string {
	switch method {
	case parentbilling.PaymentDirectDebit:
		return "Lastschrift"
	case parentbilling.PaymentTransfer:
		return "Überweisung"
	}
	return ""
}

func sollDocumentKind(kind string) string {
	switch kind {
	case parentbilling.DocumentCancellation:
		return "Storno"
	case parentbilling.DocumentCredit:
		return "Gutschrift"
	}
	return "Rechnung"
}

// encodeSoll writes the text in the encoding of the mapping; a character
// Windows-1252 lacks becomes "?".
func encodeSoll(name string, text []byte) ([]byte, error) {
	switch name {
	case parentbilling.SollEncodingUTF8:
		return text, nil
	case parentbilling.SollEncodingWindows1252:
		encoded := make([]byte, 0, len(text))
		for _, r := range string(text) {
			if b, ok := charmap.Windows1252.EncodeRune(r); ok {
				encoded = append(encoded, b)
			} else {
				encoded = append(encoded, '?')
			}
		}
		return encoded, nil
	}
	return append([]byte("\xef\xbb\xbf"), text...), nil
}
