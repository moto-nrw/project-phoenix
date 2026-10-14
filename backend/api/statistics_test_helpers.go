package api

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
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type StatisticsTestModule struct {
	Statistics     studentpresence.StatisticsReports
	SchoolCalendar schoolcalendar.Calendar
	ListExport     *lists.RendererService
	// CreateClosingDay stores a closure range through the calendar owner.
	CreateClosingDay func(ctx context.Context, start, end timezone.Date, reason string) error
}

func NewStatisticsTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (StatisticsTestModule, error) {
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
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return StatisticsTestModule{}, err
	}
	students := OverlappingRosterGroupNames{StudentRepository: r.Timetable.Student, Groups: groups}
	calendarAdministration := services.SchoolCalendarAdministration(settings.Settings, func(context.Context) error { return nil }, nil)
	calendar, err := NewSchoolCalendarWithAdministration(db, func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration })
	if err != nil {
		return StatisticsTestModule{}, err
	}
	service := services.NewStatistics(slog.Default(), presenceCompose.StatisticsDependencies{
		DB:         db.Pool(),
		StatusDays: services.StatisticsStatusDays{Query: r.CarePlan}, Courses: services.NewCourseStatistics(services.MustPresenceReads(db.Pool())),
		Holidays:    services.TenantHolidays{Calendar: calendar},
		ClosingDays: services.TenantClosingDays{Calendar: calendar}, Periods: services.StatisticsReportPeriods{Periods: calendar}, Students: services.StatisticsReportStudents{Students: students}, Rooms: services.StatisticsReportRooms{Rooms: rooms},
		AccessLog: services.StatisticsAuditLog{Log: r.AccessLog}, Retention: services.StatisticsRetention{Settings: settings.Settings}, Logger: slog.Default(), Now: services.OptionalClock(clocks),
	})
	createClosingDay := func(ctx context.Context, start, end timezone.Date, reason string) error {
		_, err := calendar.CreateClosingDay(ctx, schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
			StartDate: start.String(), EndDate: end.String(), Reason: reason,
		}})
		return err
	}
	return StatisticsTestModule{Statistics: service, SchoolCalendar: calendar, ListExport: lists.NewRendererService(), CreateClosingDay: createClosingDay}, nil
}
