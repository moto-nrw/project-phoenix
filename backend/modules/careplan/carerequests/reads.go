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
	// FindStudentRequest returns one request of the child, or ErrNotFound.
	FindStudentRequest(ctx context.Context, studentID, requestID int64) (*Request, error)
}

// PickupPermissions is the part of Submissions and Reads the pickup
// permission flow (#4780) needs: create, list and find one request.
type PickupPermissions interface {
	CreatePickupPermission(context.Context, PickupPermissionCreateInput) (*Request, error)
	ListPickupPermissionRequests(context.Context, int64, time.Time) ([]Request, error)
	FindStudentRequest(ctx context.Context, studentID, requestID int64) (*Request, error)
}
