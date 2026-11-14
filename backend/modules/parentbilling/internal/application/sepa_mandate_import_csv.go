package application

import (
	"errors"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
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
	// A NUL may stand where a CSV has a quote (#7009); the reader takes it
	// off each cell and refuses the file for a NUL anywhere else.
	read, err := textnorm.ReadCSVTable(content, delimiter, parentbilling.MaxImportRows)
	if errors.Is(err, textnorm.ErrCSVTooManyRows) {
		return importTable{}, parentbilling.ErrImportFileTooLarge
	}
	if err != nil {
		return importTable{}, parentbilling.ErrImportFileInvalid
	}
	table := importTable{encoding: read.Encoding, delimiter: read.Delimiter, headers: read.Headers, rows: read.Rows, lines: read.Lines}
	if len(table.headers) < 2 || len(table.rows) == 0 || headersHoldAnIBAN(table.headers) || textnorm.RepeatedCSVHeader(table.headers) {
		return importTable{}, parentbilling.ErrImportFileInvalid
	}
	return table, nil
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

// suggestImportMapping maps each field to the first header with one of its
// names; the payer key follows the payer header.
func suggestImportMapping(headers []string) parentbilling.SEPAMandateImportMapping {
	found := map[string]string{}
	for _, header := range headers {
		key := textnorm.HeaderKey(header)
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
	if payer := textnorm.HeaderKey(mapping.Columns.Payer); payer != "" && !strings.Contains(payer, "mail") {
		mapping.PayerKey = parentbilling.PayerByContract
	}
	// A file without contract numbers and addresses names its payers by the
	// debtor number (#7009), as the master data of OGS Connect does.
	if mapping.Columns.Payer == "" && mapping.Columns.DebtorNumber != "" {
		mapping.Columns.Payer, mapping.PayerKey = mapping.Columns.DebtorNumber, parentbilling.PayerByDebtorNumber
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
	switch textnorm.HeaderKey(withoutScheme(value)) {
	case "frst", "first", "erst", "erstlastschrift", "ersteinzug", "neu", "e":
		return parentbilling.SequenceFirst, false, true
	case "rcur", "recurring", "folge", "folgelastschrift", "wiederkehrend", "laufend", "f":
		return parentbilling.SequenceRecurring, false, true
	case "ooff", "oneoff", "einmalig", "einzellastschrift", "einzelmandat":
		return "", true, false
	}
	return "", false, false
}
