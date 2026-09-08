// Package excusedrequests is the Care Plan contract of the student
// excused-absence request workflow: sick notes and excused absences filed by
// guardians and decided by staff (#3093). It carries only the vocabulary a
// consumer needs to call the workflow: calendar days, request rows, queue
// paging, decisions, and stable error sentinels. The public Care Plan facade
// re-exports these names; the implementation lives behind the facade.
package excusedrequests

import (
	"context"
	"errors"
	"time"
)

const DateLayout = "2006-01-02"

// Date is a calendar day in canonical YYYY-MM-DD form.
type Date string

func (d Date) String() string         { return string(d) }
func (d Date) IsZero() bool           { return d == "" }
func (d Date) Before(other Date) bool { return d < other }
func (d Date) After(other Date) bool  { return d > other }
func (d Date) Compare(other Date) int {
	switch {
	case d < other:
		return -1
	case d > other:
		return 1
	default:
		return 0
	}
}

// Student status-day vocabulary shared by the workflow and its consumers.
// The values are the persisted column values.
const (
	StudentStatusDaySick      = "sick"
	StudentStatusDayExcused   = "excused"
	StudentStatusDayClassTrip = "class_trip"
	// StudentStatusDayPresent is the answer for a requested day with no active
	// status row. It is never stored.
	StudentStatusDayPresent = "present"

	StudentStatusSourceManual = "manual"
	StudentStatusSourceParent = "parent"
)

// Request statuses. Only pending rows accept a staff decision or a guardian
// edit.
const (
	StatusPending   = "pending"
	StatusApproved  = "approved"
	StatusRejected  = "rejected"
	StatusWithdrawn = "withdrawn"
	StatusDone      = "done"
	StatusCareEnded = "care_ended"
)

// Stable codes for a review item's BulkIneligibleReason. The German sentence
// travels next to them; the client maps the code and falls back to the text.
const (
	BulkIneligiblePast             = "past"
	BulkIneligibleStale            = "stale"
	BulkIneligibleConflict         = "conflict"
	BulkIneligibleChildUnavailable = "child_unavailable"
	BulkIneligibleAccessRevoked    = "access_revoked"
)

// Ledger and messaging vocabulary the workflow emits through its ports. The
// strings are the persisted values of the People Directory ledger and the
// Communication pill contract.
const (
	ParentRequestTypeExcusedAbsence = "excused"

	ParentRequestEventSubmitted    = "submitted"
	ParentRequestEventGuardianEdit = "guardian_edited"
	ParentRequestEventDecided      = "decided"
	ParentRequestEventMarkedDone   = "marked_done"
	ParentRequestEventCorrected    = "corrected"

	ParentMessageEventRequestCreated = "request_created"
	ParentMessageEventRequestStatus  = "request_status"
	ParentMessageActorGuardian       = "guardian"
	ParentMessageActorStaff          = "staff"
	ParentMessageRequestStatusOpen   = "offen"
	ParentMessageRequestStatusDone   = "erledigt"
	ParentMessageRequestStatusReject = "abgelehnt"
	ParentMessageRequestExcused      = "excused_absence"
	ParentMessageRequestSick         = "sick_absence"
)

var (
	// ErrExcusedRequestNotFound means no row with the requested id exists in
	// the current tenant.
	ErrExcusedRequestNotFound = errors.New("excused absence request not found")
	// ErrExcusedRequestNotPending means a pending-row transition lost a race
	// or the row was already decided.
	ErrExcusedRequestNotPending = errors.New("excused absence request is not pending")
	// ErrExcusedRequestNotDecided means a correction was attempted on a row
	// that carries no decision yet.
	ErrExcusedRequestNotDecided = errors.New("excused absence request is not decided")
	// ErrExcusedRequestForbidden is the authorization error for a decision.
	ErrExcusedRequestForbidden = errors.New("care plan: excused request forbidden")
	// ErrExcusedRequestGuardianAccessRevoked means the submitting guardian is
	// no longer a linked guardian of the child with parent_portal.access, so
	// approving is refused. Staff wind such a request down by rejecting it.
	ErrExcusedRequestGuardianAccessRevoked = errors.New("care plan: excused request guardian access revoked")
	// ErrExcusedRequestOverlap means a guardian already has a pending absence
	// request whose dates intersect, but are not identical to, the new
	// submission. An identical resubmit is an idempotent retry.
	ErrExcusedRequestOverlap = errors.New("care plan: excused request overlaps an existing pending request")
	// ErrExcusedRequestStatusConflict means a full-day request cannot proceed
	// for one of its dates because a competing absence already owns that day.
	ErrExcusedRequestStatusConflict = errors.New("care plan: excused request superseded by a newer status")
	// ErrExcusedRequestNoDates means the request carried no dates.
	ErrExcusedRequestNoDates = errors.New("care plan: excused request requires at least one date")
	// ErrExcusedRequestEmptyNote means the mandatory absence note was blank.
	ErrExcusedRequestEmptyNote = errors.New("care plan: excused request note must not be empty")
	// ErrExcusedRequestNoteTooLong means the absence note exceeded the bound.
	ErrExcusedRequestNoteTooLong = errors.New("care plan: excused request note too long")
	// ErrAbsenceRequestInvalidStatus means the requested absence was neither
	// sick nor excused.
	ErrAbsenceRequestInvalidStatus = errors.New("care plan: absence request status must be sick or excused")
	// ErrExcusedRequestRejectReasonRequired means staff rejected without a reason.
	ErrExcusedRequestRejectReasonRequired = errors.New("care plan: reject reason is required")
	// ErrExcusedRequestRejectReasonTooLong means the reason exceeded the bound.
	ErrExcusedRequestRejectReasonTooLong = errors.New("care plan: reject reason too long")

	// Cross-kind parent-request lifecycle sentinels. Consumers that serve
	// several request kinds match these next to the other owners' sentinels.
	ErrParentRequestStale                 = errors.New("care plan: parent request version is stale")
	ErrParentRequestReasonRequired        = errors.New("care plan: parent request reason is required")
	ErrParentRequestPast                  = errors.New("care plan: parent request only covers past days")
	ErrParentRequestNotPast               = errors.New("care plan: parent request still covers future days")
	ErrParentRequestNotDecided            = errors.New("care plan: parent request is not decided")
	ErrParentRequestDecisionRace          = errors.New("care plan: parent request changed during approval")
	ErrParentRequestCorrectionUnsupported = errors.New("care plan: parent request decision cannot be corrected")
	ErrStaffValueUnsupported              = errors.New("care plan: staff value is not supported for these requests")
)

// StudentStatusDayStatuses lists every persisted full-day absence status.
func StudentStatusDayStatuses() []string {
	return []string{StudentStatusDaySick, StudentStatusDayExcused, StudentStatusDayClassTrip}
}

// StudentStatusDayStatusesExcept lists the persisted statuses other than the
// given one, so a writer can clear every competing status before it upserts.
func StudentStatusDayStatusesExcept(status string) []string {
	statuses := StudentStatusDayStatuses()
	result := make([]string, 0, len(statuses))
	for _, candidate := range statuses {
		if candidate != status {
			result = append(result, candidate)
		}
	}
	return result
}

// ParentRequestVersion is the optimistic-concurrency token of one request
// row: its updated_at in RFC 3339 nanoseconds, empty for a zero time.
func ParentRequestVersion(updatedAt time.Time) string {
	if updatedAt.IsZero() {
		return ""
	}
	return updatedAt.UTC().Format(time.RFC3339Nano)
}

// Request is one excused-absence request row.
type Request struct {
	ID             int64
	TenantID       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StudentID      int64
	SubmittedBy    int64
	Dates          []Date
	Note           string
	AbsenceStatus  string
	Status         string
	DecisionReason *string
	ReviewedBy     *int64
	ReviewedAt     *time.Time
	AppliedAt      *time.Time
}

// QueueFilter is the owner-neutral paging contract shared by the
// immediate-notice and approval-request queues.
type QueueFilter struct {
	UrgentOnly    *bool
	UrgentDate    string
	StudentIDs    []int64
	StudentID     int64
	Search        string
	BeforeInstant time.Time
	BeforeID      int64
	Limit         int
}

// Cursor is the keyset position after one page of a request queue. A nil
// cursor means the page was the last one.
type Cursor struct {
	UpdatedAt time.Time
	ID        int64
}

// ReviewItem is one request enriched with the child's name for the staff
// review queue.
type ReviewItem struct {
	Request      *Request
	FirstName    string
	LastName     string
	BulkEligible bool
	// BulkIneligibleReason is the stable code; BulkIneligibleText the German
	// sentence the client falls back to for codes it does not know.
	BulkIneligibleReason string
	BulkIneligibleText   string
	// CurrentStatusByDate says what each requested day looks like today
	// (present, sick, excused, class_trip).
	CurrentStatusByDate map[string]string
	// CurrentValueChanged is true when one of those days was reported or
	// changed after the request was filed. Nil where it was not resolved.
	CurrentValueChanged *bool
}

// HistoryItem is one decided request enriched with the child's name and the
// reviewer's display name for the staff history.
type HistoryItem struct {
	Request      *Request
	FirstName    string
	LastName     string
	ReviewerName string
}

// DecideInput carries a staff decision on one pending request.
type DecideInput struct {
	RequestID int64
	Approve   bool
	Reason    string
	// ReasonRequired says the school's reason policy asks the deciding staff
	// member for a reason on an approval. A rejection always needs one.
	ReasonRequired  bool
	ExpectedVersion string
	// ReviewedBy is the acting staff account id.
	ReviewedBy int64
}

// SubmitInput is one guardian absence submission. NoteRequired carries the
// school's reason policy, resolved by the caller that knows the tenant.
type SubmitInput struct {
	StudentID         int64
	GuardianAccountID int64
	Dates             []Date
	Note              string
	AbsenceStatus     string
	NoteRequired      bool
}

// EditInput is one guardian edit of their own still-open absence request.
// The absence kind is not editable.
type EditInput struct {
	RequestID         int64
	StudentID         int64
	GuardianAccountID int64
	ExpectedVersion   string
	NoteRequired      bool
	Dates             []Date
	Note              string
}

// BulkCandidate is the workflow's contribution to the cross-kind bulk
// approval command.
type BulkCandidate struct {
	ID        int64
	StudentID int64
	UpdatedAt time.Time
	Eligible  bool
}

// ConflictCandidate is the minimum the cross-kind conflict resolver needs to
// validate a group before touching it.
type ConflictCandidate struct {
	StudentID int64
	UpdatedAt time.Time
}

// ConflictDecision is one verdict inside a resolve command.
type ConflictDecision struct {
	RequestID       int64
	Approve         bool
	Reason          string
	ReviewerID      int64
	ExpectedVersion string
}

// StaffValueWrite records the staff member's own verdict for exactly the
// days the rejected requests fought over. Status is one of present, sick,
// excused or class_trip; present clears the days instead of writing a status.
type StaffValueWrite struct {
	StudentID  int64
	RequestIDs []int64
	Reason     string
	Status     string
}

// Service is the student excused-absence request workflow. Every method runs
// inside the caller's ambient tenant transaction.
type Service interface {
	// CreateRequest stores a guardian's pending excused request with a
	// mandatory note.
	CreateRequest(ctx context.Context, studentID, guardianAccountID int64, dates []Date, note string) (*Request, error)
	// CreateRequestForStatus stores either a sick or excused parent absence in
	// the same approval queue with a mandatory note.
	CreateRequestForStatus(ctx context.Context, studentID, guardianAccountID int64, dates []Date, note, absenceStatus string) (*Request, error)
	// Submit is the create path that carries the school's reason policy.
	Submit(ctx context.Context, input SubmitInput) (*Request, error)
	// EditRequest lets the submitting guardian correct their own still-pending
	// request instead of withdrawing and refiling it.
	EditRequest(ctx context.Context, input EditInput) (*Request, error)
	// ListForStudent returns the child's pending requests plus any decided
	// since recentSince, newest-first.
	ListForStudent(ctx context.Context, studentID int64, recentSince time.Time) ([]*Request, error)
	// ListPending returns pending requests for the current tenant, newest
	// submission first, enriched with child names and scoped to the caller's
	// review reach.
	ListPending(ctx context.Context, filter QueueFilter) ([]*ReviewItem, *Cursor, error)
	// ListHistory returns decided requests newest-decision-first, keyset
	// paginated on (updated_at, id).
	ListHistory(ctx context.Context, filter QueueFilter) ([]*HistoryItem, *Cursor, error)
	// PendingByStudentForDate returns, per student, the newest pending request
	// whose dates cover the given day, scoped to the caller's review reach.
	PendingByStudentForDate(ctx context.Context, date Date) (map[int64]*Request, error)
	// Decide approves (writes the requested status days) or rejects (reason
	// required) one pending request and returns the refreshed row.
	Decide(ctx context.Context, input DecideInput) (*ReviewItem, error)
	// MarkDone closes a request whose days have all passed without applying
	// or refusing anything.
	MarkDone(ctx context.Context, requestID int64, expectedVersion, reason string, reviewedBy int64) error
	// Correct rewrites a decision staff already took.
	Correct(ctx context.Context, requestID int64, approve bool, expectedVersion, reason string, reviewedBy int64) error

	GetExcusedBulkCandidate(ctx context.Context, requestID int64) (*BulkCandidate, error)
	LockExcusedBulkRequest(ctx context.Context, requestID int64) error
	ApproveExcusedBulk(ctx context.Context, requestID int64, reason string, reviewerID int64, expectedVersion string) error

	ConflictCandidate(ctx context.Context, requestID int64) (*ConflictCandidate, error)
	LockConflictRequest(ctx context.Context, requestID int64) error
	DecideConflictRequest(ctx context.Context, decision ConflictDecision) error
	WriteStaffValue(ctx context.Context, write StaffValueWrite) error
}
