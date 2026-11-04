package owners

import (
	"context"
	"log/slog"
	"time"

	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewStatistics builds the Statistik report over the presence database of
// deps with the operation logging of NewLoggedStudentPresence.
func NewStatistics(logger *slog.Logger, deps presenceCompose.StatisticsDependencies) studentpresence.StatisticsReports {
	deps.Observe = presenceCompose.DebugLogger(logger)
	reports, err := presenceCompose.NewStatistics(deps)
	if err != nil {
		panic(err)
	}
	return reports
}

// StatisticsSettings keeps the history switch separate from unrelated retention consumers.
type StatisticsSettings interface {
	RetentionSettings
	ResolveBool(context.Context, string) (bool, error)
}

// newStatisticsReports binds the Statistik report to the owners it reads;
// the service graph calls the result with its access log and retention
// settings. now fixes the report clock; nil is the Berlin clock.
func newStatisticsReports(db transactionruntime.Database, carePlan careplan.StudentStatusDaysQuery, careDays careplan.CareDayQuery, calendar schoolcalendar.Days, students statisticsStudentRows, rooms facilities.Query, logger *slog.Logger, now func() time.Time) func(AccessLogLedger, StatisticsSettings) studentpresence.StatisticsReports {
	return func(accessLog AccessLogLedger, settings StatisticsSettings) studentpresence.StatisticsReports {
		return NewStatistics(logger, presenceCompose.StatisticsDependencies{
			DB:              db.Pool(),
			StatusDays:      StatisticsStatusDays{carePlan},
			Courses:         NewCourseStatistics(db),
			Holidays:        schoolCalendarCompose.NewTenantHolidays(calendar),
			ClosingDays:     schoolCalendarCompose.NewTenantClosingDays(calendar),
			Periods:         StatisticsReportPeriods{calendar},
			Students:        StatisticsReportStudents{students},
			Rooms:           StatisticsReportRooms{rooms},
			AccessLog:       StatisticsAuditLog{accessLog},
			Retention:       settingsModule.NewStatisticsRetention(settings),
			PlannedCareDays: StatisticsPlannedCareDays{careDays},
			Logger:          logger.With("service", "statistics"),
			Now:             now,
		})
	}
}
