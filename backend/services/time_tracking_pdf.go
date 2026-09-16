package services

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// RenderTimeTrackingPDF binds the time-tracking document to the shared PDF design.
func RenderTimeTrackingPDF(input timetracking.TimeTrackingDocument) ([]byte, error) {
	doc := listexport.Document{Title: input.Title, Subtitle: input.Subtitle, Footer: input.Footer, GeneratedAt: input.GeneratedAt, Filters: input.Filters}
	for _, column := range input.Columns {
		doc.Columns = append(doc.Columns, listexport.Column{ID: listexport.ColumnID(column.ID), Label: column.Label})
	}
	doc.Rows = make([]listexport.Row, 0, len(input.Rows))
	for _, row := range input.Rows {
		values := make(map[listexport.ColumnID]string, len(row.Values))
		for id, value := range row.Values {
			values[listexport.ColumnID(id)] = value
		}
		doc.Rows = append(doc.Rows, listexport.Row{Values: values})
	}
	file, err := listexport.NewService().Render(doc, listexport.FormatPDF, "zeiterfassung")
	if err != nil {
		return nil, err
	}
	return file.Data, nil
}

func RenderTimeTrackingWorkbook(sheet string, headers []string, rows [][]any, decimalColumns []int) ([]byte, error) {
	return listexport.RenderWorkbook(sheet, headers, rows, decimalColumns)
}
