package postgres

import (
	"context"
	"log/slog"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// EmailEnqueuedChannel is notified, without payload, by the transaction that
// enqueues a new e-mail. Postgres delivers it only once that transaction
// commits, so a wake-up never points at a mail that was rolled back.
const EmailEnqueuedChannel = "delivery_email_enqueued"

// EmailWakeups binds ListenEmailEnqueued to db.
func EmailWakeups(db *bun.DB) func(context.Context) <-chan struct{} {
	return func(ctx context.Context) <-chan struct{} { return ListenEmailEnqueued(ctx, db, nil) }
}

// ListenEmailEnqueued reports committed e-mail enqueues until ctx ends; the
// returned channel then closes. A wake-up is a hint, not a delivery: one that
// is lost, for example while the listener reconnects, only leaves the mail to
// the worker's next poll. Several enqueues before the receiver reads coalesce
// into one wake-up.
func ListenEmailEnqueued(ctx context.Context, db *bun.DB, logger *slog.Logger) <-chan struct{} {
	if logger == nil {
		logger = slog.Default()
	}
	if _, ok := db.Driver().(pgdriver.Driver); !ok {
		// NewListener panics without pgdriver; the worker then only polls.
		logger.Warn("e-mail outbox listener needs pgdriver, polling only")
		return nil
	}
	wakeups := make(chan struct{}, 1)
	listener := pgdriver.NewListener(db)
	// Listen records the channel before it connects, so a listener that
	// cannot connect now subscribes once its reconnect succeeds.
	if err := listener.Listen(ctx, EmailEnqueuedChannel); err != nil {
		logger.Warn("e-mail outbox listener not connected, polling until it reconnects",
			slog.String("error", err.Error()),
		)
	}
	notifications := listener.Channel()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	go func() {
		defer close(wakeups)
		for range notifications {
			select {
			case wakeups <- struct{}{}:
			default:
			}
		}
	}()
	return wakeups
}
