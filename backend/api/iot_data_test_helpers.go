//go:build testhooks

package api

import (
	"log/slog"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
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
	IoTDataTestModule, ActivitiesTestModule, peoplePorts.PersonDirectory, DeviceTestModule, facilitiesCompose.RoomService, facilitiesCompose.RoomUsage, error,
) {
	fail := func(err error) (IoTDataTestModule, ActivitiesTestModule, peoplePorts.PersonDirectory, DeviceTestModule, facilitiesCompose.RoomService, facilitiesCompose.RoomUsage, error) {
		return IoTDataTestModule{}, ActivitiesTestModule{}, nil, DeviceTestModule{}, nil, nil, err
	}
	activities, _, _, err := NewActivitiesTestModule(db)
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
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return fail(err)
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return fail(err)
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return fail(err)
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return fail(err)
	}
	offerings, err := newTestCareOfferingCatalog(db, r, settings.Settings, CareOfferingCatalogTestOptions{})
	if err != nil {
		return fail(err)
	}
	facility := facilitiesCompose.NewRoomService(facilitiesCompose.RoomServiceConfig{
		Rooms: rooms, Occupancy: facilitiesLegacy.OccupancyProjection(RoomOccupancyPresence{Source: NewLoggedStudentPresence(db, slog.Default())}, roomActivityNames{r.Timetable}, membership, people),
		History: facilitiesLegacy.HistoryProjection(RoomHistoryPresence{Source: NewLoggedStudentPresence(db, slog.Default())}, roomActivityNames{r.Timetable}, membership, people),
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
	return IoTDataTestModule{
			Configuration: devicescanCompose.NewConfiguration(DeviceConfiguration{Settings: settings.Settings}), RoomAvailability: devicescanCompose.NewRoomAvailability(facility),
			Directory: devicescanCompose.NewDirectory(NewDeviceScanPeople(persons), DeviceActivityCatalog(activities.Activities), nil), TagAssignments: identity.TagAssignments,
		}, activities, persons, devices, facility,
		&RoomUsage{Sessions: NewLoggedStudentPresence(db, slog.Default()), Offerings: offerings}, nil
}
