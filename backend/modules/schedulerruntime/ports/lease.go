package ports

import (
	"context"
	"errors"
	"time"
)

// ErrLeaseNotHeld reports that a lease term ended: it expired, was released,
// or a newer holder took it over. A job transaction that sees it must not
// commit.
var ErrLeaseNotHeld = errors.New("worker lease is not held")

// LeaseTerm is one holder's tenure of the Worker lease. Until is database
// time and is evidence only; the holder decides locally with its own
// monotonic clock when to stop (see leadership.validUntil).
type LeaseTerm struct {
	Name   string
	Holder string
	Token  int64
	Until  time.Time
}

// LeaseStore is the Worker's port to its lease (#2726). Acquire, Renew and
// Release run in their own administrative transaction. Assert runs inside the
// job transaction open in ctx and returns ErrLeaseNotHeld unless term still
// holds the lease for at least margin.
type LeaseStore interface {
	Acquire(ctx context.Context, name, holder string, ttl time.Duration) (LeaseTerm, bool, error)
	Renew(ctx context.Context, term LeaseTerm, ttl time.Duration) (LeaseTerm, bool, error)
	Release(ctx context.Context, term LeaseTerm) error
	Assert(ctx context.Context, term LeaseTerm, margin time.Duration) error
}

// LeaseEvidence receives the runtime evidence of the lease. The composition
// root binds it to metrics; nil functions are skipped.
type LeaseEvidence struct {
	// Term reports the term this process holds; held is false in standby.
	Term func(held bool, token int64, until time.Time)
	// Operation reports one acquire, renew or release with its outcome
	// (ok, busy, lost, error) and latency.
	Operation func(operation, outcome string, latency time.Duration)
	// Leadership reports a change: acquired, lost, expired, fenced, released.
	Leadership func(change string)
	// Standby reports how long this process waited before it led.
	Standby func(time.Duration)
	// Suppressed reports a job run or commit that did not happen because
	// this process did not hold the lease: standby or fenced.
	Suppressed func(reason string)
	// Ready reports readiness, which equals holding the lease.
	Ready func(bool)
	// Drain reports how long a graceful stop waited for running jobs.
	Drain func(time.Duration)
}

// WorkerLease configures the leadership every Worker process runs under.
// Only the holder of the lease named Name runs jobs.
//
// A crashed holder is replaced after at most TTL + RetryEvery, a released
// one after at most RetryEvery. The holder stops on its own TTL-FenceMargin
// after its last successful acquire or renew, measured on its monotonic
// clock from before the statement was sent, so it stops before the database
// lets a standby in, whatever either clock says. Commits additionally assert
// the term with FenceMargin of database time to spare.
type WorkerLease struct {
	Store       LeaseStore
	Name        string
	Holder      string
	TTL         time.Duration
	RenewEvery  time.Duration
	RetryEvery  time.Duration
	FenceMargin time.Duration
	Evidence    LeaseEvidence
}
