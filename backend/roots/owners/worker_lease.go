package owners

import (
	"fmt"

	schedulerRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// newWorkerLease binds the Worker's leadership (#2726) to
// platform.worker_leases and its evidence to the metrics. Scheduler Runtime
// owns the lease timing and the holder ID.
func newWorkerLease(runtime tenant.UnitOfWork) (schedulerPorts.WorkerLease, error) {
	store, err := schedulerRuntimeCompose.NewWorkerLeaseStore(runtime)
	if err != nil {
		return schedulerPorts.WorkerLease{}, err
	}
	return schedulerPorts.NewWorkerLease(store, schedulerPorts.LeaseEvidence{
		Term:       observability.SetWorkerLeaseTerm,
		Operation:  observability.RecordWorkerLeaseOperation,
		Leadership: observability.RecordWorkerLeadershipChange,
		Standby:    observability.ObserveWorkerStandby,
		Suppressed: observability.RecordWorkerDuplicateSuppression,
		Ready:      observability.SetWorkerReady,
		Drain:      observability.ObserveWorkerDrain,
	})
}

// workerLeaseName is the lease name Scheduler Runtime gives the Worker.
const workerLeaseName = schedulerPorts.WorkerLeaseName

// newWorkerRuntime constructs the lease and tenant transactions for the Worker.
func newWorkerRuntime(unitOfWork tenant.UnitOfWork) (schedulerPorts.WorkerLease, schedulerPorts.TenantRuntime, error) {
	lease, err := newWorkerLease(unitOfWork)
	if err != nil {
		return schedulerPorts.WorkerLease{}, nil, fmt.Errorf("compose worker lease: %w", err)
	}
	runtime, err := schedulerRuntimeCompose.NewTenantRuntime(unitOfWork)
	if err != nil {
		return schedulerPorts.WorkerLease{}, nil, fmt.Errorf("compose worker tenant runtime: %w", err)
	}
	return lease, runtime, nil
}
