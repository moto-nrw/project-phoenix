package services

import (
	"context"
	"errors"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

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
