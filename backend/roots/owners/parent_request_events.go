package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentRequestEvents(source peopledirectory.ParentRequestEvents) parentportal.RequestEvents {
	return parentportal.NewRequestEvents(parentportal.RequestEventCallbacks{
		Create: func(ctx context.Context, value *parentportal.RequestEventSnapshot) error {
			if value == nil {
				return source.AppendParentRequestEvent(ctx, nil)
			}
			row := peopledirectory.ParentRequestEventSnapshot(*value)
			err := source.AppendParentRequestEvent(ctx, &row)
			*value = parentportal.RequestEventSnapshot(row)
			return err
		},
		List: func(ctx context.Context, kind string, id int64) ([]*parentportal.RequestEventSnapshot, error) {
			rows, err := source.ParentRequestEventsForRequest(ctx, kind, id)
			if rows == nil {
				return nil, err
			}
			result := make([]*parentportal.RequestEventSnapshot, len(rows))
			for i, row := range rows {
				if row != nil {
					value := parentportal.RequestEventSnapshot(*row)
					result[i] = &value
				}
			}
			return result, err
		},
	})
}
