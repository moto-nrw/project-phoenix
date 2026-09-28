// Package parentrequests is the Care Plan contract of the cross-kind parent
// request commands (#2267, #3354): approving several open requests at once
// and closing a conflict group in one transaction. It carries the command
// inputs, the request kinds and the stable error sentinels the staff routes
// render; the coordination itself lives behind the Care Plan composition.
package parentrequests

import (
	"context"
	"encoding/json"
	"errors"
)

// Kind is the wire name of a parent request kind.
type Kind string

// Request kinds. care_schedule and pickup_change share one queue but are
// separate kinds, because they conflict on different things (a weekday of the
// weekly plan against one calendar day's pickup time).
const (
	KindMasterData   Kind = "master_data"
	KindExcused      Kind = "excused"
	KindCareSchedule Kind = "care_schedule"
	KindPickupChange Kind = "pickup_change"
	KindOffering     Kind = "offering"
)

// Sentinels shared by the bulk, resolve, decide, correct and mark-done routes
// of every request kind. The routes render their text verbatim, so the
// strings are part of the wire contract and predate the move of the
// coordination into Care Plan (#3354).
var (
	// ErrInvalidBulkRequest means the bulk command itself is malformed.
	ErrInvalidBulkRequest = errors.New("parent requests: invalid bulk approval")
	// ErrBulkIneligible means a request of the command may only be decided on
	// its own.
	ErrBulkIneligible = errors.New("parent requests: request is not eligible for bulk approval")
	// ErrNotFound means a request of the command is no longer open.
	ErrNotFound = errors.New("parent requests: pending request not found")
	// ErrStale means a request changed after the list the client decided on.
	ErrStale = errors.New("parent requests: request version is stale")
	// ErrDecisionRace means a request was decided by someone else while the
	// command ran.
	ErrDecisionRace = errors.New("parent requests: request changed during approval")
	// ErrForbidden means the caller may not decide this request kind.
	ErrForbidden = errors.New("parent requests: request kind is not permitted")
	// ErrReasonRequired means the school's reason policy asks this side of the
	// request for a reason and none was given.
	ErrReasonRequired = errors.New("parent requests: a reason is required")

	// ErrInvalidConflictResolution means the resolve command is malformed:
	// fewer than two requests, mismatched version list, no outcome or more
	// than one, or requests belonging to different children.
	ErrInvalidConflictResolution = errors.New("parent requests: invalid conflict resolution")
	// ErrConflictKindUnsupported means no domain is composed for the kind.
	ErrConflictKindUnsupported = errors.New("parent requests: conflict resolution is not supported for this request kind")
	// ErrStaffValueUnsupported means the domain accepts no staff-entered
	// result for this conflict.
	ErrStaffValueUnsupported = errors.New("parent requests: this request kind accepts no staff-entered value")

	// ErrPast means the request only covers days that have passed: approving
	// it would change nothing, so staff reject it or mark it done.
	ErrPast = errors.New("parent requests: request only covers past days")
	// ErrNotPast means mark-done was called on a request that still covers a
	// day to come.
	ErrNotPast = errors.New("parent requests: request still covers future days")
	// ErrNotDecided means a correction was called on a request without a
	// decision.
	ErrNotDecided = errors.New("parent requests: request is not decided")
	// ErrCorrectionUnsupported means the decision cannot be reverted safely;
	// the wrapped message names the reason.
	ErrCorrectionUnsupported = errors.New("parent requests: decision cannot be corrected")
)

// Ref names one request of a bulk command and the version the client saw.
type Ref struct {
	Kind            Kind   `json:"kind"`
	ID              int64  `json:"id"`
	ExpectedVersion string `json:"expected_version"`
}

// BulkApproveInput approves several open requests with one shared reason.
type BulkApproveInput struct {
	Requests []Ref
	Reason   string
	// ReasonRequired says the school's reason policy
	// (operations.parent_request_reason_policy) asks the deciding staff
	// member for a reason. A bulk command only ever approves, so this is the
	// only gate on its reason.
	ReasonRequired bool
	ReviewerID     int64
}

// ResolveConflictInput closes one conflict group. RequestIDs and
// ExpectedVersions are positional pairs: the client sends back exactly what
// the list gave it, so a group edited in the meantime is refused as a whole.
type ResolveConflictInput struct {
	Kind             Kind
	RequestIDs       []int64
	ExpectedVersions []string
	// ChosenRequestID is the request to approve. Zero means none is approved.
	ChosenRequestID int64
	// StaffValue is the staff member's own result: the kind's JSON payload,
	// {"value": …}, handed to the kind unread. Nil means none was typed.
	StaffValue json.RawMessage
	// None records that the staff member deliberately chose "keine Änderung".
	None bool
	// ConflictKey is the group's key from the list. Required only where the
	// requests alone do not pin the scope a staff value writes against.
	ConflictKey string
	Reason      string
	ReviewerID  int64
	// ActorRole is the reviewer's roles, passed through to the audit trails
	// that record one.
	ActorRole string
}

// BulkApprover approves several open requests atomically.
type BulkApprover interface {
	BulkApprove(ctx context.Context, input BulkApproveInput) error
}

// ConflictResolver closes a whole conflict group atomically: at most one
// request is approved and every other one is rejected.
type ConflictResolver interface {
	ResolveConflict(ctx context.Context, input ResolveConflictInput) error
}

// Coordinator is both cross-kind commands.
type Coordinator interface {
	BulkApprover
	ConflictResolver
}
