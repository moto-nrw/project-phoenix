package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
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

// RenderTimeTrackingWorkbook binds the time-tracking exports to the shared
// workbook writer.
func RenderTimeTrackingWorkbook(sheet string, headers []string, rows [][]any, decimalColumns []int) ([]byte, error) {
	return lists.RenderWorkbook(sheet, headers, rows, decimalColumns)
}

// PayrollExportSettings maps resolved settings into the exporter's input.
type PayrollExportSettings struct {
	Source settingsCompose.PayrollStatusGetter
}

func (s PayrollExportSettings) GetPayrollConfiguration(ctx context.Context) (*workforceCompose.PayrollExportConfiguration, error) {
	status, err := s.Source.GetPayrollStatus(ctx)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, errors.New("payroll settings returned no configuration")
	}
	result := &workforceCompose.PayrollExportConfiguration{
		Beraternummer:        status.Beraternummer,
		Mandantennummer:      status.Mandantennummer,
		LodasHeaderComplete:  status.LodasHeaderComplete,
		ConfiguredCategories: status.ConfiguredCategories,
		Categories:           make([]workforceCompose.PayrollExportCategory, len(status.Categories)),
	}
	for i, category := range status.Categories {
		result.Categories[i] = workforceCompose.PayrollExportCategory{
			ID: category.ID, Label: category.Label, Number: category.Number,
			Unit: category.Unit, UnitRequired: category.UnitRequired,
		}
	}
	return result, nil
}
