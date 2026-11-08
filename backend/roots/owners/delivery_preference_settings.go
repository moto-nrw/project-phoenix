package owners

import (
	"context"

	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
)

// DeliveryPreferenceSettings binds the Settings Platform to Delivery's
// preference settings port and adds the catalogue's gate snapshot (#5699):
// one batch read for the dispatch switch and all gates instead of one read
// per gate. A context with an explicit snapshot keeps it, because its keys
// own their values even when the batch needs keys absent from it.
func DeliveryPreferenceSettings(settings settingsCompose.BatchSettingsService) deliveryPreferenceSettings {
	return deliveryPreferenceSettings{BatchSettingsService: settings}
}

type deliveryPreferenceSettings struct {
	settingsCompose.BatchSettingsService
}

func (s deliveryPreferenceSettings) WithGateSnapshot(ctx context.Context, keys []string) (context.Context, error) {
	if settingsValues.HasSettingsSnapshot(ctx) {
		return ctx, nil
	}
	return settingsValues.PrefetchSettingsOrError(ctx, s.BatchSettingsService, keys...)
}
