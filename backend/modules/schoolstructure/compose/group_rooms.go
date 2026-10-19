package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

// GroupRoomRecords supplies this owner's persisted group rows at construction.
type GroupRoomRecords interface {
	List(context.Context, map[string]interface{}) ([]*domain.Group, error)
}

func NewGroupRoomDirectory(records GroupRoomRecords) *application.GroupRoomDirectory {
	return application.NewGroupRoomDirectory(records)
}
