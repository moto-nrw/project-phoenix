//go:build testhooks

package api

import (
	"context"
	"log/slog"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type GroupsTestModule struct {
	Education   schoolstructure.GroupManagement
	Active      studentpresence.Presence
	Users       peoplePorts.PersonDirectory
	People      peopleCompose.GroupRoutePeople
	UserContext *CallerRows
}

// TeacherGroupIDs exposes the same assignment projection as attendance composition.
func (m GroupsTestModule) TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error) {
	return NewAttendanceTeacherGroups(m.Education).TeacherGroupIDs(ctx, teacherID)
}

func NewGroupsTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, publishers ...deliveryCompose.RealtimePublisher) (GroupsTestModule, error) {
	members := MustNewMembershipTestRepositories(db)
	r, _, err := NewUserContextTestRepositories(db, members)
	if err != nil {
		return GroupsTestModule{}, err
	}
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return GroupsTestModule{}, err
	}
	identity, err := NewUserContextTestModule(db, unit)
	if err != nil {
		return GroupsTestModule{}, err
	}
	ttRooms, err := NewTimetableRoomRows(db)
	if err != nil {
		return GroupsTestModule{}, err
	}
	ttPresence, err := NewTimetablePresenceRows(db, tt, members)
	if err != nil {
		return GroupsTestModule{}, err
	}
	var publisher deliveryCompose.RealtimePublisher = deliveryCompose.NewRealtimeHub(slog.Default())
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	groups := education.NewGroupManagement(members.Group, members.GroupTeacher, members.ClassTeacher,
		NewEducationRooms(ttRooms.Facilities), NewEducationTeachers(members.Teacher), NewEducationStaff(members.Staff),
		members.Student, r.Substitutions, education.NewLegacyRepositoryRuntime(db.Pool()), education.GroupServiceOptions{Broadcaster: NewGroupEvents(publisher, nil)})

	persons := peopleCompose.NewPersonDirectory(peoplePorts.PersonDirectoryDependencies{
		PersonDirectory:  peopleCompose.NewPersonWrites(MustNewPeopleDirectory(db)),
		StudentDirectory: peopleCompose.NewStudentDirectory(MustNewPeopleDirectory(db)),
		PersonRepo:       members.Person, StudentRepo: members.Student, TeacherRepo: members.Teacher, AccountExists: identityaccessCompose.AccountExists(r.Profile),
		StaffDirectory: peopleCompose.NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: members.Person, Staff: members.Staff, Teachers: members.Teacher, MayManageStaff: StaffAdministrationPermitted}),
	})
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return GroupsTestModule{}, err
	}
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(ttPresence.ActiveGroup)
	presence := presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: devicescanCompose.AttendancePrincipal,
		YardRoomColor:   facilitiesCompose.YardRoomColor(rooms),
		StaffNames:      NewAttendanceStaffNames(members.Staff, persons), GroupRepo: sessionGroups, SupervisorRepo: sessionSupervisors,
		StudentRepo: PresenceStudents(db, members.Student), StaffRepo: NewAttendanceStaffDirectory(members.Staff), RoomRepo: facilitiesLegacy.NewAttendanceRooms(ttRooms.Room),
		EducationGroupRepo: NewAttendanceEducationGroups(members.Group, members.Student),
		ActivityGroupRepo:  NewSessionActivities(tt.GroupReads()), DB: db.Pool(), Logger: slog.Default(),
		SchoolPresence: NewLoggedStudentPresence(db, slog.Default()),
	})

	return GroupsTestModule{Education: groups, Active: presence, Users: persons, People: peopleCompose.NewGroupRoutePeople(persons), UserContext: identity}, nil
}

// NewAttendanceTeacherGroups supplies assignment IDs the way the active route
// composer did before the presence cutover; the behaviour tests still drive
// that seam.
func NewAttendanceTeacherGroups(records education.TeacherGroupRecords) *education.TeacherGroupIDs {
	return education.NewTeacherGroupIDs(records)
}
