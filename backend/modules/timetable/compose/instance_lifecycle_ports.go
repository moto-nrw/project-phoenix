package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

func NewOperationLifecycle(s ports.InstanceLifecycle) ports.OperationLifecycle {
	return application.NewOperationLifecycle(s)
}

func SubstituteDayLock(db *bun.DB) func(context.Context, calendar.Date) error {
	return postgres.SubstituteDayLock(db)
}
