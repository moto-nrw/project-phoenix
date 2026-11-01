package compose

import (
	"errors"
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
	publisher, err := postgres.NewRealtimePublisher(db, logger)
	if err != nil {
		return nil, nil, err
	}
	hub, err := realtime.NewRelayedHub(logger, publisher.Publish, realtime.Observers{
		Connection: observability.RecordSSEConnection,
		Broadcast:  observability.RecordSSEBroadcast,
	})
	if err != nil {
		return nil, nil, errors.Join(err, publisher.Close())
	}
	subscription, err := postgres.NewRealtimeSubscription(db, logger, hub.ReceiveSignal, listen)
	if err != nil {
		return nil, nil, errors.Join(err, publisher.Close())
	}
	return hub, realtimeResources{publisher, subscription}, nil
}

// Stop the publisher before the listener, then the root closes the pool.
type realtimeResources struct{ publisher, subscription io.Closer }

func (r realtimeResources) Close() error {
	return errors.Join(r.publisher.Close(), r.subscription.Close())
}
