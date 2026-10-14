package api

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// OwnerCapabilities bundles the migrated owner modules the legacy test graph
// still composes explicitly.
type OwnerCapabilities struct {
	Organizations services.SchoolCapability
	Persons       peopledirectory.Capability
	Groups        schoolstructure.Capability
	Rooms         facilitiesModule.Capability
	Membership    schoolmembership.Capability
	Calendar      schoolcalendar.Calendar
	// bindCalendarAdministration hands the calendar the period
	// administration collaborators once the factory exists, as the
	// production root does.
	bindCalendarAdministration func(schoolCalendarCompose.AdministrationRuntime)
	Timetable                  timetable.Capability
	Appointments               appointments.Capability
	Workforce                  workforceModule.Capability
}

// NewFactoryForTests creates the partial graph used by legacy package tests.
// Production composition must provide every migrated module explicitly.
func NewFactoryForTests(repos *RepositoryTestFactory, db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (*services.Factory, error) {
	owners, err := NewOwnerCapabilitiesForTests(db)
	if err != nil {
		return nil, err
	}
	cfg := currentTestFactoryConfig()
	tokens, err := TokenAdapterForTests(cfg)
	if err != nil {
		return nil, err
	}
	people, err := owners.peopleWiring(repos, db, logger)
	if err != nil {
		return nil, err
	}
	return owners.factory(services.NewFactoryWithConfig(repos.CarePlan(), db.Pool(), logger, cfg, tenant.UnitOfWork{}, owners.Organizations, owners.Persons, owners.Groups, owners.Rooms, owners.Membership, owners.Calendar, owners.Timetable, owners.Appointments, nil, nil, nil, nil, nil, nil, nil, func(string, time.Duration, int, error) {}, func(string, string, string, time.Duration, error) {}, func(string, string, string, time.Duration, int, error) {}, func(string, time.Duration, int64, int64, time.Duration, string, error) {}, func(string, time.Duration, int64, int64, time.Duration, string, error) {}, owners.Workforce, func(services.DataImportObservation) {}, nil, services.FileStorageWiring{}, tokens, people, true, clocks...))
}

func NewFactoryForTestsWithConfig(repos *RepositoryTestFactory, db transactionruntime.Database, logger *slog.Logger, cfg services.FactoryConfig, clocks ...func() time.Time) (*services.Factory, error) {
	owners, err := NewOwnerCapabilitiesForTests(db)
	if err != nil {
		return nil, err
	}
	tokens, err := TokenAdapterForTests(cfg)
	if err != nil {
		return nil, err
	}
	people, err := owners.peopleWiring(repos, db, logger)
	if err != nil {
		return nil, err
	}
	return owners.factory(services.NewFactoryWithConfig(repos.CarePlan(), db.Pool(), logger, cfg, tenant.UnitOfWork{}, owners.Organizations, owners.Persons, owners.Groups, owners.Rooms, owners.Membership, owners.Calendar, owners.Timetable, owners.Appointments, nil, nil, nil, nil, nil, nil, nil, func(string, time.Duration, int, error) {}, func(string, string, string, time.Duration, error) {}, func(string, string, string, time.Duration, int, error) {}, func(string, time.Duration, int64, int64, time.Duration, string, error) {}, func(string, time.Duration, int64, int64, time.Duration, string, error) {}, owners.Workforce, func(services.DataImportObservation) {}, nil, services.FileStorageWiring{}, tokens, people, true, clocks...))
}

// peopleWiring builds the People Directory, School Membership and School
// Structure wiring over the test owners, as the serving root does (#4007).
func (owners OwnerCapabilities) peopleWiring(repos *RepositoryTestFactory, db transactionruntime.Database, logger *slog.Logger) (services.PeopleWiring, error) {
	return owners.PeopleWiring(db, repos.CarePlan(), logger)
}

// PeopleWiring builds the People Directory, School Membership and School
// Structure wiring over the test owners and the given Care Plan.
func (owners OwnerCapabilities) PeopleWiring(db transactionruntime.Database, carePlan careplan.Capability, logger *slog.Logger) (services.PeopleWiring, error) {
	return NewPeopleWiring(PeopleWiringOwners{
		DB: db, Logger: logger, Persons: owners.Persons, Groups: owners.Groups, Rooms: owners.Rooms,
		Membership: owners.Membership, Workforce: owners.Workforce, CarePlan: carePlan,
		Organizations: owners.Organizations, Timetable: owners.Timetable,
	})
}

func currentTestFactoryConfig() services.FactoryConfig {
	cfg := services.CurrentFactoryConfig()
	cfg.PublicAPIURL = "http://api.test"
	return cfg
}

// caregiverProfilesForTests binds the Lehrkraft guard to real People
// Directory and School Membership compositions over db, as the factory does.
func caregiverProfilesForTests(db transactionruntime.Database) (services.CaregiverProfiles, error) {
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return services.CaregiverProfiles{}, err
	}
	membership, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		return services.CaregiverProfiles{}, err
	}
	return services.CaregiverProfiles{Persons: persons, Membership: membership}, nil
}

// NewOwnerCapabilitiesForTests composes the owner modules the legacy test
// graph binds into the factory, unobserved.
func NewOwnerCapabilitiesForTests(db transactionruntime.Database) (OwnerCapabilities, error) {
	organizations, err := services.NewOrganizationTenancy(db.Pool())
	if err != nil {
		return OwnerCapabilities{}, err
	}
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return OwnerCapabilities{}, err
	}
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return OwnerCapabilities{}, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return OwnerCapabilities{}, err
	}
	membership, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		return OwnerCapabilities{}, err
	}
	var calendarAdministration schoolCalendarCompose.AdministrationRuntime
	calendar, err := NewSchoolCalendarWithAdministration(db, func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration })
	if err != nil {
		return OwnerCapabilities{}, err
	}
	timetableCapability, err := services.NewTimetable(db.Pool(), persons, rooms)
	if err != nil {
		return OwnerCapabilities{}, err
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return OwnerCapabilities{}, err
	}
	appointmentCapability, err := NewAppointments(db)
	if err != nil {
		return OwnerCapabilities{}, err
	}
	return OwnerCapabilities{
		Organizations: organizations, Persons: persons, Groups: groups, Rooms: rooms,
		Membership: membership, Calendar: calendar, Timetable: timetableCapability, Appointments: appointmentCapability, Workforce: workTime,
		bindCalendarAdministration: func(runtime schoolCalendarCompose.AdministrationRuntime) { calendarAdministration = runtime },
	}, nil
}

// factory hands the finished factory's period administration to the
// calendar the graph was composed with.
func (o OwnerCapabilities) factory(factory *services.Factory, err error) (*services.Factory, error) {
	if err != nil {
		return nil, err
	}
	o.bindCalendarAdministration(factory.SchoolCalendarAdministration())
	return factory, nil
}

// UnboundTenantRuntime is the zero unit of work the test factories compose
// with before a suite binds its own.
func UnboundTenantRuntime() tenant.UnitOfWork { return tenant.UnitOfWork{} }
