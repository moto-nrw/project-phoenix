package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func NewOpeningYearsReader(ledger workforce.Capability) func(ctx context.Context, year int) ([]int64, error) {
	return func(ctx context.Context, year int) ([]int64, error) {
		rows, err := ledger.ListStaffVacationOpenings(ctx, workforce.StaffVacationFilter{Year: year})
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.StaffID)
		}
		return ids, nil
	}
}
