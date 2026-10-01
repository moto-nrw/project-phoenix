package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type IoTDataTestModule struct {
	RoomAvailability devicescanCompose.RoomAvailability
	Configuration    devicescanCompose.ConfigurationQuery
	Directory        devicescanCompose.Directory
	TagAssignments   devicescanCompose.TagAssignments
	ActivitiesTestModule
	DeviceTestModule
	Facilities facilities.Service
}

func NewIoTDataTestModule(db *bun.DB, unit tenant.UnitOfWork) (IoTDataTestModule, error) {
	activities, err := NewActivitiesTestModule(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	devices, err := NewDeviceTestModule(db, unit)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	r, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	people, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	offerings, err := newTestCareOfferingCatalog(r, settings.Settings, CareOfferingCatalogTestOptions{})
	if err != nil {
		return IoTDataTestModule{}, err
	}
	facility := facilities.NewServiceWithConfig(facilities.ServiceConfig{
		Rooms: rooms, Occupancy: facilitiesLegacy.OccupancyProjection(roomOccupancyPresence{newStudentPresence(db, slog.Default())}, r.ActivityGroup, membership, people),
		History: facilitiesLegacy.HistoryProjection(roomHistoryPresence{newStudentPresence(db, slog.Default())}, r.ActivityGroup, membership, people),
		ValidateDeletion: func(ctx context.Context, roomID int64) error {
			groups, err := r.ActiveGroup.FindActiveByRoomID(ctx, roomID)
			if err != nil {
				return err
			}
			if len(groups) > 0 {
				return facilitiesModule.ErrRoomInUse
			}
			if err := offerings.ValidateRoomDeletion(ctx, roomID); err != nil {
				if errors.Is(err, careplan.ErrCareOfferingConfigInvalid) {
					return facilitiesModule.ErrRoomRequiredByOffering
				}
				return err
			}
			return nil
		},
	})
	// Data routes also resolve people and RFID tags, beyond activity staff reads.
	identity, err := NewRFIDTestModule(db)
	if err != nil {
		return IoTDataTestModule{}, err
	}
	activities.Users = identity.Users
	return IoTDataTestModule{ActivitiesTestModule: activities, DeviceTestModule: devices, Facilities: facility, Configuration: devicescanCompose.NewConfiguration(settings.Settings), RoomAvailability: devicescanCompose.NewRoomAvailability(facility), Directory: devicescanCompose.NewDirectory(identity.Users, activities.Activities, nil), TagAssignments: identity.TagAssignments}, nil
}
