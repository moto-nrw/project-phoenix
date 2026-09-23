// Package timetracking holds the retained Workforce time-tracking services
// while they wait for their dissolution into the owner's application and
// domain layers (#3213). It is compatibility code: the services still speak
// the retained time-record vocabulary and are served behind the public
// modules/workforce contracts by the composition root. Nothing in here is a
// target dependency; the package goes with the last retained contract.
package timetracking

import (
	"context"
	"log/slog"
	"time"
)

// DatabaseHandle marks a live database behind the tenant unit of work. The
// retained services only test it for presence: a nil handle is the shape unit
// tests build, where every repository is a double and nothing needs a
// transaction, while production wiring always supplies one.
type DatabaseHandle interface {
	PingContext(context.Context) error
}

// EventPublisher queues the tenant-wide "staff time tracking changed"
// notification for delivery after the surrounding transaction commits. The
// composition root binds it to the realtime hub; the retained services never
// reach the Delivery platform themselves.
type EventPublisher interface {
	QueueStaffTimeTrackingChanged(ctx context.Context, logger *slog.Logger)
}

// DeletionAudit writes retention evidence in the deletion transaction.
type DeletionAudit interface {
	Create(context.Context, *DeletionEvent) error
}

// DeletionEvent carries the subject and evidence of a retention operation.
type DeletionEvent struct {
	TenantID       int64
	StudentID      *int64
	StaffID        *int64
	DeletionType   string
	RecordsDeleted int
	DeletionReason string
	DeletedBy      string
	DeletedAt      time.Time
	Metadata       map[string]interface{}
}

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

// ordered lists the field types the retained sort helpers compare.
type ordered interface {
	~int | ~int64 | ~float64 | ~string
}

// compareOrdered returns -1, 0 or +1 like a three-way comparison.
func compareOrdered[T ordered](a, b T) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// orderOr returns the first non-zero comparison result, so callers can chain
// a primary order with a tiebreak.
func orderOr(order, tiebreak int) int {
	if order != 0 {
		return order
	}
	return tiebreak
}

// loggerOrDefault keeps bare-constructed services nil-safe.
func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}
