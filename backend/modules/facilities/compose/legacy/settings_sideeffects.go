package legacy

import (
	"context"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

// SettingHandler applies one setting's write side effect; the root registers it.
type SettingHandler func(ctx context.Context, tenantID int64, value any) (postCommit func(), err error)
type provisionedActivity = facilitiesModule.SystemActivity

type settingsRegistry interface {
	Register(string, SettingHandler)
}

type schulhofProvisioner interface {
	EnsureInfrastructure(context.Context, int64) (*provisionedActivity, error)
}

type wcProvisioner interface {
	EnsureInfrastructure(context.Context) (*provisionedActivity, error)
}

// RegisterSettingsSideEffects provisions Schulhof and WC once the root's switch keys turn on.
func RegisterSettingsSideEffects(registry settingsRegistry, schulhofKey, wcKey string, schulhof schulhofProvisioner, wc wcProvisioner) {
	registry.Register(schulhofKey, func(ctx context.Context, _ int64, value any) (func(), error) {
		enabled, ok := value.(bool)
		if !ok || !enabled {
			return nil, nil
		}
		_, err := schulhof.EnsureInfrastructure(ctx, 0)
		return nil, err
	})
	registry.Register(wcKey, func(ctx context.Context, _ int64, value any) (func(), error) {
		enabled, ok := value.(bool)
		if !ok || !enabled {
			return nil, nil
		}
		_, err := wc.EnsureInfrastructure(ctx)
		return nil, err
	})
}
