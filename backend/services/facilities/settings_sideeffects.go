package facilities

import (
	"context"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
)

// RegisterSettingsSideEffects wires checkout toggles to their infrastructure
// provisioning handlers.
func RegisterSettingsSideEffects(registry *sideeffects.Registry, schulhof SchulhofService, wc WCService) {
	registry.Register(configModel.KeyCheckoutSchulhofEnabled, func(ctx context.Context, _ int64, value any) (func(), error) {
		boolVal, ok := value.(bool)
		if !ok || !boolVal {
			return nil, nil
		}
		_, err := schulhof.EnsureInfrastructure(ctx, 0)
		return nil, err
	})

	registry.Register(configModel.KeyCheckoutWCEnabled, func(ctx context.Context, _ int64, value any) (func(), error) {
		boolVal, ok := value.(bool)
		if !ok || !boolVal {
			return nil, nil
		}
		_, err := wc.EnsureInfrastructure(ctx)
		return nil, err
	})
}
