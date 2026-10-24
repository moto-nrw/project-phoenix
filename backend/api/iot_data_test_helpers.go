//go:build testhooks

package api

import (
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
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
			Directory: devicescanCompose.NewDirectory(NewDeviceScanDirectoryPeople(persons, invitations.Auth), DeviceActivityCatalog(activityService), nil, DeviceScanPrincipal), TagAssignments: identity.TagAssignments,
		}, activities, activityService, persons, devices, facility,
		&owners.RoomUsage{Sessions: owners.NewLoggedStudentPresence(db, slog.Default()), Offerings: offerings}, nil
}
