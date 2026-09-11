package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func (e engine) LockStaffSupervision(ctx context.Context, staffID int64, value string) ([]int64, error) {
	date, err := timezone.ParseDate(value)
	if err != nil {
		return nil, err
	}
	return e.Service.LockStaffSupervision(ctx, staffID, date)
}
