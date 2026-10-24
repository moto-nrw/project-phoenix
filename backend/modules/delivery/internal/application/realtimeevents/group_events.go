package realtimeevents

import (
	"context"
	"log/slog"
)

// GroupEvents produces group-access and live-supervision invalidations.
type GroupEvents struct {
	Broadcaster Publisher
	Logger      *slog.Logger
}

func (p GroupEvents) QueueGroupAccessChanged(ctx context.Context, source string) {
	QueueGroupAccessChanged(ctx, p.Broadcaster, p.Logger, source)
}
func (p GroupEvents) QueueActiveSupervisionChanged(ctx context.Context, id int64, reason string) {
	QueueActiveSupervisionChanged(ctx, p.Broadcaster, p.Logger, id, reason)
}
