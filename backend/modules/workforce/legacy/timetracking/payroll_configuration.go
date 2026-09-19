package timetracking

import "context"

// PayrollExportCategory contains only the values needed to render a wage line.
type PayrollExportCategory struct {
	ID, Label, Number, Unit string
	UnitRequired            bool
}

// PayrollExportConfiguration is the tenant's resolved DATEV configuration.
// Settings management metadata and personnel counts are not export inputs.
type PayrollExportConfiguration struct {
	Categories                     []PayrollExportCategory
	Beraternummer, Mandantennummer string
	LodasHeaderComplete            bool
	ConfiguredCategories           int
}

type PayrollConfigurationReader interface {
	GetPayrollConfiguration(context.Context) (*PayrollExportConfiguration, error)
}
