package peopledirectory

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ParentRequestEventSnapshot is the complete append-only ledger record.
type ParentRequestEventSnapshot struct {
	ID             int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	TenantID       int64
	StudentID      int64
	RequestType    string
	RequestID      int64
	EventType      string
	ActorAccountID *int64
	Version        string
	Payload        json.RawMessage
}

type ParentRequestEvents interface {
	AppendParentRequestEvent(context.Context, *ParentRequestEventSnapshot) error
	ParentRequestEventsForRequest(context.Context, string, int64) ([]*ParentRequestEventSnapshot, error)
}

// ErrParentRequestEventIncomplete refuses a ledger event that cannot name its
// request.
var ErrParentRequestEventIncomplete = errors.New("parent request event: student, request and event type are required")

// ValidateParentRequestEvent checks that a ledger event names the child, the
// request it belongs to and what happened to it. A missing part returns
// ErrParentRequestEventIncomplete.
func ValidateParentRequestEvent(studentID int64, requestType string, requestID int64, eventType string) error {
	if studentID <= 0 || requestID <= 0 || requestType == "" || eventType == "" {
		return ErrParentRequestEventIncomplete
	}
	return nil
}
