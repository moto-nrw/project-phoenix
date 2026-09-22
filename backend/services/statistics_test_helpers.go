package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type StatisticsTestModule struct {
	Statistics     studentpresence.StatisticsReports
	SchoolCalendar schoolcalendar.Calendar
	ListExport     *listexport.RendererService
	// CreateClosingDay stores a closure range through the calendar owner.
	CreateClosingDay func(ctx context.Context, start, end timezone.Date, reason string) error
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
	groups, err := repositories.NewSchoolStructure(db)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	students := overlappingRosterGroupNames{StudentRepository: r.Timetable.Student, groups: groups}
	calendarAdministration := schoolCalendarAdministration(settings.Settings, func(context.Context) error { return nil }, nil)
	calendar, err := repositories.NewSchoolCalendarWithAdministration(db, func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration })
	if err != nil {
		return StatisticsTestModule{}, err
	}
	service := newStatistics(db, slog.Default(), presenceCompose.StatisticsDependencies{
		StatusDays: statisticsStatusDays{r.CarePlan}, Courses: newCourseStatistics(db),
		Holidays:    tenantHolidays{calendar: calendar},
		ClosingDays: tenantClosingDays{calendar: calendar}, Periods: statisticsReportPeriods{calendar}, Students: statisticsReportStudents{students}, Rooms: statisticsReportRooms{rooms},
		AccessLog: statisticsAuditLog{r.AccessLog}, Retention: statisticsRetention{settings.Settings}, Logger: slog.Default(), Now: optionalClock(clocks),
	})
	createClosingDay := func(ctx context.Context, start, end timezone.Date, reason string) error {
		_, err := calendar.CreateClosingDay(ctx, schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
			StartDate: start.String(), EndDate: end.String(), Reason: reason,
		}})
		return err
	}
	return StatisticsTestModule{Statistics: service, SchoolCalendar: calendar, ListExport: listexport.NewService(), CreateClosingDay: createClosingDay}, nil
}
