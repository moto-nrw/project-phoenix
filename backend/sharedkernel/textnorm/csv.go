package textnorm

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// DecodeCSVText answers the file as text: UTF-8 when it is valid UTF-8,
// else Windows-1252. Control characters other than tab and line breaks
// mark a file that is no text, whatever its name says.
func DecodeCSVText(content []byte) (string, string, error) {
	return decodeCSVText(content, false)
}

// DecodeCSVTextWithNULQuotes is DecodeCSVText for a file that may write a
// NUL where a CSV has a quote, as OGS Connect does (#7009): it keeps the NUL
// in the text, and the reader of each cell takes it off with UnquoteNULCell.
func DecodeCSVTextWithNULQuotes(content []byte) (string, string, error) {
	return decodeCSVText(content, true)
}

func decodeCSVText(content []byte, nulQuotes bool) (string, string, error) {
	content = bytes.TrimPrefix(content, []byte("\xEF\xBB\xBF"))
	encoding := "utf-8"
	if !utf8.Valid(content) {
		decoded, err := charmap.Windows1252.NewDecoder().Bytes(content)
		if err != nil {
			return "", "", errors.New("invalid CSV text")
		}
		content, encoding = decoded, "windows-1252"
	}
	text := string(content)
	for _, r := range text {
		if nulQuotes && r == 0 {
			continue
		}
		if r == unicode.ReplacementChar || (unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r') {
			return "", "", errors.New("invalid CSV text")
		}
	}
	if strings.TrimSpace(strings.ReplaceAll(text, "\x00", "")) == "" {
		return "", "", errors.New("invalid CSV text")
	}
	return text, encoding, nil
}

// UnquoteNULCell takes the NUL quotes off a cell a csv.Reader read from a
// DecodeCSVTextWithNULQuotes text (#7009): a cell without NUL stays, a cell
// that starts and ends with a NUL and has none in between loses both. ok is
// false for any other NUL, also one a delimiter or a line break inside the
// quotes left at one end only: the file is then no text the import can read.
func UnquoteNULCell(cell string) (string, bool) {
	if !strings.Contains(cell, "\x00") {
		return cell, true
	}
	if len(cell) >= 2 && cell[0] == 0 && cell[len(cell)-1] == 0 && !strings.Contains(cell[1:len(cell)-1], "\x00") {
		return cell[1 : len(cell)-1], true
	}
	return "", false
}

// CSVDelimiter is the chosen delimiter or, when none is chosen, the one
// of semicolon, comma and tab the first nonempty line holds most often outside
// quotes; a tie takes the semicolon German spreadsheets write.
func CSVDelimiter(text, chosen string) (rune, error) {
	switch chosen {
	case ";", ",", "\t":
		return rune(chosen[0]), nil
	case "":
	default:
		return 0, errors.New("invalid CSV delimiter")
	}
	header, rest, _ := strings.Cut(text, "\n")
	for (header == "" || header == "\r") && rest != "" {
		header, rest, _ = strings.Cut(rest, "\n")
	}
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

// CSVTable is a decoded CSV file: how it was read, its trimmed headers and
// its rows with the line each starts on.
type CSVTable struct {
	Encoding  string
	Delimiter string
	Headers   []string
	Rows      [][]string
	Lines     []int
}

var (
	// ErrCSVInvalid reports content that is no readable CSV text with a
	// header row.
	ErrCSVInvalid = errors.New("invalid CSV file")
	// ErrCSVTooManyRows reports a file with more data rows than the caller
	// takes.
	ErrCSVTooManyRows = errors.New("too many CSV rows")
)

// ReadCSVTable decodes an import file and splits it: the first record is
// the header row, every further record that is not blank is a row. A NUL may
// stand where a CSV has a quote (#7009); it is taken off each cell and
// refuses the file anywhere else. Cells are trimmed. chosen is ";", "," or
// "\t", empty detects the delimiter.
func ReadCSVTable(content []byte, chosen string, maxRows int) (CSVTable, error) {
	text, encoding, err := DecodeCSVTextWithNULQuotes(content)
	if err != nil {
		return CSVTable{}, ErrCSVInvalid
	}
	comma, err := CSVDelimiter(text, chosen)
	if err != nil {
		return CSVTable{}, ErrCSVInvalid
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma, reader.FieldsPerRecord, reader.LazyQuotes, reader.ReuseRecord = comma, -1, true, false
	table := CSVTable{Encoding: encoding, Delimiter: string(comma)}
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return table, nil
		}
		if err != nil {
			return CSVTable{}, ErrCSVInvalid
		}
		blank := true
		for i, cell := range record {
			unquoted, ok := UnquoteNULCell(cell)
			if !ok {
				return CSVTable{}, ErrCSVInvalid
			}
			record[i] = strings.TrimSpace(unquoted)
			blank = blank && record[i] == ""
		}
		line, _ := reader.FieldPos(0)
		switch {
		case table.Headers == nil:
			table.Headers = record
		case blank:
		case len(table.Rows) == maxRows:
			return CSVTable{}, ErrCSVTooManyRows
		default:
			table.Rows, table.Lines = append(table.Rows, record), append(table.Lines, line)
		}
	}
}

// RepeatedCSVHeader tells whether two columns have the same header: a
// mapping names a column by its header, so it could not tell them apart.
// Columns without a header never count.
func RepeatedCSVHeader(headers []string) bool {
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

// HeaderKey folds a header for a comparison: lower case, umlauts spelled
// out, letters and digits only.
func HeaderKey(header string) string {
	folded := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss").Replace(strings.ToLower(header))
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, folded)
}
