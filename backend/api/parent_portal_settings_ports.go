package api

import (
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

var _ parentportal.BatchSettingsService = owners.ParentSettingsBatch{}
