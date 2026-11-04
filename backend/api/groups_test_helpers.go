//go:build testhooks

package api

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/realtime"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// GroupsTestModule is the School Structure side of the group routes; the
// presence, the people and the caller rows come beside it from
// NewGroupsTestModule (#4080).
type GroupsTestModule struct {
	Education schoolstructure.GroupManagement
}

func NewGroupsTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, publishers ...deliveryCompose.RealtimePublisher) (GroupsTestModule, ActivePresence, ActivePeople, *owners.CallerRows, error) {
	members := owners.MustNewMembershipTestRepositories(db)
	r, _, err := NewUserContextTestRepositories(db, members)
	if err != nil {
		return GroupsTestModule{}, ActivePresence{}, ActivePeople{}, nil, err
	}
	tt, err := owners.NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return GroupsTestModule{}, ActivePresence{}, ActivePeople{}, nil, err
	}
	identity, err := NewUserContextTestModule(db, unit)
	if err != nil {
		return GroupsTestModule{}, ActivePresence{}, ActivePeople{}, nil, err
	}
	ttRooms, err := owners.NewTimetableRoomRows(db)
	if err != nil {
		return GroupsTestModule{}, ActivePresence{}, ActivePeople{}, nil, err
	}
	ttPresence, err := NewTimetablePresenceRows(db, tt, members)
	if err != nil {
		return GroupsTestModule{}, ActivePresence{}, ActivePeople{}, nil, err
	}
	var publisher deliveryCompose.RealtimePublisher = realtime.NewHub(slog.Default())
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	structure := owners.NewSchoolStructureSuiteRepositories(db)
	groups := education.NewGroupManagement(structure.Group, structure.GroupTeacher, structure.ClassTeacher,
		owners.NewEducationRooms(ttRooms.Facilities), owners.NewEducationTeachers(members.Teacher), owners.NewEducationStaff(members.Staff),
		members.Student, r.Substitutions, education.NewLegacyRepositoryRuntime(db.Pool()), education.GroupServiceOptions{Broadcaster: NewGroupEvents(publisher, nil)})

	persons := peopleCompose.NewPersonDirectory(peoplePorts.PersonDirectoryDependencies{
		PersonDirectory:  owners.MustNewPeopleDirectory(db).PersonWrites,
		StudentDirectory: owners.MustNewPeopleDirectory(db).StudentDirectory,
		PersonRepo:       members.Person, StudentRepo: members.Student, TeacherRepo: members.Teacher, AccountExists: identityaccessCompose.AccountExists(r.Profile),
		StaffDirectory: peopleCompose.NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: members.Person, Staff: members.Staff, Teachers: members.Teacher, MayManageStaff: StaffAdministrationPermitted}),
	})
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return GroupsTestModule{}, ActivePresence{}, ActivePeople{}, nil, err
	}
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(ttPresence.ActiveGroup)
	presence := presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: AttendancePrincipal,
		YardRoomColor:   facilitiesModule.YardRoomColor(rooms),
		StaffNames:      owners.NewAttendanceStaffNames(members.Staff), GroupRepo: sessionGroups, SupervisorRepo: sessionSupervisors,
		StudentRepo: owners.PresenceStudents(db, members.Student), StaffRepo: owners.NewAttendanceStaffDirectory(members.Staff), RoomRepo: facilitiesCompose.NewAttendanceRooms(ttRooms.Room),
		EducationGroupRepo: owners.NewAttendanceEducationGroups(owners.NewSchoolStructureSuiteRepositories(db).Group, members.Student),
		ActivityGroupRepo:  owners.NewSessionActivities(tt.GroupReads()), DB: db.Pool(), Logger: slog.Default(),
		SchoolPresence: owners.NewLoggedStudentPresence(db, slog.Default()),
	})

	return GroupsTestModule{Education: groups}, ActivePresence{Active: presence}, ActivePeople{Users: persons, People: peopleCompose.NewGroupRoutePeople(persons)}, identity, nil
}

// NewAttendanceTeacherGroups supplies assignment IDs the way the active route
// composer did before the presence cutover; the behaviour tests still drive
// that seam.
func NewAttendanceTeacherGroups(records education.TeacherGroupRecords) *education.TeacherGroupIDs {
	return education.NewTeacherGroupIDs(records)
}
