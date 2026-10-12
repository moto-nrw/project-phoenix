package fileformat

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// CSVParser parses CSV files into import rows
type CSVParser struct {
	columnMapping map[string]int // CSV column name → index (lowercase)
}

// NewCSVParser creates a new CSV parser
func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

// ParseStudents parses a CSV file into student import rows
func (p *CSVParser) ParseStudents(reader io.Reader) ([]importModels.StudentImportRow, error) {
	file, err := p.ParseStudentFile(reader, importModels.StudentFileOptions{})
	return file.Rows, err
}

// ParseStudentFile parses a CSV file into student import rows. Besides the
// moto template it reads an IServ user export, which may be separated by
// semicolons and saved as Windows-1252 (#1446).
func (p *CSVParser) ParseStudentFile(reader io.Reader, opts importModels.StudentFileOptions) (importModels.StudentFile, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return importModels.StudentFile{}, fmt.Errorf("read file: %w", err)
	}
	content = studentCSVText(content)

	csvReader := csv.NewReader(bytes.NewReader(content))
	csvReader.Comma = csvDelimiter(content)
	csvReader.FieldsPerRecord = -1 // Variable columns (support any number of guardians)
	csvReader.TrimLeadingSpace = true
	csvReader.LazyQuotes = true // Handle quotes more leniently

	header, err := csvReader.Read()
	if err != nil {
		return importModels.StudentFile{}, fmt.Errorf("read header: %w", err)
	}
	rows := [][]string{header}
	for {
		values, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return importModels.StudentFile{}, rowError(len(rows)+1, err)
		}
		rows = append(rows, values)
	}

	file, mapping, err := parseStudentSheet(studentSheet{rows: rows, noRows: noCSVDataRows, keepEmptyRows: true}, opts)
	p.columnMapping = mapping
	return file, err
}

// studentCSVText drops a UTF-8 byte order mark and converts a file that is
// not UTF-8 from Windows-1252, the encoding German Excel and IServ write.
func studentCSVText(content []byte) []byte {
	content = bytes.TrimPrefix(content, []byte("\uFEFF"))
	if utf8.Valid(content) {
		return content
	}
	decoded, err := charmap.Windows1252.NewDecoder().Bytes(content)
	if err != nil {
		return content
	}
	return decoded
}

// csvDelimiter picks a semicolon when the first line holds more semicolons
// than commas, as German Excel and IServ write CSV; a comma otherwise.
func csvDelimiter(content []byte) rune {
	firstLine := content
	if end := bytes.IndexByte(content, '\n'); end >= 0 {
		firstLine = content[:end]
	}
	if bytes.Count(firstLine, []byte(";")) > bytes.Count(firstLine, []byte(",")) {
		return ';'
	}
	return ','
}
