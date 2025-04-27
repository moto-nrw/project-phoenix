package room

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/models"
	"github.com/uptrace/bun"
)

// HistoryStore defines database operations for room history
type HistoryStore interface {
	GetRoomHistoryByRoom(ctx context.Context, roomID int64) ([]models.RoomHistory, error)
	GetRoomHistoryByDateRange(ctx context.Context, startDate, endDate time.Time) ([]models.RoomHistory, error)
	GetRoomHistoryBySupervisor(ctx context.Context, supervisorID int64) ([]models.RoomHistory, error)
}

type historyStore struct {
	db           *bun.DB
	historyStore *database.RoomHistoryStore
}

// NewHistoryStore returns a new HistoryStore implementation
func NewHistoryStore(db *bun.DB) HistoryStore {
	return &historyStore{
		db:           db,
		historyStore: database.NewRoomHistoryStore(db),
	}
}

// GetRoomHistoryByRoom retrieves room history records for a specific room
func (s *historyStore) GetRoomHistoryByRoom(ctx context.Context, roomID int64) ([]models.RoomHistory, error) {
	// Check if room exists
	room := new(models.Room)
	err := s.db.NewSelect().
		Model(room).
		Where("id = ?", roomID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("room not found: %w", err)
	}

	// Get room history
	history, err := s.historyStore.GetRoomHistoryByRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}

	return history, nil
}

// GetRoomHistoryByDateRange retrieves room history records within a date range
func (s *historyStore) GetRoomHistoryByDateRange(ctx context.Context, startDate, endDate time.Time) ([]models.RoomHistory, error) {
	history, err := s.historyStore.GetRoomHistoryByDateRange(ctx, startDate, endDate)
	if err != nil {
		return nil, err
	}

	return history, nil
}

// GetRoomHistoryBySupervisor retrieves room history records for a specific supervisor
func (s *historyStore) GetRoomHistoryBySupervisor(ctx context.Context, supervisorID int64) ([]models.RoomHistory, error) {
	// Check if supervisor exists
	supervisor := new(models.PedagogicalSpecialist)
	err := s.db.NewSelect().
		Model(supervisor).
		Where("id = ?", supervisorID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("supervisor not found: %w", err)
	}

	// Get room history
	history, err := s.historyStore.GetRoomHistoryBySupervisor(ctx, supervisorID)
	if err != nil {
		return nil, err
	}

	return history, nil
}
