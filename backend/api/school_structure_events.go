package api

import (
	"context"
	"log/slog"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
)

// NewGroupEvents binds School Structure's event port to Delivery.
func NewGroupEvents(broadcaster deliveryCompose.RealtimePublisher, logger *slog.Logger) interface {
	QueueGroupAccessChanged(context.Context, string)
	QueueActiveSupervisionChanged(context.Context, int64, string)
} {
	return deliveryCompose.NewGroupEvents(broadcaster, logger)
}
