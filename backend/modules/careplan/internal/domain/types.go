package domain

import (
	"time"
)

type OperationStats struct {
	Queries           int64
	Rows              int64
	Conflicts         int64
	StatementDuration time.Duration
}

type RequestStoreStats = OperationStats

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.Conflicts += other.Conflicts
	s.StatementDuration += other.StatementDuration
}
