package compose

import (
	"context"
	"time"
)

func (e engine) GenerateRecurrenceEvents(ctx context.Context, id int64, start, end time.Time) ([]time.Time, error) {
	events, err := e.service.GenerateRecurrenceEvents(ctx, id, start, end)
	return events, mapError(err)
}
