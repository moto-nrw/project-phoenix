//go:build testhooks

package api

import (
	"log/slog"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type IoTDataTestModule struct {
	RoomAvailability devicescanCompose.RoomAvailability
	Configuration    devicescanCompose.ConfigurationQuery
	Directory        devicescanCompose.Directory
	TagAssignments   devicescanCompose.TagAssignments
	ActivitiesTestModule
	DeviceTestModule
	Facilities facilitiesCompose.RoomService
	// RoomUsage answers the room deletion's usage port over the same
	// sessions and care offerings, as the root binds it.
	RoomUsage facilitiesCompose.RoomUsage
}

func NewIoTDataTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (IoTDataTestModule, error) {
	activities, err := NewActivitiesTestModule(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	devices, err := NewDeviceTestModule(db, unit)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	r, err := NewTimetableTestRepositories(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return IoTDataTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	offerings, err := newTestCareOfferingCatalog(db, r, settings.Settings, CareOfferingCatalogTestOptions{})
	if err != nil {
		return IoTDataTestModule{}, err
	}
	facility := facilitiesCompose.NewRoomService(facilitiesCompose.RoomServiceConfig{
		Rooms: rooms, Occupancy: facilitiesLegacy.OccupancyProjection(RoomOccupancyPresence{Source: NewLoggedStudentPresence(db, slog.Default())}, roomActivityNames{r.Timetable}, membership, people),
		History: facilitiesLegacy.HistoryProjection(RoomHistoryPresence{Source: NewLoggedStudentPresence(db, slog.Default())}, roomActivityNames{r.Timetable}, membership, people),
	})
	// Data routes also resolve people and RFID tags, beyond activity staff reads.
	identity, err := NewRFIDTestModule(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	persons, err := NewRFIDTestPeople(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	activities.Users = persons
	return IoTDataTestModule{ActivitiesTestModule: activities, DeviceTestModule: devices, Facilities: facility, RoomUsage: &RoomUsage{Sessions: NewLoggedStudentPresence(db, slog.Default()), Offerings: offerings}, Configuration: devicescanCompose.NewConfiguration(DeviceConfiguration{Settings: settings.Settings}), RoomAvailability: devicescanCompose.NewRoomAvailability(facility), Directory: devicescanCompose.NewDirectory(NewDeviceScanPeople(persons), DeviceActivityCatalog(activities.Activities), nil), TagAssignments: identity.TagAssignments}, nil
}
