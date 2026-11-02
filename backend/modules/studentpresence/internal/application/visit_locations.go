package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) ListVisitLocations(ctx context.Context, filter studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error) {
	var rows []ports.VisitLocation
	err := s.runWrite(ctx, "list_visit_locations", func(ctx context.Context) (ports.Stats, error) {
		if filter.ForUpdate || filter.Limit < 0 || filter.Offset < 0 {
			return ports.Stats{}, errors.New("student presence: invalid visit location query")
		}
		var stats ports.Stats
		var err error
		rows, stats, err = s.store.ListVisitLocations(ctx, filter)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.VisitLocation, 0, len(rows))
	for _, row := range rows {
		location := studentpresence.VisitLocation{Visit: row.Visit}
		if row.GroupID != nil {
			location.Group = &studentpresence.VisitGroup{
				ID: *row.GroupID, RoomID: row.RoomID, CreatedAt: row.GroupCreatedAt, UpdatedAt: row.GroupUpdatedAt,
				StartTime: row.StartTime, LastActivity: row.LastActivity, EndTime: row.EndTime,
				TemplateID: row.TemplateID, DeviceID: row.DeviceID, TimeoutMinutes: row.TimeoutMinutes,
			}
		}
		result = append(result, location)
	}
	return result, nil
}

func (s *Service) CountOpenVisitsInGroup(ctx context.Context, id int64) (result int, err error) {
	err = s.runWrite(ctx, "count_open_visits_in_group", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.CountOpenVisitsInGroup(ctx, id)
		return stats, err
	})
	return result, err
}

// CountOpenVisitsInGroupWithRevision returns the open visit count of a
// session together with the revision that orders it (#5128).
func (s *Service) CountOpenVisitsInGroupWithRevision(ctx context.Context, id int64) (count int, revision int64, err error) {
	err = s.runWrite(ctx, "count_open_visits_in_group_with_revision", func(ctx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid session ID")
		}
		var stats ports.Stats
		count, revision, stats, err = s.store.CountOpenVisitsInGroupWithRevision(ctx, id)
		return stats, err
	})
	return count, revision, err
}

func (s *Service) CountOpenVisitsInRoom(ctx context.Context, id int64) (result int, err error) {
	err = s.runWrite(ctx, "count_open_visits_in_room", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.CountOpenVisitsInRoom(ctx, id)
		return stats, err
	})
	return result, err
}

// CountOpenVisitsInRoomWithRevision returns the open visit count of a room
// together with the revision that orders it (#5205).
func (s *Service) CountOpenVisitsInRoomWithRevision(ctx context.Context, id int64) (count int, revision int64, err error) {
	err = s.runWrite(ctx, "count_open_visits_in_room_with_revision", func(ctx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid room ID")
		}
		var stats ports.Stats
		count, revision, stats, err = s.store.CountOpenVisitsInRoomWithRevision(ctx, id)
		return stats, err
	})
	return count, revision, err
}

func (s *Service) ListOpenVisitRooms(ctx context.Context, roomID int64) (result []studentpresence.OpenVisitRoom, err error) {
	err = s.runWrite(ctx, "list_open_visit_rooms", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.ListOpenVisitRooms(ctx, roomID)
		return stats, err
	})
	return listResult(result, err)
}
