package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	app "github.com/moto-nrw/project-phoenix/modules/settings/internal/application"
)

// NewAnalyseFreigabe answers whether a school has the Analyse-Freigabe
// (analytics.freigabe, #3603), for the usage analytics' core-action
// middleware. That middleware runs router-wide, outside the school's tenant
// middleware, so it resolves for an explicit school. It fails closed: a
// settings error keeps the event anonymous and is logged.
func NewAnalyseFreigabe(reader settings.TenantReader, logger *slog.Logger) func(ctx context.Context, schoolID int64) bool {
	return app.NewAnalyseFreigabeReader(reader, logger, settings.KeyAnalyticsFreigabe)
}
