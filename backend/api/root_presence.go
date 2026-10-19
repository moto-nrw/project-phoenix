package api

import (
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// FacilitiesRooms are the retained room service with its occupancy and
// history over Student Presence, and the running sessions the yard reads.
type FacilitiesRooms struct {
	RoomService facilitiesCompose.RoomService
	OpenGroups  facilitiesCompose.OpenGroupCatalog
}

// buildFacilitiesRooms builds the retained room service and the open
// groups of the yard over Student Presence.
func buildFacilitiesRooms(presence *studentpresence.Module, rooms facilitiesModule.Catalog, timetableOwner timetable.Capability, membership schoolmembership.Capability, persons peopledirectory.Capability) FacilitiesRooms {
	return FacilitiesRooms{
		RoomService: facilitiesCompose.NewRoomService(facilitiesCompose.RoomServiceConfig{
			Rooms:     rooms,
			Occupancy: facilitiesCompose.OccupancyProjection(RoomOccupancyPresence{presence}, roomActivityNames{timetableOwner}, membership, persons),
			History:   facilitiesCompose.HistoryProjection(RoomHistoryPresence{presence}, roomActivityNames{timetableOwner}, membership, persons),
		}),
		OpenGroups: facilitiesCompose.NewOpenGroupCatalog(FacilitiesGroupSupervisions(presence), FacilitiesRoomSessions(presence), FacilitiesGroupVisits(presence)),
	}
}
