package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func newStaffIdentityForTests(db *bun.DB) (*repositories.CallerRows, repositories.MembershipTestRepositories, error) {
	members, err := repositories.NewMembershipTestRepositories(db)
	if err != nil {
		return nil, members, err
	}
	identity, err := newCallerRowsForTests(db, nil)
	if err != nil {
		return nil, members, err
	}
	return identity, members, nil
}

type AbsenceTypeTestModule struct {
	Catalog     workforce.Capability
	UserContext *repositories.CallerRows
}

func NewAbsenceTypeTestModule(db *bun.DB) (AbsenceTypeTestModule, error) {
	identity, _, err := newStaffIdentityForTests(db)
	if err != nil {
		return AbsenceTypeTestModule{}, err
	}
	return AbsenceTypeTestModule{Catalog: repositories.NewAbsenceTypeTestCapability(db), UserContext: identity}, nil
}

type BirthdayTestModule struct {
	Birthdays   peopledirectory.Birthdays
	UserContext *repositories.CallerRows
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
	birthdays := NewBirthdays(
		BirthdayRepositories{Students: repositories.NewStudentLookupTestRepository(db), Staff: members.Staff, Persons: members.Person},
		settings.Settings, slog.Default(), optionalClock(clocks))
	return BirthdayTestModule{Birthdays: birthdays, UserContext: identity, Settings: settings.Settings, ListExport: listexport.NewService()}, nil
}

// NewBirthdayCapabilityForTests composes the birthday capability over the
// retained repositories the way the factory does, with caller-supplied
// settings and clock.
func NewBirthdayCapabilityForTests(db *bun.DB, settings BirthdaySettingsSource, now func() time.Time) peopledirectory.Birthdays {
	members, err := repositories.NewMembershipTestRepositories(db)
	if err != nil {
		panic(err)
	}
	return NewBirthdays(
		BirthdayRepositories{Students: repositories.NewStudentLookupTestRepository(db), Staff: members.Staff, Persons: members.Person},
		settings, slog.Default(), now)
}

type ShiftTypeTestModule struct {
	ShiftTypes   workforce.ShiftTypeAdministration
	Activities   activities.ActivityService
	Repositories repositories.ShiftTypeTestRepositories
}

// NewShiftTypeTestModule composes the shift-type administration the way the
// factory does: the Workforce planning composition over the timetable test
// repositories, with the Kategorie↔Schichtart linker of the activity service.
func NewShiftTypeTestModule(db *bun.DB) (ShiftTypeTestModule, error) {
	repos := repositories.NewShiftTypeTestRepositories(db)
	linker, err := activities.NewService(repos.Timetable, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		return ShiftTypeTestModule{}, err
	}
	timetable, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return ShiftTypeTestModule{}, err
	}
	planning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce: repositories.NewAbsenceTypeTestCapability(db), Staff: timetable.Staff, CalendarPeriods: timetable.SchoolCalendar(),
		Instances: repositories.NewTimetableInstanceReads(timetable.ActivityInstance), InstanceStaff: repositories.NewTimetableInstanceStaffReads(timetable.InstanceStaff),
		Rooms: timetable.Room, ActivityGroups: repositories.NewTimetableGroupReads(timetable.ActivityGroup),
		CategoryLinker: repositories.ShiftTypeCategoryLinker(linker.SetCategoryShiftTypeLinks), DB: db, Logger: slog.Default(),
	})
	if err != nil {
		return ShiftTypeTestModule{}, err
	}
	return ShiftTypeTestModule{ShiftTypes: planning.ShiftTypes, Activities: linker, Repositories: repos}, nil
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
