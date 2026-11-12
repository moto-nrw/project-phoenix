package api

import (
	owners "github.com/moto-ogs/project-phoenix/roots/owners"

	"github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

var _ parentportal.BatchSettingsService = owners.ParentSettingsBatch{}
