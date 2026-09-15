package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/education"
)

// GroupRoomRecords is the legacy group read accepted during composition.
type GroupRoomRecords interface {
	List(context.Context, map[string]interface{}) ([]*education.Group, error)
}

type GroupRoom struct {
	ID     int64
	RoomID *int64
}

type GroupRoomDirectory struct{ records GroupRoomRecords }

func NewGroupRoomDirectory(records GroupRoomRecords) *GroupRoomDirectory {
	return &GroupRoomDirectory{records: records}
}

func (r *GroupRoomDirectory) ListGroupRooms(ctx context.Context) ([]*GroupRoom, error) {
	rows, err := r.records.List(ctx, nil)
	if rows == nil {
		return nil, err
	}
	result := make([]*GroupRoom, len(rows))
	for i, row := range rows {
		if row != nil {
			result[i] = &GroupRoom{ID: row.ID, RoomID: row.RoomID}
		}
	}
	return result, err
}
