package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
)

// persistedLeaseTerm bounds the storage value accepted by the lease port.
// It is a constraint, not a second row type or an alias of the adapter.
type persistedLeaseTerm interface {
	~struct {
		Name   string
		Holder string
		Token  int64
		Until  time.Time
	}
}

type leasePersistence[Term persistedLeaseTerm] interface {
	Acquire(context.Context, string, string, time.Duration) (Term, bool, error)
	Renew(context.Context, Term, time.Duration) (Term, bool, error)
	Release(context.Context, Term) error
	Assert(context.Context, Term, time.Duration) error
}

type leaseStore[Term persistedLeaseTerm] struct {
	store   leasePersistence[Term]
	notHeld error
}

func NewLeaseStore[Term persistedLeaseTerm](store leasePersistence[Term], notHeld error) ports.LeaseStore {
	return leaseStore[Term]{store: store, notHeld: notHeld}
}

func (s leaseStore[Term]) Acquire(ctx context.Context, name, holder string, ttl time.Duration) (ports.LeaseTerm, bool, error) {
	term, acquired, err := s.store.Acquire(ctx, name, holder, ttl)
	return ports.LeaseTerm(term), acquired, err
}
func (s leaseStore[Term]) Renew(ctx context.Context, term ports.LeaseTerm, ttl time.Duration) (ports.LeaseTerm, bool, error) {
	renewed, ok, err := s.store.Renew(ctx, Term(term), ttl)
	return ports.LeaseTerm(renewed), ok, err
}
func (s leaseStore[Term]) Release(ctx context.Context, term ports.LeaseTerm) error {
	return s.store.Release(ctx, Term(term))
}
func (s leaseStore[Term]) Assert(ctx context.Context, term ports.LeaseTerm, margin time.Duration) error {
	err := s.store.Assert(ctx, Term(term), margin)
	if errors.Is(err, s.notHeld) {
		return fmt.Errorf("%w: %w", ports.ErrLeaseNotHeld, err)
	}
	return err
}
