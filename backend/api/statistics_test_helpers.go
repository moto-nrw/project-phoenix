//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
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
	// The occupancy reads the plan with the stored weekly rows and booking
	// authority off: no booked offering exists in these fixtures.
	care, _, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	pickups, err := careplanCompose.NewPickupBaselines(r.CarePlan, noApprovedBookings{}, func(context.Context) (bool, error) { return false, nil })
	if err != nil {
		return StatisticsTestModule{}, err
	}
	careDays, err := careplanCompose.NewCareDayQuery(careplanCompose.CareDayDependencies{Records: r.CarePlan, PickupBaselines: pickups, CareParticipation: care.CareLifecycle})
	if err != nil {
		return StatisticsTestModule{}, err
	}
	service := owners.NewStatistics(slog.Default(), presenceCompose.StatisticsDependencies{
		PlannedCareDays: owners.StatisticsPlannedCareDays{CareDays: careDays},
		DB:              db.Pool(),
		StatusDays:      owners.StatisticsStatusDays{Query: r.CarePlan}, Courses: owners.NewCourseStatistics(db),
		Holidays:    schoolCalendarCompose.NewTenantHolidays(calendar),
		ClosingDays: schoolCalendarCompose.NewTenantClosingDays(calendar), Periods: owners.StatisticsReportPeriods{Periods: calendar}, Students: owners.StatisticsReportStudents{Students: students}, Rooms: owners.StatisticsReportRooms{Rooms: rooms},
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

// noApprovedBookings is the approved-booking read of a school without
// enrollment bookings.
type noApprovedBookings struct{}

func (noApprovedBookings) ListApprovedByStudentIDsInRange(context.Context, []int64, timezone.Date, timezone.Date) ([]*careplan.ApprovedBooking, error) {
	return nil, nil
}
