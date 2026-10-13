package services

import (
	"context"
	"log/slog"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type StatisticsTestModule struct {
	Statistics     studentpresence.StatisticsReports
	SchoolCalendar schoolcalendar.Calendar
	ListExport     *lists.RendererService
	// CreateClosingDay stores a closure range through the calendar owner.
	CreateClosingDay func(ctx context.Context, start, end timezone.Date, reason string) error
}

func NewStatisticsTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (StatisticsTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return StatisticsTestModule{}, err
	}
	r, err := NewStatisticsTestRepositories(db, command, clocks...)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	students := overlappingRosterGroupNames{StudentRepository: r.Timetable.Student, groups: groups}
	calendarAdministration := schoolCalendarAdministration(settings.Settings, func(context.Context) error { return nil }, nil)
	calendar, err := NewSchoolCalendarWithAdministration(db, func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration })
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
	return StatisticsTestModule{Statistics: service, SchoolCalendar: calendar, ListExport: lists.NewRendererService(), CreateClosingDay: createClosingDay}, nil
}
