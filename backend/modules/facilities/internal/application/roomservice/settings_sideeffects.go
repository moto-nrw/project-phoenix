package facilities

import (
	"context"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

// SettingHandlers are the side effects of the Schulhof and WC switch keys:
// they provision Schulhof and WC once the switch turns on.
func SettingHandlers(schulhof facilitiesModule.SchulhofProvisioner, wc facilitiesModule.WCProvisioner) (schulhofHandler, wcHandler facilitiesModule.SettingHandler) {
	return onEnable(func(ctx context.Context) error { _, err := schulhof.EnsureInfrastructure(ctx, 0); return err }),
		onEnable(func(ctx context.Context) error { _, err := wc.EnsureInfrastructure(ctx); return err })
}

func onEnable(ensure func(context.Context) error) facilitiesModule.SettingHandler {
	return func(ctx context.Context, _ int64, value any) (func(), error) {
		if enabled, ok := value.(bool); ok && enabled {
			return nil, ensure(ctx)
		}
		return nil, nil
	}
}
