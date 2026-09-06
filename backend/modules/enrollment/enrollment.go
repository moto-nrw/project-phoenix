// Package enrollment exposes the Enrollment query and command capability.
package enrollment

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SubmissionRateLimitKeyType discriminates between IP-based and
// email-based rate limit rows in the same table.
const (
	SubmissionRateLimitKeyTypeIP    = "ip"
	SubmissionRateLimitKeyTypeEmail = "email"
)

// SubmissionRateLimitState is the post-increment view the service uses
// to decide whether to admit a submission.
type SubmissionRateLimitState struct {
	Attempts int
	RetryAt  time.Time
}

type engine interface {
	DeletionRequestCounts(context.Context, int64) (*DeletionRequestCounts, error)
	DeletionChildTarget(context.Context, int64, int64) (*DeletionChildTarget, error)
	DeletionChildCounts(context.Context, int64, int64) (*DeletionChildCounts, error)
	DeletionGuardianProfileIDs(context.Context, int64) ([]int64, error)
	DeletionBlockingStudentIDs(context.Context, int64, *int64) ([]int64, error)
	DeleteRequestTree(context.Context, int64) error
	DeleteRequestChildTree(context.Context, int64, int64) error
	ChangeRequestByID(context.Context, int64) (*ChangeRequest, error)
	ChangeRequestByIDForUpdate(context.Context, int64) (*ChangeRequest, error)
	ChangeRequestsForRequest(context.Context, int64) ([]*ChangeRequest, error)
	OpenChangeRequestsForRequestForUpdate(context.Context, int64) ([]*ChangeRequest, error)
	ListChangeRequests(context.Context, ChangeRequestListFilters) ([]*ChangeRequest, error)
	ChangeRequestsForReview(context.Context, ChangeRequestReviewFilters) ([]*ChangeRequest, error)
	InsertChangeRequest(context.Context, *ChangeRequest) error
	ChangeRequestMessages(context.Context, []int64, bool) ([]*ChangeRequestMessage, error)
	InsertChangeRequestMessage(context.Context, *ChangeRequestMessage) error
	SetChangeRequestStatus(context.Context, int64, string) error
	MarkChangeRequestReviewed(context.Context, int64, string, *string, int64, time.Time) error
	CountChangeRequestsForReview(context.Context, []string) (int, error)
	ApprovedSelectionsForStudents(context.Context, []int64, Date, Date) ([]*ApprovedOfferingSelection, error)
	OfferingGradeCounts(context.Context, []int64, Date, Date) ([]*OfferingGradeCount, error)
	MaterializableOfferingCount(context.Context, int64, Date) (int, error)
	OfferingCapacityPeaks(context.Context, []int64, Date, Date) (map[int64]int, error)
	OfferingCapacityPeak(context.Context, int64, []int64, Date, Date) (int, error)
	ApprovedSelectionsForOfferings(context.Context, []int64, Date) ([]*ApprovedOfferingSelection, error)
	RequestChildOfferingsAtDates(context.Context, map[int64]Date) ([]*RequestChildOffering, error)
	RequestChildOfferingHistoryForChildren(context.Context, []int64) ([]*RequestChildOffering, error)
	RequestChildOfferingsForChildrenAtDate(context.Context, []int64, Date) ([]*RequestChildOffering, error)
	RequestChildOfferingsAtDate(context.Context, int64, Date) ([]*RequestChildOffering, error)
	RequestChildOfferingHistory(context.Context, int64) ([]*RequestChildOffering, error)
	ScheduleRequestChildOfferings(context.Context, int64, Date, []*RequestChildOffering) error
	ReplaceRequestChildOfferings(context.Context, int64, []*RequestChildOffering) error
	InsertRequestChildOffering(context.Context, *RequestChildOffering) error
	InsertLateInvite(context.Context, *LateInvite) error
	UsableLateInvite(context.Context, string, int64, time.Time, bool) (*LateInvite, error)
	LateInviteByUsedRequestID(context.Context, int64) (*LateInvite, error)
	MarkLateInviteUsed(context.Context, int64, int64, time.Time) error
	DeleteLateInvitesByUsedRequestID(context.Context, int64) (int64, error)
	PhaseExpirySnapshots(context.Context, PhaseExpiryInput) ([]*PhaseExpirySnapshot, error)
	InsertChild(ctx context.Context, child *RequestChild) error
	ChildByID(ctx context.Context, id int64) (*RequestChild, error)
	ChildrenByID(ctx context.Context, ids []int64) ([]*RequestChild, error)
	ChildrenForRequest(ctx context.Context, requestID int64, forUpdate bool) ([]*RequestChild, error)
	ChildrenForRequests(ctx context.Context, requestIDs []int64) ([]*RequestChild, error)
	ChildrenByPhaseStatuses(ctx context.Context, phaseID int64, statuses []string) ([]*RequestChild, error)
	UpdateChildData(ctx context.Context, child *RequestChild) error
	StudentCarePeriods(context.Context, int64) ([]*StudentCarePeriod, error)
	UpdateChildActivationPlan(context.Context, int64, string, *Date) error
	LinkCreatedStudent(context.Context, int64, int64) error
	UpdateMatchedStudent(context.Context, int64, *int64) error
	RestoreWithdrawnChildren(context.Context, int64, []int64) ([]int64, error)
	TransitionPhaseChildren(context.Context, int64, string, string) (int, error)
	UpdateChildStatus(context.Context, int64, string, *string, int64) error
	ReviewRolloverChild(context.Context, int64, string, *string, *int16, int64) error
	DeleteRequestChildren(context.Context, int64) error
	CountCreatedStudentsByPhase(context.Context, int64) (int, error)
	InsertRequest(context.Context, *Request) error
	RequestsByID(context.Context, []int64) ([]*Request, error)
	RequestByID(context.Context, int64, bool) (*Request, error)
	RequestByToken(context.Context, string, bool) (*Request, error)
	AdminRequests(context.Context, RequestListFilters) ([]*Request, error)
	UpdateRequestGuardian(context.Context, *Request, bool) error
	PinDecisionNotificationMode(context.Context, int64, string) (string, error)
	SetRequestWithdrawal(context.Context, int64, *time.Time) error
	ActiveDuplicateChildren(context.Context, int64, string, []DuplicateChildKey, int64) ([]DuplicateChildKey, error)
	HasActiveRequestForMatchedStudent(context.Context, int64, int64, int64) (bool, error)
	AcquireSubmissionDedupLock(context.Context, int64, uint64) error
	AcquireExistingStudentMatchLock(context.Context, int64) error
	CountPhaseRequests(context.Context, int64) (int, error)
	DeletePhaseRequests(context.Context, int64) (int, error)
	FullyRejectedRequestsBefore(context.Context, time.Time) ([]int64, error)
	DeleteRequest(context.Context, int64) error
	CreateRequestGuardian(context.Context, *RequestGuardian) error
	RequestGuardians(context.Context, []int64) ([]*RequestGuardian, error)
	DeleteRequestGuardians(context.Context, int64) error
	StampRequestGuardianProfile(context.Context, int64, int64) error
	CountRequestSchemaReferences(context.Context, []int64) (int, error)
	PendingAnnouncementApplicantsForSchools(context.Context, []int64) ([]PendingAnnouncementApplicant, error)
	PendingAnnouncementApplicants(context.Context) ([]PendingAnnouncementApplicant, error)
	CareExitApplicationLinks(context.Context, []int64) ([]CareExitApplicationLink, error)
	CreatedStudentRequestChildIDs(context.Context, []int64) ([]int64, error)
	CountStudentReferences(context.Context, int64) (int, error)
	ApprovedBookings(context.Context) ([]ApprovedBooking, error)
	OpenPhaseCandidates(context.Context) ([]*Phase, error)
	AccountRequests(context.Context, int64, string) ([]AccountRequest, error)
	InsertPhase(ctx context.Context, phase *Phase) error
	Phase(ctx context.Context, id int64) (*Phase, error)
	PhasesByID(ctx context.Context, ids []int64) ([]*Phase, error)
	UpdatePhase(ctx context.Context, phase *Phase) error
	DeletePhase(ctx context.Context, id int64) error
	Phases(ctx context.Context) ([]*Phase, error)
	PublicOpenPhases(ctx context.Context, now time.Time) ([]*Phase, error)
	PhasesWithExpiredRolloverDeadline(ctx context.Context, asOf time.Time) ([]*Phase, error)
	HasActiveClassRestrictedPhase(ctx context.Context) (bool, error)
	HasActiveGradeRestrictedPhase(ctx context.Context) (bool, error)
	MaxActivePhaseGrade(ctx context.Context) (int, error)
	HasRolloverSuccessor(ctx context.Context, sourcePhaseID int64) (bool, error)
	CountPhaseSchemaReferences(context.Context, []int64) (int, error)
	RepointPhaseSchemas(context.Context, []int64, int64) (int64, error)
	PhaseCountsByCalendarPeriod(context.Context) (map[int64]int, error)
	InsertSchemaVersion(ctx context.Context, schema *FormSchema) error
	NextSchemaVersion(ctx context.Context) (int, error)
	NextSchemaVersionForName(ctx context.Context, name string) (int, error)
	SchemaNameExists(ctx context.Context, name string) (bool, error)
	DeactivateSchemas(ctx context.Context) error
	SetSchemaActive(ctx context.Context, id int64, active bool) error
	RenameSchemaLineage(ctx context.Context, oldName, newName string) error
	DeleteSchemaLineage(ctx context.Context, name string) error
	LockSchemaLineages(ctx context.Context) error
	SchemaReferencesLegalDocument(ctx context.Context, storedURL, publicURL string) (bool, error)
	IncrementAttempts(context.Context, int64, string, string, time.Duration) (*SubmissionRateLimitState, error)
	CleanupExpired(context.Context) (int, error)
	BackfillGuardianAccountID(context.Context, int64, string) (int, error)
	Schema(context.Context, int64) (*FormSchema, error)
	ActiveSchema(context.Context) (*FormSchema, error)
	SchemaVersions(context.Context) ([]*FormSchema, error)
	Schemas(context.Context, []int64) ([]*FormSchema, error)
}

// Module owns enrollment operations. Persistence remains behind its private ports.
type Module struct {
	engine       engine
	transactions interface {
		RunInTx(context.Context, func(context.Context) error) error
	}
}

func NewModule(engine engine, transactions interface {
	RunInTx(context.Context, func(context.Context) error) error
}) *Module {
	if engine == nil || transactions == nil {
		panic("enrollment: engine is required")
	}
	return &Module{engine: engine, transactions: transactions}
}

// IncrementAttempts atomically records an attempt in the school's IP or email bucket.
func (m *Module) IncrementAttempts(ctx context.Context, tenantID int64, keyType, keyValue string, window time.Duration) (*SubmissionRateLimitState, error) {
	return m.engine.IncrementAttempts(ctx, tenantID, keyType, keyValue, window)
}

// CleanupExpired removes buckets older than the longest submission window.
func (m *Module) CleanupExpired(ctx context.Context) (int, error) {
	return m.engine.CleanupExpired(ctx)
}

// Schema returns the immutable version pinned to a submission or phase.
func (m *Module) Schema(ctx context.Context, id int64) (*FormSchema, error) {
	var schema *FormSchema
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var queryErr error
		schema, queryErr = m.engine.Schema(txCtx, id)
		return queryErr
	})
	return schema, err
}

// ActiveSchema returns the active form visible in the caller's tenant.
func (m *Module) ActiveSchema(ctx context.Context) (*FormSchema, error) {
	var schema *FormSchema
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var queryErr error
		schema, queryErr = m.engine.ActiveSchema(txCtx)
		return queryErr
	})
	return schema, err
}

// SchemaVersions lists versions newest first within the caller's tenant.
func (m *Module) SchemaVersions(ctx context.Context) ([]*FormSchema, error) {
	var schemas []*FormSchema
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var queryErr error
		schemas, queryErr = m.engine.SchemaVersions(txCtx)
		return queryErr
	})
	return schemas, err
}

// BackfillGuardianAccountID attaches previously anonymous requests after account
// verification. The caller supplies its authorized tenant or admin transaction.
func (m *Module) BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error) {
	if accountID <= 0 {
		return 0, fmt.Errorf("parent: account_id must be positive")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return 0, nil
	}
	return m.engine.BackfillGuardianAccountID(ctx, accountID, email)
}
