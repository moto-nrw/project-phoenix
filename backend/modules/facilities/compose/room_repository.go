package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"
)

// NewRoomRepository serves the retained room repository contract over the
// Facilities capability that owns facilities.rooms (#2665). Consumers build
// it where they use it instead of reading it from a repository factory.
func NewRoomRepository(rooms facilities.Capability) *repositoryadapter.Repository {
	repository := repositoryadapter.New()
	repository.Bind(rooms)
	return repository
}

// NewRoomRecords serves the retained room rows over the Facilities queries
// alone, for the readers a root builds before the room deletion (#4080).
func NewRoomRecords(rooms facilities.Query) *repositoryadapter.Records {
	return repositoryadapter.NewRecords(rooms)
}
