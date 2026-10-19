package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

type ReminderRoomReader struct {
	Source interface {
		ListRoomsByID(context.Context, []int64) ([]facilities.Room, error)
	}
}

func (r ReminderRoomReader) FindByIDs(ctx context.Context, ids []int64) ([]*ports.Room, error) {
	values, err := r.Source.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*ports.Room, 0, len(values))
	for _, value := range values {
		result = append(result, &ports.Room{ID: value.ID, Name: value.Name})
	}
	return result, nil
}
