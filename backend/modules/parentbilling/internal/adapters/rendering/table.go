package rendering

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/application"
)

// RenderTable renders the overview of a run as a list document whose values
// are free of the renderer's style markers (#4672, ADR 0122).
func (r *Renderer) RenderTable(table application.ExportTable, format, filenameBase string) (parentbilling.ExportFile, error) {
	columns := make([]lists.Column, len(table.Columns))
	for i, column := range table.Columns {
		columns[i] = lists.Column{ID: lists.ColumnID(column.ID), Label: column.Label}
	}
	rows := make([]lists.Row, len(table.Rows))
	for i, row := range table.Rows {
		values := make(map[lists.ColumnID]string, len(row))
		for id, value := range row {
			values[lists.ColumnID(id)] = lists.SanitizeUserText(value)
		}
		rows[i] = lists.Row{Values: values}
	}
	lines := make([]string, len(table.Lines))
	for i, line := range table.Lines {
		lines[i] = lists.SanitizeUserText(line)
	}
	file, err := r.tables.Render(lists.Document{
		Title: table.Title, Subtitle: lists.SanitizeUserText(table.Subtitle), GeneratedAt: time.Now(),
		Filters: lines, Columns: columns, Rows: rows, Footer: table.Footer,
	}, lists.Format(format), filenameBase)
	if err != nil {
		return parentbilling.ExportFile{}, err
	}
	return parentbilling.ExportFile{Filename: file.Filename, ContentType: file.ContentType, Data: file.Data}, nil
}
