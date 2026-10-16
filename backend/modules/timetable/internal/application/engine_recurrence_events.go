package application

import (
	"context"
	"time"
)

func (e Engine) GenerateRecurrenceEvents(ctx context.Context, id int64, start, end time.Time) ([]time.Time, error) {
	events, err := e.service.GenerateRecurrenceEvents(ctx, id, start, end)
	return events, MapError(err)
}
