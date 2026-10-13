package services

import (
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// NewRoomRepository serves the retained room repository contract over the
// Facilities capability that owns facilities.rooms (#2665). Consumers build
// it where they use it instead of reading it from the repository factory.
func NewRoomRepository(rooms facilitiesModule.Capability) *facilitiesRepositoryAdapter.Repository {
	repository := facilitiesRepositoryAdapter.New()
	repository.Bind(rooms)
	return repository
}

// NewTimetableRoomNames names and locks the Facilities rooms for the
// Timetable owner rows (#3551).
func NewTimetableRoomNames(rooms facilitiesModule.Capability) timetableCompose.OwnerRooms {
	return facilitiesCompose.NewRoomNameDirectory(NewRoomRepository(rooms))
}
