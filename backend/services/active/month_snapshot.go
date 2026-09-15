package active

import (
	"context"
	"time"
)

// MonthSnapshot is the frozen carry balance consumed by month-close and overview.
type MonthSnapshot struct {
	ID                    int64      `json:"id"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	TenantID              int64      `json:"tenant_id"`
	StaffID               int64      `json:"staff_id"`
	Year                  int        `json:"year"`
	Month                 int        `json:"month"`
	ClosingBalanceMinutes int        `json:"closing_balance_minutes"`
	CarryInMinutes        int        `json:"carry_in_minutes"`
	TargetMinutes         int        `json:"target_minutes"`
	ActualMinutes         int        `json:"actual_minutes"`
	CreditedMinutes       int        `json:"credited_minutes"`
	AdjustmentMinutes     int        `json:"adjustment_minutes"`
	ClosedAt              time.Time  `json:"closed_at"`
	ClosedBy              int64      `json:"closed_by"`
	CloseReason           string     `json:"close_reason,omitempty"`
	Source                string     `json:"source"`
	ReopenedAt            *time.Time `json:"reopened_at,omitempty"`
	ReopenedBy            *int64     `json:"reopened_by,omitempty"`
	ReopenReason          string     `json:"reopen_reason,omitempty"`
}

func (s *MonthSnapshot) IsActive() bool { return s.ReopenedAt == nil }

const SnapshotSourceAdmin = "admin"

// MonthSnapshots is the consumer-owned port for frozen balances.
type MonthSnapshots interface {
	LatestClosedMonth(context.Context, int64, int, int) (*MonthSnapshot, error)
	ClosedMonthSnapshots(context.Context, int, int) ([]*MonthSnapshot, error)
	ClosedMonthSnapshotsForStaff(context.Context, []int64) ([]*MonthSnapshot, error)
	RecordClosedMonth(context.Context, MonthSnapshot) (MonthSnapshot, error)
	ReopenMonthSnapshot(context.Context, int64, int64, time.Time, string) (int64, error)
	LockStaffBalanceWrites(context.Context, int64) error
}
