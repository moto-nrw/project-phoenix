package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ownerCapabilities bundles the migrated owner modules the legacy test graph
// still composes explicitly.
type ownerCapabilities struct {
	organizations SchoolCapability
	persons       peopledirectory.Capability
	groups        schoolstructure.Capability
	rooms         facilitiesModule.Capability
	membership    schoolmembership.Capability
	calendar      schoolcalendar.Calendar
	// bindCalendarAdministration hands the calendar the period
	// administration collaborators once the factory exists, as the
	// production root does.
	bindCalendarAdministration func(schoolCalendarCompose.AdministrationRuntime)
	timetable                  timetable.Capability
	workforce                  workforceModule.Capability
}

// NewFactoryForTests creates the partial graph used by legacy package tests.
// Production composition must provide every migrated module explicitly.
func NewFactoryForTests(repos *repositories.Factory, db *bun.DB, logger *slog.Logger, clocks ...func() time.Time) (*Factory, error) {
	owners, err := newOwnerCapabilitiesForTests(db)
	if err != nil {
		return nil, err
	}
	return owners.factory(newFactory(repos, db, logger, currentTestFactoryConfig(), tenant.UnitOfWork{}, owners.organizations, owners.persons, owners.groups, owners.rooms, owners.membership, owners.calendar, owners.timetable, nil, nil, nil, nil, nil, nil, nil, func(string, time.Duration, int, error) {}, func(string, string, string, time.Duration, error) {}, func(string, string, string, time.Duration, int, error) {}, func(string, time.Duration, int64, int64, time.Duration, string, error) {}, func(string, time.Duration, int64, int64, time.Duration, string, error) {}, owners.workforce, func(DataImportObservation) {}, FileStorageWiring{}, true, clocks...))
}

func NewFactoryForTestsWithConfig(repos *repositories.Factory, db *bun.DB, logger *slog.Logger, cfg FactoryConfig, clocks ...func() time.Time) (*Factory, error) {
	owners, err := newOwnerCapabilitiesForTests(db)
	if err != nil {
		return nil, err
	}
	return owners.factory(newFactory(repos, db, logger, cfg, tenant.UnitOfWork{}, owners.organizations, owners.persons, owners.groups, owners.rooms, owners.membership, owners.calendar, owners.timetable, nil, nil, nil, nil, nil, nil, nil, func(string, time.Duration, int, error) {}, func(string, string, string, time.Duration, error) {}, func(string, string, string, time.Duration, int, error) {}, func(string, time.Duration, int64, int64, time.Duration, string, error) {}, func(string, time.Duration, int64, int64, time.Duration, string, error) {}, owners.workforce, func(DataImportObservation) {}, FileStorageWiring{}, true, clocks...))
}

func currentTestFactoryConfig() FactoryConfig {
	cfg := currentFactoryConfig()
	cfg.PublicAPIURL = "http://api.test"
	return cfg
}

// caregiverProfilesForTests binds the Lehrkraft guard to real People
// Directory and School Membership compositions over db, as the factory does.
func caregiverProfilesForTests(db *bun.DB) (caregiverProfiles, error) {
	persons, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return caregiverProfiles{}, err
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return caregiverProfiles{}, err
	}
	return caregiverProfiles{persons: persons, membership: membership}, nil
}

func newOwnerCapabilitiesForTests(db *bun.DB) (ownerCapabilities, error) {
	organizations, err := repositories.NewOrganizationTenancy(db)
	if err != nil {
		return ownerCapabilities{}, err
	}
	persons, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return ownerCapabilities{}, err
	}
	groups, err := repositories.NewSchoolStructure(db)
	if err != nil {
		return ownerCapabilities{}, err
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return ownerCapabilities{}, err
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return ownerCapabilities{}, err
	}
	var calendarAdministration schoolCalendarCompose.AdministrationRuntime
	calendar, err := repositories.NewSchoolCalendarWithAdministration(db, func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration })
	if err != nil {
		return ownerCapabilities{}, err
	}
	careLocks, err := carePlanCompose.NewDayLocks(db, persons.LockStudent, peopledirectory.ErrStudentNotFound)
	if err != nil {
		return ownerCapabilities{}, err
	}
	timetableCapability, err := repositories.NewTimetable(db, persons, rooms, careLocks)
	if err != nil {
		return ownerCapabilities{}, err
	}
	workTime, err := repositories.NewWorkforce(db, membership)
	if err != nil {
		return ownerCapabilities{}, err
	}
	return ownerCapabilities{
		organizations: organizations, persons: persons, groups: groups, rooms: rooms,
		membership: membership, calendar: calendar, timetable: timetableCapability, workforce: workTime,
		bindCalendarAdministration: func(runtime schoolCalendarCompose.AdministrationRuntime) { calendarAdministration = runtime },
	}, nil
}

// factory hands the finished factory's period administration to the
// calendar the graph was composed with.
func (o ownerCapabilities) factory(factory *Factory, err error) (*Factory, error) {
	if err != nil {
		return nil, err
	}
	o.bindCalendarAdministration(factory.SchoolCalendarAdministration())
	return factory, nil
}
