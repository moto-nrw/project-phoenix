package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/settings"
	configSvc "github.com/moto-nrw/project-phoenix/modules/settings/internal/application"
)

// NewExportFields projects setting values for the return export (#3838); only the administrative export CLI binds it, never HTTP.
func NewExportFields() settings.ExportFields { return configSvc.SettingExportFields{} }
