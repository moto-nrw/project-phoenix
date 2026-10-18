//go:build testhooks

package api

import (
	"log/slog"
	"time"

	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func newStaffIdentityForTests(db transactionruntime.Database) (*CallerRows, MembershipTestRepositories, error) {
	members, err := NewMembershipTestRepositories(db)
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
	Catalog workforce.Capability
	// Administration is the Abwesenheitsarten administration over Catalog,
	// composed the way the root composes it.
	Administration workforce.AbsenceTypeAdministration
	UserContext    *CallerRows
}

func NewAbsenceTypeTestModule(db transactionruntime.Database, clocks ...func() time.Time) (AbsenceTypeTestModule, error) {
	identity, _, err := newStaffIdentityForTests(db)
	if err != nil {
		return AbsenceTypeTestModule{}, err
	}
	catalog := NewAbsenceTypeTestCapability(db, clocks...)
	return AbsenceTypeTestModule{
		Catalog: catalog, Administration: workforceCompose.NewAbsenceTypeAdministration(catalog, slog.Default()), UserContext: identity,
	}, nil
}

type BirthdayTestModule struct {
	Birthdays   peopledirectory.Birthdays
	UserContext *CallerRows
	Settings    settingsCompose.SettingsService
	ListExport  *lists.RendererService
}

func NewBirthdayTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (BirthdayTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return BirthdayTestModule{}, err
	}
	identity, members, err := newStaffIdentityForTests(db)
	if err != nil {
		return BirthdayTestModule{}, err
	}
	birthdays := NewBirthdays(
		BirthdayRepositories{Students: NewStudentLookupTestRepository(db), Staff: members.Staff, Persons: members.Person},
		settings.Settings, slog.Default(), OptionalClock(clocks))
	return BirthdayTestModule{Birthdays: birthdays, UserContext: identity, Settings: settings.Settings, ListExport: lists.NewRendererService()}, nil
}

// NewBirthdayCapabilityForTests composes the birthday capability over the
// retained repositories the way the service graph does, with caller-supplied
// settings and clock.
func NewBirthdayCapabilityForTests(db transactionruntime.Database, settings BirthdaySettingsSource, now func() time.Time) peopledirectory.Birthdays {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		panic(err)
	}
	return NewBirthdays(
		BirthdayRepositories{Students: NewStudentLookupTestRepository(db), Staff: members.Staff, Persons: members.Person},
		settings, slog.Default(), now)
}

type ShiftTypeTestModule struct {
	ShiftTypes   workforce.ShiftTypeAdministration
	Activities   timetableCompose.ActivityService
	Repositories ShiftTypeTestRepositories
}

// NewShiftTypeTestModule composes the shift-type administration the way the
// factory does: the Workforce planning composition over the timetable test
// repositories, with the Kategorie↔Schichtart linker of the activity service.
func NewShiftTypeTestModule(db transactionruntime.Database) (ShiftTypeTestModule, error) {
	repos := NewShiftTypeTestRepositories(db, MustNewMembershipTestRepositories(db))
	linker, err := timetableCompose.NewService(repos.Timetable, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		return ShiftTypeTestModule{}, err
	}
	timetable, err := NewTimetableTestRepositories(db)
	if err != nil {
		return ShiftTypeTestModule{}, err
	}
	planning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce: NewAbsenceTypeTestCapability(db), Staff: NewPlanStaffDirectory(timetable.Staff), CalendarPeriods: timetable.SchoolCalendar(),
		Instances: timetableCompose.NewTimetableInstanceReads(timetable.ActivityInstance), InstanceStaff: timetableCompose.NewTimetableInstanceStaffReads(timetable.InstanceStaff),
		Rooms: facilitiesCompose.NewRoomNameDirectory(timetable.Room), ActivityGroups: timetableCompose.NewTimetableGroupReads(timetable.ActivityGroup),
		CategoryLinker: timetableCompose.ShiftTypeCategoryLinker(linker.SetCategoryShiftTypeLinks), DB: db.Pool(), Logger: slog.Default(),
	})
	if err != nil {
		return ShiftTypeTestModule{}, err
	}
	return ShiftTypeTestModule{ShiftTypes: planning.ShiftTypes, Activities: linker, Repositories: repos}, nil
}

type DeviceTestModule struct{ IoT devicefleetCompose.IoTService }

func NewDeviceTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (DeviceTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return DeviceTestModule{}, err
	}
	fleet, err := NewDeviceFleet(db, NewOnlineWindowMinutes(settings.Settings, nil))
	if err != nil {
		return DeviceTestModule{}, err
	}
	return DeviceTestModule{IoT: devicefleetCompose.NewIoTService(fleet)}, nil
}
