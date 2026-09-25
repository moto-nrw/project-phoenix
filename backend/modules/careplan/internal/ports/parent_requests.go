package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
)

// ParentRequestRights is what the caller may decide across the request
// queues. The permission model stays with its owner; the coordinator only
// applies the answer.
type ParentRequestRights struct {
	// WriteQueues covers the Stammdaten, Betreuungszeiten and Angebote
	// queues (users:update).
	WriteQueues bool
	// Absences covers the sick and excused queue (users:update or
	// users:absence).
	Absences bool
}

// ParentRequestRightsResolver answers the caller's rights from the request
// context.
type ParentRequestRightsResolver func(ctx context.Context) ParentRequestRights

// RollbackMarker asks the ambient tenant transaction to roll back when the
// command fails, so a half-applied bulk or resolve can never commit.
type RollbackMarker interface {
	MarkRollback(ctx context.Context)
}

// ConflictCandidate is the minimum the resolver needs to validate a group
// before touching it. The payload stays with the request kind.
type ConflictCandidate struct {
	StudentID int64
	UpdatedAt time.Time
}

// ConflictDecision is one verdict inside a resolve command.
type ConflictDecision struct {
	RequestID int64
	Approve   bool
	Reason    string
	// ReviewerID is the acting staff account; ActorRole their roles as the
	// audit trails record them.
	ReviewerID      int64
	ActorRole       string
	ExpectedVersion string
}

// StaffValueWrite is the staff member's own result for a conflict group,
// written through the kind's normal staff write path. Value is the kind's
// payload shape, unread by the resolver.
type StaffValueWrite struct {
	StudentID int64
	// RequestIDs are the requests just rejected, in the order the client
	// sent them; the kind reads the group's scope from them.
	RequestIDs []int64
	// ConflictKey is the group's key exactly as the list emitted it.
	ConflictKey string
	ReviewerID  int64
	ActorRole   string
	Reason      string
	Value       map[string]any
}

// ConflictPort is what one request kind contributes to the resolve command.
// WriteStaffValue answers parentrequests.ErrStaffValueUnsupported when the
// kind accepts no typed result.
type ConflictPort interface {
	ConflictCandidate(ctx context.Context, requestID int64) (*ConflictCandidate, error)
	LockConflictRequest(ctx context.Context, requestID int64) error
	DecideConflictRequest(ctx context.Context, decision ConflictDecision) error
	WriteStaffValue(ctx context.Context, write StaffValueWrite) error
}

// ExcusedBulkCandidate is the absence queue's contribution to a bulk
// approval.
type ExcusedBulkCandidate struct {
	ID        int64
	StudentID int64
	UpdatedAt time.Time
	Eligible  bool
}

// ExcusedBulkPort is the absence queue's side of the bulk approval.
type ExcusedBulkPort interface {
	GetExcusedBulkCandidate(ctx context.Context, requestID int64) (*ExcusedBulkCandidate, error)
	LockExcusedBulkRequest(ctx context.Context, requestID int64) error
	ApproveExcusedBulk(ctx context.Context, requestID int64, reason string, reviewerID int64, expectedVersion string) error
}

// MasterDataBulkPort is the Stammdaten queue's side of the bulk approval. It
// also holds the one student-lock frontier every cross-kind command takes
// after its request rows.
type MasterDataBulkPort interface {
	GetBulkCandidate(ctx context.Context, requestID int64) (*careplan.MasterDataReviewItem, error)
	LockBulkRequest(ctx context.Context, requestID int64) error
	LockBulkStudents(ctx context.Context, studentIDs []int64) error
	Decide(ctx context.Context, input masterdatarequests.DecideInput) (*careplan.MasterDataReviewItem, error)
}
