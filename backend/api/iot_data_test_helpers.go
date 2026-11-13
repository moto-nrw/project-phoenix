//go:build testhooks

package api

import (
	"context"
	"log/slog"

	owners "github.com/moto-ogs/project-phoenix/roots/owners"

	activityContract "github.com/moto-ogs/project-phoenix/modules/timetable/activitycontract"

	devicescanCompose "github.com/moto-ogs/project-phoenix/modules/devicescan/compose"
	facilitiesCompose "github.com/moto-ogs/project-phoenix/modules/facilities/compose"
	peoplePorts "github.com/moto-ogs/project-phoenix/modules/peopledirectory/ports"
	membershipCompose "github.com/moto-ogs/project-phoenix/modules/schoolmembership/compose"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// IoTDataTestModule is the device-scan side of the kiosk data routes. The
// activities, the person directory, the device fleet, the room service and
// the room usage it runs beside come from NewIoTDataTestModule (#4080).
type IoTDataTestModule struct {
	RoomAvailability devicescanCompose.RoomAvailability
	Configuration    devicescanCompose.ConfigurationQuery
	Directory        devicescanCompose.Directory
	TagAssignments   devicescanCompose.TagAssignments
}

// NewIoTTeacherDirectoryWithEmploymentFailure binds the real roster to a
// failing Workforce read without exposing a foreign composition to its test.
func NewIoTTeacherDirectoryWithEmploymentFailure(db transactionruntime.Database, failure error) (devicescanCompose.Directory, *int, error) {
	calls := new(int)
	membership, err := membershipCompose.New(membershipCompose.Dependencies{
		DB: db.Pool(), Observe: func(membershipCompose.Observation) {},
		Employment: kioskTeacherEmploymentFailureBinding{failure: failure, calls: calls},
	})
	if err != nil {
		return nil, nil, err
	}
	people, err := NewRFIDTestPeople(db)
	if err != nil {
		return nil, nil, err
	}
	teachers := owners.TeacherMembershipRepository{Membership: membership,
		Deps: owners.NewStaffMembershipDeps(owners.NewPersonRepository(db), nil, nil)}
	return devicescanCompose.NewDirectory(NewDeviceScanDirectoryPeople(
		kioskTeacherTestPeople{deviceScanUserRows: people, teachers: teachers}, nil), nil, NewDeviceScanHolidayGuestSchools(nil), nil, DeviceScanPrincipal), calls, nil
}

type kioskTeacherEmploymentFailureBinding struct {
	failure error
	calls   *int
}

func (e kioskTeacherEmploymentFailureBinding) StaffEmployments(context.Context, []int64) (map[int64]membershipCompose.StaffEmploymentProfile, error) {
	*e.calls++
	return nil, e.failure
}
func (e kioskTeacherEmploymentFailureBinding) SaveStaffEmployment(context.Context, membershipCompose.StaffEmploymentProfile) error {
	return e.failure
}

type kioskTeacherTestPeople struct {
	deviceScanUserRows
	teachers peoplePorts.TeacherRepository
}

func (p kioskTeacherTestPeople) TeacherRows() peoplePorts.TeacherRepository { return p.teachers }

// NewIoTDataTestModule composes the kiosk data routes and returns the
// device-scan side with the owners beside it. The room usage answers the
// room deletion's usage port over the same sessions and care offerings, as
// the root binds it.
func NewIoTDataTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (
	IoTDataTestModule, ActivitiesTestModule, activityContract.ActivityService, peoplePorts.PersonDirectory, DeviceTestModule, facilitiesCompose.RoomService, facilitiesCompose.RoomUsage, error,
) {
	fail := func(err error) (IoTDataTestModule, ActivitiesTestModule, activityContract.ActivityService, peoplePorts.PersonDirectory, DeviceTestModule, facilitiesCompose.RoomService, facilitiesCompose.RoomUsage, error) {
		return IoTDataTestModule{}, ActivitiesTestModule{}, nil, nil, DeviceTestModule{}, nil, nil, err
	}
	activities, activityService, _, _, err := NewActivitiesTestModule(db)
	if err != nil {
		return fail(err)
	}
	devices, err := NewDeviceTestModule(db, unit)
	if err != nil {
		return fail(err)
	}
	r, err := NewTimetableTestRepositories(db)
	if err != nil {
		return fail(err)
	}
	people, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return fail(err)
	}
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {
		return fail(err)
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return fail(err)
	}
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return fail(err)
	}
	offerings, err := newTestCareOfferingCatalog(db, r, settings.Settings, CareOfferingCatalogTestOptions{})
	if err != nil {
		return fail(err)
	}
	facility := facilitiesCompose.NewRoomService(facilitiesCompose.RoomServiceConfig{
		Rooms: rooms, Occupancy: facilitiesCompose.OccupancyProjection(RoomOccupancyPresence{Source: owners.NewLoggedStudentPresence(db, slog.Default())}, roomActivityNames{r.Timetable}, membership, people),
		History: facilitiesCompose.HistoryProjection(RoomHistoryPresence{Source: owners.NewLoggedStudentPresence(db, slog.Default())}, roomActivityNames{r.Timetable}, membership, people),
	})
	// Data routes also resolve people and RFID tags, beyond activity staff reads.
	identity, err := NewRFIDTestModule(db)
	if err != nil {
		return fail(err)
	}
	persons, err := NewRFIDTestPeople(db)
	if err != nil {
		return fail(err)
	}
	// The kiosk teacher list reads the open invitations (#4573).
	invitations, err := NewAuthTestModule(db, unit)
	if err != nil {
		return fail(err)
	}
	return IoTDataTestModule{
			Configuration: devicescanCompose.NewConfiguration(DeviceConfiguration{Settings: settings.Settings}, DeviceScanPrincipal), RoomAvailability: devicescanCompose.NewRoomAvailability(facility, DeviceScanPrincipal),
			Directory: devicescanCompose.NewDirectory(NewDeviceScanDirectoryPeople(persons, invitations.Auth), DeviceActivityCatalog(activityService), NewDeviceScanHolidayGuestSchools(nil), nil, DeviceScanPrincipal), TagAssignments: identity.TagAssignments,
		}, activities, activityService, persons, devices, facility,
		&owners.RoomUsage{Sessions: owners.NewLoggedStudentPresence(db, slog.Default()), Offerings: offerings}, nil
}

// NewIoTTeacherStudentsTestDirectory composes the positive teacher-filtered
// roster with School Membership's real staff-to-group binding. The legacy
// unbound RFID graph remains available for its failure-fallback contracts.
func NewIoTTeacherStudentsTestDirectory(db transactionruntime.Database) (devicescanCompose.Directory, error) {
	members, err := owners.NewMembershipTestRepositories(db)
	if err != nil {
		return nil, err
	}
	persons := NewTestPersonDirectory(db, TestPersonDirectorySources{
		Persons: members.Person, Students: members.Student,
		Teachers: members.Teacher, Staff: members.Staff,
	})
	return devicescanCompose.NewDirectory(NewDeviceScanPeople(persons), nil, NewDeviceScanHolidayGuestSchools(nil), nil, DeviceScanPrincipal), nil
}
