package compose

import (
	"io"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/uptrace/bun"
)

// NewPostgresRealtimeHub builds Delivery's process-scoped transport. API roots
// listen and publish; standalone Workers publish without a listener connection.
func NewPostgresRealtimeHub(db *bun.DB, logger *slog.Logger, listen bool) (*realtime.Hub, io.Closer, error) {
	publish, err := postgres.NewRealtimePublisher(db, logger)
	if err != nil {
		return nil, nil, err
	}
	hub, err := realtime.NewRelayedHub(logger, publish, realtime.Observers{
		Connection: observability.RecordSSEConnection,
		Broadcast:  observability.RecordSSEBroadcast,
	})
	if err != nil {
		return nil, nil, err
	}
	subscription, err := postgres.NewRealtimeSubscription(db, logger, hub.ReceiveSignal, listen)
	if err != nil {
		return nil, nil, err
	}
	return hub, subscription, nil
}
