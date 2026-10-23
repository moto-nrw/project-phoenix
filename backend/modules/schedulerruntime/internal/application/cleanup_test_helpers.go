//go:build testhooks

package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
)

// NewCleanupTestRunner drives the production nightly checks without starting
// polling goroutines or unrelated worker jobs. Dependencies come from the root.
func NewCleanupTestRunner(deps ports.WorkerDependencies) func() {
	s := &Scheduler{
		logger: deps.Logger, schoolRepo: deps.SchoolRepo, tenantRuntime: deps.TenantRuntime,
		settings: deps.Settings, cleanupRuns: deps.CleanupRuns, cleanupClock: deps.Now,
		cleanupService: deps.ActiveCleanup, workerTracer: deps.Tracer,
	}
	return func() {
		s.runJobCheck(&ScheduledTask{Name: "visit-cleanup"}, s.checkAndRunCleanup)
	}
}

// ActiveSchoolsForTest runs the unchanged settings-aware active-school loop.
func ActiveSchoolsForTest(deps ports.WorkerDependencies) []int64 {
	s := &Scheduler{schoolRepo: deps.SchoolRepo, tenantRuntime: deps.TenantRuntime, settings: deps.Settings, logger: deps.Logger}
	return s.forEachTenantSettings(context.Background(), "active-school-check", func(context.Context, int64) error { return nil })
}
