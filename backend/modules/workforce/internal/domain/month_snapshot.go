package domain

import (
	"errors"
	"slices"
	"time"
)

type StaffMonthBalanceSnapshot struct {
	ID                    int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	TenantID              int64
	StaffID               int64
	Year                  int
	Month                 int
	ClosingBalanceMinutes int
	CarryInMinutes        int
	TargetMinutes         int
	ActualMinutes         int
	CreditedMinutes       int
	AdjustmentMinutes     int
	ClosedAt              time.Time
	ClosedBy              int64
	CloseReason           string
	Source                string
	ReopenedAt            *time.Time
	ReopenedBy            *int64
	ReopenReason          string
}

func (s StaffMonthBalanceSnapshot) Validate() error {
	if s.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if s.Month < 1 || s.Month > 12 {
		return errors.New("month must be between 1 and 12")
	}
	if s.Year < 2000 || s.Year > 2100 {
		return errors.New("year must be between 2000 and 2100")
	}
	if s.ClosedBy <= 0 {
		return errors.New("closed_by is required")
	}
	if !slices.Contains([]string{"admin", "scheduler", "migration"}, s.Source) {
		return errors.New("invalid snapshot source")
	}
	return nil
}
