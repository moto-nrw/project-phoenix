package compose

import "github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"

var (
	// NewRoomRepository serves the retained room repository contract over the
	// Facilities capability that owns facilities.rooms (#2665). Consumers
	// build it where they use it instead of reading it from a factory.
	NewRoomRepository = repositoryadapter.New
	// NewRoomRecords serves the retained room rows over the Facilities
	// queries alone, for the readers a root builds before the room deletion.
	NewRoomRecords = repositoryadapter.NewRecords
)
