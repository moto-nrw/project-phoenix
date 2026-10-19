// Package compose builds the Scheduler Runtime for the roots (ADR 0053): the
// Worker over its complete dependencies and its stores over the root's
// tenant runtime. The root imports neither the Worker nor its stores.
package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/internal/adapters/cleanupruns"
	scheduler "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/jobruntime"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/workerlease"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewWorker builds the Worker; a missing dependency is an error (ADR 0059).
func NewWorker(deps ports.WorkerDependencies) (ports.Worker, error) {
	return scheduler.NewWorker(deps)
}

// SettingKeys names every Settings Platform key the Worker reads.
func SettingKeys() []string { return scheduler.SettingKeys() }

// NewCleanupRunLedger builds the ledger of each school's last successful
// nightly cleanup run (#3950). It joins the tenant transaction of each call.
func NewCleanupRunLedger() ports.CleanupRunLedger { return cleanupruns.NewStore() }

// NewTenantRuntime runs the Worker's transactions in the root's tenant
// runtime.
func NewTenantRuntime(unitOfWork tenant.UnitOfWork) (ports.TenantRuntime, error) {
	bound, err := jobruntime.New(unitOfWork)
	if err != nil {
		return nil, err
	}
	return bound, nil
}

// NewWorkerLeaseStore binds the Worker's lease port to
// platform.worker_leases (#2726).
func NewWorkerLeaseStore(unitOfWork tenant.UnitOfWork) (ports.LeaseStore, error) {
	store, err := workerlease.NewStore(unitOfWork)
	if err != nil {
		return nil, err
	}
	return workerLeaseStore{store: store}, nil
}

// workerLeaseStore adapts the lease persistence to the Worker's port.
type workerLeaseStore struct{ store *workerlease.Store }

func (s workerLeaseStore) Acquire(ctx context.Context, name, holder string, ttl time.Duration) (ports.LeaseTerm, bool, error) {
	term, acquired, err := s.store.Acquire(ctx, name, holder, ttl)
	return schedulerLeaseTerm(term), acquired, err
}

func (s workerLeaseStore) Renew(ctx context.Context, term ports.LeaseTerm, ttl time.Duration) (ports.LeaseTerm, bool, error) {
	renewed, ok, err := s.store.Renew(ctx, persistedLeaseTerm(term), ttl)
	return schedulerLeaseTerm(renewed), ok, err
}

func (s workerLeaseStore) Release(ctx context.Context, term ports.LeaseTerm) error {
	return s.store.Release(ctx, persistedLeaseTerm(term))
}

func (s workerLeaseStore) Assert(ctx context.Context, term ports.LeaseTerm, margin time.Duration) error {
	err := s.store.Assert(ctx, persistedLeaseTerm(term), margin)
	if errors.Is(err, workerlease.ErrNotHeld) {
		return fmt.Errorf("%w: %w", ports.ErrLeaseNotHeld, err)
	}
	return err
}

func schedulerLeaseTerm(term workerlease.Term) ports.LeaseTerm {
	return ports.LeaseTerm{Name: term.Name, Holder: term.Holder, Token: term.Token, Until: term.Until}
}

func persistedLeaseTerm(term ports.LeaseTerm) workerlease.Term {
	return workerlease.Term{Name: term.Name, Holder: term.Holder, Token: term.Token, Until: term.Until}
}
