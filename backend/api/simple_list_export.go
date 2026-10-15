package api

import (
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
)

type SimpleListColumn struct {
	ID    string
	Label string
}

type SimpleListDocument struct {
	Title       string
	Subtitle    string
	GeneratedAt time.Time
	Filters     []string
	Columns     []SimpleListColumn
	Rows        [][]string
	Footer      string
}

type SimpleListFile struct {
	Data        []byte
	ContentType string
	Filename    string
}

type SimpleListRenderer func(SimpleListDocument, string, string) (SimpleListFile, error)

func NewSimpleListRenderer() SimpleListRenderer {
	renderer := lists.NewRendererService()
	return func(input SimpleListDocument, format, filename string) (SimpleListFile, error) {
		columns := make([]lists.Column, 0, len(input.Columns))
		for _, column := range input.Columns {
			columns = append(columns, lists.Column{ID: lists.ColumnID(column.ID), Label: column.Label})
		}
		rows := make([]lists.Row, 0, len(input.Rows))
		for rowIndex, inputRow := range input.Rows {
			if len(inputRow) != len(columns) {
				return SimpleListFile{}, fmt.Errorf("simple list export: row %d has %d values for %d columns", rowIndex, len(inputRow), len(columns))
			}
			values := make(map[lists.ColumnID]string, len(columns))
			for columnIndex, column := range columns {
				values[column.ID] = lists.SanitizeUserText(inputRow[columnIndex])
			}
			rows = append(rows, lists.Row{Values: values})
		}
		file, err := renderer.Render(lists.Document{
			Title: input.Title, Subtitle: input.Subtitle, GeneratedAt: input.GeneratedAt,
			Filters: input.Filters, Columns: columns, Rows: rows, Footer: input.Footer,
		}, lists.Format(format), filename)
		if err != nil {
			return SimpleListFile{}, err
		}
		return SimpleListFile{Data: file.Data, ContentType: file.ContentType, Filename: file.Filename}, nil
	}
}
