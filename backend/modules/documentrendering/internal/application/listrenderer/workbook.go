package listexport

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// RenderWorkbook renders typed cells with a header and optional decimal columns.
// Decimal column positions are one-based, matching spreadsheet coordinates.
func RenderWorkbook(sheet string, headers []string, rows [][]any, decimalColumns []int) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(idx)
	if sheet != "Sheet1" {
		_ = f.DeleteSheet("Sheet1")
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E2E8F0"}, Pattern: 1},
	})
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, header)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}
	decimalColumnSet := make(map[int]bool, len(decimalColumns))
	for _, column := range decimalColumns {
		decimalColumnSet[column] = true
	}
	decimalStyle := 0
	if len(decimalColumnSet) > 0 {
		numberFormat := "0.00"
		decimalStyle, err = f.NewStyle(&excelize.Style{CustomNumFmt: &numberFormat})
		if err != nil {
			return nil, fmt.Errorf("failed to create decimal cell style: %w", err)
		}
	}
	for rowIdx, row := range rows {
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheet, cell, value)
			if decimalColumnSet[colIdx+1] {
				_ = f.SetCellStyle(sheet, cell, cell, decimalStyle)
			}
		}
	}
	for i := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, 16)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to write XLSX: %w", err)
	}
	return buf.Bytes(), nil
}
