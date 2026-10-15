package api

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"

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
	Organizations organizationtenancy.Capability
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

// TestFactory is the partial legacy graph of a test together with the
// Identity & Access the root composed for it (#4006). Auth, MFA,
// Invitation, GuardianInvitation and the operator fields are the one
// composed module under the names the suites drive it by.
type TestFactory struct {
	*services.Factory
	Auth               *identityaccess.Module
	MFA                identityaccess.AccountMFA
	Invitation         InvitationCapability
	GuardianInvitation GuardianInvitationCapability
	OperatorAuth       *identityaccess.Module
	OperatorInvitation *identityaccess.Module
	OperatorMFA        identityaccess.OperatorMFAFlows
	UserContext        *CallerRows
	// Facilities and StudentStatusDays are the room service and the status
	// days the root built for this graph (#4010), under the names the suites
	// drive them by.
	Facilities        facilitiesCompose.RoomService
	StudentStatusDays studentpresence.StatusDays
}

// AccountAuthentication returns the composed Identity & Access module the
// session, lifecycle, MFA and passkey routes consume.
func (f *TestFactory) AccountAuthentication() *identityaccess.Module { return f.Auth }

// SetTenantRuntime binds the suite's unit of work to the legacy graph and
// to the Identity & Access module the root composed for it.
func (f *TestFactory) SetTenantRuntime(runtime any) error {
	if err := f.Factory.SetTenantRuntime(runtime); err != nil {
		return err
	}
	return f.Auth.SetTenantRuntime(runtime)
}

// NewFactoryForTests creates the partial graph used by legacy package tests.
// Production composition must provide every migrated module explicitly.
func NewFactoryForTests(repos *RepositoryTestFactory, db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (*TestFactory, error) {
	return NewFactoryForTestsWithConfig(repos, db, logger, currentTestFactoryConfig(), clocks...)
}

func NewFactoryForTestsWithConfig(repos *RepositoryTestFactory, db transactionruntime.Database, logger *slog.Logger, cfg services.FactoryConfig, clocks ...func() time.Time) (*TestFactory, error) {
	owners, err := NewOwnerCapabilitiesForTests(db)
	if err != nil {
		return nil, err
	}
	tokens, err := TokenAdapterForTests(cfg)
	if err != nil {
		return nil, err
	}
	people, err := owners.PeopleWiring(db, repos.CarePlan(), logger)
	if err != nil {
		return nil, err
	}
	platform, err := owners.PlatformWiring(db, people, clocks...)
	if err != nil {
		return nil, err
	}
	presence, err := owners.PresenceWiring(db, repos.CarePlan(), people, logger, clocks...)
	if err != nil {
		return nil, err
	}
	timetable, err := owners.TimetableWiring(db, people, logger)
	if err != nil {
		return nil, err
	}
	wiring, identity := newIdentityRoot(db, tokens)
	carePlan, err := owners.CarePlanWiring(db, repos.CarePlan(), people, wiring.AuditActor, logger)
	if err != nil {
		return nil, err
	}
	factory, err := owners.factory(services.NewFactoryWithConfig(repos.CarePlan(), db.Pool(), logger, cfg, tenant.UnitOfWork{}, owners.Organizations, owners.Persons, owners.Groups, owners.Rooms, owners.Membership, owners.Calendar, owners.Timetable, owners.Appointments, nil, nil, nil, nil, nil, nil, func(string, time.Duration, int, error) {}, func(string, string, string, time.Duration, error) {}, func(string, string, string, time.Duration, int, error) {}, func(string, time.Duration, int64, int64, time.Duration, string, error) {}, func(string, time.Duration, int64, int64, time.Duration, string, error) {}, owners.Workforce, owners.WorkforceWiring(db), nil, services.FileStorageWiring{}, wiring, people, platform, presence, timetable, carePlan, owners.EnrollmentWiring(db, repos.CarePlan(), people), true, clocks...))
	if err != nil {
		return nil, err
	}
	module, userContext := identity()
	return &TestFactory{
		Factory: factory, Auth: module, MFA: module, Invitation: module, GuardianInvitation: module,
		OperatorAuth: module, OperatorInvitation: module, OperatorMFA: module,
		UserContext: userContext,
		Facilities:  presence.RoomService, StudentStatusDays: presence.StatusDays,
	}, nil
}

// CarePlanWiring builds the Care Plan wiring over the test owners, the given
// Care Plan and People Directory repositories, as the serving root does
// (#4011).
func (owners OwnerCapabilities) CarePlanWiring(db transactionruntime.Database, carePlan careplan.Capability, people services.PeopleWiring, auditActor peopleCompose.RequestAuditActor, logger *slog.Logger) (services.CarePlanWiring, error) {
	root, err := newCarePlanWiring(carePlanWiringOwners{
		DB: db, Logger: logger, CarePlan: carePlan, Persons: owners.Persons, Membership: owners.Membership,
		Timetable: owners.Timetable, Calendar: owners.Calendar, People: people, AuditActor: auditActor,
	})
	return root.wiring, err
}

// PeopleWiring builds the People Directory, School Membership and School
// Structure wiring over the test owners and the given Care Plan, as the
// serving root does (#4007).
func (owners OwnerCapabilities) PeopleWiring(db transactionruntime.Database, carePlan careplan.Capability, logger *slog.Logger) (services.PeopleWiring, error) {
	return NewPeopleWiring(PeopleWiringOwners{
		DB: db, Logger: logger, Persons: owners.Persons, Groups: owners.Groups, Rooms: owners.Rooms,
		Membership: owners.Membership, Workforce: owners.Workforce, CarePlan: carePlan,
		Organizations: owners.Organizations, Timetable: owners.Timetable,
	})
}

// PlatformWiring builds the platform, Communication and Organisation &
// Tenancy wiring over the test owners and the given People Directory wiring,
// as the serving root does (#4013). The parent announcements read the
// graph's clock.
func (owners OwnerCapabilities) PlatformWiring(db transactionruntime.Database, people services.PeopleWiring, clocks ...func() time.Time) (services.PlatformWiring, error) {
	return NewPlatformWiring(PlatformWiringOwners{
		DB: db, Organizations: owners.Organizations, Persons: owners.Persons, Groups: owners.Groups,
		Membership: owners.Membership, People: people, Now: services.OptionalClock(clocks),
	})
}

// EnrollmentWiring binds Enrollment over the test owners, the given Care Plan
// and the People wiring's schools, as the serving root does (#4012).
func (owners OwnerCapabilities) EnrollmentWiring(db transactionruntime.Database, carePlan careplan.Capability, people services.PeopleWiring) services.EnrollmentWiring {
	return newEnrollmentWiring(enrollmentWiringOwners{
		DB: db, CarePlan: carePlan, Persons: owners.Persons, Calendar: owners.Calendar,
		Timetable: owners.Timetable, Schools: people.EnrollmentSchools,
	})
}

// PresenceWiring builds the Student Presence, Facilities, Device Fleet and
// class-day wiring over the test owners, the given Care Plan and People
// wiring, as the serving root does (#4010).
func (owners OwnerCapabilities) PresenceWiring(db transactionruntime.Database, carePlan careplan.Capability, people services.PeopleWiring, logger *slog.Logger, clocks ...func() time.Time) (services.PresenceWiring, error) {
	rows := NewTimetableRepositories(db, TimetableDependencies{
		Capability: owners.Timetable, Students: owners.Persons, Groups: owners.Groups, Rooms: owners.Rooms,
		Calendar: owners.Calendar, Membership: owners.Membership, Workforce: owners.Workforce,
	})
	return NewPresenceWiring(PresenceWiringOwners{
		DB: db, Logger: logger, Persons: owners.Persons, Rooms: owners.Rooms, Membership: owners.Membership,
		CarePlan: carePlan, Calendar: owners.Calendar, Timetable: owners.Timetable, TimetableRows: rows,
		People: people, Now: services.OptionalClock(clocks),
	})
}

// TimetableWiring builds the Timetable, School Calendar and reminder wiring
// over the test owners and the given People wiring, as the serving root does
// (#4009).
func (owners OwnerCapabilities) TimetableWiring(db transactionruntime.Database, people services.PeopleWiring, logger *slog.Logger) (services.TimetableWiring, error) {
	repositories := NewTimetableRepositories(db, TimetableDependencies{
		Capability: owners.Timetable, Students: owners.Persons, Groups: owners.Groups, Rooms: owners.Rooms,
		Calendar: owners.Calendar, Membership: owners.Membership, Workforce: owners.Workforce,
	})
	return NewTimetableWiring(TimetableWiringOwners{
		DB: db, Logger: logger, Repositories: repositories, Timetable: owners.Timetable, Calendar: owners.Calendar,
		Rooms: owners.Rooms, Membership: owners.Membership, Persons: owners.Persons, People: people,
	})
}

func currentTestFactoryConfig() services.FactoryConfig {
	cfg := services.CurrentFactoryConfig()
	cfg.PublicAPIURL = "http://api.test"
	return cfg
}

// caregiverProfilesForTests binds the Lehrkraft guard to real People
// Directory and School Membership compositions over db, as the factory does.
func caregiverProfilesForTests(db transactionruntime.Database) (CaregiverProfiles, error) {
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return CaregiverProfiles{}, err
	}
	membership, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		return CaregiverProfiles{}, err
	}
	return CaregiverProfiles{Persons: persons, Membership: membership}, nil
}

// NewOwnerCapabilitiesForTests composes the owner modules the legacy test
// graph binds into the factory, unobserved.
func NewOwnerCapabilitiesForTests(db transactionruntime.Database) (OwnerCapabilities, error) {
	organizations, err := NewOrganizationTenancy(db)
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
	timetableCapability, err := NewUnobservedTimetable(db, persons, rooms)
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

// WorkforceWiring is the Workforce wiring the root hands the factory, over
// the owners of this test graph.
func (o OwnerCapabilities) WorkforceWiring(db transactionruntime.Database) services.WorkforceWiring {
	staff := NewMembershipRepositories(db, NewPersonRepository(db), NewPeopleIdentityAccess(db, nil), o.Membership, o.Rooms, o.Workforce).Staff
	return newWorkforceWiring(db, o.Workforce, o.Membership, o.Persons, o.Organizations, staff)
}

// factory hands the finished factory's period administration to the
// calendar the graph was composed with.
func (o OwnerCapabilities) factory(factory *services.Factory, err error) (*services.Factory, error) {
	if err != nil {
		return nil, err
	}
	o.bindCalendarAdministration(GuardedCalendarAdministration(factory.Settings, factory.TimetableData.RecurrenceLock, factory.EnrollmentCareOffering))
	return factory, nil
}

// UnboundTenantRuntime is the zero unit of work the test factories compose
// with before a suite binds its own.
func UnboundTenantRuntime() tenant.UnitOfWork { return tenant.UnitOfWork{} }
