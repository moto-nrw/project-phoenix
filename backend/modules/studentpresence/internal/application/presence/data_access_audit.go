package presence

import (
	"context"
	"time"
)

// DataAccessAudit records access evidence in the caller's tenant transaction.
type DataAccessAudit interface {
	Create(context.Context, *DataAccessEvent) error
}

// DataAccessEvent describes disclosed data without exposing audit persistence.
type DataAccessEvent struct {
	ActorAccountID int64
	ActorRole      string
	ResourceType   string
	StudentID      *int64
	RangeStart     time.Time
	RangeEnd       time.Time
	AccessedAt     time.Time
	Metadata       map[string]interface{}
}
