package legacy

import (
	"context"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
	"github.com/moto-nrw/project-phoenix/services/facilities"
)

func RegisterSettingsSideEffects(registry *sideeffects.Registry, schulhof facilities.SchulhofService, wc facilities.WCService) {
	registry.Register(configModels.KeyCheckoutSchulhofEnabled, func(ctx context.Context, _ int64, value any) (func(), error) {
		enabled, ok := value.(bool)
		if !ok || !enabled {
			return nil, nil
		}
		_, err := schulhof.EnsureInfrastructure(ctx, 0)
		return nil, err
	})
	registry.Register(configModels.KeyCheckoutWCEnabled, func(ctx context.Context, _ int64, value any) (func(), error) {
		enabled, ok := value.(bool)
		if !ok || !enabled {
			return nil, nil
		}
		_, err := wc.EnsureInfrastructure(ctx)
		return nil, err
	})
}
