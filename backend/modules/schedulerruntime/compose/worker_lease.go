// Package compose builds the Scheduler Runtime's persistence over the root's
// tenant runtime and hands it to the Worker as the scheduler's own ports
// (#4003). The root passes the unit of work and imports neither the lease
// store nor the job runtime.
package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/jobruntime"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/workerlease"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewTenantRuntime runs the Worker's transactions in the root's tenant
// runtime.
func NewTenantRuntime(unitOfWork tenant.UnitOfWork) (scheduler.TenantRuntime, error) {
	bound, err := jobruntime.New(unitOfWork)
	if err != nil {
		return nil, err
	}
	return bound, nil
}

// NewWorkerLeaseStore binds the scheduler's lease port to
// platform.worker_leases (#2726).
func NewWorkerLeaseStore(unitOfWork tenant.UnitOfWork) (scheduler.LeaseStore, error) {
	store, err := workerlease.NewStore(unitOfWork)
	if err != nil {
		return nil, err
	}
	return workerLeaseStore{store: store}, nil
}

// workerLeaseStore adapts the lease persistence to the scheduler's port.
type workerLeaseStore struct{ store *workerlease.Store }

func (s workerLeaseStore) Acquire(ctx context.Context, name, holder string, ttl time.Duration) (scheduler.LeaseTerm, bool, error) {
	term, acquired, err := s.store.Acquire(ctx, name, holder, ttl)
	return schedulerLeaseTerm(term), acquired, err
}

func (s workerLeaseStore) Renew(ctx context.Context, term scheduler.LeaseTerm, ttl time.Duration) (scheduler.LeaseTerm, bool, error) {
	renewed, ok, err := s.store.Renew(ctx, persistedLeaseTerm(term), ttl)
	return schedulerLeaseTerm(renewed), ok, err
}

func (s workerLeaseStore) Release(ctx context.Context, term scheduler.LeaseTerm) error {
	return s.store.Release(ctx, persistedLeaseTerm(term))
}

func (s workerLeaseStore) Assert(ctx context.Context, term scheduler.LeaseTerm, margin time.Duration) error {
	err := s.store.Assert(ctx, persistedLeaseTerm(term), margin)
	if errors.Is(err, workerlease.ErrNotHeld) {
		return fmt.Errorf("%w: %w", scheduler.ErrLeaseNotHeld, err)
	}
	return err
}

func schedulerLeaseTerm(term workerlease.Term) scheduler.LeaseTerm {
	return scheduler.LeaseTerm{Name: term.Name, Holder: term.Holder, Token: term.Token, Until: term.Until}
}

func persistedLeaseTerm(term scheduler.LeaseTerm) workerlease.Term {
	return workerlease.Term{Name: term.Name, Holder: term.Holder, Token: term.Token, Until: term.Until}
}
