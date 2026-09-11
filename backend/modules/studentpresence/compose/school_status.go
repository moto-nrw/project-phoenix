package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func (e engine) ListSchoolStatuses(ctx context.Context, ids []int64, date string) ([]studentpresence.SchoolStatus, error) {
	if _, err := timezone.ParseDate(date); err != nil {
		return nil, err
	}
	rows, err := e.Service.ListSchoolStatuses(ctx, ids, date)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.SchoolStatus, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.SchoolStatus(row))
	}
	return result, nil
}
