package fileformat

import (
	"errors"
	"io"

	"github.com/xuri/excelize/v2"
)

// The upload cap bounds only the compressed file. excelize defaults to a
// 16 GB unzip limit, so a small, highly compressed workbook could exhaust the
// shared API process. These limits sit far above any real import template.
const (
	xlsxUnzipSizeLimit    = 64 << 20
	xlsxUnzipXMLSizeLimit = 16 << 20
	// excelize pads each row with empty strings up to its last filled cell,
	// so one value in column XFD costs 16,384 strings. The budget counts those
	// padded cells; real imports stay far below it.
	xlsxCellBudget = 1_000_000
)

var errXLSXTooManyCells = errors.New("excel file has too many cells")

// openXLSX opens an import workbook with bounded decompression. A file over
// the limit fails like any other unreadable file.
func openXLSX(reader io.Reader, opts excelize.Options) (*excelize.File, error) {
	opts.UnzipSizeLimit = xlsxUnzipSizeLimit
	opts.UnzipXMLSizeLimit = xlsxUnzipXMLSizeLimit
	return excelize.OpenReader(reader, opts)
}

// readXLSXRows returns the rows of a sheet like excelize's GetRows, but stops
// once the padded cells exceed xlsxCellBudget and reports row read errors
// instead of truncating silently.
func readXLSXRows(f *excelize.File, sheet string) (result [][]string, err error) {
	rows, err := f.Rows(sheet)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); err == nil {
			err = closeErr
		}
	}()

	cells := 0
	for rowNum := 1; rows.Next(); rowNum++ {
		row, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		if len(row) == 0 {
			continue
		}
		if cells += len(row); cells > xlsxCellBudget {
			return nil, errXLSXTooManyCells
		}
		for len(result) < rowNum-1 {
			result = append(result, nil)
		}
		result = append(result, row)
	}
	return result, rows.Error()
}
