package enrollment

import (
	"context"
	"time"
)

// SchemaQueries reads immutable versions, lineages and their references.
type SchemaQueries interface {
	Schema(ctx context.Context, id int64) (*FormSchema, error)
	ActiveSchema(ctx context.Context) (*FormSchema, error)
	SchemaVersions(ctx context.Context) ([]*FormSchema, error)
	Schemas(ctx context.Context, ids []int64) ([]*FormSchema, error)
	HasSchemaRequests(ctx context.Context, id int64) (bool, error)
	SchemaNameExists(ctx context.Context, name string) (bool, error)
	SchemaReferencesLegalDocument(ctx context.Context, storedURL, publicURL string) (bool, error)
	NextSchemaVersion(ctx context.Context) (int, error)
	NextSchemaVersionForName(ctx context.Context, name string) (int, error)
}

// SchemaCommands publishes, renames and removes form-schema lineages.
type SchemaCommands interface {
	PublishSchema(ctx context.Context, input FormSchema) (*FormSchema, error)
	RenameSchema(ctx context.Context, id int64, newName string) (*FormSchema, error)
	DeleteUnusedSchema(ctx context.Context, id int64) (string, error)
	InsertSchemaVersion(ctx context.Context, schema *FormSchema) error
	DeactivateSchemas(ctx context.Context) error
	SetSchemaActive(ctx context.Context, id int64, active bool) error
	LockSchemaLineages(ctx context.Context) error
}

// PhaseQueries reads phases, their eligibility and calendar-period counts.
type PhaseQueries interface {
	Phase(ctx context.Context, id int64) (*Phase, error)
	Phases(ctx context.Context) ([]*Phase, error)
	PhasesByID(ctx context.Context, ids []int64) ([]*Phase, error)
	PublicOpenPhases(ctx context.Context, now time.Time) ([]*Phase, error)
	OpenPhaseCandidates(ctx context.Context) ([]*Phase, error)
	PhasesWithExpiredRolloverDeadline(ctx context.Context, asOf time.Time) ([]*Phase, error)
	HasActiveClassRestrictedPhase(ctx context.Context) (bool, error)
	HasActiveGradeRestrictedPhase(ctx context.Context) (bool, error)
	MaxActivePhaseGrade(ctx context.Context) (int, error)
	HasRolloverSuccessor(ctx context.Context, sourcePhaseID int64) (bool, error)
	PhaseCountsByCalendarPeriod(ctx context.Context) (map[int64]int, error)
}

// PhaseCommands creates, updates and removes enrollment phases.
type PhaseCommands interface {
	InsertPhase(ctx context.Context, phase *Phase) error
	UpdatePhase(ctx context.Context, phase *Phase) error
	DeletePhase(ctx context.Context, id int64) error
	RemovePhase(ctx context.Context, id int64) (int, error)
}

// PhaseSchemaLinks reads and advances phase references to schema versions.
type PhaseSchemaLinks interface {
	CountPhaseSchemaReferences(ctx context.Context, schemaIDs []int64) (int, error)
	RepointPhaseSchemas(ctx context.Context, fromIDs []int64, toID int64) (int64, error)
}

// PhaseExpiryReports reads phase-expiry snapshots for the supplied horizon.
type PhaseExpiryReports interface {
	PhaseExpirySnapshots(ctx context.Context, input PhaseExpiryInput) ([]*PhaseExpirySnapshot, error)
}

// RequestQueries reads submissions, duplicate candidates and retention cohorts.
type RequestQueries interface {
	RequestsByID(ctx context.Context, ids []int64) ([]*Request, error)
	RequestByID(ctx context.Context, id int64, forUpdate bool) (*Request, error)
	RequestByToken(ctx context.Context, token string, forUpdate bool) (*Request, error)
	AdminRequests(ctx context.Context, filters RequestListFilters) ([]*Request, error)
	CountPhaseRequests(ctx context.Context, phaseID int64) (int, error)
	FullyRejectedRequestsBefore(ctx context.Context, cutoff time.Time) ([]int64, error)
	SettledRequestsBefore(ctx context.Context, withdrawnCutoff, rejectedCutoff time.Time) ([]int64, error)
	ActiveDuplicateChildren(ctx context.Context, phaseID int64, email string, children []DuplicateChildKey, excludedRequestID int64) ([]DuplicateChildKey, error)
	HasActiveRequestForMatchedStudent(ctx context.Context, phaseID, studentID, excludedChildID int64) (bool, error)
	AccountRequests(ctx context.Context, accountID int64, accountEmail string) ([]AccountRequest, error)
}

// RequestCommands writes submissions and holds their transaction-scoped locks.
type RequestCommands interface {
	InsertRequest(ctx context.Context, req *Request) error
	UpdateRequestGuardian(ctx context.Context, req *Request, includeEmail bool) error
	SetRequestWithdrawal(ctx context.Context, id int64, at *time.Time) error
	DeletePhaseRequests(ctx context.Context, phaseID int64) (int, error)
	DeleteRequest(ctx context.Context, id int64) error
	AcquireSubmissionDedupLock(ctx context.Context, phaseID int64, emailHash uint64) error
	AcquireExistingStudentMatchLock(ctx context.Context, phaseID int64) error
	PinDecisionNotificationMode(ctx context.Context, requestID int64, proposed string) (string, error)
	BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error)
	MarkRequestParentChanged(ctx context.Context, requestID int64) error
}

// ChildQueries reads request children, phase responses and linked student periods.
type ChildQueries interface {
	ChildByID(ctx context.Context, id int64) (*RequestChild, error)
	ChildrenByID(ctx context.Context, ids []int64) ([]*RequestChild, error)
	ChildrenForRequest(ctx context.Context, requestID int64, forUpdate bool) ([]*RequestChild, error)
	ChildrenForRequests(ctx context.Context, requestIDs []int64) ([]*RequestChild, error)
	ChildrenByPhaseStatuses(ctx context.Context, phaseID int64, statuses []string) ([]*RequestChild, error)
	PhaseResponseChildren(ctx context.Context, phaseID int64, statuses []string) ([]PhaseResponseChild, error)
	CountCreatedStudentsByPhase(ctx context.Context, phaseID int64) (int, error)
	StudentCarePeriods(ctx context.Context, studentID int64) ([]*StudentCarePeriod, error)
	StudentCarePeriodsByStudentIDs(ctx context.Context, studentIDs []int64) ([]*StudentCarePeriod, error)
	CreatedStudentRequestChildIDs(ctx context.Context, studentIDs []int64) ([]int64, error)
}

// ChildCommands writes request-child data, review state and student links.
type ChildCommands interface {
	InsertChild(ctx context.Context, child *RequestChild) error
	UpdateChildData(ctx context.Context, child *RequestChild) error
	UpdateChildActivationPlan(ctx context.Context, id int64, mode string, on *Date) error
	LinkCreatedStudent(ctx context.Context, childID, studentID int64) error
	UpdateMatchedStudent(ctx context.Context, childID int64, studentID *int64) error
	RestoreWithdrawnChildren(ctx context.Context, id int64, waitlistedIDs []int64) ([]int64, error)
	TransitionPhaseChildren(ctx context.Context, id int64, currentStatus, newStatus string) (int, error)
	UpdateChildStatus(ctx context.Context, id int64, status string, reason *string, reviewedBy int64) error
	ReviewRolloverChild(ctx context.Context, id int64, status string, reason *string, grade *int16, reviewedBy int64) error
	HoldAutoRenewedChild(ctx context.Context, id int64, reviewReason string) (bool, error)
	DeleteRequestChildren(ctx context.Context, requestID int64) error
}

// ClassSwitches records and reads the class switches of enrollment children.
type ClassSwitches interface {
	SetChildClassSwitch(ctx context.Context, childID int64, change *ClassSwitch) error
	ChildClassSwitch(ctx context.Context, childID int64) (*ClassSwitch, error)
	DueClassSwitches(ctx context.Context, asOf Date) ([]DueClassSwitch, error)
}

// ChangeRequestQueries reads change requests, their review queue and messages.
type ChangeRequestQueries interface {
	ChangeRequestByID(ctx context.Context, id int64) (*ChangeRequest, error)
	ChangeRequestByIDForUpdate(ctx context.Context, id int64) (*ChangeRequest, error)
	ChangeRequestsForRequest(ctx context.Context, requestID int64) ([]*ChangeRequest, error)
	OpenChangeRequestsForRequestForUpdate(ctx context.Context, requestID int64) ([]*ChangeRequest, error)
	ListChangeRequests(ctx context.Context, filters ChangeRequestListFilters) ([]*ChangeRequest, error)
	ChangeRequestsForReview(ctx context.Context, filters ChangeRequestReviewFilters) ([]*ChangeRequest, error)
	CountChangeRequestsForReview(ctx context.Context, statuses []string) (int, error)
	ChangeRequestMessages(ctx context.Context, ids []int64, includeInternal bool) ([]*ChangeRequestMessage, error)
}

// ChangeRequestCommands writes change requests, messages and review decisions.
type ChangeRequestCommands interface {
	InsertChangeRequest(ctx context.Context, row *ChangeRequest) error
	InsertChangeRequestMessage(ctx context.Context, message *ChangeRequestMessage) error
	SetChangeRequestStatus(ctx context.Context, id int64, status string) error
	MarkChangeRequestReviewed(ctx context.Context, id int64, status string, note *string, reviewerID int64, at time.Time) error
}

// LateInvitationRecords issues, redeems and expires late enrollment invitations.
type LateInvitationRecords interface {
	InsertLateInvite(ctx context.Context, invite *LateInvite) error
	UsableLateInvite(ctx context.Context, tokenHash string, phaseID int64, now time.Time, lock bool) (*LateInvite, error)
	LateInviteByUsedRequestID(ctx context.Context, requestID int64) (*LateInvite, error)
	MarkLateInviteUsed(ctx context.Context, inviteID, requestID int64, at time.Time) error
	DeleteLateInvitesSettledBefore(ctx context.Context, cutoff time.Time) (int64, error)
	DeleteLateInvitesByUsedRequestID(ctx context.Context, requestID int64) (int64, error)
}

// RequestGuardianRecords writes and reads the guardians declared in a submission.
type RequestGuardianRecords interface {
	CreateRequestGuardian(ctx context.Context, guardian *RequestGuardian) error
	RequestGuardians(ctx context.Context, requestIDs []int64) ([]*RequestGuardian, error)
	DeleteRequestGuardians(ctx context.Context, requestID int64) error
	StampRequestGuardianProfile(ctx context.Context, id, profileID int64) error
}

// DeletionRecords previews and removes enrollment records and student references.
type DeletionRecords interface {
	DeletionRequestCounts(ctx context.Context, requestID int64) (*DeletionRequestCounts, error)
	DeletionChildTarget(ctx context.Context, requestID, childID int64) (*DeletionChildTarget, error)
	DeletionChildCounts(ctx context.Context, requestID, childID int64) (*DeletionChildCounts, error)
	DeletionGuardianProfileIDs(ctx context.Context, requestID int64) ([]int64, error)
	DeletionBlockingStudentIDs(ctx context.Context, requestID int64, childID *int64) ([]int64, error)
	DeleteRequestTree(ctx context.Context, requestID int64) error
	DeleteRequestChildTree(ctx context.Context, requestID, childID int64) error
	CountStudentReferencesForDeletion(ctx context.Context, studentID int64) (approved, total int, err error)
	DeleteApprovedEnrollmentsOfStudent(ctx context.Context, studentID int64) (int, error)
}

// OfferingFacts reads enrollment-child identity and approved booking facts.
type OfferingFacts interface {
	ApprovedOfferingChildrenForStudents(ctx context.Context, studentIDs []int64) (rows []OfferingChildFacts, err error)
	OfferingChildren(ctx context.Context, childIDs []int64) (rows []OfferingChildFacts, err error)
	CareExitApplicationLinks(ctx context.Context, studentIDs []int64) ([]CareExitApplicationLink, error)
	ApprovedBookings(ctx context.Context) ([]ApprovedBooking, error)
}

// AnnouncementApplicants reads the pending applicants for school announcements.
type AnnouncementApplicants interface {
	PendingAnnouncementApplicantsForSchools(ctx context.Context, schoolIDs []int64) ([]PendingAnnouncementApplicant, error)
	PendingAnnouncementApplicants(ctx context.Context) ([]PendingAnnouncementApplicant, error)
}

// SubmissionAttempts atomically counts attempts in a school submission bucket.
type SubmissionAttempts interface {
	IncrementAttempts(ctx context.Context, tenantID int64, keyType, keyValue string, window time.Duration) (*SubmissionRateLimitState, error)
}
