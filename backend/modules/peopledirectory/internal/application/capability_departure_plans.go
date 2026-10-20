package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (s *Service) ListStudentDepartureModes(ctx context.Context, ids []int64) (map[int64]map[string][]string, error) {
	ids = peopledirectory.PositiveIDs(ids)
	if len(ids) == 0 {
		return map[int64]map[string][]string{}, nil
	}

	return s.students.ListDepartureModes(ctx, ids)
}
