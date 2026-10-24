package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/settings"
)

// DisplayFeatureEnabled binds the display routes' feature toggle to the
// current tenant's display.enabled setting (#1325 follow-up: it defaults off).
func DisplayFeatureEnabled(resolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		return resolver.ResolveBool(ctx, settings.KeyDisplayEnabled)
	}
}
