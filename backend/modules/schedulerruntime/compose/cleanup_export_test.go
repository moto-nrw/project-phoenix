package compose

import (
	scheduler "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
)

func NewCleanupTestRunner(deps ports.WorkerDependencies) func() {
	return scheduler.NewCleanupTestRunner(deps)
}

func ActiveSchoolsForTest(deps ports.WorkerDependencies) []int64 {
	return scheduler.ActiveSchoolsForTest(deps)
}
