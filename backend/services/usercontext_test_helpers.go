package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type UserContextTestModule struct {
	UserContext usercontext.UserContextService
}

func NewUserContextTestModule(db *bun.DB, unit tenant.UnitOfWork) (UserContextTestModule, error) {
	r, err := repositories.NewUserContextTestRepositories(db)
	if err != nil {
		return UserContextTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return UserContextTestModule{}, err
	}
	tt := r.Timetable
	activeQueries := active.NewService(active.ServiceDependencies{
		GroupRepo: tt.ActiveGroup, SupervisorRepo: tt.GroupSupervisor, RoomRepo: tt.Room,
		ActivityGroupRepo: tt.ActivityGroup, StudentRepo: tt.Student, StaffRepo: tt.Staff,
		PersonRepo: tt.Person, DB: db, Logger: slog.Default(),
	})
	service := usercontext.NewUserContextServiceWithRepos(usercontext.UserContextRepositories{
		AccountRepo: r.Account, PersonRepo: tt.Person, StaffRepo: tt.Staff, TeacherRepo: tt.Teacher,
		StudentRepo: tt.Student, EducationGroupRepo: tt.Group, ActivityGroupRepo: tt.ActivityGroup,
		ActiveGroupRepo: tt.ActiveGroup, VisitsRepo: tt.ActiveVisit, SupervisorRepo: tt.GroupSupervisor,
		ProfileRepo: r.Profile, SubstitutionRepo: r.Substitutions, ClassTeacherRepo: tt.ClassTeacher,
		ActiveService: activeQueries, SSESettings: settings.Settings,
	}, slog.Default())
	return UserContextTestModule{UserContext: service}, nil
}
