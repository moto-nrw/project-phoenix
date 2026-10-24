// Package compose builds the Scheduler Runtime for the roots (ADR 0053): the
// Worker over its complete dependencies and its stores over the root's
// tenant runtime. The root imports neither the Worker nor its stores.
package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/internal/adapters/cleanupruns"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/internal/adapters/jobruntime"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/internal/adapters/workerlease"
	scheduler "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
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
	return scheduler.NewLeaseStore(store, workerlease.ErrNotHeld), nil
}
