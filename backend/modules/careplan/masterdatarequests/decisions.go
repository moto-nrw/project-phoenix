package masterdatarequests

import (
	"context"
	"errors"
)

// Persisted vocabulary of a Stammdaten request: its targets, its statuses and
// the names the shared ledger and the parent chat know it by.
const (
	TargetPerson    = "person"
	TargetStudent   = "student"
	TargetDeparture = "departure"

	StatusPending     = "pending"
	StatusAutoApplied = "auto_applied"
	StatusApproved    = "approved"
	StatusRejected    = "rejected"

	// ParentRequestType names the request kind in the parent-request ledger
	// and the chat pill; RefTable names its row.
	ParentRequestType = "master_data"
	RefTable          = "users.student_data_change_requests"
)

// Staff-decision sentinels. The students routes render their text verbatim,
// so the strings are part of the wire contract and predate the move of the
// decision into Care Plan (#3354).
var (
	// ErrReviewNotFound means no change request matched the id in the tenant,
	// or its child is no longer one the school may decide for.
	ErrReviewNotFound = errors.New("users: change request not found")
	// ErrReviewNotPending means the request was already decided (lost race)
	// or is an audit row that cannot be decided.
	ErrReviewNotPending = errors.New("users: change request is not pending")
	// ErrReviewInvalidTarget means the request's target or field is not one
	// the decision knows how to apply.
	ErrReviewInvalidTarget = errors.New("users: change request target is not applicable")
	// ErrReviewInvalidValue means the stored value could not be decoded when
	// applying it.
	ErrReviewInvalidValue = errors.New("users: change request value is invalid")
	// ErrReviewStaleValue means the live value no longer matches the request's
	// baseline, so approving would overwrite a newer staff or import edit.
	ErrReviewStaleValue = errors.New("users: change request baseline changed")
	// ErrReviewForbidden means the caller's review scope does not cover the
	// child.
	ErrReviewForbidden = errors.New("users: change request forbidden")
)

// DecideInput carries a staff decision on one pending request.
type DecideInput struct {
	RequestID int64
	Approve   bool
	Reason    string
	// ReasonRequired says the school's reason policy asks the deciding staff
	// member for a reason on an approval; a rejection always needs one.
	ReasonRequired bool
	// ReviewedBy is the acting staff account id.
	ReviewedBy      int64
	ExpectedVersion string
}

// Decisions is the staff side of a parent Stammdaten request: approving
// (which applies the value to the child's record), rejecting, and correcting
// a decision already taken. Every method runs inside the caller's tenant
// transaction.
type Decisions interface {
	// Decide approves or rejects one pending request and returns the
	// refreshed row with the child's name.
	Decide(ctx context.Context, input DecideInput) (*ReviewItem, error)
	// Correct rewrites a decision staff already took. Turning an approval
	// into a rejection restores the old value only while the live value is
	// still the one the approval wrote.
	Correct(ctx context.Context, requestID int64, approve bool, expectedVersion, reason string, reviewedBy int64) error
}
