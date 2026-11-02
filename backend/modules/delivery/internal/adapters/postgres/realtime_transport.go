package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	realtimeChannel      = "moto_realtime_v1"
	realtimeReadyChannel = "moto_realtime_ready_v1"
	// realtimeDropLogInterval throttles the overflow warning: a burst that
	// fills the queue logs once with the running count, not once per signal.
	realtimeDropLogInterval = 30 * time.Second
)

// RealtimePublisher owns a bounded, lossy queue. After-commit hooks never
// acquire a pool connection; overflow is counted, logged at most once per
// interval, and polling reconciles the loss.
type RealtimePublisher struct {
	db     *bun.DB
	logger *slog.Logger
	queue  chan string
	done   chan struct{}
	mu     sync.RWMutex
	closed bool
	ctx    context.Context
	cancel context.CancelFunc
	// dropped counts signals the full queue discarded; lastDropLog is the
	// UnixNano of the last overflow warning.
	dropped     atomic.Int64
	lastDropLog atomic.Int64
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
	default:
		// Local delivery already happened and the signal is lossy by
		// contract, so the caller gets no error to log once per signal.
		p.recordDrop()
	}
	return nil
}

func (p *RealtimePublisher) recordDrop() {
	total := p.dropped.Add(1)
	now := time.Now().UnixNano()
	last := p.lastDropLog.Load()
	if last != 0 && now-last < int64(realtimeDropLogInterval) {
		return
	}
	if p.lastDropLog.CompareAndSwap(last, now) {
		p.logger.Warn("realtime publish queue full; signals dropped",
			"dropped_total", total,
		)
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
				// The cause comes from the driver, never from the payload.
				p.logger.Warn("realtime publish failed",
					"error", err,
				)
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

// RealtimeSubscription owns the process's publisher and, in an API, its
// dedicated listener and receive loop. Close stops the publisher before the
// listener; the root closes the pool afterwards.
type RealtimeSubscription struct {
	publisher io.Closer
	listener  *pgdriver.Listener
	done      chan struct{}
	once      sync.Once
	closeErr  error
}

// NewRealtimeSubscription binds the receiver at construction. pgdriver reconnects
// and re-subscribes; clients' poll reconcile recovers signals lost while offline.
func NewRealtimeSubscription(db *bun.DB, logger *slog.Logger, receive func(string) error, listen bool, publisher io.Closer) (*RealtimeSubscription, error) {
	if !listen {
		return &RealtimeSubscription{publisher: publisher}, nil
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
				// Receive errors name the rejected field or the lookup that
				// failed, never the payload.
				logger.Warn("discarding realtime signal",
					"error", err,
				)
			}
		}
	}()
	return &RealtimeSubscription{publisher: publisher, listener: listener, done: done}, nil
}

func (s *RealtimeSubscription) Close() error {
	s.once.Do(func() {
		if s.publisher != nil {
			s.closeErr = s.publisher.Close()
		}
		if s.listener != nil {
			s.closeErr = errors.Join(s.closeErr, s.listener.Close())
			<-s.done
		}
	})
	return s.closeErr
}
