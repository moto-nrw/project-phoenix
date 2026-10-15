package api

import (
	"context"
	"errors"
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services"
)

// PeopleWiringOwners are the owners the root has composed when it builds the
// People Directory, School Membership and School Structure wiring of the
// legacy service graph (#4007).
type PeopleWiringOwners struct {
	DB            transactionruntime.Database
	Logger        *slog.Logger
	Persons       peopledirectory.Capability
	Groups        schoolstructure.Query
	Rooms         facilitiesModule.Query
	Membership    schoolmembership.Capability
	Workforce     workforce.Capability
	CarePlan      careplan.Capability
	Organizations organizationtenancy.Query
	Timetable     timetable.Capability
	// ObserveIdentityAccess records the Identity & Access operations the
	// retained repositories read through; nil leaves them unobserved.
	ObserveIdentityAccess services.IdentityAccessObserver
}

// NewPeopleWiring builds the retained People Directory, School Membership and
// School Structure repositories over the owners, and the adapters the legacy
// service graph binds them through. The graph takes the result in place of
// building it itself (ADR 0053 point 8).
func NewPeopleWiring(owners PeopleWiringOwners) (services.PeopleWiring, error) {
	if owners.Persons == nil || owners.Groups == nil || owners.Rooms == nil || owners.Membership == nil ||
		owners.Workforce == nil || owners.CarePlan == nil || owners.Organizations == nil || owners.Timetable == nil {
		return services.PeopleWiring{}, errors.New("people wiring: the People Directory, School Structure, Facilities, School Membership, Workforce, Care Plan, Organisation & Tenancy and Timetable owners are required")
	}
	logger := owners.Logger
	if logger == nil {
		logger = slog.Default()
	}
	db, persons, groups := owners.DB, owners.Persons, owners.Groups
	identity := NewPeopleIdentityAccess(db, owners.ObserveIdentityAccess)
	personRepository := NewPersonRepository(db)
	students := NewStudentRepository(db, owners.Membership, groups)
	members := newMembershipAdapters(db, personRepository, identity, owners.Membership, owners.Rooms, owners.Workforce)
	presence := services.NewLoggedStudentPresence(db.Pool(), logger)
	return services.PeopleWiring{
		Identity:             identity,
		Persons:              personRepository,
		Students:             students,
		StudentGuardians:     newStudentGuardianRepository(db, identity, owners.ObserveIdentityAccess),
		GuardianProfiles:     NewGuardianProfileRepository(db),
		Staff:                members.Staff,
		Teachers:             members.Teacher,
		Groups:               members.Group,
		GroupTeachers:        members.GroupTeacher,
		ClassTeachers:        members.ClassTeacher,
		CaregiverBindingLock: NewCaregiverBindingLocker(owners.Membership, owners.Workforce, presence, owners.Timetable),

		GroupRooms:            NewEducationRooms(owners.Rooms),
		GroupTeacherDirectory: NewEducationTeachers(members.Teacher),
		StaffChecks:           NewEducationStaff(members.Staff),
		SubstitutionPersons:   NewEducationPersonQuery(persons),
		Caregivers:            NewEducationCaregivers(members.Teacher),
		ExternalCaregivers:    NewEducationExternalCaregivers(members.Staff, members.Guest),

		StudentDisplay:      StudentDisplayProjection{Students: persons, Groups: groups},
		CrossTenantStudents: NewCrossTenantQuery(presence, persons, groups),
		PresenceSchools:     NewActiveSchoolQuery(owners.Organizations),
		PresenceStudents:    PresenceStudents(db, students),

		SchoolContacts:    SchoolContactDirectory{Schools: owners.Organizations},
		EnrollmentSchools: EnrollmentSchoolDirectory{Schools: owners.Organizations},
		DisplaySchools:    displaySchoolDirectory{schools: owners.Organizations},
		SchoolNames:       schoolNameDirectory{schools: owners.Organizations},

		StudentAudit: NewStudentAuditFor(persons),

		PickupSchedules:   NewPickupScheduleRepository(owners.CarePlan),
		ArrivalSchedules:  NewArrivalScheduleRepository(owners.CarePlan),
		PickupExceptions:  NewPickupExceptionRepository(owners.CarePlan),
		ArrivalExceptions: NewArrivalExceptionRepository(owners.CarePlan),

		SupervisionRecords: func(records studentpresence.SupervisionRecords) studentpresence.SupervisionRecords {
			return NewGroupSupervisionRecords(records, groups)
		},
		SupervisionBlockers: func(records studentpresence.SupervisionRecords) peopleCompose.SupervisionBlockers {
			return CaregiverSupervisionBlockers{Records: records}
		},
		CaregiverIdentity: func(module *identityaccess.Module) peopleCompose.CaregiverIdentity {
			return caregiverIdentity{module}
		},
		CaregiverCapabilityAudit: newCaregiverCapabilityAudit,
		GuardianFinancialAudit:   NewGuardianFinancialAudit,
		GuardianDataAccessLog:    NewGuardianDataAccessLog,
		ClassAssignmentAudit: func(changes auditModels.StaffMasterDataChangeCreator) education.ClassAssignmentAudit {
			return NewEducationClassAssignmentAudit(changes)
		},
		SubstitutionAudit: func(changes auditModels.SubstitutionChangeCreator) education.SubstitutionAudit {
			return NewEducationSubstitutionAudit(changes)
		},
		StudentConsents: func(trail services.StudentConsentAppender) services.StudentConsentRecords {
			return NewStudentConsentsFor(persons, trail)
		},
		StudentChangeLogCleanup: NewStudentChangeLogCleanup,
		StatusDayPeople: func(directory *peopleCompose.PersonDirectory) presenceservice.StatusDayOverviewPeople {
			return StatusDayOverviewPeople(directory)
		},
		GroupRosterPeople: func(directory *peopleCompose.PersonDirectory) services.GroupRoster {
			return GroupRosterPeople(directory)
		},
		MasterDataDecisions:      NewMasterDataDecisions,
		ParentRequestCoordinator: NewParentRequestCoordinator,
		ParentRequestWriteScope: func(policy services.ParentRequestScopePolicy, permissions func(context.Context) []string) careplanCompose.ReviewScopeResolver {
			return ParentRequestWriteScope(policy, permissions)
		},
	}, nil
}
