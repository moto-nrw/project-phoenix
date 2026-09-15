package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config"
)

// PayrollExportSettings maps resolved settings into the exporter's input.
type PayrollExportSettings struct {
	Source config.PayrollStatusGetter
}

func (s PayrollExportSettings) GetPayrollConfiguration(ctx context.Context) (*active.PayrollExportConfiguration, error) {
	status, err := s.Source.GetPayrollStatus(ctx)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, errors.New("payroll settings returned no configuration")
	}
	result := &active.PayrollExportConfiguration{
		Beraternummer:        status.Beraternummer,
		Mandantennummer:      status.Mandantennummer,
		LodasHeaderComplete:  status.LodasHeaderComplete,
		ConfiguredCategories: status.ConfiguredCategories,
		Categories:           make([]active.PayrollExportCategory, len(status.Categories)),
	}
	for i, category := range status.Categories {
		result.Categories[i] = active.PayrollExportCategory{
			ID: category.ID, Label: category.Label, Number: category.Number,
			Unit: category.Unit, UnitRequired: category.UnitRequired,
		}
	}
	return result, nil
}
