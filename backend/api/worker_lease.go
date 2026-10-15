package api

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	schedulerRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
	"github.com/moto-nrw/project-phoenix/tenant"
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
func newWorkerLease(runtime tenant.UnitOfWork) (scheduler.WorkerLease, error) {
	store, err := schedulerRuntimeCompose.NewWorkerLeaseStore(runtime)
	if err != nil {
		return scheduler.WorkerLease{}, err
	}
	holder, err := workerHolderID()
	if err != nil {
		return scheduler.WorkerLease{}, err
	}
	return scheduler.WorkerLease{
		Store:       store,
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
