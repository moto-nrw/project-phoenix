package services

import (
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// RenderTimeTrackingPDF binds the time-tracking document to the shared PDF design.
func RenderTimeTrackingPDF(input workforceCompose.TimeTrackingDocument) ([]byte, error) {
	doc := lists.Document{Title: input.Title, Subtitle: input.Subtitle, Footer: input.Footer, GeneratedAt: input.GeneratedAt, Filters: input.Filters}
	for _, column := range input.Columns {
		doc.Columns = append(doc.Columns, lists.Column{ID: lists.ColumnID(column.ID), Label: column.Label})
	}
	doc.Rows = make([]lists.Row, 0, len(input.Rows))
	for _, row := range input.Rows {
		values := make(map[lists.ColumnID]string, len(row.Values))
		for id, value := range row.Values {
			values[lists.ColumnID(id)] = value
		}
		doc.Rows = append(doc.Rows, lists.Row{Values: values})
	}
	file, err := lists.NewRendererService().Render(doc, lists.FormatPDF, "zeiterfassung")
	if err != nil {
		return nil, err
	}
	return file.Data, nil
}

func RenderTimeTrackingWorkbook(sheet string, headers []string, rows [][]any, decimalColumns []int) ([]byte, error) {
	return lists.RenderWorkbook(sheet, headers, rows, decimalColumns)
}
