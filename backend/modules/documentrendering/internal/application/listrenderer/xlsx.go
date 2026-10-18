package listexport

import (
	"bytes"
	"fmt"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"

	"github.com/xuri/excelize/v2"
)

func renderXLSX(doc listrecords.Document) ([]byte, error) {
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
	styles, err := newXLSXStyles(f)
	if err != nil {
		return nil, err
	}
	if err := writeXLSXHeading(f, sheet, doc, styles); err != nil {
		return nil, err
	}
	if err := writeXLSXRows(f, sheet, doc, styles); err != nil {
		return nil, err
	}
	if err := setXLSXWidths(f, sheet, doc); err != nil {
		return nil, err
	}
	if err := applyPrintSetup(f, sheet, 6); err != nil {
		return nil, err
	}
	if err := applyConfidentialityFooter(f, sheet, doc.Footer); err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	if err := f.Write(&buffer); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

type xlsxStyles struct{ title, header, group, body int }

func newXLSXStyles(f *excelize.File) (xlsxStyles, error) {
	definitions := []*excelize.Style{
		&excelize.Style{Font: &excelize.Font{Bold: true, Size: 16}},
		&excelize.Style{
			Font:   &excelize.Font{Bold: true, Color: "FFFFFF"},
			Fill:   excelize.Fill{Type: "pattern", Color: []string{"1F2937"}, Pattern: 1},
			Border: []excelize.Border{{Type: "bottom", Color: "D1D5DB", Style: 1}},
		},
		&excelize.Style{
			Font: &excelize.Font{Bold: true, Color: "111827"},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"E5E7EB"}, Pattern: 1},
			Border: []excelize.Border{
				{Type: "top", Color: "D1D5DB", Style: 1},
				{Type: "bottom", Color: "D1D5DB", Style: 1},
			},
		},
		&excelize.Style{
			Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
		},
	}
	ids := make([]int, 0, len(definitions))
	for _, definition := range definitions {
		id, err := f.NewStyle(definition)
		if err != nil {
			return xlsxStyles{}, err
		}
		ids = append(ids, id)
	}
	return xlsxStyles{title: ids[0], header: ids[1], group: ids[2], body: ids[3]}, nil
}

func writeXLSXHeading(f *excelize.File, sheet string, doc listrecords.Document, styles xlsxStyles) error {
	if err := f.SetCellValue(sheet, "A1", doc.Title); err != nil {
		return err
	}
	if err := f.SetCellStyle(sheet, "A1", "A1", styles.title); err != nil {
		return err
	}
	if err := f.SetCellValue(sheet, "A2", doc.Subtitle); err != nil {
		return err
	}
	if err := f.SetCellValue(sheet, "A3", "Erstellt: "+listrecords.GeneratedAtLabel(doc.GeneratedAt)); err != nil {
		return err
	}

	for idx, filter := range doc.Filters {
		cell, _ := excelize.CoordinatesToCellName(idx+1, 4)
		if err := f.SetCellValue(sheet, cell, filter); err != nil {
			return err
		}
	}

	headerRow := 6
	for idx, column := range doc.Columns {
		cell, _ := excelize.CoordinatesToCellName(idx+1, headerRow)
		if err := f.SetCellValue(sheet, cell, column.Label); err != nil {
			return err
		}
	}
	lastHeaderCell, _ := excelize.CoordinatesToCellName(len(doc.Columns), headerRow)
	if err := f.SetCellStyle(sheet, "A6", lastHeaderCell, styles.header); err != nil {
		return err
	}

	return nil
}

func writeXLSXRows(f *excelize.File, sheet string, doc listrecords.Document, styles xlsxStyles) error {
	excelRow, currentGroup, hasPrintedGroup := 7, "", false
	for _, row := range doc.Rows {
		hasValues := len(row.Values) > 0
		if row.GroupTitle != "" && (!hasValues || row.GroupTitle != currentGroup) {
			currentGroup = row.GroupTitle
			if err := writeXLSXGroup(f, sheet, row.GroupTitle, excelRow, len(doc.Columns), hasPrintedGroup, styles.group); err != nil {
				return err
			}
			hasPrintedGroup = true
			excelRow++
		}
		if !hasValues {
			continue
		}
		if err := writeXLSXBodyRow(f, sheet, excelRow, row, doc.Columns, styles.body); err != nil {
			return err
		}
		// Excel calculates soft and explicit wrapping only for auto-sized rows.
		excelRow++
	}
	return nil
}

func writeXLSXGroup(f *excelize.File, sheet, title string, row, columns int, pageBreak bool, style int) error {
	startCell, _ := excelize.CoordinatesToCellName(1, row)
	if pageBreak {
		if err := f.InsertPageBreak(sheet, startCell); err != nil {
			return err
		}
	}
	lastColumn := columns
	if lastColumn < 1 {
		lastColumn = 1
	}
	endCell, _ := excelize.CoordinatesToCellName(lastColumn, row)
	if err := f.SetCellValue(sheet, startCell, title); err != nil {
		return err
	}
	if err := f.MergeCell(sheet, startCell, endCell); err != nil {
		return err
	}
	return f.SetCellStyle(sheet, startCell, endCell, style)
}

func writeXLSXBodyRow(f *excelize.File, sheet string, excelRow int, row listrecords.Row, columns []listrecords.Column, style int) error {
	for index, column := range columns {
		cell, _ := excelize.CoordinatesToCellName(index+1, excelRow)
		// XLSX has no per-line emphasis; preserve the wrapped line structure.
		value := listrecords.StripStyleMarkers(row.Values[column.ID])
		if err := f.SetCellValue(sheet, cell, value); err != nil {
			return err
		}
	}
	if len(columns) > 0 {
		firstCell, _ := excelize.CoordinatesToCellName(1, excelRow)
		lastCell, _ := excelize.CoordinatesToCellName(len(columns), excelRow)
		if err := f.SetCellStyle(sheet, firstCell, lastCell, style); err != nil {
			return err
		}
	}
	return nil
}

func setXLSXWidths(f *excelize.File, sheet string, doc listrecords.Document) error {
	for idx := range doc.Columns {
		col, _ := excelize.ColumnNumberToName(idx + 1)
		width := 18.0
		switch doc.Columns[idx].ID {
		case listrecords.ColumnName:
			width = 26
		case listrecords.ColumnDeparture, listrecords.ColumnGuardianContacts:
			width = 32
		case listrecords.ColumnPlanRowLabel:
			width = 24
		case listrecords.ColumnPlanMonday, listrecords.ColumnPlanTuesday, listrecords.ColumnPlanWednesday, listrecords.ColumnPlanThursday,
			listrecords.ColumnPlanFriday, listrecords.ColumnPlanSaturday, listrecords.ColumnPlanSunday:
			width = 30
		}
		if err := f.SetColWidth(sheet, col, col, width); err != nil {
			return err
		}
	}

	return nil
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
