package listexport

import (
	"bytes"
	"encoding/csv"
	"regexp"
	"strings"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
)

// renderCSV writes the list as a flat table (#4663): the column labels, then
// one line per row with values. Title, filters and group headings stay out,
// so a program reads the file as data. Semicolons and a UTF-8 byte order mark
// let German spreadsheet programs open it directly; a cell that a
// spreadsheet would read as a formula gets a leading apostrophe.
func renderCSV(doc listrecords.Document) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buffer)
	writer.Comma = ';'
	header := make([]string, len(doc.Columns))
	for i, column := range doc.Columns {
		header[i] = csvCell(column.Label)
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	for _, row := range doc.Rows {
		if len(row.Values) == 0 {
			continue
		}
		line := make([]string, len(doc.Columns))
		for i, column := range doc.Columns {
			line[i] = csvCell(listrecords.StripStyleMarkers(row.Values[column.ID]))
		}
		if err := writer.Write(line); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// germanNumber is a plain amount such as "-1.234,56": a spreadsheet reads it
// as a number, never as a formula (#4672).
var germanNumber = regexp.MustCompile(`^-?\d{1,3}(\.\d{3})*(,\d+)?$|^-?\d+(,\d+)?$`)

// csvCell keeps a value from being read as a spreadsheet formula. A negative
// amount stays a number.
func csvCell(value string) string {
	if value != "" && strings.ContainsRune("=+-@\t\r", rune(value[0])) && !germanNumber.MatchString(value) {
		return "'" + value
	}
	return value
}
