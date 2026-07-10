package active

import (
	"context"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// openCareMode fails closed to fixed-group behavior. A repository failure is
// logged as an operational fault and is never interpreted as a tenant choice.
func (rs *Resource) openCareMode(ctx context.Context) bool {
	if rs.SettingsService == nil {
		return false
	}
	mode, err := rs.SettingsService.ResolveString(ctx, configModel.KeyGroupMode)
	if err != nil {
		rs.getLogger().ErrorContext(ctx, "failed to resolve operational group mode", slog.String("error", err.Error()))
		return false
	}
	return mode == configModel.GroupModeOpenCare
}
