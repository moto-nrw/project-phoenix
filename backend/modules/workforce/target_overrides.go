package workforce

import (
	"context"
	"errors"
)

var (
	ErrStaffTargetOverrideNotFound = errors.New("staff target override not found")
	ErrInvalidStaffTargetOverride  = errors.New("invalid staff target override")
	// ErrStaffTargetOverrideRejected marks a well-formed Sonderarbeitszeit the
	// time account cannot take: it overlaps another one or touches a closed
	// month.
	ErrStaffTargetOverrideRejected = errors.New("staff target override rejected")
)

// TargetOverrideError carries the caller-facing reason of an invalid or
// rejected Sonderarbeitszeit and unwraps to its kind.
type TargetOverrideError struct {
	Kind   error
	Reason string
}

func (e *TargetOverrideError) Error() string { return e.Reason }
func (e *TargetOverrideError) Unwrap() error { return e.Kind }

// StaffTargetOverride is a Sonderarbeitszeit (#3259): on every Monday to
// Friday in [StartDate, EndDate] the staff member's daily target is
// DailyMinutes. It wins over closure days and the work-time schedule;
// statutory holidays stay at zero. Dates use DateLayout, EndDate is inclusive.
type StaffTargetOverride struct {
	ID           int64  `json:"id"`
	StaffID      int64  `json:"staff_id"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	DailyMinutes int    `json:"daily_minutes"`
	CreatedBy    *int64 `json:"created_by,omitempty"`
}

// StaffTargetOverrideFields is the writable part of a Sonderarbeitszeit.
type StaffTargetOverrideFields struct {
	StartDate    string
	EndDate      string
	DailyMinutes int
}

// TargetSourceOverride is the DailyProjection.TargetSource of a day whose
// Soll a Sonderarbeitszeit set.
const TargetSourceOverride = "override"

// DailyProjection is the priced work-time picture of one calendar day.
// TargetSource names what set the Soll; empty is the regular schedule.
type DailyProjection struct {
	Date           string `json:"date"`
	TargetMinutes  int    `json:"target_minutes"`
	TargetSource   string `json:"target_source,omitempty"`
	CreditMinutes  int    `json:"credit_minutes"`
	ActualMinutes  int    `json:"actual_minutes"`
	BalanceMinutes int    `json:"balance_minutes"`
}

// DailyTarget is the contractual Soll of one calendar day.
type DailyTarget struct {
	Date          string `json:"date"`
	TargetMinutes int    `json:"target_minutes"`
}

// TargetOverrideDays maps staff member and calendar day (DateLayout) to the
// daily target a Sonderarbeitszeit sets on that day. Days without one are
// absent.
type TargetOverrideDays map[int64]map[string]int

// StaffTargetOverrideQuery reads Sonderarbeitszeiten.
type StaffTargetOverrideQuery interface {
	// ListStaffTargetOverrides returns every range of one staff member
	// ordered by start date.
	ListStaffTargetOverrides(ctx context.Context, staffID int64) ([]StaffTargetOverride, error)
	// StaffTargetOverrideDays expands the ranges touching [from, to] into the
	// days they set a target on: Monday to Friday, never a statutory holiday.
	// Every Soll reader resolves a day through this one expansion.
	StaffTargetOverrideDays(ctx context.Context, staffIDs []int64, from, to string) (TargetOverrideDays, error)
}

// StaffTargetOverrideCommand writes Sonderarbeitszeiten. Every write takes the
// staff balance lock and refuses overlapping ranges and closed months.
type StaffTargetOverrideCommand interface {
	CreateStaffTargetOverride(ctx context.Context, staffID int64, fields StaffTargetOverrideFields, createdBy *int64) (StaffTargetOverride, error)
	DeleteStaffTargetOverride(ctx context.Context, staffID, id int64) error
}

// StaffTargetOverrides is the full Sonderarbeitszeit capability.
type StaffTargetOverrides interface {
	StaffTargetOverrideQuery
	StaffTargetOverrideCommand
}

type targetOverrideEngine interface {
	StaffTargetOverrides
}

func (m *Module) ListStaffTargetOverrides(ctx context.Context, staffID int64) ([]StaffTargetOverride, error) {
	if staffID <= 0 {
		return nil, invalid("staff ID is required")
	}
	return m.engine.ListStaffTargetOverrides(ctx, staffID)
}

func (m *Module) StaffTargetOverrideDays(ctx context.Context, staffIDs []int64, from, to string) (TargetOverrideDays, error) {
	if len(staffIDs) == 0 {
		return TargetOverrideDays{}, nil
	}
	return m.engine.StaffTargetOverrideDays(ctx, staffIDs, from, to)
}

func (m *Module) CreateStaffTargetOverride(ctx context.Context, staffID int64, fields StaffTargetOverrideFields, createdBy *int64) (StaffTargetOverride, error) {
	if staffID <= 0 {
		return StaffTargetOverride{}, invalid("staff ID is required")
	}
	return m.engine.CreateStaffTargetOverride(ctx, staffID, fields, createdBy)
}

func (m *Module) DeleteStaffTargetOverride(ctx context.Context, staffID, id int64) error {
	if staffID <= 0 || id <= 0 {
		return invalid("staff ID and override ID are required")
	}
	return m.engine.DeleteStaffTargetOverride(ctx, staffID, id)
}
