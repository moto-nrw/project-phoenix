package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// DemoStateStore is only composed in the isolated demo CLI with its privileged
// database connection. HTTP roles have no access to the credentials table.
type DemoStateStore struct{ db *bun.DB }

type DemoState struct {
	TenantID int64  `bun:"tenant_id"`
	SeedJSON string `bun:"seed_state"`
}

func NewDemoStateStore(db *bun.DB) *DemoStateStore { return &DemoStateStore{db: db} }

func (s *DemoStateStore) Load(ctx context.Context, name string) (*DemoState, error) {
	var state DemoState
	err := s.db.NewRaw(`SELECT tenant_id, seed_state FROM platform.demo_school_states WHERE name = ?`, name).Scan(ctx, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load demo school state: %w", err)
	}
	return &state, nil
}

func (s *DemoStateStore) Remember(ctx context.Context, name string, state DemoState) error {
	_, err := s.db.NewRaw(`INSERT INTO platform.demo_school_states (name, tenant_id, seed_state) VALUES (?, ?, ?::jsonb)`, name, state.TenantID, state.SeedJSON).Exec(ctx)
	// Do not include a driver error here: constraint errors may contain the
	// rejected row, including the seed credentials.
	if err != nil {
		return errors.New("could not persist demo school state")
	}
	return nil
}

func (s *DemoStateStore) WithLease(ctx context.Context, name string, run func(context.Context) error) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("open demo lease connection: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var acquired bool
	if err := tx.NewRaw(`SELECT pg_try_advisory_xact_lock(hashtextextended(?, 3461))`, name).Scan(ctx, &acquired); err != nil {
		return false, fmt.Errorf("acquire demo lease: %w", err)
	}
	if !acquired {
		return false, nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := make(chan struct{})
	watched := make(chan error, 1)
	go watchDemoLease(runCtx, tx, cancel, finished, watched)
	runErr := run(runCtx)
	close(finished)
	return true, errors.Join(runErr, <-watched)
}

// If the lease connection dies, stop the owner even if its HTTP client and
// the rest of its database pool can reconnect successfully.
func watchDemoLease(ctx context.Context, tx bun.Tx, cancel context.CancelFunc, finished <-chan struct{}, result chan<- error) {
	timer := time.NewTicker(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-finished:
			result <- nil
			return
		case <-ctx.Done():
			result <- ctx.Err()
			return
		case <-timer.C:
			probeCtx, stop := context.WithTimeout(ctx, 5*time.Second)
			var alive int
			err := tx.NewRaw(`SELECT 1`).Scan(probeCtx, &alive)
			stop()
			if err != nil {
				cancel()
				result <- fmt.Errorf("demo lease connection lost: %w", err)
				return
			}
		}
	}
}
