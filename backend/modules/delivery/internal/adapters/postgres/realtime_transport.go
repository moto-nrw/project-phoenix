package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

const realtimeChannel = "moto_realtime_v1"

// NewRealtimePublisher uses the existing pool after the producer has committed.
// The adapter transports bytes; school and audience validation stay with the hub.
func NewRealtimePublisher(db *bun.DB, logger *slog.Logger) (func(string) error, error) {
	if db == nil || logger == nil {
		return nil, fmt.Errorf("realtime database and logger are required")
	}
	return func(payload string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := pgdriver.Notify(ctx, db, realtimeChannel, payload); err != nil {
			// Database errors may include statement payloads. Do not log them.
			logger.Warn("realtime publish failed")
			return errors.New("realtime publish failed")
		}
		return nil
	}, nil
}

// RealtimeSubscription owns only an API's dedicated listener and receive loop.
type RealtimeSubscription struct {
	listener *pgdriver.Listener
	done     chan struct{}
	once     sync.Once
	closeErr error
}

// NewRealtimeSubscription binds the receiver at construction. pgdriver reconnects
// and re-subscribes; clients' poll reconcile recovers signals lost while offline.
func NewRealtimeSubscription(db *bun.DB, logger *slog.Logger, receive func(string) error, listen bool) (*RealtimeSubscription, error) {
	if !listen {
		return &RealtimeSubscription{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	listener := pgdriver.NewListener(db)
	if err := listener.Listen(ctx, realtimeChannel); err != nil {
		_ = listener.Close()
		return nil, errors.New("realtime listener startup failed")
	}
	done := make(chan struct{})
	notifications := listener.Channel(pgdriver.WithChannelSize(128))
	go func() {
		defer close(done)
		for notification := range notifications {
			if err := receive(notification.Payload); err != nil {
				logger.Warn("discarding invalid realtime signal")
			}
		}
	}()
	return &RealtimeSubscription{listener: listener, done: done}, nil
}

func (s *RealtimeSubscription) Close() error {
	s.once.Do(func() {
		if s.listener != nil {
			s.closeErr = s.listener.Close()
			<-s.done
		}
	})
	return s.closeErr
}
