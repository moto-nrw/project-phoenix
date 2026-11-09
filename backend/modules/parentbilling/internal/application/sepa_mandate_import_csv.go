package application

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	text, encoding, err := decodeImportText(content)
	if err != nil {
		return importTable{}, err
	}
	comma, err := importDelimiter(text, delimiter)
	if err != nil {
		return importTable{}, err
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma, reader.FieldsPerRecord, reader.LazyQuotes, reader.ReuseRecord = comma, -1, true, false
	table := importTable{encoding: encoding, delimiter: string(comma)}
	if err := table.read(reader); err != nil {
		return importTable{}, err
	}
	if len(table.headers) < 2 || len(table.rows) == 0 {
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

// decodeImportText answers the file as text: UTF-8 when it is valid UTF-8,
// else Windows-1252. Control characters other than tab and line breaks
// mark a file that is no text, whatever its name says.
func decodeImportText(content []byte) (string, string, error) {
	content = bytes.TrimPrefix(content, []byte("\xEF\xBB\xBF"))
	encoding := encodingUTF8
	if !utf8.Valid(content) {
		decoded, err := charmap.Windows1252.NewDecoder().Bytes(content)
		if err != nil {
			return "", "", parentbilling.ErrImportFileInvalid
		}
		content, encoding = decoded, encodingWindows1252
	}
	text := string(content)
	for _, r := range text {
		if r == unicode.ReplacementChar || (unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r') {
			return "", "", parentbilling.ErrImportFileInvalid
		}
	}
	if strings.TrimSpace(text) == "" {
		return "", "", parentbilling.ErrImportFileInvalid
	}
	return text, encoding, nil
}

// importDelimiter is the chosen delimiter or, when none is chosen, the one
// of semicolon, comma and tab the header line holds most often outside
// quotes; a tie takes the semicolon German spreadsheets write.
func importDelimiter(text, chosen string) (rune, error) {
	switch chosen {
	case ";", ",", "\t":
		return rune(chosen[0]), nil
	case "":
	default:
		return 0, fmt.Errorf("%w: unknown delimiter", parentbilling.ErrImportFileInvalid)
	}
	header, _, _ := strings.Cut(text, "\n")
	counts := map[rune]int{}
	quoted := false
	for _, r := range header {
		switch {
		case r == '"':
			quoted = !quoted
		case !quoted && (r == ';' || r == ',' || r == '\t'):
			counts[r]++
		}
	}
	best := ';'
	for _, candidate := range []rune{',', '\t'} {
		if counts[candidate] > counts[best] {
			best = candidate
		}
	}
	return best, nil
}

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
		LastCollected: found[parentbilling.ImportFieldLastCollected],
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

// parseImportDate reads a German (31.12.2024, 1.2.2024, 31/12/2024) or an
// ISO date (2024-12-31). A two-digit year is ambiguous and refused.
func parseImportDate(value string) (calendar.Date, bool) {
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

// importSequence reads the sequence a file names: the SEPA sequence type
// or its German name. ok is false for a value the import does not know;
// oneOff marks a one-off mandate the import does not take over.
func importSequence(value string) (sequence string, oneOff, ok bool) {
	switch importHeaderKey(value) {
	case "frst", "first", "erst", "erstlastschrift", "ersteinzug", "neu", "e":
		return parentbilling.SequenceFirst, false, true
	case "rcur", "recurring", "folge", "folgelastschrift", "wiederkehrend", "laufend", "f":
		return parentbilling.SequenceRecurring, false, true
	case "ooff", "oneoff", "einmalig", "einzellastschrift", "einzelmandat":
		return "", true, false
	}
	return "", false, false
}
