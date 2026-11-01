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

const (
	realtimeChannel      = "moto_realtime_v1"
	realtimeReadyChannel = "moto_realtime_ready_v1"
)

// RealtimePublisher owns a bounded, lossy queue. After-commit hooks never
// acquire a pool connection; overflow is logged and polling reconciles loss.
type RealtimePublisher struct {
	db     *bun.DB
	logger *slog.Logger
	queue  chan string
	done   chan struct{}
	mu     sync.RWMutex
	closed bool
	ctx    context.Context
	cancel context.CancelFunc
}

func NewRealtimePublisher(db *bun.DB, logger *slog.Logger) (*RealtimePublisher, error) {
	if db == nil || logger == nil {
		return nil, fmt.Errorf("realtime database and logger are required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &RealtimePublisher{db: db, logger: logger, queue: make(chan string, 128), done: make(chan struct{}), ctx: ctx, cancel: cancel}
	go p.run()
	return p, nil
}

func (p *RealtimePublisher) Publish(payload string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return errors.New("realtime publisher closed")
	}
	select {
	case p.queue <- payload:
		return nil
	default:
		p.logger.Warn("realtime publish queue full; signal dropped")
		return errors.New("realtime publish queue full")
	}
}

func (p *RealtimePublisher) run() {
	defer close(p.done)
	for {
		select {
		case <-p.ctx.Done():
			return
		case payload := <-p.queue:
			if err := p.notify(payload); err != nil {
				p.logger.Warn("realtime publish failed")
			}
		}
	}
}

func (p *RealtimePublisher) notify(payload string) error {
	ctx, cancel := context.WithTimeout(p.ctx, 2*time.Second)
	defer cancel()
	return notifyRealtime(ctx, p.db, realtimeChannel, payload)
}

func notifyRealtime(ctx context.Context, db *bun.DB, channel, payload string) (err error) {
	// pgdriver's ordinary ExecContext interpolates parameters. Prepare on the
	// underlying sql.DB to use Parse/Bind instead, never SQL payload literals.
	stmt, err := db.DB.PrepareContext(ctx, "SELECT pg_notify($1, $2)")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stmt.Close()) }()
	_, err = stmt.ExecContext(ctx, channel, payload)
	return err
}

func (p *RealtimePublisher) Close() error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		p.cancel()
	}
	p.mu.Unlock()
	<-p.done
	return nil
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
	err := listener.Listen(ctx, realtimeChannel, realtimeReadyChannel)
	if err == nil {
		err = awaitRealtimeReady(ctx, db, listener)
	}
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("realtime listener startup failed: %w", err)
	}
	done := make(chan struct{})
	notifications := listener.Channel(pgdriver.WithChannelSize(128))
	go func() {
		defer close(done)
		for notification := range notifications {
			if notification.Channel != realtimeChannel {
				continue
			}
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
