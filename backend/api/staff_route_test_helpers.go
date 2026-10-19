//go:build testhooks

package api

import (
	"log/slog"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func newStaffIdentityForTests(db transactionruntime.Database) (*owners.CallerRows, owners.MembershipTestRepositories, error) {
	members, err := owners.NewMembershipTestRepositories(db)
	if err != nil {
		return nil, members, err
	}
	identity, err := newCallerRowsForTests(db, nil)
	if err != nil {
		return nil, members, err
	}
	return identity, members, nil
}

// AbsenceTypeTestModule is the Workforce owner's Abwesenheitsarten catalog.
type AbsenceTypeTestModule struct {
	Catalog workforce.Capability
	// Administration is the Abwesenheitsarten administration over Catalog,
	// composed the way the root composes it.
	Administration workforce.AbsenceTypeAdministration
}

// NewAbsenceTypeTestModule composes the Abwesenheitsarten catalog over db and
// returns it with the caller context of its routes.
func NewAbsenceTypeTestModule(db transactionruntime.Database, clocks ...func() time.Time) (AbsenceTypeTestModule, *owners.CallerRows, error) {
	identity, _, err := newStaffIdentityForTests(db)
	if err != nil {
		return AbsenceTypeTestModule{}, nil, err
	}
	catalog := NewAbsenceTypeTestCapability(db, clocks...)
	return AbsenceTypeTestModule{
		Catalog: catalog, Administration: workforceCompose.NewAbsenceTypeAdministration(catalog, slog.Default()),
	}, identity, nil
}

// BirthdayTestModule is the People Directory's birthday capability.
type BirthdayTestModule struct {
	Birthdays peopledirectory.Birthdays
}

// NewBirthdayTestModule composes the birthday capability over db and returns
// it with the caller context of its routes and the tenant settings it reads.
func NewBirthdayTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (BirthdayTestModule, *owners.CallerRows, owners.SettingsTestModule, error) {
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return BirthdayTestModule{}, nil, owners.SettingsTestModule{}, err
	}
	identity, members, err := newStaffIdentityForTests(db)
	if err != nil {
		return BirthdayTestModule{}, nil, owners.SettingsTestModule{}, err
	}
	birthdays := NewBirthdays(
		BirthdayRepositories{Students: NewStudentLookupTestRepository(db), Staff: members.Staff, Persons: members.Person},
		settings.Settings, slog.Default(), OptionalClock(clocks))
	return BirthdayTestModule{Birthdays: birthdays}, identity, settings, nil
}

// NewBirthdayCapabilityForTests composes the birthday capability over the
// retained repositories the way the service graph does, with caller-supplied
// settings and clock.
func NewBirthdayCapabilityForTests(db transactionruntime.Database, settings BirthdaySettingsSource, now func() time.Time) peopledirectory.Birthdays {
	members, err := owners.NewMembershipTestRepositories(db)
	if err != nil {
		panic(err)
	}
	return NewBirthdays(
		BirthdayRepositories{Students: NewStudentLookupTestRepository(db), Staff: members.Staff, Persons: members.Person},
		settings, slog.Default(), now)
}

// ShiftTypeTestModule is the Timetable side of the shift-type
// administration: the activity service whose Kategorie↔Schichtart linker it
// calls and the timetable rows it runs over.
type ShiftTypeTestModule struct {
	Repositories ShiftTypeTestRepositories
}

// NewShiftTypeTestModule composes the shift-type administration the way the
// factory does: the Workforce planning composition over the timetable test
// repositories, with the Kategorie↔Schichtart linker of the activity service.
// It returns the administration with the Timetable side it runs over.
func NewShiftTypeTestModule(db transactionruntime.Database) (workforce.ShiftTypeAdministration, ShiftTypeTestModule, error) {
	repos := NewShiftTypeTestRepositories(db, owners.MustNewMembershipTestRepositories(db))
	linker, err := timetableCompose.NewService(repos.Timetable, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, ShiftTypeTestModule{}, err
	}
	timetable, err := NewTimetableTestRepositories(db)
	if err != nil {
		return nil, ShiftTypeTestModule{}, err
	}
	rooms, err := owners.NewTimetableRoomRows(db)
	if err != nil {
		return nil, ShiftTypeTestModule{}, err
	}
	calendar, err := NewTimetableSchoolCalendar(db)
	if err != nil {
		return nil, ShiftTypeTestModule{}, err
	}
	planning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce: NewAbsenceTypeTestCapability(db), Staff: owners.NewPlanStaffDirectory(owners.MustNewMembershipTestRepositories(db).Staff), CalendarPeriods: calendar.SchoolCalendar(),
		Instances: timetableCompose.NewTimetableInstanceReads(timetable.ActivityInstance), InstanceStaff: timetableCompose.NewTimetableInstanceStaffReads(timetable.InstanceStaff),
		Rooms: facilitiesCompose.NewRoomNameDirectory(rooms.Room), ActivityGroups: timetableCompose.NewTimetableGroupReads(timetable.ActivityGroup),
		CategoryLinker: timetableCompose.ShiftTypeCategoryLinker(linker.SetCategoryShiftTypeLinks), DB: db.Pool(), Logger: slog.Default(),
	})
	if err != nil {
		return nil, ShiftTypeTestModule{}, err
	}
	return planning.ShiftTypes, ShiftTypeTestModule{Repositories: repos}, nil
}

type DeviceTestModule struct{ IoT devicefleetCompose.IoTService }

func NewDeviceTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (DeviceTestModule, error) {
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return DeviceTestModule{}, err
	}
	fleet, err := owners.NewDeviceFleet(db, owners.NewOnlineWindowMinutes(settings.Settings, nil))
	if err != nil {
		return DeviceTestModule{}, err
	}
	return DeviceTestModule{IoT: devicefleetCompose.NewIoTService(fleet)}, nil
}
