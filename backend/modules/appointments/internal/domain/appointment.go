package domain

import "time"

// Date is the storage form of a calendar day in the Postgres rows. It is a
// plain string, so BUN binds it verbatim and the day cannot shift between
// time zones (TestDateColumnTypes). Everything above the rows uses the public
// appointments.Date.
type Date string

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
