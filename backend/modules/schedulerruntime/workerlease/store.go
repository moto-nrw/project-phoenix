// Package workerlease keeps the Worker's runtime leadership in
// platform.worker_leases (#2726). Every statement takes its time from the
// database clock, so no process clock decides who leads, and every new holder
// receives a larger fencing token than the one before.
package workerlease

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ErrNotHeld reports that a term no longer holds its lease: it expired, was
// released, or a newer holder took it over.
var ErrNotHeld = errors.New("worker lease is not held")

// notHeldSQLState is raised by platform.assert_worker_lease.
const notHeldSQLState = "PWL01"

// Term is one holder's tenure of a lease. Until is database time.
type Term struct {
	Name   string
	Holder string
	Token  int64
	Until  time.Time
}

// Store acquires, renews, releases and asserts Worker leases. Every
// statement is a compile-time constant so the architecture evaluator can
// read which tables it touches.
type Store struct{ runtime tenant.UnitOfWork }

// NewStore builds the store over the root's transaction runtime. Acquire,
// Renew and Release open an administrative transaction of their own; Assert
// joins the job transaction already open in its context.
func NewStore(runtime tenant.UnitOfWork) (*Store, error) {
	if runtime.IsZero() {
		return nil, errors.New("worker lease store: transaction runtime is required")
	}
	return &Store{runtime: runtime}, nil
}

func (s *Store) inAdminTransaction(ctx context.Context, fn func(context.Context, bun.IDB) error) error {
	ctx = tenant.WithUnitOfWork(tenant.ContextWithoutTransaction(ctx), s.runtime)
	return tenant.WithAdminTx(ctx, (*bun.DB)(nil), func(txCtx context.Context, tx bun.Tx) error {
		return fn(txCtx, tx)
	})
}

func currentTransaction(ctx context.Context) (bun.IDB, error) {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("a transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("unsupported transaction %T", transaction)
	}
	return tx, nil
}

// acquireLeaseSQL takes a free or expired lease, or renews the caller's own
// term under a new token. A competing insert waits for the row lock and then
// re-checks the condition against the winner's row, so two callers can never
// both receive a term.
const acquireLeaseSQL = `INSERT INTO platform.worker_leases AS lease
  (lease_name, holder_id, fencing_token, lease_until, updated_at)
VALUES (?, ?, 1, clock_timestamp() + make_interval(secs => ?), clock_timestamp())
ON CONFLICT (lease_name) DO UPDATE
SET holder_id = EXCLUDED.holder_id,
    fencing_token = lease.fencing_token + 1,
    lease_until = clock_timestamp() + make_interval(secs => ?),
    updated_at = clock_timestamp()
WHERE lease.lease_until <= clock_timestamp() OR lease.holder_id = EXCLUDED.holder_id
RETURNING fencing_token, lease_until`

const renewLeaseSQL = `UPDATE platform.worker_leases
SET lease_until = clock_timestamp() + make_interval(secs => ?), updated_at = clock_timestamp()
WHERE lease_name = ? AND holder_id = ? AND fencing_token = ? AND lease_until > clock_timestamp()
RETURNING lease_until`

// releaseLeaseSQL ends the term at once. The token stays, so the next holder
// still receives a larger one.
const releaseLeaseSQL = `UPDATE platform.worker_leases
SET lease_until = clock_timestamp(), updated_at = clock_timestamp()
WHERE lease_name = ? AND holder_id = ? AND fencing_token = ? AND lease_until > clock_timestamp()`

const assertLeaseSQL = `SELECT platform.assert_worker_lease(?, ?, ?, make_interval(secs => ?))`

// Acquire starts a new term for holder when the lease is free, expired, or
// already held by holder. It reports false while another holder's term runs.
func (s *Store) Acquire(ctx context.Context, name, holder string, ttl time.Duration) (Term, bool, error) {
	if name == "" || holder == "" || ttl <= 0 {
		return Term{}, false, errors.New("worker lease: name, holder and a positive TTL are required")
	}
	term := Term{Name: name, Holder: holder}
	err := s.inAdminTransaction(ctx, func(ctx context.Context, db bun.IDB) error {
		return db.NewRaw(acquireLeaseSQL, name, holder, ttl.Seconds(), ttl.Seconds()).Scan(ctx, &term.Token, &term.Until)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Term{}, false, nil
	}
	if err != nil {
		return Term{}, false, fmt.Errorf("acquire worker lease %s: %w", name, err)
	}
	return term, true, nil
}

// Renew extends a running term. It reports false when the term already
// ended; the holder must then stop and acquire a new one.
func (s *Store) Renew(ctx context.Context, term Term, ttl time.Duration) (Term, bool, error) {
	if ttl <= 0 {
		return Term{}, false, errors.New("worker lease: a positive TTL is required")
	}
	renewed := term
	err := s.inAdminTransaction(ctx, func(ctx context.Context, db bun.IDB) error {
		return db.NewRaw(renewLeaseSQL, ttl.Seconds(), term.Name, term.Holder, term.Token).Scan(ctx, &renewed.Until)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Term{}, false, nil
	}
	if err != nil {
		return Term{}, false, fmt.Errorf("renew worker lease %s: %w", term.Name, err)
	}
	return renewed, true, nil
}

// Release ends a running term so a standby can take over without waiting
// for its expiry. Releasing an ended term changes nothing.
func (s *Store) Release(ctx context.Context, term Term) error {
	err := s.inAdminTransaction(ctx, func(ctx context.Context, db bun.IDB) error {
		_, err := db.NewRaw(releaseLeaseSQL, term.Name, term.Holder, term.Token).Exec(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("release worker lease %s: %w", term.Name, err)
	}
	return nil
}

// Assert confirms inside the transaction open in ctx that term still holds
// its lease for at least margin. It returns ErrNotHeld otherwise, and the
// transaction must roll back.
func (s *Store) Assert(ctx context.Context, term Term, margin time.Duration) error {
	db, err := currentTransaction(ctx)
	if err != nil {
		return fmt.Errorf("assert worker lease %s: %w", term.Name, err)
	}
	if _, err := db.NewRaw(assertLeaseSQL, term.Name, term.Holder, term.Token, margin.Seconds()).Exec(ctx); err != nil {
		var state interface{ Field(byte) string }
		if errors.As(err, &state) && state.Field('C') == notHeldSQLState {
			return fmt.Errorf("%w: %s token %d", ErrNotHeld, term.Name, term.Token)
		}
		return fmt.Errorf("assert worker lease %s: %w", term.Name, err)
	}
	return nil
}
