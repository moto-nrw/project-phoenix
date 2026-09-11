package ports

import (
	"context"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// ReviewStudent is the People Directory projection the excused-absence
// workflow needs to scope, gate, and name a child. EnrolledUntil is empty
// when no end of care is recorded.
type ReviewStudent struct {
	ID            int64
	PersonID      int64
	GroupID       *int64
	Alumnus       bool
	EnrolledUntil domain.Date
}

// CareEndedOn reports whether the child's care ended before the given day.
func (s ReviewStudent) CareEndedOn(day domain.Date) bool {
	return !s.EnrolledUntil.IsZero() && day.After(s.EnrolledUntil)
}

// PersonName is the display identity behind one student.
type PersonName struct {
	FirstName string
	LastName  string
}

// ExcusedRequestStudents reads and locks the People Directory rows the
// workflow authorizes against. Every call runs in the caller's tenant
// transaction.
type ExcusedRequestStudents interface {
	// FindStudents returns the non-deleted students among ids, keyed by id.
	FindStudents(ctx context.Context, ids []int64) (map[int64]ReviewStudent, error)
	// LockStudent takes the student row FOR UPDATE and returns it, so a
	// concurrent grade transition cannot flip the child underneath a decision.
	LockStudent(ctx context.Context, id int64) (ReviewStudent, error)
	// PersonNames resolves display names by person id.
	PersonNames(ctx context.Context, personIDs []int64) (map[int64]PersonName, error)
	// ReviewerNames resolves display names by reviewer account id. Unknown
	// accounts are absent from the map.
	ReviewerNames(ctx context.Context, accountIDs []int64) (map[int64]PersonName, error)
	// SetLiveAbsenceFlags updates today's live sick/excused flags on the
	// student row after an approved absence that covers today.
	SetLiveAbsenceFlags(ctx context.Context, studentID int64, status string, at time.Time) error
}

// ReviewScope is the caller's reach over parent requests: every child of the
// school, or only the children of the listed groups.
type ReviewScope struct {
	SchoolWide bool
	GroupIDs   []int64
}

// Allows reports whether the scope covers the student. A nil student is
// never covered.
func (s ReviewScope) Allows(student *ReviewStudent) bool {
	if student == nil {
		return false
	}
	if s.SchoolWide {
		return true
	}
	return student.GroupID != nil && slices.Contains(s.GroupIDs, *student.GroupID)
}

// ReviewScopeResolver answers the caller's review reach from the request
// context. The identity and settings decision stays with its owners; the
// workflow only applies the result.
type ReviewScopeResolver func(ctx context.Context) (ReviewScope, error)

// RequestEvent is one chat pill about a request, in the Communication
// vocabulary. RefID is the request id; RefTable names its table.
type RequestEvent struct {
	EventType      string
	ActorKind      string
	ActorAccountID int64
	Body           string
	RequestType    string
	RequestStatus  string
	DecisionReason string
	RefTable       string
	RefID          int64
}

// DecisionAudience says which co-guardians hear the full decision and which
// only the neutral line.
type DecisionAudience struct {
	Full    []int64
	Neutral []int64
}

// RequestMessenger is the Communication side of the workflow. Enqueue runs in
// the transaction; every Emit is fire-and-forget after commit.
type RequestMessenger interface {
	EnqueueRequestDecision(ctx context.Context, tenantID, studentID, guardianAccountID int64, event RequestEvent) error
	EmitChildEvent(tenantID, studentID, guardianAccountID int64, event RequestEvent)
	BroadcastChildUpdateToGuardians(tenantID, studentID int64)
	GuardianHasChildAccess(ctx context.Context, studentID, guardianAccountID int64) (bool, error)
	ResolveDecisionAudience(ctx context.Context, studentID, submitterAccountID int64, sharedAccountIDs []int64) (DecisionAudience, error)
	EmitDecisionAudience(tenantID, studentID int64, audience DecisionAudience, full, neutral RequestEvent)
}

// StaffBroadcaster wakes staff tabs after a request transition committed.
type StaffBroadcaster interface {
	// StudentUpdated invalidates child cards and student list/detail caches.
	StudentUpdated(tenantID int64) error
	// ChangeRequestsChanged refreshes the review queue and its badge.
	ChangeRequestsChanged(tenantID int64) error
}

// AbsenceReport is the notification about an approved absence.
type AbsenceReport struct {
	TenantID           int64
	StudentIDs         []int64
	Status             string
	Dates              []domain.Date
	FromParent         bool
	ActorAccountID     int64
	ExcludedAccountIDs []int64
}

// AbsenceNotifier enqueues the staff notification inside the transaction.
type AbsenceNotifier interface {
	NotifyAbsenceReported(ctx context.Context, report AbsenceReport) error
}

// RequestLedgerEntry is one append-only history line of a parent request.
type RequestLedgerEntry struct {
	StudentID      int64
	RequestType    string
	RequestID      int64
	EventType      string
	ActorAccountID int64
	UpdatedAt      time.Time
	Payload        map[string]any
}

// RequestLedger records request history inside the ambient transaction.
type RequestLedger interface {
	Record(ctx context.Context, entry RequestLedgerEntry) error
}

// ShareVisibility answers who the guardian explicitly shared a request with.
type ShareVisibility interface {
	SharedRecipientAccountIDs(ctx context.Context, studentID int64, requestType string, requestID int64) ([]int64, error)
}

// TransactionHooks binds the workflow to the tenant runtime without importing
// it: the tenant of the ambient transaction and its after-commit queue.
type TransactionHooks interface {
	TenantID(ctx context.Context) int64
	AfterCommit(ctx context.Context, fn func())
}
