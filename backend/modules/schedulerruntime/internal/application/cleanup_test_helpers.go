//go:build testhooks

package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/schedulerruntime/ports"
)

// NewCleanupTestRunner drives the production nightly checks without starting
// polling goroutines or unrelated worker jobs. Dependencies come from the root.
func NewCleanupTestRunner(deps ports.WorkerDependencies) func() {
	retention, err := newRetentionRun(deps.RetentionSteps, deps.RetentionLog, deps.Logger)
	if err != nil {
		panic(err)
	}
	s := &Scheduler{
		retentionCleanupJobs: newRetentionCleanupJobs(deps),
		logger:               deps.Logger, schoolRepo: deps.SchoolRepo, tenantRuntime: deps.TenantRuntime,
		retention: retention,
		settings:  deps.Settings, cleanupRuns: deps.CleanupRuns, cleanupClock: deps.Now,
		cleanupService: deps.ActiveCleanup, workerTracer: deps.Tracer,
		workSessionCleanup:        deps.WorkSessionCleanup,
		enrollmentRejectedCleanup: deps.EnrollmentRejectedCleanup,
	}
	return func() {
		s.runJobCheck(&ScheduledTask{Name: "visit-cleanup"}, s.checkAndRunCleanup)
		if s.pwaUsageCleanup != nil {
			s.runJobCheck(&ScheduledTask{Name: "pwa-usage-cleanup"}, s.checkAndRunPWAUsageCleanup)
		}
		if s.staffMessageCleanup != nil {
			s.runJobCheck(&ScheduledTask{Name: "staff-message-cleanup"}, s.checkAndRunStaffMessageCleanup)
		}
	}
}

// ActiveSchoolsForTest runs the unchanged settings-aware active-school loop.
func ActiveSchoolsForTest(deps ports.WorkerDependencies) []int64 {
	s := &Scheduler{schoolRepo: deps.SchoolRepo, tenantRuntime: deps.TenantRuntime, settings: deps.Settings, logger: deps.Logger}
	return s.forEachTenantSettings(context.Background(), "active-school-check", func(context.Context, int64) error { return nil })
}

// NewFileCleanupTestRunner drives the same three recovery checks as the Worker.
func NewFileCleanupTestRunner(deps ports.WorkerDependencies) func() {
	s := &Scheduler{
		logger: deps.Logger, schoolRepo: deps.SchoolRepo, tenantRuntime: deps.TenantRuntime,
		staffDocumentFileCleaner:   deps.StaffDocumentCleaner,
		studentDocumentFileCleaner: deps.StudentDocumentCleaner, fileStoreCleaner: deps.FileStoreCleaner,
		workerTracer: deps.Tracer,
	}
	return func() {
		if s.staffDocumentFileCleaner != nil {
			s.runJobCheck(&ScheduledTask{Name: "staff-document-file-cleanup"}, s.checkAndRunStaffDocumentFileCleanup)
		}
		if s.studentDocumentFileCleaner != nil {
			s.runJobCheck(&ScheduledTask{Name: "student-document-file-cleanup"}, s.checkAndRunStudentDocumentFileCleanup)
		}
		if s.fileStoreCleaner != nil {
			s.runJobCheck(&ScheduledTask{Name: "file-store-cleanup"}, s.checkAndRunFileStoreCleanup)
		}
	}
}
