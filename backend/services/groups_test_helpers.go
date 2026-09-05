package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
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
		PersonRepo: tt.Person, StudentRepo: tt.Student, StaffRepo: tt.Staff, TeacherRepo: tt.Teacher, AccountRepo: r.Account, DB: db, Logger: slog.Default(),
	})
	presence := active.NewService(active.ServiceDependencies{
		UsersService: persons, GroupRepo: tt.ActiveGroup, VisitRepo: tt.ActiveVisit, SupervisorRepo: tt.GroupSupervisor,
		StudentRepo: tt.Student, PersonRepo: tt.Person, StaffRepo: tt.Staff, RoomRepo: tt.Room,
		ActivityGroupRepo: tt.ActivityGroup, AttendanceRepo: repositories.NewAttendanceTestRepository(db), DB: db, Logger: slog.Default(),
	})

	return GroupsTestModule{Education: groups, Active: presence, Users: persons, UserContext: identity.UserContext}, nil
}
