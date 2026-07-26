package listexport

import (
	"bytes"
	"fmt"

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
	groupStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "111827"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"E5E7EB"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "top", Color: "D1D5DB", Style: 1},
			{Type: "bottom", Color: "D1D5DB", Style: 1},
		},
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

	excelRow := headerRow + 1
	currentGroup := ""
	for _, row := range doc.Rows {
		hasValues := len(row.Values) > 0
		if row.GroupTitle != "" && (!hasValues || row.GroupTitle != currentGroup) {
			currentGroup = row.GroupTitle
			startCell, _ := excelize.CoordinatesToCellName(1, excelRow)
			lastColumn := len(doc.Columns)
			if lastColumn < 1 {
				lastColumn = 1
			}
			endCell, _ := excelize.CoordinatesToCellName(lastColumn, excelRow)
			if err := f.SetCellValue(sheet, startCell, row.GroupTitle); err != nil {
				return nil, err
			}
			if err := f.MergeCell(sheet, startCell, endCell); err != nil {
				return nil, err
			}
			if err := f.SetCellStyle(sheet, startCell, endCell, groupStyle); err != nil {
				return nil, err
			}
			excelRow++
		}
		if !hasValues {
			continue
		}
		for colIdx, column := range doc.Columns {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, excelRow)
			if err := f.SetCellValue(sheet, cell, row.Values[column.ID]); err != nil {
				return nil, err
			}
		}
		excelRow++
	}

	for idx := range doc.Columns {
		col, _ := excelize.ColumnNumberToName(idx + 1)
		width := 18.0
		switch doc.Columns[idx].ID {
		case ColumnName:
			width = 26
		case ColumnDeparture, ColumnGuardianContacts:
			width = 32
		}
		if err := f.SetColWidth(sheet, col, col, width); err != nil {
			return nil, err
		}
	}

	if err := applyPrintSetup(f, sheet, headerRow); err != nil {
		return nil, err
	}
	if err := applyConfidentialityFooter(f, sheet, doc.Footer); err != nil {
		return nil, err
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

// applyPrintSetup gives the sheet sane print defaults so the file is
// usable on paper, not just on screen: A4 landscape, scaled to one page
// wide (as many pages tall as needed), with the header row repeated on
// every printed page and frozen on screen. XLSX is still a data format
// — for a print-perfect layout of variable-width data the PDF export is
// the right artifact — but these defaults stop a plain print from
// splitting columns awkwardly across pages with no header.
func applyPrintSetup(f *excelize.File, sheet string, headerRow int) error {
	a4 := 9
	landscape := "landscape"
	fitWidth := 1
	fitHeight := 0 // 0 = unlimited pages tall
	if err := f.SetPageLayout(sheet, &excelize.PageLayoutOptions{
		Size:        &a4,
		Orientation: &landscape,
		FitToWidth:  &fitWidth,
		FitToHeight: &fitHeight,
	}); err != nil {
		return err
	}

	fit := true
	if err := f.SetSheetProps(sheet, &excelize.SheetPropsOptions{FitToPage: &fit}); err != nil {
		return err
	}

	// Repeat the header row on every printed page.
	if err := f.SetDefinedName(&excelize.DefinedName{
		Name:     "_xlnm.Print_Titles",
		RefersTo: fmt.Sprintf("'%s'!$%d:$%d", sheet, headerRow, headerRow),
		Scope:    sheet,
	}); err != nil {
		return err
	}

	// Freeze the header row on screen so it stays visible while scrolling.
	return f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      headerRow,
		TopLeftCell: fmt.Sprintf("A%d", headerRow+1),
		ActivePane:  "bottomLeft",
	})
}

// applyConfidentialityFooter stamps the document footer (e.g. a GDPR
// confidentiality note) at the centre foot of every printed page — the
// XLSX counterpart of the PDF's per-page footer, so a printed XLSX
// carries the same handling instruction as the PDF. Empty footer is a
// no-op. "&C" centres the section, "&8" sets 8pt; Excel caps each footer
// section at 255 chars, well above the note. Set on odd/even/first so it
// shows regardless of page parity or the "different first page" flag.
func applyConfidentialityFooter(f *excelize.File, sheet, footer string) error {
	if footer == "" {
		return nil
	}
	text := "&C&8" + footer
	return f.SetHeaderFooter(sheet, &excelize.HeaderFooterOptions{
		OddFooter:   text,
		EvenFooter:  text,
		FirstFooter: text,
	})
}
