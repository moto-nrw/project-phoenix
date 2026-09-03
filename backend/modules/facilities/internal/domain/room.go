package domain

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("room not found")

type Room struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	Building  string
	Floor     *int
	Capacity  *int
	Category  *string
	Color     *string
	IsSystem  bool
}

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}
