package compose

import (
	"errors"
	"io"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewPostgresRealtimeHub builds Delivery's process-scoped transport. API roots
// listen and publish; standalone Workers publish without a listener connection.
// Relayed notifications load in the school transaction of runtime (#5504).
func NewPostgresRealtimeHub(db *bun.DB, runtime tenant.UnitOfWork, logger *slog.Logger, listen bool) (*realtime.Hub, io.Closer, error) {
	content, err := notifications.NewRelayedContent(runtime, db, postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase))
	if err != nil {
		return nil, nil, err
	}
	publisher, err := postgres.NewRealtimePublisher(db, logger)
	if err != nil {
		return nil, nil, err
	}
	hub, err := realtime.NewRelayedHub(logger, publisher.Publish, content, realtime.Observers{
		Connection: observability.RecordSSEConnection,
		Broadcast:  observability.RecordSSEBroadcast,
	})
	if err != nil {
		return nil, nil, errors.Join(err, publisher.Close())
	}
	subscription, err := postgres.NewRealtimeSubscription(db, logger, hub.ReceiveSignal, listen, publisher)
	if err != nil {
		return nil, nil, errors.Join(err, publisher.Close())
	}
	return hub, subscription, nil
}
