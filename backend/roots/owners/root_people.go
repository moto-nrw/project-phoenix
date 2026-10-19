package owners

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// PeopleRows are the retained People Directory rows the roots read and
// write through (#2743, #4007): the person, student, guardian, staff and
// teacher rows. They need only the database and School Membership, so the
// guardian service the People Directory module serves is built over them
// before the module.
type PeopleRows struct {
	peopleCompose.Repositories
}

// StructureRows are School Structure's group rows and directories over the
// People Directory, School Membership and Facilities (#4007).
type StructureRows struct {
	Groups                EducationGroupRows
	GroupTeachers         GroupTeacherRepository
	ClassTeachers         ClassTeacherRepository
	GroupRooms            education.RoomDirectory
	GroupTeacherDirectory education.TeacherDirectory
	StaffChecks           interface {
		education.StaffDirectory
		education.StaffLockStore
	}
	SubstitutionPersons education.PersonQuery
	Caregivers          education.CaregiverDirectory
	ExternalCaregivers  education.ExternalCaregiverDirectory
}

// PresencePeopleReads are Student Presence's reads of the students, the
// visitors and the schools.
type PresencePeopleReads struct {
	StudentDisplay      presenceservice.StudentDisplayReader
	CrossTenantStudents presenceservice.CrossTenantRepo
	PresenceSchools     presenceservice.SchoolQuery
	PresenceStudents    presenceservice.PresenceStudents
}

// TimetableCareRows are Care Plan's schedule rows in Timetable's retained
// vocabulary.
type TimetableCareRows struct {
	PickupSchedules   timetablePorts.CarePickupScheduleRows
	ArrivalSchedules  timetablePorts.CareArrivalScheduleRows
	PickupExceptions  timetablePorts.CarePickupExceptionRows
	ArrivalExceptions timetablePorts.CareArrivalExceptionRows
}

// buildPeopleRows builds the retained People Directory and School
// Membership rows over the owners and the Identity & Access reads the rows
// filter through (#4007).
func BuildPeopleRows(
	db transactionruntime.Database, observeIdentity IdentityAccessObserver, groups schoolstructure.Query, rooms facilitiesModule.Query,
	membership schoolmembership.Capability, workTime workforce.Capability,
) (PeopleRows, *identityaccess.Module) {
	identity := NewPeopleIdentityAccess(db, observeIdentity)
	personRepository := NewPersonRepository(db)
	staff, teachers, _, _, _, _ := newMembershipAdapters(db, personRepository, identity, membership, rooms, workTime)
	return PeopleRows{Repositories: peopleCompose.Repositories{
		Persons: personRepository, Students: NewStudentRepository(db, membership, groups),
		StudentGuardians: newStudentGuardianRepository(db, identity, observeIdentity),
		GuardianProfiles: NewGuardianProfileRepository(db), Staff: staff, Teachers: teachers,
	}}, identity
}

// buildStructureRows builds School Structure's group rows and directories
// over the People Directory, School Membership and Facilities.
func BuildStructureRows(
	db transactionruntime.Database, persons peopledirectory.Capability, people PeopleRows, peopleIdentity *identityaccess.Module,
	rooms facilitiesModule.Query, membership schoolmembership.Capability, workTime workforce.Capability,
) StructureRows {
	staff, teachers, guests, groups, groupTeachers, classTeachers := newMembershipAdapters(db, people.Persons, peopleIdentity, membership, rooms, workTime)
	return StructureRows{
		Groups: groups, GroupTeachers: groupTeachers, ClassTeachers: classTeachers,
		GroupRooms:            NewEducationRooms(rooms),
		GroupTeacherDirectory: NewEducationTeachers(teachers),
		StaffChecks:           NewEducationStaff(staff),
		SubstitutionPersons:   NewEducationPersonQuery(persons),
		Caregivers:            NewEducationCaregivers(teachers),
		ExternalCaregivers:    NewEducationExternalCaregivers(staff, guests),
	}
}

// buildPresencePeopleReads binds Student Presence's student, visitor and
// school reads to the People Directory, School Structure and Organisation &
// Tenancy.
func BuildPresencePeopleReads(db transactionruntime.Database, logger *slog.Logger, persons peopledirectory.Capability, groups schoolstructure.Query, organizations organizationtenancy.Query, people PeopleRows) PresencePeopleReads {
	presence := NewLoggedStudentPresence(db, logger)
	return PresencePeopleReads{
		StudentDisplay:      StudentDisplayProjection{Students: persons, Groups: groups},
		CrossTenantStudents: NewCrossTenantQuery(presence, persons, groups),
		PresenceSchools:     NewActiveSchoolQuery(organizations),
		PresenceStudents:    PresenceStudents(db, people.Students),
	}
}

// buildTimetableCareRows serves Care Plan's schedule rows in Timetable's
// retained vocabulary.
func BuildTimetableCareRows(carePlan careplan.Capability) TimetableCareRows {
	return TimetableCareRows{
		PickupSchedules:   NewPickupScheduleRepository(carePlan),
		ArrivalSchedules:  NewArrivalScheduleRepository(carePlan),
		PickupExceptions:  NewPickupExceptionRepository(carePlan),
		ArrivalExceptions: NewArrivalExceptionRepository(carePlan),
	}
}
