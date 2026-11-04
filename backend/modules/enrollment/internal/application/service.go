package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type Store interface {
	DirectStore
	enrollment.RequestUploadRecords
	ApprovedOfferingChildrenForStudents(context.Context, []int64) ([]enrollment.OfferingChildFacts, error)
	OfferingChildren(context.Context, []int64) ([]enrollment.OfferingChildFacts, error)
	enrollment.SubmittedOfferingQueries
	enrollment.SubmittedOfferingCommands
	DeletionRequestCounts(context.Context, int64) (*enrollment.DeletionRequestCounts, error)
	DeletionChildTarget(context.Context, int64, int64) (*enrollment.DeletionChildTarget, error)
	DeletionChildCounts(context.Context, int64, int64) (*enrollment.DeletionChildCounts, error)
	DeletionGuardianProfileIDs(context.Context, int64) ([]int64, error)
	DeletionBlockingStudentIDs(context.Context, int64, *int64) ([]int64, error)
	DeleteRequestTree(context.Context, int64) error
	DeleteRequestChildTree(context.Context, int64, int64) error
	ChangeRequestByID(context.Context, int64) (*enrollment.ChangeRequest, error)
	ChangeRequestByIDForUpdate(context.Context, int64) (*enrollment.ChangeRequest, error)
	ChangeRequestsForRequest(context.Context, int64) ([]*enrollment.ChangeRequest, error)
	OpenChangeRequestsForRequestForUpdate(context.Context, int64) ([]*enrollment.ChangeRequest, error)
	ListChangeRequests(context.Context, enrollment.ChangeRequestListFilters) ([]*enrollment.ChangeRequest, error)
	ChangeRequestsForReview(context.Context, enrollment.ChangeRequestReviewFilters) ([]*enrollment.ChangeRequest, error)
	InsertChangeRequest(context.Context, *enrollment.ChangeRequest) error
	ChangeRequestMessages(context.Context, []int64, bool) ([]*enrollment.ChangeRequestMessage, error)
	InsertChangeRequestMessage(context.Context, *enrollment.ChangeRequestMessage) error
	SetChangeRequestStatus(context.Context, int64, string) error
	MarkChangeRequestReviewed(context.Context, int64, string, *string, int64, time.Time) error
	CountChangeRequestsForReview(context.Context, []string) (int, error)
	CountUnreadRequests(context.Context, int64) (int, error)
	UnreadRequestIDs(context.Context, int64, []int64) ([]int64, error)
	MarkRequestsRead(context.Context, int64, []int64) error
	MarkAllRequestsRead(context.Context, int64) error
	MarkRequestUnread(context.Context, int64, int64) error
	MarkRequestParentChanged(context.Context, int64) error
	InsertLateInvite(context.Context, *enrollment.LateInvite) error
	UsableLateInvite(context.Context, string, int64, time.Time, bool) (*enrollment.LateInvite, error)
	LateInviteByUsedRequestID(context.Context, int64) (*enrollment.LateInvite, error)
	MarkLateInviteUsed(context.Context, int64, int64, time.Time) error
	DeleteLateInvitesByUsedRequestID(context.Context, int64) (int64, error)
	DeleteLateInvitesSettledBefore(context.Context, time.Time) (int64, error)
	PhaseExpirySnapshots(context.Context, enrollment.PhaseExpiryInput) ([]*enrollment.PhaseExpirySnapshot, error)
	InsertChild(ctx context.Context, child *enrollment.RequestChild) error
	ChildByID(ctx context.Context, id int64) (*enrollment.RequestChild, error)
	ChildrenByID(ctx context.Context, ids []int64) ([]*enrollment.RequestChild, error)
	PhaseResponseChildren(ctx context.Context, phaseID int64, statuses []string) ([]enrollment.PhaseResponseChild, error)
	ChildrenForRequest(ctx context.Context, requestID int64, forUpdate bool) ([]*enrollment.RequestChild, error)
	ChildrenForRequests(ctx context.Context, requestIDs []int64) ([]*enrollment.RequestChild, error)
	ChildrenByPhaseStatuses(ctx context.Context, phaseID int64, statuses []string) ([]*enrollment.RequestChild, error)
	UpdateChildData(ctx context.Context, child *enrollment.RequestChild) error
	StudentCarePeriods(context.Context, int64) ([]*enrollment.StudentCarePeriod, error)
	StudentCarePeriodsByStudentIDs(context.Context, []int64) ([]*enrollment.StudentCarePeriod, error)
	UpdateChildActivationPlan(context.Context, int64, string, *enrollment.Date) error
	LinkCreatedStudent(context.Context, int64, int64) error
	SetChildClassSwitch(context.Context, int64, *enrollment.ClassSwitch) error
	ChildClassSwitch(context.Context, int64) (*enrollment.ClassSwitch, error)
	DueClassSwitches(context.Context, enrollment.Date) ([]enrollment.DueClassSwitch, error)
	UpdateMatchedStudent(context.Context, int64, *int64) error
	RestoreWithdrawnChildren(context.Context, int64, []int64) ([]int64, error)
	TransitionPhaseChildren(context.Context, int64, string, string) (int, error)
	UpdateChildStatus(context.Context, int64, string, *string, int64) error
	ReviewRolloverChild(context.Context, int64, string, *string, *int16, int64) error
	HoldAutoRenewedChild(context.Context, int64, string) (bool, error)
	DeleteRequestChildren(context.Context, int64) error
	CountCreatedStudentsByPhase(context.Context, int64) (int, error)
	InsertRequest(context.Context, *enrollment.Request) error
	RequestsByID(context.Context, []int64) ([]*enrollment.Request, error)
	RequestByID(context.Context, int64, bool) (*enrollment.Request, error)
	RequestByToken(context.Context, string, bool) (*enrollment.Request, error)
	AdminRequests(context.Context, enrollment.RequestListFilters) ([]*enrollment.Request, error)
	UpdateRequestGuardian(context.Context, *enrollment.Request, bool) error
	PinDecisionNotificationMode(context.Context, int64, string) (string, error)
	SetRequestWithdrawal(context.Context, int64, *time.Time) error
	ActiveDuplicateChildren(context.Context, int64, string, []enrollment.DuplicateChildKey, int64) ([]enrollment.DuplicateChildKey, error)
	HasActiveRequestForMatchedStudent(context.Context, int64, int64, int64) (bool, error)
	CountPhaseRequests(context.Context, int64) (int, error)
	DeletePhaseRequests(context.Context, int64) (int, error)
	FullyRejectedRequestsBefore(context.Context, time.Time) ([]int64, error)
	SettledRequestsBefore(ctx context.Context, withdrawnCutoff, rejectedCutoff time.Time) ([]int64, error)
	DeleteRequest(context.Context, int64) error
	CreateRequestGuardian(context.Context, *enrollment.RequestGuardian) error
	RequestGuardians(context.Context, []int64) ([]*enrollment.RequestGuardian, error)
	DeleteRequestGuardians(context.Context, int64) error
	StampRequestGuardianProfile(context.Context, int64, int64) error
	CountRequestSchemaReferences(context.Context, []int64) (int, error)
	PendingAnnouncementApplicantsForSchools(context.Context, []int64) ([]enrollment.PendingAnnouncementApplicant, error)
	PendingAnnouncementApplicants(context.Context) ([]enrollment.PendingAnnouncementApplicant, error)
	CareExitApplicationLinks(context.Context, []int64) ([]enrollment.CareExitApplicationLink, error)
	CreatedStudentRequestChildIDs(context.Context, []int64) ([]int64, error)
	CountStudentReferencesForDeletion(context.Context, int64) (approved, total int, err error)
	ApprovedBookings(context.Context) ([]enrollment.ApprovedBooking, error)
	AccountRequests(context.Context, int64, string) ([]enrollment.AccountRequest, error)
	InsertPhase(ctx context.Context, phase *enrollment.Phase) error
	Phase(ctx context.Context, id int64) (*enrollment.Phase, error)
	PhasesByID(ctx context.Context, ids []int64) ([]*enrollment.Phase, error)
	UpdatePhase(ctx context.Context, phase *enrollment.Phase) error
	DeletePhase(ctx context.Context, id int64) error
	Phases(ctx context.Context) ([]*enrollment.Phase, error)
	PublicOpenPhases(ctx context.Context, now time.Time) ([]*enrollment.Phase, error)
	PhasesWithExpiredRolloverDeadline(ctx context.Context, asOf time.Time) ([]*enrollment.Phase, error)
	HasActiveClassRestrictedPhase(ctx context.Context) (bool, error)
	HasActiveGradeRestrictedPhase(ctx context.Context) (bool, error)
	MaxActivePhaseGrade(ctx context.Context) (int, error)
	HasRolloverSuccessor(ctx context.Context, sourcePhaseID int64) (bool, error)
	CountPhaseSchemaReferences(context.Context, []int64) (int, error)
	RepointPhaseSchemas(context.Context, []int64, int64) (int64, error)
	PhaseCountsByCalendarPeriod(context.Context) (map[int64]int, error)
	InsertSchemaVersion(ctx context.Context, schema *enrollment.FormSchema) error
	NextSchemaVersion(ctx context.Context) (int, error)
	NextSchemaVersionForName(ctx context.Context, name string) (int, error)
	SchemaNameExists(ctx context.Context, name string) (bool, error)
	DeactivateSchemas(ctx context.Context) error
	SetSchemaActive(ctx context.Context, id int64, active bool) error
	RenameSchemaLineage(ctx context.Context, oldName, newName string) error
	DeleteSchemaLineage(ctx context.Context, name string) error
	LockSchemaLineages(ctx context.Context) error
	SchemaReferencesLegalDocument(ctx context.Context, storedURL, publicURL string) (bool, error)
	CleanupExpired(context.Context) (int, error)
	BackfillGuardianAccountID(context.Context, int64, string) (int, error)
	Schema(context.Context, int64) (*enrollment.FormSchema, error)
	ActiveSchema(context.Context) (*enrollment.FormSchema, error)
	SchemaVersions(context.Context) ([]*enrollment.FormSchema, error)
	Schemas(context.Context, []int64) ([]*enrollment.FormSchema, error)
}

// DirectStore is the part of the store the Service serves unchanged: the
// submission rate-limit buckets, the request locks that live as long as the
// caller's transaction, and the parent workflow's phase discovery candidates.
type DirectStore interface {
	// IncrementAttempts atomically records an attempt in the school's IP or
	// email bucket.
	IncrementAttempts(context.Context, int64, string, string, time.Duration) (*enrollment.SubmissionRateLimitState, error)
	// AcquireSubmissionDedupLock requires the caller's transaction so the
	// lock remains held through duplicate detection and submission
	// persistence.
	AcquireSubmissionDedupLock(context.Context, int64, uint64) error
	// AcquireExistingStudentMatchLock serializes matching across all
	// guardians submitting to a phase, until the caller's transaction ends.
	AcquireExistingStudentMatchLock(context.Context, int64) error
	// OpenPhaseCandidates supplies the parent workflow's phase discovery
	// candidates. Unlike the anonymous list, it includes restricted
	// audiences for that workflow to authorize against guardian
	// relationships. Cross-school discovery requires the caller's
	// authorized admin transaction.
	OpenPhaseCandidates(context.Context) ([]*enrollment.Phase, error)
}

// OwnerTransactions enforces or opens the caller's authorized transaction.
type OwnerTransactions interface {
	RunInTx(context.Context, func(context.Context) error) error
	RequireTransaction(context.Context) error
}

// Service owns enrollment guards and commands over its persistence port.
type Service struct {
	DirectStore
	store                   Store
	transactions            OwnerTransactions
	offeringStorageObserver func(enrollment.OfferingStorageObservation)
}

func NewService(store Store, transactions OwnerTransactions, observers ...func(enrollment.OfferingStorageObservation)) *Service {
	if store == nil || transactions == nil {
		panic("enrollment: engine is required")
	}
	service := &Service{DirectStore: store, store: store, transactions: transactions}
	if len(observers) > 0 {
		service.offeringStorageObserver = observers[0]
	}
	return service
}

const submissionRateLimitRetentionDays = 1

var _ enrollment.Owner = (*Service)(nil)
