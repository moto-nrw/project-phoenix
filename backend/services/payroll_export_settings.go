package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/services/config"
)

// PayrollExportSettings maps resolved settings into the exporter's input.
type PayrollExportSettings struct {
	Source config.PayrollStatusGetter
}

func (s PayrollExportSettings) GetPayrollConfiguration(ctx context.Context) (*timetracking.PayrollExportConfiguration, error) {
	status, err := s.Source.GetPayrollStatus(ctx)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, errors.New("payroll settings returned no configuration")
	}
	result := &timetracking.PayrollExportConfiguration{
		Beraternummer:        status.Beraternummer,
		Mandantennummer:      status.Mandantennummer,
		LodasHeaderComplete:  status.LodasHeaderComplete,
		ConfiguredCategories: status.ConfiguredCategories,
		Categories:           make([]timetracking.PayrollExportCategory, len(status.Categories)),
	}
	for i, category := range status.Categories {
		result.Categories[i] = timetracking.PayrollExportCategory{
			ID: category.ID, Label: category.Label, Number: category.Number,
			Unit: category.Unit, UnitRequired: category.UnitRequired,
		}
	}
	return result, nil
}
