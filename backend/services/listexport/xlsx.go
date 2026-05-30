package listexport

import (
	"bytes"

	"github.com/xuri/excelize/v2"
)

func renderXLSX(doc Document) ([]byte, error) {
	f := excelize.NewFile()

	sheet := "Export"
	index, err := f.NewSheet(sheet)
	if err != nil {
		return nil, err
	}
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return nil, err
	}
	f.SetActiveSheet(index)

	titleStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 16}})
	if err != nil {
		return nil, err
	}
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:   excelize.Fill{Type: "pattern", Color: []string{"1F2937"}, Pattern: 1},
		Border: []excelize.Border{{Type: "bottom", Color: "D1D5DB", Style: 1}},
	})
	if err != nil {
		return nil, err
	}

	if err := f.SetCellValue(sheet, "A1", doc.Title); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", "A1", titleStyle); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, "A2", doc.Subtitle); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, "A3", "Erstellt: "+GeneratedAtLabel(doc.GeneratedAt)); err != nil {
		return nil, err
	}

	for idx, filter := range doc.Filters {
		cell, _ := excelize.CoordinatesToCellName(idx+1, 4)
		if err := f.SetCellValue(sheet, cell, filter); err != nil {
			return nil, err
		}
	}

	headerRow := 6
	for idx, column := range doc.Columns {
		cell, _ := excelize.CoordinatesToCellName(idx+1, headerRow)
		if err := f.SetCellValue(sheet, cell, column.Label); err != nil {
			return nil, err
		}
	}
	lastHeaderCell, _ := excelize.CoordinatesToCellName(len(doc.Columns), headerRow)
	if err := f.SetCellStyle(sheet, "A6", lastHeaderCell, headerStyle); err != nil {
		return nil, err
	}

	for rowIdx, row := range doc.Rows {
		for colIdx, column := range doc.Columns {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, headerRow+rowIdx+1)
			if err := f.SetCellValue(sheet, cell, row.Values[column.ID]); err != nil {
				return nil, err
			}
		}
	}

	for idx := range doc.Columns {
		col, _ := excelize.ColumnNumberToName(idx + 1)
		width := 18.0
		if doc.Columns[idx].ID == ColumnName {
			width = 26
		}
		if err := f.SetColWidth(sheet, col, col, width); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
