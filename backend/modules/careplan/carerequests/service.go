package carerequests

import (
	"context"
	"time"

	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Service owns the weekly-care and pickup-change request lifecycle.
// Callers authorize guardian access and supply the ambient tenant transaction.
// Decisions enforce staff review access; effects are registered after commit.
type Service interface {
	Submissions
	Reads
	Edits
	Decisions

	// CreatePickupChangeRequest is the mandatory-reason submission entry point.
	CreatePickupChangeRequest(context.Context, int64, int64, calendar.Date, time.Time, string) (*Request, error)
}
