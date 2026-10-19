package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

// GroupRoomRecords is the legacy group read accepted during composition.
type GroupRoomRecords interface {
	List(context.Context, map[string]interface{}) ([]*domain.Group, error)
}

type GroupRoomDirectory struct{ records GroupRoomRecords }

func NewGroupRoomDirectory(records GroupRoomRecords) *GroupRoomDirectory {
	return &GroupRoomDirectory{records: records}
}

func (r *GroupRoomDirectory) ListGroupRooms(ctx context.Context) ([]*schoolstructure.GroupRoomAssignment, error) {
	rows, err := r.records.List(ctx, nil)
	if rows == nil {
		return nil, err
	}
	result := make([]*schoolstructure.GroupRoomAssignment, len(rows))
	for i, row := range rows {
		if row != nil {
			result[i] = &schoolstructure.GroupRoomAssignment{ID: row.ID, RoomID: row.RoomID}
		}
	}
	return result, err
}
