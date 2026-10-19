package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

func NewRoomReferenceReader(rooms facilities.Query) func(ctx context.Context) ([]dataimport.Reference, error) {
	return func(ctx context.Context) ([]dataimport.Reference, error) {
		rows, err := rooms.ListRooms(ctx, facilities.RoomFilter{})
		if err != nil {
			return nil, err
		}
		result := make([]dataimport.Reference, 0, len(rows))
		for _, row := range rows {
			result = append(result, dataimport.Reference{ID: row.ID, Name: row.Name})
		}
		return result, nil
	}
}
