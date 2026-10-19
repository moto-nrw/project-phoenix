package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// StaffNoticeDependencies binds the staff notices (#2180) to the database
// and to the readers the root provides; see application.StaffNoticeDependencies
// for the optional ones.
type StaffNoticeDependencies struct {
	DB          *bun.DB
	Periods     application.StaffNoticePeriodLookup
	Names       func(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	Logger      *slog.Logger
	CurrentDate func() calendar.Date
}

// NewStaffNotices builds the staff notices over the owner's repository.
func NewStaffNotices(deps StaffNoticeDependencies) timetable.StaffNotices {
	return application.NewStaffNotices(NewStaffNoticeRepository(deps.DB), application.StaffNoticeDependencies{
		Periods: deps.Periods, Names: deps.Names, Logger: deps.Logger, CurrentDate: deps.CurrentDate,
	})
}

// NewStaffNoticeRepository builds the retained staff notice repository.
func NewStaffNoticeRepository(db *bun.DB) ports.StaffNoticeRecords {
	return application.NewStaffNoticeRecords(postgres.NewStaffNoticeRepository(postgres.New(databaseRuntime(db))), transaction{})
}
