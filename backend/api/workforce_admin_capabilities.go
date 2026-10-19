package api

import (
	"context"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// timeTrackingAccountStartDate resolves the tenant's time-account start day
// for the time-tracking config endpoint; the setting key stays next to its
// registry definition. A nil settings service leaves the start date unset.
func timeTrackingAccountStartDate(settings settingsCompose.SettingsService) func(context.Context) (string, error) {
	if settings == nil {
		return nil
	}
	return func(ctx context.Context) (string, error) {
		return settings.ResolveString(ctx, configModels.KeyTimeTrackingAccountStartDate)
	}
}
