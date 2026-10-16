package application

import (
	"context"
	"errors"
	"time"
)

// The retained models/schedule Dienstplan rows are Workforce records
// (#2689). This composition serves those rows to the retained consumers that
// still speak them (the coverage probe, the staff calendar feed, the plan
// export and the legacy suites); Workforce's capability arrives through the
// WorkforceShiftRecords seam below, in this composition's own vocabulary,
// because the Timetable owner may not name Workforce. The composition root
// binds the capability behind it.

// WorkforceShift is one planned staff shift.
type WorkforceShift struct {
	ID                   int64
	TenantID             int64
	StaffID              int64
	Date                 string
	StartTime            string
	EndTime              string
	BreakMinutes         int
	ShiftTypeID          *int64
	Notes                string
	SeriesID             *int64
	Detached             bool
	SeriesOccurrenceDate string
	Cancelled            bool
	ChangeReason         *string
	OriginShiftID        *int64
	SickAbsenceID        *int64
	CreatedBy            int64
	UpdatedBy            *int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// WorkforceShiftType is one tenant-defined Schichtart.
type WorkforceShiftType struct {
	ID          int64
	TenantID    int64
	Name        string
	Color       string
	Description string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// WorkforceShiftOrderField names a column a shift listing may be ordered by.
type WorkforceShiftOrderField string

// The order columns the retained rows list by.
const (
	WorkforceShiftOrderDate      WorkforceShiftOrderField = "date"
	WorkforceShiftOrderStaffID   WorkforceShiftOrderField = "staff_id"
	WorkforceShiftOrderStartTime WorkforceShiftOrderField = "start_time"
)

// WorkforceShiftOrder is one ordering column of a shift listing.
type WorkforceShiftOrder struct {
	Field WorkforceShiftOrderField
}

// WorkforceShiftFilter selects planned shifts.
type WorkforceShiftFilter struct {
	StaffID  int64
	StaffIDs []int64
	From     string
	To       string
	Dates    []string
	Order    []WorkforceShiftOrder
}

// The Workforce answers the root marks its errors with (MarkRecordError), so
// the rows keep their retained error shapes.
var (
	ErrWorkforceShiftNotFound = errors.New("workforce staff shift not found")
	ErrWorkforceShiftInvalid  = errors.New("invalid workforce shift input")
)

// WorkforceShiftRecords is the Workforce capability the Dienstplan rows run
// over.
type WorkforceShiftRecords interface {
	CreateStaffShift(context.Context, WorkforceShift) (WorkforceShift, error)
	FindStaffShift(context.Context, int64) (WorkforceShift, error)
	UpdateStaffShift(context.Context, WorkforceShift) (WorkforceShift, error)
	DeleteStaffShift(context.Context, int64) error
	ListStaffShifts(context.Context, WorkforceShiftFilter) ([]WorkforceShift, error)
	UsedStaffShiftWeeks(ctx context.Context, from, to string) ([]string, error)

	CreateShiftType(context.Context, WorkforceShiftType) (WorkforceShiftType, error)
	DeleteShiftType(context.Context, int64) error
	ListShiftTypes(context.Context) ([]WorkforceShiftType, error)
}
