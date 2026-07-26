package active

import (
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// SnapshotSource constants (#1417) record how a month came to be frozen.
// Only 'admin' is produced today; 'scheduler' is reserved for the automatic
// close that #1417 defers to a follow-up, 'migration' for backfills.
const (
	SnapshotSourceAdmin     = "admin"
	SnapshotSourceScheduler = "scheduler"
	SnapshotSourceMigration = "migration"
)

// ValidSnapshotSources lists all valid snapshot sources.
var ValidSnapshotSources = []string{
	SnapshotSourceAdmin,
	SnapshotSourceScheduler,
	SnapshotSourceMigration,
}

// StaffMonthBalanceSnapshot is the frozen closing balance of one staff member's
// month (#1417). It exists because the Stundenkonto is otherwise computed live
// all the way back to the account start, so a retroactive shift or session edit
// rewrites the balance of every month after it.
//
// Once a month is frozen, the carry chain for later months starts at
// ClosingBalanceMinutes instead of re-summing from the anchor. The frozen month
// itself is still recomputed for display; the gap between the recomputed and
// the frozen value is reported as drift.
//
// This is not a booking. StaffBalanceAdjustment (#1420) carries a signed
// MinutesDelta and moves the balance; this row carries no delta and moves
// nothing — it only pins where the chain starts.
//
// The remaining fields beyond ClosingBalanceMinutes are a display copy of the
// month as it stood at close time, so the Monatskarte can show what was frozen
// without re-deriving it.
//
// A reopened snapshot is superseded, not deleted: ReopenedAt/ReopenedBy/
// ReopenReason stay on the row, which makes the row its own audit record.
type StaffMonthBalanceSnapshot struct {
	base.Model `bun:"schema:active,table:staff_month_balance_snapshots"`
	base.TenantModel
	StaffID               int64      `bun:"staff_id,notnull" json:"staff_id"`
	Year                  int        `bun:"year,notnull" json:"year"`
	Month                 int        `bun:"month,notnull" json:"month"`
	ClosingBalanceMinutes int        `bun:"closing_balance_minutes,notnull" json:"closing_balance_minutes"`
	CarryInMinutes        int        `bun:"carry_in_minutes,notnull" json:"carry_in_minutes"`
	TargetMinutes         int        `bun:"target_minutes,notnull" json:"target_minutes"`
	ActualMinutes         int        `bun:"actual_minutes,notnull" json:"actual_minutes"`
	CreditedMinutes       int        `bun:"credited_minutes,notnull" json:"credited_minutes"`
	AdjustmentMinutes     int        `bun:"adjustment_minutes,notnull" json:"adjustment_minutes"`
	ClosedAt              time.Time  `bun:"closed_at,notnull,default:current_timestamp" json:"closed_at"`
	ClosedBy              int64      `bun:"closed_by,notnull" json:"closed_by"`
	CloseReason           string     `bun:"close_reason,notnull,default:''" json:"close_reason,omitempty"`
	Source                string     `bun:"source,notnull,default:'admin'" json:"source"`
	ReopenedAt            *time.Time `bun:"reopened_at" json:"reopened_at,omitempty"`
	ReopenedBy            *int64     `bun:"reopened_by" json:"reopened_by,omitempty"`
	ReopenReason          string     `bun:"reopen_reason,notnull,default:''" json:"reopen_reason,omitempty"`
}

// Validate validates the snapshot record.
func (s *StaffMonthBalanceSnapshot) Validate() error {
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
	if !slices.Contains(ValidSnapshotSources, s.Source) {
		return errors.New("invalid snapshot source")
	}
	return nil
}

// IsActive reports whether this snapshot still freezes its month. A reopened
// row is superseded history and must never seed a carry chain.
func (s *StaffMonthBalanceSnapshot) IsActive() bool {
	return s.ReopenedAt == nil
}
