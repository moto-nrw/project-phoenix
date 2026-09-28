package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/settings"
)

// NewAnalyseFreigabe answers whether a school has the Analyse-Freigabe
// (analytics.freigabe, #3603), for the usage analytics' core-action
// middleware. That middleware runs router-wide, outside the school's tenant
// middleware, so it resolves for an explicit school. It fails closed: a
// settings error keeps the event anonymous and is logged.
func NewAnalyseFreigabe(reader settings.TenantReader, logger *slog.Logger) func(ctx context.Context, schoolID int64) bool {
	return func(ctx context.Context, schoolID int64) bool {
		freigabe, err := reader.ResolveBoolForTenant(ctx, schoolID, settings.KeyAnalyticsFreigabe)
		if err != nil {
			logger.WarnContext(ctx, "analyse-freigabe unresolved, core action stays anonymous",
				slog.Int64("school_id", schoolID),
				slog.String("error", err.Error()),
			)
			return false
		}
		return freigabe
	}
}
