package fileformat

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/xuri/excelize/v2"
)

func (Decoder) Template(kind dataimport.TemplateKind, format dataimport.FileFormat) ([]byte, error) {
	var headers []string
	var examples [][]any
	var sheetName string
	var writeNotes func(*excelize.File)
	width := 20.0
	switch kind {
	case dataimport.StudentTemplate:
		headers, examples = getStudentImportHeaders(), getStudentImportExamples()
		sheetName, writeNotes, width = "Kinder", writeHinweiseSheet, 15
	case dataimport.StaffTemplate:
		headers, examples = getStaffImportHeaders(), getStaffImportExamples()
		sheetName, writeNotes = "Mitarbeiter", writeStaffHinweiseSheet
	case dataimport.ClassListTemplate:
		headers = getClassListImportHeaders()
		sheetName, writeNotes = "Klassenliste", writeClassListHinweiseSheet
	case dataimport.OpeningBalanceTemplate:
		headers, examples = getOpeningBalanceHeaders(), getOpeningBalanceExamples()
		sheetName, writeNotes = "Eröffnungssalden", writeOpeningBalanceHinweiseSheet
	default:
		return nil, fmt.Errorf("unsupported import template %q", kind)
	}
	var output bytes.Buffer
	switch format {
	case dataimport.CSV:
		writer := csv.NewWriter(&output)
		if err := writer.Write(headers); err != nil {
			return nil, err
		}
		for _, row := range examples {
			values := make([]string, len(row))
			for i, value := range row {
				values[i] = fmt.Sprint(value)
			}
			if err := writer.Write(values); err != nil {
				return nil, err
			}
		}
		writer.Flush()
		return output.Bytes(), writer.Error()
	case dataimport.XLSX:
		file := excelize.NewFile()
		defer func() {
			if err := file.Close(); err != nil {
				slog.Default().Error("Error closing Excel file", slog.String("error", err.Error()))
			}
		}()
		if err := setupExcelSheet(file, sheetName); err != nil {
			return nil, err
		}
		writeExcelHeaders(file, sheetName, headers)
		writeExcelExampleRows(file, sheetName, examples)
		setExcelColumnWidths(file, sheetName, len(headers), width)
		writeNotes(file)
		if err := file.Write(&output); err != nil {
			return nil, err
		}
		return output.Bytes(), nil
	default:
		return nil, fmt.Errorf("unsupported import format %q", format)
	}
}
