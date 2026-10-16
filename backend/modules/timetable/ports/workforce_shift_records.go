package ports

import (
	"errors"
	"time"
)

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
