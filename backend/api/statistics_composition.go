package api

import (
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

// statisticsOwners are the owners the Statistik report reads besides the
// access log and the retention settings the service graph hands in.
type statisticsOwners struct {
	DB       transactionruntime.Database
	CarePlan careplan.StudentStatusDaysQuery
	Calendar schoolcalendar.Calendar
	Students statisticsStudentRows
	Rooms    facilities.Query
	Logger   *slog.Logger
	Now      func() time.Time
}

// newStatisticsReports binds the Statistik report to the owners; the service
// graph calls the result with its access log and settings.
func newStatisticsReports(owners statisticsOwners) func(StatisticsSources) studentpresence.StatisticsReports {
	return func(sources StatisticsSources) studentpresence.StatisticsReports {
		return NewStatistics(owners.Logger, presenceCompose.StatisticsDependencies{
			DB:          owners.DB.Pool(),
			StatusDays:  StatisticsStatusDays{owners.CarePlan},
			Courses:     NewCourseStatistics(owners.DB),
			Holidays:    schoolCalendarCompose.TenantHolidays{Calendar: owners.Calendar},
			ClosingDays: schoolCalendarCompose.TenantClosingDays{Calendar: owners.Calendar},
			Periods:     StatisticsReportPeriods{owners.Calendar},
			Students:    StatisticsReportStudents{owners.Students},
			Rooms:       StatisticsReportRooms{owners.Rooms},
			AccessLog:   StatisticsAuditLog{sources.AccessLog},
			Retention:   settingsModule.NewStatisticsRetention(sources.Settings),
			Logger:      owners.Logger.With("service", "statistics"),
			Now:         owners.Now,
		})
	}
}
