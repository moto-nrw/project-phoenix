package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func NewOpeningStaffIDsReader(ledger workforce.Capability) func(ctx context.Context) ([]int64, error) {
	return func(ctx context.Context) ([]int64, error) {
		rows, err := ledger.ListStaffBalanceAdjustments(ctx, workforce.StaffBalanceAdjustmentFilter{Types: []string{workforce.BalanceAdjustmentTypeOpening}})
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
