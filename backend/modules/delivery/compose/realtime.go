package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
)

func NewRealtimeHub(logger *slog.Logger) *realtime.Hub {
	return realtime.NewHub(logger, realtime.Observers{
		Connection: observability.RecordSSEConnection,
		Broadcast:  observability.RecordSSEBroadcast,
	})
}
