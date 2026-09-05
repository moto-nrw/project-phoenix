package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type IoTDataTestModule struct {
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
	offerings := enrollment.NewCareOfferingService(enrollment.CareOfferingServiceConfig{
		Repo: r.CareOffering, PhaseRepo: r.Phase, RequestChildOfferingRepo: r.RequestChildOffering, Settings: settings.Settings,
		ActivityGroupRepo: r.ActivityGroup, ActivityScheduleRepo: r.ActivitySchedule,
		CalendarPeriodRepo: r.CalendarPeriod, TimeframeRepo: r.Timeframe, ActivityExceptionRepo: r.ActivityException,
	}).(enrollment.CareOfferingMaterializationResourceValidator)
	facility := facilities.NewServiceWithConfig(facilities.ServiceConfig{
		Rooms: rooms, Occupancy: facilitiesLegacy.OccupancyProjection(r.ActiveGroup, r.ActivityGroup, membership, people),
		History: facilitiesLegacy.HistoryProjection(r.ActiveGroup),
		ValidateDeletion: func(ctx context.Context, roomID int64) error {
			groups, err := r.ActiveGroup.FindActiveByRoomID(ctx, roomID)
			if err != nil {
				return err
			}
			if len(groups) > 0 {
				return facilitiesModule.ErrRoomInUse
			}
			if err := offerings.ValidateRoomDeletion(ctx, roomID); err != nil {
				if errors.Is(err, enrollment.ErrCareOfferingInvalid) {
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
	return IoTDataTestModule{ActivitiesTestModule: activities, DeviceTestModule: devices, Facilities: facility}, nil
}
