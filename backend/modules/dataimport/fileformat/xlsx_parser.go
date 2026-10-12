package fileformat

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// XLSXParser parses Excel (.xlsx) files into import rows
type XLSXParser struct {
	columnMapping map[string]int // Excel column name → index (lowercase)
}

const (
	studentImportShortDatePattern = "yyyy-MM-dd"
	minValidBirthdaySerial        = 61
)

// NewXLSXParser creates a new XLSX parser
func NewXLSXParser() *XLSXParser {
	return &XLSXParser{}
}

// ParseStudents parses an Excel file into student import rows
func (p *XLSXParser) ParseStudents(reader io.Reader) ([]importModels.StudentImportRow, error) {
	file, err := p.ParseStudentFile(reader, importModels.StudentFileOptions{})
	return file.Rows, err
}

// ParseStudentFile parses an Excel file into student import rows. The header
// may sit below a title row; an IServ user export keeps the chosen user types.
func (p *XLSXParser) ParseStudentFile(reader io.Reader, opts importModels.StudentFileOptions) (importModels.StudentFile, error) {
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return importModels.StudentFile{}, fmt.Errorf("read file: %w", err)
	}

	f, err := excelize.OpenReader(buf, excelize.Options{
		ShortDatePattern: studentImportShortDatePattern,
	})
	if err != nil {
		return importModels.StudentFile{}, fmt.Errorf("open excel file: %w", err)
	}
	defer func() {
		_ = f.Close() // Ignore close errors in defer
	}()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return importModels.StudentFile{}, fmt.Errorf("no sheets found in Excel file")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return importModels.StudentFile{}, fmt.Errorf("read rows: %w", err)
	}
	if len(rows) == 0 {
		return importModels.StudentFile{}, fmt.Errorf("empty Excel file")
	}

	file, mapping, err := parseStudentSheet(studentSheet{
		rows: rows,
		normalize: func(rowNum int, header, values []string) []string {
			return normalizeXLSXRowValues(f, sheetName, rowNum, header, values)
		},
		noRows: noExcelDataRows,
	}, opts)
	p.columnMapping = mapping
	return file, err
}

// isEmptyRow checks if an Excel row is completely empty
func isEmptyRow(values []string) bool {
	for _, val := range values {
		if strings.TrimSpace(val) != "" {
			return false
		}
	}
	return true
}

func normalizeXLSXRowValues(f *excelize.File, sheetName string, rowNum int, header, values []string) []string {
	normalized := append([]string(nil), values...)
	limit := len(header)
	if len(normalized) < limit {
		limit = len(normalized)
	}

	for colIdx := 0; colIdx < limit; colIdx++ {
		headerKey := normalizeHeaderKey(header[colIdx])
		cellValue := strings.TrimSpace(normalized[colIdx])
		if cellValue == "" {
			continue
		}

		cellName, err := excelize.CoordinatesToCellName(colIdx+1, rowNum)
		if err != nil {
			continue
		}

		style, err := getCellStyle(f, sheetName, cellName)
		if err != nil {
			continue
		}

		if headerKey != "geburtstag" || !isDateNumberFormat(style) {
			continue
		}

		if rawValue, ok := suspiciousBirthdayRawValue(f, sheetName, cellName); ok {
			// Force obviously invalid Excel serials back through normal validation.
			normalized[colIdx] = rawValue
		}
	}

	return normalized
}

func getCellStyle(f *excelize.File, sheetName, cellName string) (*excelize.Style, error) {
	styleID, err := f.GetCellStyle(sheetName, cellName)
	if err != nil {
		return nil, err
	}
	return f.GetStyle(styleID)
}

func suspiciousBirthdayRawValue(f *excelize.File, sheetName, cellName string) (string, bool) {
	rawValue, err := f.GetCellValue(sheetName, cellName, excelize.Options{RawCellValue: true})
	if err != nil {
		return "", false
	}

	serial, err := strconv.ParseFloat(strings.TrimSpace(rawValue), 64)
	if err != nil {
		return "", false
	}

	if serial <= 0 || serial > minValidBirthdaySerial {
		return "", false
	}

	return rawValue, true
}

func isDateNumberFormat(style *excelize.Style) bool {
	if style == nil {
		return false
	}
	if style.CustomNumFmt != nil {
		custom := strings.ToLower(*style.CustomNumFmt)
		return strings.Contains(custom, "d") && strings.Contains(custom, "y")
	}

	switch style.NumFmt {
	case 14, 15, 16, 17, 22, 27, 30, 36, 50, 57, 58:
		return true
	default:
		return false
	}
}
