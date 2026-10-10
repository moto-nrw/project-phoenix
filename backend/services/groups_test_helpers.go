package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type GroupsTestModule struct {
	Education   schoolstructure.GroupManagement
	Active      studentpresence.Presence
	Users       *peopleCompose.PersonDirectory
	People      peopleCompose.GroupRoutePeople
	UserContext *repositories.CallerRows
}

// TeacherGroupIDs exposes the same assignment projection as attendance composition.
func (m GroupsTestModule) TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error) {
	return NewAttendanceTeacherGroups(m.Education).TeacherGroupIDs(ctx, teacherID)
}

func NewGroupsTestModule(db *bun.DB, unit tenant.UnitOfWork, publishers ...realtimeevents.Publisher) (GroupsTestModule, error) {
	r, err := repositories.NewUserContextTestRepositories(db)
	if err != nil {
		return GroupsTestModule{}, err
	}
	identity, err := NewUserContextTestModule(db, unit)
	if err != nil {
		return GroupsTestModule{}, err
	}
	tt := r.Timetable
	var publisher realtimeevents.Publisher = deliveryCompose.NewRealtimeHub(slog.Default())
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	groups := education.NewGroupManagement(tt.Group, tt.GroupTeacher, tt.ClassTeacher,
		repositories.NewEducationRooms(tt.Room), NewEducationTeachers(tt.Teacher), repositories.NewEducationStaff(tt.Staff),
		tt.Student, r.Substitutions, education.NewLegacyRepositoryRuntime(db), education.GroupServiceOptions{Broadcaster: publisher})

	persons := peopleCompose.NewPersonDirectory(peopleCompose.PersonDirectoryDependencies{
		PersonDirectory:  repositories.NewPersonDirectory(repositories.MustNewPeopleDirectory(db)),
		StudentDirectory: repositories.NewStudentDirectory(repositories.MustNewPeopleDirectory(db)),
		PersonRepo:       tt.Person, StudentRepo: tt.Student, TeacherRepo: tt.Teacher, AccountExists: repositories.AccountExists(r.Profile),
		StaffDirectory: NewStaffDirectory(StaffDirectoryDependencies{DB: db, Persons: tt.Person, Staff: tt.Staff, Teachers: tt.Teacher}),
	})
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return GroupsTestModule{}, err
	}
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(tt.ActiveGroup)
	presence := presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: AttendancePrincipal,
		YardRoomColor:   yardRoomColorQuery(rooms),
		StaffNames:      NewAttendanceStaffNames(tt.Staff, persons), GroupRepo: sessionGroups, SupervisorRepo: sessionSupervisors,
		StudentRepo: PresenceStudents(db, tt.Student), StaffRepo: NewAttendanceStaffDirectory(tt.Staff), RoomRepo: NewAttendanceRooms(tt.Room),
		EducationGroupRepo: NewAttendanceEducationGroups(tt.Group, tt.Student),
		ActivityGroupRepo:  repositories.NewSessionActivities(tt.ActivityGroup), DB: db, Logger: slog.Default(),
		SchoolPresence: newStudentPresence(db, slog.Default()),
	})

	return GroupsTestModule{Education: groups, Active: presence, Users: persons, People: peopleCompose.NewGroupRoutePeople(persons), UserContext: identity.UserContext}, nil
}

// NewAttendanceTeacherGroups supplies assignment IDs the way the active route
// composer did before the presence cutover; the behaviour tests still drive
// that seam.
func NewAttendanceTeacherGroups(records education.TeacherGroupRecords) *education.TeacherGroupIDs {
	return education.NewTeacherGroupIDs(records)
}
