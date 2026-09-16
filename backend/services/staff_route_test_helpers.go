package services

import (
	"log/slog"
	"time"

	shiftplanning "github.com/moto-nrw/project-phoenix/modules/workforce/legacy/shiftplanning"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/usercontext"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func newStaffIdentityForTests(db *bun.DB) (usercontext.UserContextService, repositories.MembershipTestRepositories, error) {
	members, err := repositories.NewMembershipTestRepositories(db)
	if err != nil {
		return nil, members, err
	}
	identity := usercontext.NewUserContextServiceWithRepos(usercontext.UserContextRepositories{
		AccountRepo: members.Account, PersonRepo: members.Person, StaffRepo: members.Staff, TeacherRepo: members.Teacher,
	}, slog.Default())
	return identity, members, nil
}

type AbsenceTypeTestModule struct {
	Catalog     workforce.Capability
	UserContext usercontext.UserContextService
}

func NewAbsenceTypeTestModule(db *bun.DB) (AbsenceTypeTestModule, error) {
	identity, _, err := newStaffIdentityForTests(db)
	if err != nil {
		return AbsenceTypeTestModule{}, err
	}
	return AbsenceTypeTestModule{Catalog: repositories.NewAbsenceTypeTestCapability(db), UserContext: identity}, nil
}

type BirthdayTestModule struct {
	Birthdays   users.BirthdayService
	UserContext usercontext.UserContextService
	Settings    config.SettingsService
	ListExport  *listexport.RendererService
}

func NewBirthdayTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (BirthdayTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return BirthdayTestModule{}, err
	}
	identity, members, err := newStaffIdentityForTests(db)
	if err != nil {
		return BirthdayTestModule{}, err
	}
	birthdays := users.NewBirthdayService(users.BirthdayServiceDependencies{
		StudentRepo: repositories.NewStudentLookupTestRepository(db), StaffRepo: members.Staff, PersonRepo: members.Person,
		SettingsService: settings.Settings, Logger: slog.Default(), Now: optionalClock(clocks),
	})
	return BirthdayTestModule{Birthdays: birthdays, UserContext: identity, Settings: settings.Settings, ListExport: listexport.NewService()}, nil
}

type ShiftTypeTestModule struct {
	ShiftTypes   shiftplanning.ShiftTypeService
	Activities   activities.ActivityService
	Repositories repositories.ShiftTypeTestRepositories
}

func NewShiftTypeTestModule(db *bun.DB) (ShiftTypeTestModule, error) {
	repos := repositories.NewShiftTypeTestRepositories(db)
	linker, err := activities.NewService(repos.Timetable, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		return ShiftTypeTestModule{}, err
	}
	return ShiftTypeTestModule{
		ShiftTypes: shiftplanning.NewShiftTypeService(repos.Types, slog.Default()), Activities: linker, Repositories: repos,
	}, nil
}

type DeviceTestModule struct{ IoT iot.Service }

func NewDeviceTestModule(db *bun.DB, unit tenant.UnitOfWork) (DeviceTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return DeviceTestModule{}, err
	}
	fleet, err := repositories.NewDeviceFleet(db, devicefleetLegacy.NewOnlineWindowResolver(settings.Settings, nil))
	if err != nil {
		return DeviceTestModule{}, err
	}
	return DeviceTestModule{IoT: iot.NewService(fleet)}, nil
}
