package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/usercontext"
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
	service := usercontext.NewUserContextServiceWithRepos(usercontext.UserContextRepositories{
		AccountRepo: repositories.NewCurrentAccountAccess(r.Profile), PersonRepo: tt.Person, StaffRepo: tt.Staff, TeacherRepo: tt.Teacher,
		StudentRepo: tt.Student, EducationGroupRepo: tt.Group, ActivityGroupRepo: tt.ActivityGroup,
		ActiveGroupRepo: tt.ActiveGroup, Presence: newStudentPresence(db, slog.Default()), SupervisorRepo: tt.GroupSupervisor,
		ProfileRepo: r.Profile, StaffGroups: r.StaffGroups,
		ActiveService: NewSSEPresence(newStudentPresence(db, slog.Default())), SSESettings: settings.Settings,
	}, slog.Default())
	return UserContextTestModule{UserContext: service}, nil
}
