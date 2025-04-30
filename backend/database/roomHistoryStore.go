package database

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models"
	"github.com/uptrace/bun"
)

// RoomHistoryStore defines operations for room history records
type RoomHistoryStore struct {
	db *bun.DB
}

// NewRoomHistoryStore creates a new RoomHistoryStore
func NewRoomHistoryStore(db *bun.DB) *RoomHistoryStore {
	return &RoomHistoryStore{db: db}
}

// CreateRoomHistory inserts a new room history record
func (s *RoomHistoryStore) CreateRoomHistory(ctx context.Context, history *models.RoomHistory) error {
	_, err := s.db.NewInsert().Model(history).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create room history: %w", err)
	}
	return nil
}

// GetRoomHistoryByID retrieves a room history record by ID
func (s *RoomHistoryStore) GetRoomHistoryByID(ctx context.Context, id int64) (*models.RoomHistory, error) {
	history := new(models.RoomHistory)
	err := s.db.NewSelect().
		Model(history).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get room history: %w", err)
	}
	return history, nil
}

// GetRoomHistoryByRoom retrieves room history records for a specific room
func (s *RoomHistoryStore) GetRoomHistoryByRoom(ctx context.Context, roomID int64) ([]models.RoomHistory, error) {
	var history []models.RoomHistory
	err := s.db.NewSelect().
		Model(&history).
		Where("room_id = ?", roomID).
		Order("day DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get room history: %w", err)
	}
	return history, nil
}

// GetRoomHistoryByDateRange retrieves room history records within a date range
func (s *RoomHistoryStore) GetRoomHistoryByDateRange(ctx context.Context, startDate, endDate time.Time) ([]models.RoomHistory, error) {
	var history []models.RoomHistory
	err := s.db.NewSelect().
		Model(&history).
		Where("day BETWEEN ? AND ?", startDate, endDate).
		Order("day DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get room history: %w", err)
	}
	return history, nil
}

// GetRoomHistoryBySupervisor retrieves room history records for a specific supervisor
func (s *RoomHistoryStore) GetRoomHistoryBySupervisor(ctx context.Context, supervisorID int64) ([]models.RoomHistory, error) {
	var history []models.RoomHistory
	err := s.db.NewSelect().
		Model(&history).
		Where("supervisor_id = ?", supervisorID).
		Order("day DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get room history: %w", err)
	}
	return history, nil
}

// DeleteRoomHistory deletes a room history record
func (s *RoomHistoryStore) DeleteRoomHistory(ctx context.Context, id int64) error {
	_, err := s.db.NewDelete().
		Model((*models.RoomHistory)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete room history: %w", err)
	}
	return nil
}

// CreateRoomHistoryFromOccupancy creates a room history record from a room occupancy
func (s *RoomHistoryStore) CreateRoomHistoryFromOccupancy(ctx context.Context, occupancy *models.RoomOccupancy) error {
	// First, load all related data
	err := s.db.NewSelect().
		Model(occupancy).
		Relation("Room").
		Relation("Ag").
		Relation("Group").
		Relation("Timespan").
		Relation("Supervisors").
		WherePK().
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to load room occupancy data: %w", err)
	}

	// Create a new RoomHistory record
	history := &models.RoomHistory{
		RoomID:     occupancy.RoomID,
		Day:        time.Now(),
		TimespanID: occupancy.TimespanID,
	}

	// Add group or AG information based on what's available
	if occupancy.GroupID != nil && occupancy.Group != nil {
		history.GroupID = occupancy.GroupID
		history.AgName = occupancy.Group.Name
	}

	if occupancy.AgID != nil && occupancy.Ag != nil {
		history.AgName = occupancy.Ag.Name
		history.MaxParticipant = occupancy.Ag.MaxParticipant
		history.AgCategoryID = &occupancy.Ag.AgCategoryID
	}

	// Add the first supervisor if available
	if len(occupancy.Supervisors) > 0 {
		history.SupervisorID = occupancy.Supervisors[0].ID
	}

	// Insert the new record
	_, err = s.db.NewInsert().Model(history).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create room history: %w", err)
	}

	return nil
}