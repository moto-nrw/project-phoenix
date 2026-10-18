//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"time"

	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
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
	calendar, err := NewSchoolCalendarWithAdministration(db, settings.Settings, func(context.Context) error { return nil }, nil)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	service := NewStatistics(slog.Default(), presenceCompose.StatisticsDependencies{
		DB:         db.Pool(),
		StatusDays: StatisticsStatusDays{Query: r.CarePlan}, Courses: NewCourseStatistics(db),
		Holidays:    schoolCalendarCompose.TenantHolidays{Calendar: calendar},
		ClosingDays: schoolCalendarCompose.TenantClosingDays{Calendar: calendar}, Periods: StatisticsReportPeriods{Periods: calendar}, Students: StatisticsReportStudents{Students: students}, Rooms: StatisticsReportRooms{Rooms: rooms},
		AccessLog: StatisticsAuditLog{Log: r.AccessLog}, Retention: settingsModule.NewStatisticsRetention(settings.Settings), Logger: slog.Default(), Now: OptionalClock(clocks),
	})
	createClosingDay := func(ctx context.Context, start, end timezone.Date, reason string) error {
		_, err := calendar.CreateClosingDay(ctx, schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
			StartDate: start.String(), EndDate: end.String(), Reason: reason,
		}})
		return err
	}
	return StatisticsTestModule{Statistics: service, SchoolCalendar: calendar, ListExport: lists.NewRendererService(), CreateClosingDay: createClosingDay}, nil
}
