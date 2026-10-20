package domain

import (
	"time"
)

const DefaultDeviceOnlineWindow = 5 * time.Minute

type UpdateDevice struct {
	ID                    int64
	UpdatedAt             time.Time
	DeviceID              string
	DeviceType            string
	Name                  *string
	Status                string
	APIKey                *string
	LastSeen              *time.Time
	RegisteredByID        *int64
	RoomID                *int64
	ArchivedAt            *time.Time
	TransferredToDeviceID *int64
}

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

// Add folds another statement's counters into s.
func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}
