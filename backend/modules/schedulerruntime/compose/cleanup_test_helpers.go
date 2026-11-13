//go:build testhooks

package compose

import (
	scheduler "github.com/moto-ogs/project-phoenix/modules/schedulerruntime/internal/application"
	"github.com/moto-ogs/project-phoenix/modules/schedulerruntime/ports"
)

// NewCleanupTestRunner runs the cleanup pass of the worker once over deps.
func NewCleanupTestRunner(deps ports.WorkerDependencies) func() {
	return scheduler.NewCleanupTestRunner(deps)
}

// ActiveSchoolsForTest returns the schools the cleanup pass visits.
func ActiveSchoolsForTest(deps ports.WorkerDependencies) []int64 {
	return scheduler.ActiveSchoolsForTest(deps)
}

// NewFileCleanupTestRunner runs the file cleanup pass of the worker once.
func NewFileCleanupTestRunner(deps ports.WorkerDependencies) func() {
	return scheduler.NewFileCleanupTestRunner(deps)
}
