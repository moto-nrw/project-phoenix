package domain

import (
	"time"
)

// CalendarPeriodOverlapError names the active same-type periods a mutation
// would collide with; it unwraps to ErrCalendarPeriodOverlapConflict.

// Dateframe ranges are instants: the table stores TIMESTAMPTZ columns.

// CalendarPeriod dates are calendar days in YYYY-MM-DD; WeekCycleAnchor is
// empty when unset.

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
