package services

import (
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"
)

// NewRoomRepository serves the retained room repository contract over the
// Facilities capability that owns facilities.rooms (#2665). Consumers build
// it where they use it instead of reading it from the repository factory.
func NewRoomRepository(rooms facilitiesModule.Capability) *facilitiesRepositoryAdapter.Repository {
	repository := facilitiesRepositoryAdapter.New()
	repository.Bind(rooms)
	return repository
}
