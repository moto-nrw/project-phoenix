package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func (e engine) ListVisitRetentionCounts(ctx context.Context) ([]studentpresence.VisitRetentionCount, error) {
	rows, err := e.Service.ListVisitRetentionCounts(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.VisitRetentionCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.VisitRetentionCount(row))
	}
	return result, nil
}
func (e engine) ListExpiredVisitMonths(ctx context.Context) ([]studentpresence.VisitMonthCount, error) {
	rows, err := e.Service.ListExpiredVisitMonths(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.VisitMonthCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.VisitMonthCount(row))
	}
	return result, nil
}
