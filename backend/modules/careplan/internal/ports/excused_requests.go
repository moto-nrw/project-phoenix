package ports

import (
	"context"
	"time"

	careplanNative "github.com/moto-nrw/project-phoenix/modules/careplan"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// ExcusedRequestStudents reads and locks the People Directory rows the
// workflow authorizes against. Every call runs in the caller's tenant
// transaction.
type ExcusedRequestStudents interface {
	// FindStudents returns the non-deleted students among ids, keyed by id.
	FindStudents(ctx context.Context, ids []int64) (map[int64]careplanNative.ReviewStudent, error)
	// LockStudent takes the student row FOR UPDATE and returns it, so a
	// concurrent grade transition cannot flip the child underneath a decision.
	LockStudent(ctx context.Context, id int64) (careplanNative.ReviewStudent, error)
	// PersonNames resolves display names by person id.
	PersonNames(ctx context.Context, personIDs []int64) (map[int64]careplanNative.PersonName, error)
	// ReviewerNames resolves display names by reviewer account id. Unknown
	// accounts are absent from the map.
	ReviewerNames(ctx context.Context, accountIDs []int64) (map[int64]careplanNative.PersonName, error)
	// SetLiveAbsenceFlags updates today's live sick/excused flags on the
	// student row after an approved absence that covers today.
	SetLiveAbsenceFlags(ctx context.Context, studentID int64, status string, at time.Time) error
}

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
	// Payload carries structured fields a localized client renders beside
	// the body, such as the confirmed effective date.
	Payload map[string]any
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
	EmitChildEvent(ctx context.Context, tenantID, studentID, guardianAccountID int64, event RequestEvent) error
	BroadcastChildUpdateToGuardians(tenantID, studentID int64)
	GuardianHasChildAccess(ctx context.Context, studentID, guardianAccountID int64) (bool, error)
	ResolveDecisionAudience(ctx context.Context, studentID, submitterAccountID int64, sharedAccountIDs []int64) (DecisionAudience, error)
	EmitDecisionAudience(ctx context.Context, tenantID, studentID int64, audience DecisionAudience, full, neutral RequestEvent) error
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
