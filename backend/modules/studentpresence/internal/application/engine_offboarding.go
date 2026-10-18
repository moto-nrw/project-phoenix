package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (e engine) LockStaffSupervision(ctx context.Context, staffID int64, value string) ([]int64, error) {
	date, err := calendar.ParseDate(value)
	if err != nil {
		return nil, err
	}
	return e.Service.LockStaffSupervision(ctx, staffID, date)
}
