package carerequests

import (
	"context"
	"time"
)

// Reads requires the caller to authorize access to the student's requests.
type Reads interface {
	GetPendingForStudent(context.Context, int64) (*Request, []DiffEntry, error)
	ListPickupChangeRequests(context.Context, int64, time.Time) ([]Request, error)
	// ListPickupPermissionRequests lists the child's pickup permission
	// requests created since the instant, newest first (#4780).
	ListPickupPermissionRequests(context.Context, int64, time.Time) ([]Request, error)
}
