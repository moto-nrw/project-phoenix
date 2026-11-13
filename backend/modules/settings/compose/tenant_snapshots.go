package compose

import (
	config "github.com/moto-ogs/project-phoenix/modules/settings/internal/application"
	configModel "github.com/moto-ogs/project-phoenix/modules/settings/records"
)

func NewTenantSnapshots(service configModel.SettingsService) (configModel.TenantSnapshots, error) {
	return config.NewTenantSnapshots(service)
}
func NewPreloadKeyVerifier() func([]string) error { return config.VerifyPreloadKeys }
