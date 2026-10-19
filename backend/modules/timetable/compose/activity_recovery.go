package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/uptrace/bun"
)

func NewRetainedActivityRecoveryRepository(db *bun.DB, assignments ports.InstanceStudentRecords) ports.ActivityRecoveryRecords {
	a, p := application.RetainedActivityRecovery(assignments)
	return application.NewActivityRecoveryRepository(postgres.New(postgres.DatabaseRuntime(db)), a, p)
}
