package peopledirectory

import "context"

// StudentDepartureQuery reads canonical departure modes for request baselines.
// It does not reconstruct or expose the legacy pickup and bus mirrors.
type StudentDepartureQuery interface {
	ListStudentDepartureModes(context.Context, []int64) (map[int64]map[string][]string, error)
}

func (m *Module) ListStudentDepartureModes(ctx context.Context, ids []int64) (map[int64]map[string][]string, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return map[int64]map[string][]string{}, nil
	}
	return m.engine.ListStudentDepartureModes(ctx, ids)
}
