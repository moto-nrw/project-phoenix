package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) ListVisitLocations(ctx context.Context, filter ports.VisitLocationFilter) (result []ports.VisitLocation, err error) {
	err = s.run("list_visit_locations", func() (ports.Stats, error) {
		if filter.ForUpdate || filter.Limit < 0 || filter.Offset < 0 {
			return ports.Stats{}, errors.New("student presence: invalid visit location query")
		}
		var stats ports.Stats
		result, stats, err = s.store.ListVisitLocations(ctx, filter)
		return stats, err
	})
	return result, err
}

func (s *Service) CountOpenVisitsInGroup(ctx context.Context, id int64) (result int, err error) {
	err = s.run("count_open_visits_in_group", func() (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.CountOpenVisitsInGroup(ctx, id)
		return stats, err
	})
	return result, err
}

func (s *Service) CountOpenVisitsInRoom(ctx context.Context, id int64) (result int, err error) {
	err = s.run("count_open_visits_in_room", func() (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.CountOpenVisitsInRoom(ctx, id)
		return stats, err
	})
	return result, err
}

func (s *Service) ListOpenVisitRooms(ctx context.Context, roomID int64) (result []ports.OpenVisitRoom, err error) {
	err = s.run("list_open_visit_rooms", func() (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.ListOpenVisitRooms(ctx, roomID)
		return stats, err
	})
	return result, err
}
