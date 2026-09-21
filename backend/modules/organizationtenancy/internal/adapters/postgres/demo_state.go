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
	err := s.db.NewRaw(`SELECT tenant_id, seed_state FROM platform.demo_school_states WHERE name = ? AND seed_state IS NOT NULL`, name).Scan(ctx, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load demo school state: %w", err)
	}
	return &state, nil
}

func (s *DemoStateStore) Remember(ctx context.Context, name string, state DemoState) error {
	// A queued order has its row already and stays preparing until its first
	// tick; the standing school has no order and is ready with its seed.
	// Seeded credentials are never overwritten.
	result, err := s.db.NewRaw(`INSERT INTO platform.demo_school_states AS state (name, tenant_id, seed_state, status) VALUES (?, ?, ?::jsonb, 'ready')
		ON CONFLICT (name) DO UPDATE SET tenant_id = EXCLUDED.tenant_id, seed_state = EXCLUDED.seed_state
		WHERE state.seed_state IS NULL`, name, state.TenantID, state.SeedJSON).Exec(ctx)
	// Do not include a driver error here: constraint errors may contain the
	// rejected row, including the seed credentials.
	if err != nil {
		return errors.New("could not persist demo school state")
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return errors.New("demo school state already exists")
	}
	return nil
}

func (s *DemoStateStore) WithLease(ctx context.Context, name string, run func(context.Context) error) (bool, error) {
	// The caller's cancellation stops the demo callback, but must not make
	// database/sql roll back the lease transaction asynchronously. This method
	// releases the advisory lock synchronously before it returns.
	tx, err := s.db.BeginTx(context.WithoutCancel(ctx), nil)
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

// DemoOrder is a queued demo school of the public demo (#3463).
type DemoOrder struct {
	Name       string `bun:"name"`
	SchoolName string `bun:"school_name"`
	PersonName string `bun:"person_name"`
	Attempts   int    `bun:"attempts"`
	Seeded     bool   `bun:"seeded"`
}

// ReleaseClaims returns orders a stopped process held to the queue. The demo
// lease admits one process, so every claim it finds at its start is stale.
func (s *DemoStateStore) ReleaseClaims(ctx context.Context) error {
	_, err := s.db.NewRaw(`UPDATE platform.demo_school_states SET claimed_at = NULL WHERE status = 'preparing' AND claimed_at IS NOT NULL`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("release demo school orders: %w", err)
	}
	return nil
}

// Claim takes the oldest waiting order, or none.
func (s *DemoStateStore) Claim(ctx context.Context) (*DemoOrder, error) {
	var order DemoOrder
	err := s.db.NewRaw(`UPDATE platform.demo_school_states AS state
		SET claimed_at = CURRENT_TIMESTAMP, attempts = state.attempts + 1
		WHERE state.name = (
			SELECT name FROM platform.demo_school_states
			WHERE status = 'preparing' AND claimed_at IS NULL
			ORDER BY created_at, name LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING state.name, state.school_name, state.person_name, state.attempts, state.seed_state IS NOT NULL AS seeded`).Scan(ctx, &order)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim demo school order: %w", err)
	}
	return &order, nil
}

// Finish opens the school for its demo access.
func (s *DemoStateStore) Finish(ctx context.Context, name string, visitorAccountID int64) error {
	_, err := s.db.NewRaw(`UPDATE platform.demo_school_states SET status = 'ready', claimed_at = NULL, visitor_account_id = NULLIF(?, 0) WHERE name = ?`,
		visitorAccountID, name).Exec(ctx)
	if err != nil {
		return fmt.Errorf("finish demo school order: %w", err)
	}
	return nil
}

// Fail returns the order to the queue, or closes it as failed once it used
// its attempts. A seed that stopped halfway cannot be resumed, so its state
// is dropped and the next attempt starts over.
func (s *DemoStateStore) Fail(ctx context.Context, name string, maxAttempts int) (bool, error) {
	var failed bool
	err := s.db.NewRaw(`UPDATE platform.demo_school_states
		SET status = CASE WHEN attempts >= ? THEN 'failed' ELSE 'preparing' END, claimed_at = NULL, tenant_id = NULL, seed_state = NULL
		WHERE name = ? AND status = 'preparing' RETURNING status = 'failed'`, maxAttempts, name).Scan(ctx, &failed)
	if err != nil {
		return false, fmt.Errorf("fail demo school order: %w", err)
	}
	return failed, nil
}

// ReadyNames lists the demo schools the simulation keeps alive.
func (s *DemoStateStore) ReadyNames(ctx context.Context) ([]string, error) {
	var names []string
	err := s.db.NewRaw(`SELECT name FROM platform.demo_school_states WHERE status = 'ready' ORDER BY created_at, name`).Scan(ctx, &names)
	if err != nil {
		return nil, fmt.Errorf("list demo schools: %w", err)
	}
	return names, nil
}

// ActiveNames lists the ready demo schools entered since the given instant:
// the only ones the simulation serves (#3464).
func (s *DemoStateStore) ActiveNames(ctx context.Context, since time.Time) ([]string, error) {
	var names []string
	err := s.db.NewRaw(`SELECT name FROM platform.demo_school_states WHERE status = 'ready' AND last_used_at >= ? ORDER BY created_at, name`, since).Scan(ctx, &names)
	if err != nil {
		return nil, fmt.Errorf("list demo schools in use: %w", err)
	}
	return names, nil
}

// DemoOrderStore is the serving backend's side of the queue. Its role may
// only add an order, read its progress and note its use, never the seed state.
type DemoOrderStore struct{ database Database }

func NewDemoOrderStore(database Database) *DemoOrderStore {
	return &DemoOrderStore{database: database}
}

// DemoProgress is what the serving backend may know about a demo school.
type DemoProgress struct {
	Status           string `bun:"status"`
	TenantID         int64  `bun:"tenant_id"`
	VisitorAccountID int64  `bun:"visitor_account_id"`
}

// Enqueue queues the order unless maxActive demo schools hold a place: queued
// ones and ready ones whose school is not deleted. A failed order holds none.
// The lock makes concurrent orders take turns until the caller's transaction
// ends, so they cannot pass the count together.
func (s *DemoOrderStore) Enqueue(ctx context.Context, name, schoolName, personName string, maxActive int) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:capacity', 0))`).Exec(ctx); err != nil {
		return fmt.Errorf("lock demo school capacity: %w", err)
	}
	var active int
	err = db.NewRaw(`SELECT COUNT(*) FROM platform.demo_school_states AS state
		LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
		WHERE state.status = 'preparing' OR (state.status = 'ready' AND school.deleted_at IS NULL)`).Scan(ctx, &active)
	if err != nil {
		return fmt.Errorf("count active demo schools: %w", err)
	}
	if active >= maxActive {
		return ErrDemoCapacityReached
	}
	_, err = db.NewRaw(`INSERT INTO platform.demo_school_states (name, school_name, person_name) VALUES (?, ?, ?)`,
		name, schoolName, personName).Exec(ctx)
	if err != nil {
		if isIntegrityViolation(err) {
			return ErrDemoOrderExists
		}
		return fmt.Errorf("queue demo school: %w", err)
	}
	return nil
}

func (s *DemoOrderStore) Progress(ctx context.Context, name string) (*DemoProgress, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var progress DemoProgress
	err = db.NewRaw(`SELECT status, COALESCE(tenant_id, 0) AS tenant_id, COALESCE(visitor_account_id, 0) AS visitor_account_id
		FROM platform.demo_school_states WHERE name = ?`, name).Scan(ctx, &progress)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read demo school progress: %w", err)
	}
	return &progress, nil
}

// MarkUsed notes that a visitor entered the school at usedAt.
func (s *DemoOrderStore) MarkUsed(ctx context.Context, name string, usedAt time.Time) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`UPDATE platform.demo_school_states SET last_used_at = ? WHERE name = ?`, usedAt, name).Exec(ctx); err != nil {
		return fmt.Errorf("note demo school use: %w", err)
	}
	return nil
}

// ErrDemoOrderExists reports a name that is already queued or seeded.
var ErrDemoOrderExists = errors.New("demo school order already exists")

// ErrDemoCapacityReached reports that maxActive demo schools hold a place.
var ErrDemoCapacityReached = errors.New("demo capacity reached")

// Return hands a claimed order back without counting the attempt: what
// stopped it was not the order's fault.
func (s *DemoStateStore) Return(ctx context.Context, name string) error {
	_, err := s.db.NewRaw(`UPDATE platform.demo_school_states SET claimed_at = NULL, attempts = GREATEST(attempts - 1, 0)
		WHERE name = ? AND status = 'preparing'`, name).Exec(ctx)
	if err != nil {
		return fmt.Errorf("return demo school order: %w", err)
	}
	return nil
}
