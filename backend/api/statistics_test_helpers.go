//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// StatisticsTestModule is Student Presence's statistics reports over the
// test database.
type StatisticsTestModule struct {
	Statistics studentpresence.StatisticsReports
	// CreateClosingDay stores a closure range through the calendar owner.
	CreateClosingDay func(ctx context.Context, start, end timezone.Date, reason string) error
}

func NewStatisticsTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (StatisticsTestModule, error) {
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	command, err := auditCompose.NewCommand(owners.NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return StatisticsTestModule{}, err
	}
	r, err := NewStatisticsTestRepositoriesWithMembers(db, owners.MustNewMembershipTestRepositories(db), clocks...)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	groups, err := owners.NewSchoolStructure(db)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return StatisticsTestModule{}, err
	}
	students := owners.OverlappingRosterGroupNames{StudentRepository: owners.MustNewMembershipTestRepositories(db).Student, Groups: groups}
	calendar, err := NewSchoolCalendarWithAdministration(db, settings.Settings, func(context.Context) error { return nil }, nil)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	service := owners.NewStatistics(slog.Default(), presenceCompose.StatisticsDependencies{
		DB:         db.Pool(),
		StatusDays: owners.StatisticsStatusDays{Query: r.CarePlan}, Courses: owners.NewCourseStatistics(db),
		Holidays:    schoolCalendarCompose.TenantHolidays{Calendar: calendar},
		ClosingDays: schoolCalendarCompose.TenantClosingDays{Calendar: calendar}, Periods: owners.StatisticsReportPeriods{Periods: calendar}, Students: owners.StatisticsReportStudents{Students: students}, Rooms: owners.StatisticsReportRooms{Rooms: rooms},
		AccessLog: owners.StatisticsAuditLog{Log: owners.NewTestAuditPersistence(db, command).DataAccessLog}, Retention: settingsModule.NewStatisticsRetention(settings.Settings), Logger: slog.Default(), Now: OptionalClock(clocks),
	})
	createClosingDay := func(ctx context.Context, start, end timezone.Date, reason string) error {
		_, err := calendar.CreateClosingDay(ctx, schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
			StartDate: start.String(), EndDate: end.String(), Reason: reason,
		}})
		return err
	}
	return StatisticsTestModule{Statistics: service, CreateClosingDay: createClosingDay}, nil
}
