package compose

import facilitiesService "github.com/moto-nrw/project-phoenix/modules/facilities/internal/application/roomservice"

// The retained room, Schulhof and WC services the root composes over the
// Facilities capability (#2747). The legacy composition names them through
// this package, never through the retained application package.
type (
	RoomService         = facilitiesService.Service
	RoomServiceConfig   = facilitiesService.ServiceConfig
	SchulhofService     = facilitiesService.SchulhofService
	WCService           = facilitiesService.WCService
	OpenGroup           = facilitiesService.OpenGroup
	OpenGroupSupervisor = facilitiesService.OpenGroupSupervisor
	OpenGroupVisit      = facilitiesService.OpenGroupVisit
	OpenGroupCatalog    = facilitiesService.OpenGroupCatalog
)

var (
	NewRoomService     = facilitiesService.NewServiceWithConfig
	NewSchulhofService = facilitiesService.NewSchulhofService
	NewWCService       = facilitiesService.NewWCService
)
