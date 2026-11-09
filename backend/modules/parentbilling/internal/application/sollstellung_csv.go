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

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// sollTextFields are the fields with free text of a person; a value that a
// spreadsheet would read as a formula gets a leading apostrophe there.
var sollTextFields = []string{parentbilling.SollFieldRecipientName, parentbilling.SollFieldRecipientStreet,
	parentbilling.SollFieldRecipientCity, parentbilling.SollFieldChildName, parentbilling.SollFieldSchoolName,
	parentbilling.SollFieldMandateReference, parentbilling.SollFieldDebtorNumber, parentbilling.SollFieldConstant}

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
	case column.Field == parentbilling.SollFieldConstant && (utf8.RuneCountInString(column.Value) > parentbilling.MaxSollConstantLength ||
		strings.ContainsAny(column.Value, "\r\n")):
		errs[key+".value"] = errors.New("is too long or has a line break")
	}
	if column.Field != parentbilling.SollFieldConstant {
		column.Value = ""
	}
}

// sollFileName writes the file number into the pattern.
func sollFileName(pattern string, fileNumber int64) string {
	return strings.Replace(pattern, parentbilling.SollFileNumberToken, strconv.FormatInt(fileNumber, 10), 1)
}

// renderSollstellung writes the file of the invoices with the mapping.
func renderSollstellung(mapping parentbilling.SollstellungMapping, invoices []parentbilling.ExportInvoice, fileNumber int64) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	writer.Comma, _ = utf8.DecodeRuneInString(mapping.Delimiter)
	writer.UseCRLF = true
	if mapping.HeaderRow {
		header := make([]string, len(mapping.Columns))
		for i, column := range mapping.Columns {
			header[i] = column.Header
		}
		if err := writer.Write(header); err != nil {
			return nil, err
		}
	}
	for _, invoice := range invoices {
		record := make([]string, len(mapping.Columns))
		for i, column := range mapping.Columns {
			record[i] = sollCell(mapping, column, invoice, fileNumber)
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
func sollCell(mapping parentbilling.SollstellungMapping, column parentbilling.SollstellungColumn, invoice parentbilling.ExportInvoice, fileNumber int64) string {
	value := sollValue(mapping, column, invoice, fileNumber)
	if slices.Contains(sollTextFields, column.Field) && value != "" && strings.ContainsRune("=+-@\t\r", rune(value[0])) {
		return "'" + value
	}
	return value
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
	case parentbilling.SollFieldAmount:
		return sollAmount(invoice.TotalCents, mapping.DecimalSeparator)
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
	case "cancellation":
		return "Storno"
	case "credit":
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
