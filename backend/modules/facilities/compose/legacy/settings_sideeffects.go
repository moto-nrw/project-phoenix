package legacy

import (
	"context"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

// SettingHandler applies one setting's write side effect; the root registers it.
type SettingHandler func(ctx context.Context, tenantID int64, value any) (postCommit func(), err error)
type provisionedActivity = facilitiesModule.SystemActivity

type settingsRegistry interface{ Register(string, SettingHandler) }

type schulhofProvisioner interface {
	EnsureInfrastructure(context.Context, int64) (*provisionedActivity, error)
}

type wcProvisioner interface {
	EnsureInfrastructure(context.Context) (*provisionedActivity, error)
}

// RegisterSettingsSideEffects provisions Schulhof and WC once the root's switch keys turn on.
func RegisterSettingsSideEffects(registry settingsRegistry, schulhofKey, wcKey string, schulhof schulhofProvisioner, wc wcProvisioner) {
	schulhofHandler, wcHandler := SettingHandlers(schulhof, wc)
	registry.Register(schulhofKey, schulhofHandler)
	registry.Register(wcKey, wcHandler)
}

// SettingHandlers are the side effects of the Schulhof and WC switch keys.
func SettingHandlers(schulhof schulhofProvisioner, wc wcProvisioner) (schulhofHandler, wcHandler SettingHandler) {
	return onEnable(func(ctx context.Context) error { _, err := schulhof.EnsureInfrastructure(ctx, 0); return err }),
		onEnable(func(ctx context.Context) error { _, err := wc.EnsureInfrastructure(ctx); return err })
}

func onEnable(ensure func(context.Context) error) SettingHandler {
	return func(ctx context.Context, _ int64, value any) (func(), error) {
		if enabled, ok := value.(bool); ok && enabled {
			return nil, ensure(ctx)
		}
		return nil, nil
	}
}
