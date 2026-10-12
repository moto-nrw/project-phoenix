package fileformat

import (
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// maxTitleRows is how many rows above the header a student file may carry,
// such as the file name IServ or Excel writes into the first row (#1446).
const maxTitleRows = 5

// studentSheet is a student upload read into rows, shared by the CSV and
// Excel parsers.
type studentSheet struct {
	rows [][]string
	// normalize adjusts a data row before mapping; nil keeps it as read.
	normalize func(rowNum int, header, values []string) []string
	// noRows is the error for a file without data rows.
	noRows func() error
	// keepEmptyRows maps blank rows too, so the preview flags them (CSV).
	keepEmptyRows bool
}

// findHeaderRow returns the index of the first row that names both
// "Vorname" and "Nachname". A file without such a row keeps the first row
// as its header, so the usual missing-column errors still apply.
func findHeaderRow(rows [][]string) int {
	for i := 0; i < len(rows) && i <= maxTitleRows; i++ {
		mapping := headerMapping(rows[i])
		_, first := mapping["vorname"]
		_, last := mapping["nachname"]
		if first && last {
			return i
		}
	}
	return 0
}

// headerMapping maps each normalized header key to its column index.
func headerMapping(header []string) map[string]int {
	mapping := make(map[string]int, len(header))
	for i, col := range header {
		mapping[normalizeHeaderKey(col)] = i
	}
	return mapping
}

// parseStudentSheet maps the data rows below the header. Row numbers are the
// file's (1-based). An IServ export may legitimately keep no row, when no
// user type is chosen; the preview then still lists the user types. It also
// returns the header mapping it used.
func parseStudentSheet(sheet studentSheet, opts importModels.StudentFileOptions) (importModels.StudentFile, map[string]int, error) {
	headerIdx := findHeaderRow(sheet.rows)
	header := sheet.rows[headerIdx]
	mapping := headerMapping(header)
	filter := newIServFilter(mapping, opts)

	var rows []importModels.StudentImportRow
	for i := headerIdx + 1; i < len(sheet.rows); i++ {
		rowNum := i + 1
		values := sheet.rows[i]
		if sheet.normalize != nil {
			values = sheet.normalize(rowNum, header, values)
		}
		if !sheet.keepEmptyRows && isEmptyRow(values) {
			continue
		}
		mapper := NewColumnMapper(mapping, values)
		if !filter.keep(mapper) {
			continue
		}
		row, err := MapStudentRow(mapper)
		if err != nil {
			return importModels.StudentFile{}, mapping, rowError(rowNum, err)
		}
		rows = append(rows, row)
	}

	// Empty files (only headers) likely indicate the template was uploaded
	// by mistake.
	if filter == nil && len(rows) == 0 {
		return importModels.StudentFile{}, mapping, sheet.noRows()
	}
	return importModels.StudentFile{Rows: rows, IServ: filter.summary()}, mapping, nil
}
