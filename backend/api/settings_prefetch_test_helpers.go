//go:build testhooks

package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/settings"
)

func SettingsPrefetchForTest() func(context.Context, any, ...string) context.Context {
	return settings.PrefetchSettings
}
