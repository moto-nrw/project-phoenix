package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/facilities"
	"github.com/moto-ogs/project-phoenix/modules/facilities/internal/domain"
)

// DeleteRoom removes a room (#6845). A room a session runs in, a care
// offering needs, or activities, upcoming blocks, education groups or
// devices still use stays: removing it would leave them without a room. A
// room only past records name is archived, so the history keeps its room.
// A room nothing references is deleted. The database decides between the
// last two: every reference to facilities.rooms is a NO ACTION key.
func (s *Service) DeleteRoom(ctx context.Context, id int64) (result facilities.RoomRemoval, err error) {
	if err := s.requireTenant(ctx, "delete_room"); err != nil {
		return facilities.RoomRemoval{}, err
	}
	if id <= 0 {
		err := &facilities.InvalidRoomError{Reason: "room ID is required"}
		s.ObserveRejection("delete_room", 0, err)
		return facilities.RoomRemoval{}, err
	}
	err = s.runWrite(ctx, "delete_room", func(txCtx context.Context, stats *domain.OperationStats) error {
		// Timetable writers take the recurrence lock before room FK locks. Keep
		// deletion in the same order so concurrent template creation cannot
		// deadlock with a room delete.
		if err := s.lockDeletion(txCtx); err != nil {
			return err
		}
		existing, err := s.findRemovableRoom(txCtx, id, stats)
		if err != nil {
			return err
		}
		if err := s.refuseUsedRoom(txCtx, id); err != nil {
			return err
		}
		references, err := s.references(txCtx, id)
		if err != nil {
			return err
		}
		if references.Live() {
			return &facilities.RoomStillPlannedError{References: references}
		}
		result, err = s.removeRoom(txCtx, existing, stats)
		return err
	})
	return result, err
}

// RoomRemovalImpact previews DeleteRoom: the outcome and every fact behind
// it, read in the caller's transaction.
func (s *Service) RoomRemovalImpact(ctx context.Context, id int64) (result facilities.RoomRemovalImpact, err error) {
	if err := s.requireTenant(ctx, "room_removal_impact"); err != nil {
		return facilities.RoomRemovalImpact{}, err
	}
	if id <= 0 {
		err := &facilities.InvalidRoomError{Reason: "room ID is required"}
		s.ObserveRejection("room_removal_impact", 0, err)
		return facilities.RoomRemovalImpact{}, err
	}
	err = s.runWrite(ctx, "room_removal_impact", func(txCtx context.Context, stats *domain.OperationStats) error {
		if _, err := s.findRemovableRoom(txCtx, id, stats); err != nil {
			return err
		}
		active, err := s.usage.HasActiveSessions(txCtx, id)
		if err != nil {
			return err
		}
		offering, err := s.usage.RequiredByCareOffering(txCtx, id)
		if err != nil {
			return err
		}
		references, err := s.references(txCtx, id)
		if err != nil {
			return err
		}
		result = facilities.NewRoomRemovalImpact(active, offering, references)
		return nil
	})
	return result, err
}

// findRemovableRoom locks the room against concurrent writes and rejects
// unknown and system rooms. An archived room may be removed again: once
// retention has cleared its history it is deleted for good.
func (s *Service) findRemovableRoom(ctx context.Context, id int64, stats *domain.OperationStats) (facilities.Room, error) {
	existing, found, queryStats, err := s.store.FindByID(ctx, id, "UPDATE")
	stats.Add(queryStats)
	if err != nil {
		return facilities.Room{}, err
	}
	if !found {
		return facilities.Room{}, facilities.ErrRoomNotFound
	}
	if existing.IsSystem {
		return facilities.Room{}, facilities.ErrSystemRoomProtected
	}
	return existing, nil
}

func (s *Service) removeRoom(ctx context.Context, existing facilities.Room, stats *domain.OperationStats) (facilities.RoomRemoval, error) {
	deleted, queryStats, err := s.store.Delete(ctx, existing.ID)
	stats.Add(queryStats)
	if err != nil {
		return facilities.RoomRemoval{}, err
	}
	if deleted {
		return facilities.RoomRemoval{Outcome: facilities.RoomRemovalDelete}, nil
	}
	if existing.ArchivedAt == nil {
		_, queryStats, err = s.store.Archive(ctx, existing.ID)
		stats.Add(queryStats)
		if err != nil {
			return facilities.RoomRemoval{}, err
		}
	}
	return facilities.RoomRemoval{Outcome: facilities.RoomRemovalArchive}, nil
}
