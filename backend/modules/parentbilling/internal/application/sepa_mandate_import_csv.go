package application

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/sepa"
	"github.com/moto-ogs/project-phoenix/sharedkernel/textnorm"
)

// Reading the CSV file of a mandate import (#6198): the encoding (UTF-8,
// with or without BOM, else Windows-1252 as Excel writes it in Germany),
// the delimiter (semicolon, comma or tab) and the header names moto maps
// by itself.

// Encodings the import reads.
const (
	encodingUTF8        = "utf-8"
	encodingWindows1252 = "windows-1252"
)

// importTable is a decoded file: its headers and its rows with the line
// each starts on.
type importTable struct {
	encoding  string
	delimiter string
	headers   []string
	rows      [][]string
	lines     []int
}

// readImportTable decodes and splits the file. It refuses binary content,
// a file without a header and data row, and more than MaxImportRows rows.
func readImportTable(content []byte, delimiter string) (importTable, error) {
	if len(content) > parentbilling.MaxImportFileBytes {
		return importTable{}, parentbilling.ErrImportFileTooLarge
	}
	// A NUL may stand where a CSV has a quote (#7009); read takes it off
	// each cell and refuses the file for a NUL anywhere else.
	text, encoding, err := textnorm.DecodeCSVTextWithNULQuotes(content)
	if err != nil {
		return importTable{}, parentbilling.ErrImportFileInvalid
	}
	comma, err := textnorm.CSVDelimiter(text, delimiter)
	if err != nil {
		return importTable{}, parentbilling.ErrImportFileInvalid
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma, reader.FieldsPerRecord, reader.LazyQuotes, reader.ReuseRecord = comma, -1, true, false
	table := importTable{encoding: encoding, delimiter: string(comma)}
	if err := table.read(reader); err != nil {
		return importTable{}, err
	}
	if len(table.headers) < 2 || len(table.rows) == 0 || headersHoldAnIBAN(table.headers) || headersRepeat(table.headers) {
		return importTable{}, parentbilling.ErrImportFileInvalid
	}
	return table, nil
}

// read takes the first record as the headers and every further record
// that is not blank as a row with the line it starts on.
func (table *importTable) read(reader *csv.Reader) error {
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return parentbilling.ErrImportFileInvalid
		}
		for i, cell := range record {
			unquoted, ok := textnorm.UnquoteNULCell(cell)
			if !ok {
				return parentbilling.ErrImportFileInvalid
			}
			record[i] = unquoted
		}
		line, _ := reader.FieldPos(0)
		switch {
		case table.headers == nil:
			table.headers = trimCells(record)
		case blankRecord(record):
		case len(table.rows) == parentbilling.MaxImportRows:
			return parentbilling.ErrImportFileTooLarge
		default:
			table.rows, table.lines = append(table.rows, trimCells(record)), append(table.lines, line)
		}
	}
}

// headersHoldAnIBAN tells whether the first row is a mandate rather than
// headers: the answer repeats the headers, and an IBAN may leave moto only
// masked (ADR 0171).
func headersHoldAnIBAN(headers []string) bool {
	for _, header := range headers {
		if containsIBAN(header) {
			return true
		}
	}
	return false
}

// headersRepeat tells whether two columns have the same header: a mapping
// names a column by its header, so it could not tell them apart.
func headersRepeat(headers []string) bool {
	seen := map[string]bool{}
	for _, header := range headers {
		if header == "" {
			continue
		}
		if seen[header] {
			return true
		}
		seen[header] = true
	}
	return false
}

// containsIBAN tells whether a value holds a valid IBAN anywhere, also
// inside other text („Eva (DE89 3704 …)“): every position where two letters
// and two digits start is tried with every IBAN length.
func containsIBAN(value string) bool {
	compact := sepa.Compact(value)
	for from := 0; from+15 <= len(compact); from++ {
		if !ibanStart(compact[from:]) {
			continue
		}
		for to := from + 15; to <= len(compact) && to-from <= 34; to++ {
			if sepa.ValidIBAN(compact[from:to]) {
				return true
			}
		}
	}
	return false
}

// ibanStart tells whether a value starts like an IBAN: country letters and
// check digits.
func ibanStart(value string) bool {
	return len(value) >= 4 && isUpperLetter(value[0]) && isUpperLetter(value[1]) && isDigit(value[2]) && isDigit(value[3])
}

func isUpperLetter(b byte) bool { return b >= 'A' && b <= 'Z' }

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func trimCells(record []string) []string {
	cells := make([]string, len(record))
	for i, cell := range record {
		cells[i] = strings.TrimSpace(cell)
	}
	return cells
}

func blankRecord(record []string) bool {
	for _, cell := range record {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// importHeaderNames are the header names moto maps without help, after
// importHeaderKey folded them.
var importHeaderNames = map[string][]string{
	parentbilling.ImportFieldReference: {"mandatsreferenz", "mandatreferenz", "referenz", "mandatsref", "mandatsnummer",
		"mandatsid", "mandatereference", "mandateid", "reference"},
	parentbilling.ImportFieldSignedOn: {"mandatsdatum", "datumdermandatserteilung", "unterschriftsdatum", "unterschriebenam",
		"erteiltam", "unterzeichnetam", "datumderunterschrift", "mandatdatum", "signaturdatum", "dateofsignature", "signedon", "mandatedate"},
	parentbilling.ImportFieldIBAN:          {"iban", "kontoiban"},
	parentbilling.ImportFieldBIC:           {"bic", "swift", "swiftbic"},
	parentbilling.ImportFieldAccountHolder: {"kontoinhaber", "kontoinhaberin", "inhaber", "namedeskontoinhabers", "accountholder", "debtorname"},
	parentbilling.ImportFieldPayer: {"email", "emailadresse", "mail", "emailzahler", "emaildereltern", "vertragsnummer",
		"vertragsid", "vertrag", "contractid"},
	parentbilling.ImportFieldCreditorID: {"glaeubigerid", "glaeubigeridentifikationsnummer", "glaeubigeridnr", "creditorid", "ci"},
	parentbilling.ImportFieldSequence: {"sequenz", "sequenztyp", "lastschriftart", "lastschrifttyp", "erstfolge",
		"erstoderfolgelastschrift", "sequencetype", "status"},
	parentbilling.ImportFieldLastCollected: {"letztereinzug", "letztelastschrift", "zuletzteingezogen", "letzteeinziehung",
		"datumletztereinzug", "lastcollection", "lastcollectedon"},
	parentbilling.ImportFieldDebtorNumber: {"debitorennummer", "kinderdebitorennummer", "debitornummer", "debitor",
		"debitorennr", "debitornr", "kundennummer", "personenkonto", "debtornumber"},
}

// importHeaderKey folds a header for the comparison: lower case, umlauts
// spelled out, letters and digits only.
func importHeaderKey(header string) string {
	folded := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss").Replace(strings.ToLower(header))
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, folded)
}

// suggestImportMapping maps each field to the first header with one of its
// names; the payer key follows the payer header.
func suggestImportMapping(headers []string) parentbilling.SEPAMandateImportMapping {
	found := map[string]string{}
	for _, header := range headers {
		key := importHeaderKey(header)
		for field, names := range importHeaderNames {
			if _, done := found[field]; !done && containsString(names, key) {
				found[field] = header
			}
		}
	}
	mapping := parentbilling.SEPAMandateImportMapping{Columns: parentbilling.SEPAMandateImportColumns{
		Reference: found[parentbilling.ImportFieldReference], SignedOn: found[parentbilling.ImportFieldSignedOn],
		IBAN: found[parentbilling.ImportFieldIBAN], BIC: found[parentbilling.ImportFieldBIC],
		AccountHolder: found[parentbilling.ImportFieldAccountHolder], Payer: found[parentbilling.ImportFieldPayer],
		CreditorID: found[parentbilling.ImportFieldCreditorID], Sequence: found[parentbilling.ImportFieldSequence],
		LastCollected: found[parentbilling.ImportFieldLastCollected], DebtorNumber: found[parentbilling.ImportFieldDebtorNumber],
	}, PayerKey: parentbilling.PayerByEmail}
	if payer := importHeaderKey(mapping.Columns.Payer); payer != "" && !strings.Contains(payer, "mail") {
		mapping.PayerKey = parentbilling.PayerByContract
	}
	return mapping
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// importDateTime is a date with a time of day after it, as OGS Connect
// writes "31.12.2024 14:05" (#7009); the time is dropped.
var importDateTime = regexp.MustCompile(`^(\S+)[ T]\d{1,2}:\d{2}(:\d{2})?$`)

// parseImportDate reads a German (31.12.2024, 1.2.2024, 31/12/2024) or an
// ISO date (2024-12-31), also with a time of day after it. A two-digit year
// is ambiguous and refused.
func parseImportDate(value string) (calendar.Date, bool) {
	if match := importDateTime.FindStringSubmatch(value); match != nil {
		value = match[1]
	}
	if date, err := calendar.ParseDate(value); err == nil {
		return date, true
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '.' || r == '/' })
	if len(parts) != 3 || len(parts[2]) != 4 {
		return "", false
	}
	day, errDay := strconv.Atoi(parts[0])
	month, errMonth := strconv.Atoi(parts[1])
	year, errYear := strconv.Atoi(parts[2])
	if errDay != nil || errMonth != nil || errYear != nil {
		return "", false
	}
	date, err := calendar.ParseDate(fmt.Sprintf("%04d-%02d-%02d", year, month, day))
	if err != nil {
		return "", false
	}
	return date, true
}

// withoutScheme drops the SEPA scheme from a value that names it beside
// the sequence, as "CORE/RCUR" or "CORE RCUR" (#7009): the scheme of a
// mandate the import takes over is always CORE.
func withoutScheme(value string) string {
	words := strings.FieldsFunc(value, func(r rune) bool { return r == '/' || r == ' ' || r == ',' || r == '-' || r == '|' })
	if len(words) < 2 {
		return value
	}
	kept := words[:0]
	for _, word := range words {
		if !strings.EqualFold(word, "CORE") {
			kept = append(kept, word)
		}
	}
	return strings.Join(kept, " ")
}

// importSequence reads the sequence a file names: the SEPA sequence type
// or its German name. ok is false for a value the import does not know;
// oneOff marks a one-off mandate the import does not take over.
func importSequence(value string) (sequence string, oneOff, ok bool) {
	switch importHeaderKey(withoutScheme(value)) {
	case "frst", "first", "erst", "erstlastschrift", "ersteinzug", "neu", "e":
		return parentbilling.SequenceFirst, false, true
	case "rcur", "recurring", "folge", "folgelastschrift", "wiederkehrend", "laufend", "f":
		return parentbilling.SequenceRecurring, false, true
	case "ooff", "oneoff", "einmalig", "einzellastschrift", "einzelmandat":
		return "", true, false
	}
	return "", false, false
}
