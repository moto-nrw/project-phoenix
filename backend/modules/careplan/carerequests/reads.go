package carerequests

import (
	"context"
	"time"
)

// Reads requires the caller to authorize access to the student's requests.
type Reads interface {
	GetPendingForStudent(context.Context, int64) (*Request, []DiffEntry, error)
	ListPickupChangeRequests(context.Context, int64, time.Time) ([]Request, error)
}
