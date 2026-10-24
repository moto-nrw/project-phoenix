package peopledirectory

import "context"

// StudentDepartureQuery reads canonical departure modes for request baselines.
// It does not reconstruct or expose the legacy pickup and bus mirrors.
type StudentDepartureQuery interface {
	ListStudentDepartureModes(context.Context, []int64) (map[int64]map[string][]string, error)
}
