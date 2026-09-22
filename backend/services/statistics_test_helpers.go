package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type StatisticsTestModule struct {
	Statistics  studentpresence.StatisticsReports
	ClosingDays timetableplanning.ClosingDayService
	ListExport  *listexport.RendererService
}

func NewStatisticsTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (StatisticsTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return StatisticsTestModule{}, err
	}
	r, err := repositories.NewStatisticsTestRepositories(db, command, clocks...)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	calendar, err := repositories.NewSchoolCalendar(db)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	groups, err := repositories.NewSchoolStructure(db)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	students := overlappingRosterGroupNames{StudentRepository: r.Timetable.Student, groups: groups}
	closing := timetableplanning.NewClosingDayService(r.Timetable.ClosingDay)
	service := newStatistics(db, slog.Default(), presenceCompose.StatisticsDependencies{
		StatusDays: statisticsStatusDays{r.CarePlan}, Courses: statisticsReportCourses{r.Timetable.Timetable},
		Holidays:    timetableplanning.NewHolidayService(settings.Settings, schoolCalendarHolidayAdapter{query: calendar}, slog.Default()),
		ClosingDays: closing, Periods: statisticsReportPeriods{calendar}, Students: statisticsReportStudents{students}, Rooms: statisticsReportRooms{rooms},
		AccessLog: statisticsAuditLog{r.AccessLog}, Retention: statisticsRetention{settings.Settings}, Logger: slog.Default(), Now: optionalClock(clocks),
	})
	return StatisticsTestModule{Statistics: service, ClosingDays: closing, ListExport: listexport.NewService()}, nil
}
