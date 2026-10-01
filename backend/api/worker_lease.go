package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/workerlease"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
)

// The Worker lease (#2726). A crashed leader is replaced after at most
// TTL + retry (35 s), a stopped one after at most the retry interval (5 s).
const (
	workerLeaseName        = "worker"
	workerLeaseTTL         = 30 * time.Second
	workerLeaseRenewEvery  = 10 * time.Second
	workerLeaseRetryEvery  = 5 * time.Second
	workerLeaseFenceMargin = 5 * time.Second
)

// newWorkerLease binds the Worker's leadership to platform.worker_leases.
// Every process gets a holder ID of its own, so a restarted process never
// resumes the term of its predecessor.
func newWorkerLease(runtime apiCommon.TenantRuntime) (scheduler.WorkerLease, error) {
	store, err := workerlease.NewStore(runtime)
	if err != nil {
		return scheduler.WorkerLease{}, err
	}
	holder, err := workerHolderID()
	if err != nil {
		return scheduler.WorkerLease{}, err
	}
	return scheduler.WorkerLease{
		Store:       workerLeaseStore{store: store},
		Name:        workerLeaseName,
		Holder:      holder,
		TTL:         workerLeaseTTL,
		RenewEvery:  workerLeaseRenewEvery,
		RetryEvery:  workerLeaseRetryEvery,
		FenceMargin: workerLeaseFenceMargin,
		Evidence: scheduler.LeaseEvidence{
			Term:       observability.SetWorkerLeaseTerm,
			Operation:  observability.RecordWorkerLeaseOperation,
			Leadership: observability.RecordWorkerLeadershipChange,
			Standby:    observability.ObserveWorkerStandby,
			Suppressed: observability.RecordWorkerDuplicateSuppression,
			Ready:      observability.SetWorkerReady,
			Drain:      observability.ObserveWorkerDrain,
		},
	}, nil
}

// workerHolders numbers the leases one process composes.
var workerHolders atomic.Int64

// workerHolderID names one Worker: host, process ID, and start time with a
// sequence number. A restarted process with a reused PID still differs in
// its start time.
func workerHolderID() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("worker lease holder: read hostname: %w", err)
	}
	started := strconv.FormatInt(time.Now().UnixNano(), 36)
	return fmt.Sprintf("%s/%d/%s-%d", host, os.Getpid(), started, workerHolders.Add(1)), nil
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
