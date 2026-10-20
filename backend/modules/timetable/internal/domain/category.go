package domain

import (
	"time"
)

const CategoryNameActiveIndex = "idx_categories_tenant_name_active"

type OperationStats struct {
	Queries                      int64
	Rows                         int64
	DuplicatePreventionConflicts int64
	StatementDuration            time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.DuplicatePreventionConflicts += other.DuplicatePreventionConflicts
	s.StatementDuration += other.StatementDuration
}
