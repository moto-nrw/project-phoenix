package requestreview

import (
	"context"
	"time"
)

// Date is a calendar day (YYYY-MM-DD) without clock or zone, the same shape
// the owner contracts the ports adapt use.
type Date string

func (d Date) String() string { return string(d) }

// Cursor is the keyset position after one page of a queue: the instant the
// page sorted on (created_at on the open queue, updated_at in the history,
// changed_at for a correction) and the row ID. A nil cursor means the page
// was the last one.
type Cursor struct {
	Instant time.Time
	ID      int64
}

// QueueFilter is the part of a listing the owner queues evaluate themselves:
// which children, the urgency phase, and the keyset the projection fills in.
// Every port reads the tenant in context; none of them writes.
type QueueFilter struct {
	// UrgentOnly selects the open queue's urgency phase. Nil leaves urgency
	// unrestricted (history); true/false select exactly one phase.
	UrgentOnly *bool
	// UrgentDate is the calendar day (YYYY-MM-DD) urgency is judged against.
	UrgentDate string
	// StudentIDs limits the queue to a set of children (the conflict scan).
	// Empty means no restriction; it composes with StudentID.
	StudentIDs []int64
	// StudentID limits the queue to one child. Zero means every child.
	StudentID int64
	// Search matches the child's name case-insensitively.
	Search string
	// Before is the keyset position of the last row the caller consumed. Nil
	// returns the first page.
	Before *Cursor
	// Limit caps the page. Zero or less means unbounded.
	Limit int
}

// Row is one request of any queue as the projection merges, filters and
// serializes it. The owner adapters translate their typed items into this
// shape and derive the per-type facts (urgency, past scope, conflict keys,
// correctability) with the owner's own rules; the projection decides the
// order, the paging, the bulk consequences and the conflict grouping.
type Row struct {
	// Type is the wire request type (TypeMasterData ... TypeDirectCorrection).
	Type string
	ID   int64
	// SortTime is the keyset instant of the underlying row: created_at on the
	// open queue, updated_at in the history, changed_at for a correction.
	SortTime    time.Time
	StudentID   int64
	StudentName string
	Status      string
	// Version is the expected_version the decide routes compare against,
	// rendered by the owner rule from the row's updated_at.
	Version string
	// UrgentToday marks a request that touches today (open queue).
	UrgentToday bool
	// Past marks a request whose whole effective scope lies before today.
	Past bool
	// BulkEligible and the ineligibility reason/text come from the owner's
	// bulk-approval rule; the projection overrides them for past rows.
	BulkEligible         bool
	BulkIneligibleReason string
	BulkIneligibleText   string
	// ConflictKeys name what this request would write.
	ConflictKeys []string
	// CurrentValueChanged warns that the OGS changed the value after the
	// request was filed; nil where the type keeps no submission baseline.
	CurrentValueChanged *bool
	// CurrentStatusByDate is what each requested day looks like today
	// (absence requests only).
	CurrentStatusByDate map[string]string
	// CanCorrect marks a decided request whose decision staff can still
	// rewrite (history).
	CanCorrect bool
	// DecidedAt is the display decision instant the history range filter
	// reads (history).
	DecidedAt time.Time
	// Data is the unchanged per-type projection (the wire shapes in wire.go).
	Data any
}

// Queue is one owner's request queue: its open requests, its decided
// requests, and the number of open requests the caller may review.
type Queue interface {
	// Open lists the pending requests the caller may review, newest first,
	// strictly before the filter's keyset position.
	Open(ctx context.Context, filter QueueFilter) ([]Row, *Cursor, error)
	// History lists the decided requests the caller may review, newest
	// decision first.
	History(ctx context.Context, filter QueueFilter) ([]Row, *Cursor, error)
	// OpenCount is the number of pending requests the caller may review.
	OpenCount(ctx context.Context, today Date) (int, error)
}

// CorrectionLog is the office's own booking corrections. A correction has no
// open state, so it only contributes to the history.
type CorrectionLog interface {
	History(ctx context.Context, filter QueueFilter) ([]Row, *Cursor, error)
}

// Queues are the four parent-request queues plus the correction log.
type Queues struct {
	MasterData        Queue
	CareSchedule      Queue
	Offering          Queue
	Excused           Queue
	DirectCorrections CorrectionLog
}

// Caller carries the request principal's projection-relevant rights.
type Caller struct {
	// ReviewsWriteQueues reports whether the caller may open the Stammdaten,
	// Betreuungszeiten and Angebote queues (users:update). Without it only
	// the excused-absence queue is served.
	ReviewsWriteQueues bool
}

// Access resolves the caller's rights for the current request.
type Access interface {
	Caller(ctx context.Context) (Caller, error)
	// ReviewAccess is the caller's coarse reach over the queues ("admin",
	// "group_leader", "none"), or "" when no review policy is configured.
	ReviewAccess(ctx context.Context) (string, error)
}

// StudentDirectory resolves the group each child belongs to.
type StudentDirectory interface {
	// GroupNames returns the group name per student ID; children without a
	// group are absent.
	GroupNames(ctx context.Context, studentIDs []int64) (map[int64]string, error)
}

// FamilyProtection reports the Familienschutz flag per child.
type FamilyProtection interface {
	Protected(ctx context.Context, studentIDs []int64) (map[int64]bool, error)
}
