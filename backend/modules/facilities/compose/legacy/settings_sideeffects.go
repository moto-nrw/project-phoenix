package legacy

import (
	"context"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

// SettingHandler applies one setting's write side effect; the root binds it.
type SettingHandler func(ctx context.Context, tenantID int64, value any) (postCommit func(), err error)
type provisionedActivity = facilitiesModule.SystemActivity

type schulhofProvisioner interface {
	EnsureInfrastructure(context.Context, int64) (*provisionedActivity, error)
}

type wcProvisioner interface {
	EnsureInfrastructure(context.Context) (*provisionedActivity, error)
}

// SettingHandlers are the side effects of the Schulhof and WC switch keys:
// they provision Schulhof and WC once the switch turns on.
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
