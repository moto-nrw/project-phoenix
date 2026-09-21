package timetable

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalidClassArrivalException = errors.New("invalid class arrival exception")

// ClassArrivalExceptionInput contains writable fields only. The current tenant
// and persisted identity are selected by Timetable, never by the caller.
type ClassArrivalExceptionInput struct {
	SchoolClass string
	Date        string
	ArrivalTime time.Time
	Reason      *string
	CreatedBy   *int64
	Origin      string
}

func (v ClassArrivalExceptionInput) Valid() bool {
	return strings.TrimSpace(v.SchoolClass) != "" && validDate(v.Date) && !v.ArrivalTime.IsZero() &&
		(v.Reason == nil || utf8.RuneCountInString(*v.Reason) <= 255) &&
		(v.CreatedBy == nil || *v.CreatedBy > 0) && (v.Origin == "" || v.Origin == "ogs" || v.Origin == "school")
}

type ClassArrivalExceptions interface {
	ListClassArrivalExceptions(context.Context, []string, string, string) ([]ClassArrivalException, error)
	UpsertClassArrivalException(context.Context, ClassArrivalExceptionInput) (ClassArrivalException, error)
	DeleteClassArrivalException(context.Context, string, string) (bool, error)
}

type ClassArrivals interface {
	ClassArrivalQuery
	ClassArrivalExceptions
	// LockClassArrivalPlan must be held in the same transaction as a plan's
	// read-modify-write update. It also serializes concurrent first inserts.
	LockClassArrivalPlan(context.Context, string) error
	UpsertClassArrivalPlan(context.Context, ClassArrivalPlanInput) (ClassArrivalPlan, error)
}
