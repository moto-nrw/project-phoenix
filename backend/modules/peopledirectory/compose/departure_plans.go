package compose

import "context"

func (e engine) ListStudentDepartureModes(ctx context.Context, ids []int64) (map[int64]map[string][]string, error) {
	return e.students.ListDepartureModes(ctx, ids)
}
