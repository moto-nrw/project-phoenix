package application

import (
	"context"
)

func (e Engine) ListStudentDepartureModes(ctx context.Context, ids []int64) (map[int64]map[string][]string, error) {
	return e.students.ListDepartureModes(ctx, ids)
}
