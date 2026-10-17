package peopledirectory

import (
	"context"
	"encoding/json"
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
