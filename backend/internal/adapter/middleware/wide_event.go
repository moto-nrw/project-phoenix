package middleware

import (
	"context"
	"time"
)

type contextKey string

const wideEventKey contextKey = "wide_event"

// WideEvent captures request-scoped context for canonical logging.
type WideEvent struct {
	// Request metadata
	Timestamp  time.Time
	RequestID  string
	Method     string
	Path       string
	StatusCode int
	DurationMS int64

	// Service metadata
	Service     string
	Version     string
	Environment string

	// User context
	UserID    string
	UserRole  string
	AccountID string

	// Business context
	StudentID  string
	GroupID    string
	ActivityID string
	RoomID     string
	Action     string
	ResourceID string

	// Error context
	ErrorType    string
	ErrorCode    string
	ErrorMessage string
}

// GetWideEvent retrieves the current request event from context.
// It returns an empty event if none is found.
func GetWideEvent(ctx context.Context) *WideEvent {
	if ctx == nil {
		return &WideEvent{}
	}
	if event, ok := ctx.Value(wideEventKey).(*WideEvent); ok && event != nil {
		return event
	}
	return &WideEvent{}
}

func withWideEvent(ctx context.Context, event *WideEvent) context.Context {
	return context.WithValue(ctx, wideEventKey, event)
}
