package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// ReviewBooking is the Enrollment-owned selection used by a care baseline.
// ValidUntil is exclusive; an empty bound is open-ended.
type ReviewBooking struct {
	StudentID    int64
	OfferingID   int64
	SelectedDays []string
	ValidFrom    domain.Date
	ValidUntil   domain.Date
}

type ReviewBookings interface {
	ApprovedForStudents(context.Context, []int64, domain.Date, domain.Date) ([]ReviewBooking, error)
}

type ReviewClassTimes interface {
	ListClassArrivalTimes(context.Context, []string) (map[string]map[string]string, error)
}
