package domain

import (
	"net"
	"time"
)

type AuditEntry struct {
	OperatorID int64
	Action     string
	ResourceID int64
	RequestIP  net.IP
	Changes    map[string]any
}

type OperationStats struct {
	Queries                      int
	Rows                         int64
	StatementDuration            time.Duration
	DuplicatePreventionConflicts int
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
	s.DuplicatePreventionConflicts += other.DuplicatePreventionConflicts
}
