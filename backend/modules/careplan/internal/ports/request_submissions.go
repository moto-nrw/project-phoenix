package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type RequestSubmissionRecords interface {
	Create(context.Context, carerequests.Request) (carerequests.Request, error)
	IsPendingConflict(error) bool
}

type RequestSubmissionPickup interface {
	PickupTime(context.Context, int64, calendar.Date) (*time.Time, error)
}

type RequestSubmissionEffects interface {
	RecordSubmission(context.Context, *carerequests.Request) error
	NotifySubmission(context.Context, *carerequests.Request) error
	WakeGuardians(context.Context, *carerequests.Request)
}
