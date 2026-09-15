package compose

import "context"

func (e engine) CurrentFamilyProtection(ctx context.Context, ids []int64) (map[int64]bool, error) {
	return e.students.CurrentFamilyProtection(ctx, ids)
}
