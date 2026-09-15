package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// NewReferences binds import name matching to the native owner queries.
// The 1000-group bound preserves the former import preload contract.
func NewReferences(groups schoolstructure.GroupListing, rooms facilities.Query) dataimport.References {
	return dataimport.References{
		Groups: func(ctx context.Context) ([]dataimport.Reference, error) {
			rows, err := groups.ListGroups(ctx, 1000)
			if err != nil {
				return nil, err
			}
			result := make([]dataimport.Reference, 0, len(rows))
			for _, row := range rows {
				result = append(result, dataimport.Reference{ID: row.ID, Name: row.Name})
			}
			return result, nil
		},
		Rooms: func(ctx context.Context) ([]dataimport.Reference, error) {
			rows, err := rooms.ListRooms(ctx, facilities.RoomFilter{})
			if err != nil {
				return nil, err
			}
			result := make([]dataimport.Reference, 0, len(rows))
			for _, row := range rows {
				result = append(result, dataimport.Reference{ID: row.ID, Name: row.Name})
			}
			return result, nil
		},
	}
}
