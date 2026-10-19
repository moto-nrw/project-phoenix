package api

import (
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
)

// FacilitiesYard are the yard and WC views of Facilities over the room
// service and the activities.
type FacilitiesYard struct {
	Schulhof facilitiesCompose.SchulhofService
	WC       facilitiesCompose.WCService
}

// buildFacilitiesYard binds the yard and WC views over the retained room
// service (#4010); Facilities owns the room deletion decision behind its
// room usage port.
func buildFacilitiesYard(env owners.RootEnvironment, rooms FacilitiesRooms, activities activityContract.ActivityService) FacilitiesYard {
	logger := env.Logger.With("service", "facilities")
	return FacilitiesYard{
		Schulhof: facilitiesCompose.NewSchulhofService(rooms.RoomService, FacilitiesActivityCatalog(activities), rooms.OpenGroups, logger),
		WC:       facilitiesCompose.NewWCService(rooms.RoomService, FacilitiesActivityCatalog(activities), logger),
	}
}
