package legacy

import (
	"context"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
	"github.com/moto-nrw/project-phoenix/services/facilities"
)

type settingHandler = sideeffects.KeyHandler
type provisionedActivity = facilities.SystemActivity

type settingsRegistry interface {
	Register(string, settingHandler)
}

type schulhofProvisioner interface {
	EnsureInfrastructure(context.Context, int64) (*provisionedActivity, error)
}

type wcProvisioner interface {
	EnsureInfrastructure(context.Context) (*provisionedActivity, error)
}

func RegisterSettingsSideEffects(registry settingsRegistry, schulhof schulhofProvisioner, wc wcProvisioner) {
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
