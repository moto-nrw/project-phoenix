package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) PrepareVisitWrites(ctx context.Context, scope studentpresence.VisitWriteScope) error {
	return s.run("prepare_visit_writes", func() (ports.Stats, error) {
		if !validIDs(scope.VisitIDs) || !validIDs(scope.StudentIDs) || !validIDs(scope.GroupIDs) || !validIDs(scope.SessionIDs) || scope.DeviceID < 0 || scope.RoomID < 0 {
			return ports.Stats{}, errors.New("prepare visit writes: invalid scope")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.PrepareVisitWrites(ctx, scope)
	})
}
