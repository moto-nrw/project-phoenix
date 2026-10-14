package api

import (
	"context"
	"log/slog"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type GroupsTestModule struct {
	Education   schoolstructure.GroupManagement
	Active      studentpresence.Presence
	Users       *peopleCompose.PersonDirectory
	People      peopleCompose.GroupRoutePeople
	UserContext *services.CallerRows
}

// TeacherGroupIDs exposes the same assignment projection as attendance composition.
func (m GroupsTestModule) TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error) {
	return NewAttendanceTeacherGroups(m.Education).TeacherGroupIDs(ctx, teacherID)
}

func NewGroupsTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, publishers ...deliveryCompose.RealtimePublisher) (GroupsTestModule, error) {
	r, err := NewUserContextTestRepositories(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return GroupsTestModule{}, err
	}
	identity, err := NewUserContextTestModule(db, unit)
	if err != nil {
		return GroupsTestModule{}, err
	}
	tt := r.Timetable
	var publisher deliveryCompose.RealtimePublisher = deliveryCompose.NewRealtimeHub(slog.Default())
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	groups := education.NewGroupManagement(tt.Group, tt.GroupTeacher, tt.ClassTeacher,
		NewEducationRooms(tt.Facilities), NewEducationTeachers(tt.Teacher), NewEducationStaff(tt.Staff),
		tt.Student, r.Substitutions, education.NewLegacyRepositoryRuntime(db.Pool()), education.GroupServiceOptions{Broadcaster: publisher})

	persons := peopleCompose.NewPersonDirectory(peopleCompose.PersonDirectoryDependencies{
		PersonDirectory:  peopleCompose.NewPersonWrites(MustNewPeopleDirectory(db)),
		StudentDirectory: peopleCompose.NewStudentDirectory(MustNewPeopleDirectory(db)),
		PersonRepo:       tt.Person, StudentRepo: tt.Student, TeacherRepo: tt.Teacher, AccountExists: services.AccountExists(r.Profile),
		StaffDirectory: peopleCompose.NewStaffDirectory(peopleCompose.StaffDirectoryDependencies{Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: tt.Person, Staff: tt.Staff, Teachers: tt.Teacher, MayManageStaff: StaffAdministrationPermitted}),
	})
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return GroupsTestModule{}, err
	}
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(tt.ActiveGroup)
	presence := presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: services.AttendancePrincipal,
		YardRoomColor:   services.YardRoomColorQuery(rooms),
		StaffNames:      services.NewAttendanceStaffNames(tt.Staff, persons), GroupRepo: sessionGroups, SupervisorRepo: sessionSupervisors,
		StudentRepo: PresenceStudents(db, tt.Student), StaffRepo: services.NewAttendanceStaffDirectory(tt.Staff), RoomRepo: services.NewAttendanceRooms(tt.Room),
		EducationGroupRepo: services.NewAttendanceEducationGroups(tt.Group, tt.Student),
		ActivityGroupRepo:  services.NewSessionActivities(tt.GroupReads()), DB: db.Pool(), Logger: slog.Default(),
		SchoolPresence: services.NewLoggedStudentPresence(db.Pool(), slog.Default()),
	})

	return GroupsTestModule{Education: groups, Active: presence, Users: persons, People: peopleCompose.NewGroupRoutePeople(persons), UserContext: identity.UserContext}, nil
}

// NewAttendanceTeacherGroups supplies assignment IDs the way the active route
// composer did before the presence cutover; the behaviour tests still drive
// that seam.
func NewAttendanceTeacherGroups(records education.TeacherGroupRecords) *education.TeacherGroupIDs {
	return education.NewTeacherGroupIDs(records)
}
