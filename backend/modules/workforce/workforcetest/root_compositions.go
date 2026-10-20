// Package workforcetest is the Workforce owner's test support. It holds the
// seams through which the internal route tests of the owner's HTTP packages
// reach the typed root's compositions: the root imports those packages, so
// their internal tests cannot import it, and an external test file in each
// directory binds the seams (#4005).
package workforcetest

import (
	"context"
	"testing"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// AdminCapabilities are the public Workforce contracts the staff
// administration and time-tracking routes consume, as the root binds them.
type AdminCapabilities struct {
	Staff              workforce.StaffDirectory
	Documents          workforce.StaffDocuments
	WorkSessions       workforce.WorkSessions
	StaffAbsences      workforce.StaffAbsences
	WorkTimeMonth      workforce.WorkTimeMonths
	BalanceAdjustments workforce.BalanceAdjustments
	MonthClosing       workforce.MonthClosing
	Overview           workforce.StaffOverview
	AuditLog           workforce.TimeTrackingAuditLog
	TimeExport         workforce.StaffTimeExport
}

// Rows read back the native values the route suites assert on: the staff
// absences, and the shift and staff fields the absence cascade and the
// schedule writes change.
type Rows struct {
	StaffAbsence interface {
		FindStaffAbsence(context.Context, int64) (workforce.StaffAbsence, error)
		ListStaffAbsences(context.Context, workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error)
	}
	FindShift func(ctx context.Context, shiftID int64) (*ShiftRow, error)
	FindStaff func(ctx context.Context, staffID int64) (*StaffRow, error)
}

// ShiftRow is the part of a planned shift the absence cascade changes.
type ShiftRow struct {
	Cancelled     bool
	SickAbsenceID *int64
}

// StaffRow is the part of a staff record the schedule writes change.
type StaffRow struct {
	WorkTimeModelID *int64
}

// The compositions an external test file binds for the internal suites of
// its directory.
var (
	// ComposeAdmin composes the root's workforce graph over a fresh test
	// database and adapts it as the root does.
	ComposeAdmin func(t *testing.T, clocks ...func() time.Time) (*bun.DB, AdminCapabilities)
	// ComposeCapability composes the Workforce module over db, its clock
	// pinned to now unless now is nil.
	ComposeCapability func(t *testing.T, db *bun.DB, now func() time.Time) workforce.Capability
	// ComposeRows composes the retained rows over db.
	ComposeRows func(t *testing.T, db *bun.DB) Rows
)
