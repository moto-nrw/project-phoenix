package timetable

import (
	"context"
	"errors"
	"time"
)

// ClassArrivalPlan preserves the entered class label and recurring dismissal
// times. Class matching uses trimmed, case-insensitive keys, not the label.
type ClassArrivalPlan struct {
	ID           int64
	TenantID     int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SchoolClass  string
	ArrivalTimes map[string]string
	UpdatedBy    *int64
}

// ClassArrivalException is a class-wide dismissal change. Date is the
// timezone-free YYYY-MM-DD value used by Timetable's public contracts.
type ClassArrivalException struct {
	ID          int64
	TenantID    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	SchoolClass string
	Date        string
	ArrivalTime time.Time
	Reason      *string
	CreatedBy   *int64
	Origin      string
}

var ErrInvalidClassArrivalExceptionRange = errors.New("invalid class arrival exception date range")

func ValidClassArrivalExceptionDates(from, to string) bool {
	return validDate(from) && validDate(to)
}

// ClassArrivalQuery supplies the recurring dismissal times used by Care Plan.
// Class keys are trimmed and lowercased; weekday keys and HH:MM values retain
// their persisted form. Only requested classes in the current tenant return.
type ClassArrivalQuery interface {
	ListClassArrivalTimes(context.Context, []string) (map[string]map[string]string, error)
	ListClassArrivalPlans(context.Context, []string) ([]ClassArrivalPlan, error)
	ListClassArrivalExceptions(context.Context, []string, string, string) ([]ClassArrivalException, error)
}
