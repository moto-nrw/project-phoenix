package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/usercontext"
	schoolStructure "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type GroupsTestModule struct {
	Education   education.Service
	Active      active.Service
	Users       users.PersonService
	UserContext usercontext.UserContextService
}

// TeacherGroupIDs exposes the same assignment projection as attendance composition.
func (m GroupsTestModule) TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error) {
	return NewAttendanceTeacherGroups(m.Education).TeacherGroupIDs(ctx, teacherID)
}

func NewGroupsTestModule(db *bun.DB, unit tenant.UnitOfWork) (GroupsTestModule, error) {
	r, err := repositories.NewUserContextTestRepositories(db)
	if err != nil {
		return GroupsTestModule{}, err
	}
	identity, err := NewUserContextTestModule(db, unit)
	if err != nil {
		return GroupsTestModule{}, err
	}
	tt := r.Timetable
	groups := education.NewService(tt.Group, tt.GroupTeacher, tt.ClassTeacher, tt.Room, tt.Teacher, tt.Staff, tt.Student, r.Substitutions, db)
	groups.(interface{ SetBroadcaster(realtime.Broadcaster) }).SetBroadcaster(deliveryCompose.NewRealtimeHub(slog.Default()))
	persons := users.NewPersonService(users.PersonServiceDependencies{
		PersonDirectory:  repositories.NewPersonDirectory(repositories.MustNewPeopleDirectory(db)),
		StudentDirectory: repositories.NewStudentDirectory(repositories.MustNewPeopleDirectory(db)),
		PersonRepo:       tt.Person, StudentRepo: tt.Student, StaffRepo: tt.Staff, TeacherRepo: tt.Teacher, AccountRepo: r.Account, DB: db, Logger: slog.Default(),
	})
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return GroupsTestModule{}, err
	}
	presence := active.NewService(active.ServiceDependencies{
		PrincipalReader: AttendancePrincipal,
		YardRoomColor:   yardRoomColorQuery(rooms),
		StaffNames:      NewAttendanceStaffNames(tt.Staff, persons), GroupRepo: tt.ActiveGroup, SupervisorRepo: tt.GroupSupervisor,
		StudentRepo: PresenceStudents(tt.Student), StaffRepo: NewAttendanceStaffDirectory(tt.Staff), RoomRepo: NewAttendanceRooms(tt.Room),
		EducationGroupRepo: NewAttendanceEducationGroups(tt.Group, tt.Student),
		ActivityGroupRepo:  repositories.NewSessionActivities(tt.ActivityGroup), DB: db, Logger: slog.Default(),
		SchoolPresence: newStudentPresence(db, slog.Default()),
	})

	return GroupsTestModule{Education: groups, Active: presence, Users: persons, UserContext: identity.UserContext}, nil
}

// NewAttendanceTeacherGroups supplies assignment IDs the way the active route
// composer did before the presence cutover; the behaviour tests still drive
// that seam.
func NewAttendanceTeacherGroups(records schoolStructure.TeacherGroupRecords) *schoolStructure.TeacherGroupIDs {
	return schoolStructure.NewTeacherGroupIDs(records)
}
