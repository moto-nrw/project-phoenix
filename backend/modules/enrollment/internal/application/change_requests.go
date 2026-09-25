package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Change requests (#3565): a family's proposed correction of its enrollment
// over the status token, the question-and-answer thread, staff's review, and
// staff's direct correction of an approved child.

// ChangeRequestRecords is the owner capability the change requests read and
// write through.
type ChangeRequestRecords interface {
	IntakeRequests
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
}

// CareBookingChanges replaces a request child's Care Plan bookings when an
// approved change moves it onto other offerings.
type CareBookingChanges interface {
	ReplaceCareBookings(context.Context, int64, []enrollment.CareBookingInput) error
}

// CareBookingGates are the Care Plan gates an approval takes around its
// booking-derived writes.
type CareBookingGates interface {
	LockOfferingDerivedWrites(ctx context.Context) error
	ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64) error
}

// CompanionGraphCoordinator takes the row locks of several children's "läuft
// mit" graph in the global ascending-id order, and decides the stranding
// verdicts a coordinated write deferred.
type CompanionGraphCoordinator interface {
	LockCompanionGraph(ctx context.Context, subjectIDs []int64, additional []int64) error
	VerifyCompanionStrandingBatch(ctx context.Context) error
}

// ReviewerNames resolves the display names of the accounts that decided,
// keyed by account id; an account without a person is missing.
type ReviewerNames interface {
	ReviewerNames(ctx context.Context, accountIDs []int64) (map[int64]string, error)
}

// ChangeRequestDependencies bind the change requests to their owners.
// Decisions, BookingGates, Companions, Reviewers and the People ports are
// optional where the flow degrades without them.
type ChangeRequestDependencies struct {
	Requests           ChangeRequestRecords
	Children           IntakeChildren
	Guardians          IntakeGuardians
	LateInvites        DecisionLateInvites
	Catalog            IntakeCatalog
	Offerings          IntakeOfferings
	Bookings           CareBookingChanges
	Capacity           enrollment.OfferingCapacity
	Notifications      enrollment.Notifications
	Students           StudentMatches
	GuardianAuthorizer GuardianStudentAuthorizer
	// Decisions applies an approved change to a child the decision flow
	// already approved; BookingGates wrap those writes.
	Decisions    enrollment.ApprovedChildChanges
	BookingGates CareBookingGates
	Companions   CompanionGraphCoordinator
	// CompanionLockBusy is the retriable conflict an approval answers when
	// Care Plan reports a linked child held elsewhere.
	CompanionLockBusy error
	People            PeopleDirectory
	Reviewers         ReviewerNames
	Settings          IntakeSettings
	Outbox            MailOutbox
	FrontendURL       string
	ParentsURL        string
	Runtime           Runtime
	Logger            *slog.Logger
}

// ChangeRequests implements the public ChangeRequests capability.
type ChangeRequests struct {
	deps   ChangeRequestDependencies
	intake *Intake
}

var _ enrollment.ChangeRequests = (*ChangeRequests)(nil)

// NewChangeRequests composes the change requests. ParentsURL is required:
// every mail links the family's status page.
func NewChangeRequests(deps ChangeRequestDependencies) *ChangeRequests {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	deps.ParentsURL = strings.TrimRight(strings.TrimSpace(deps.ParentsURL), "/")
	if deps.ParentsURL == "" {
		panic("PARENTS_URL is required")
	}
	deps.FrontendURL = strings.TrimRight(strings.TrimSpace(deps.FrontendURL), "/")
	intake := NewIntake(IntakeDependencies{
		Requests: deps.Requests, Children: deps.Children, Catalog: deps.Catalog, Offerings: deps.Offerings,
		Capacity: deps.Capacity, Students: deps.Students, GuardianAuthorizer: deps.GuardianAuthorizer,
		Settings: deps.Settings, Runtime: deps.Runtime, Logger: deps.Logger,
	})
	return &ChangeRequests{deps: deps, intake: intake}
}

// ChangeRequest is a decoded change request: the owner's row with its
// snapshots as maps, because the review compares and applies them field by
// field.
type ChangeRequest struct {
	ID                             int64
	TenantID                       int64
	CreatedAt                      time.Time
	UpdatedAt                      time.Time
	RequestID                      int64
	RequestChildID                 *int64
	Origin                         string
	Status                         string
	ParentNote                     *string
	AdminDecisionNote              *string
	BaseSnapshot                   map[string]any
	ProposedSnapshot               map[string]any
	Diff                           map[string]any
	CareOfferingsEnabledAtCreation bool
	CreatedByAccountID             *int64
	ReviewedByAccountID            *int64
	ReviewedAt                     *time.Time
}

// decisionInstant is when a decided change request was decided.
func (r *ChangeRequest) decisionInstant() time.Time {
	if r.ReviewedAt != nil {
		return *r.ReviewedAt
	}
	return r.UpdatedAt
}

// changeRequestCase is a decoded change request with what it concerns.
type changeRequestCase struct {
	ChangeRequest *ChangeRequest
	Request       *enrollmentModels.Request
	Children      []*RequestChild
	Messages      []*enrollment.ChangeRequestMessage
	Phase         *enrollment.Phase
}

// Create files a family's change request against its request.
func (s *ChangeRequests) Create(ctx context.Context, token string, input enrollment.CreateChangeRequestInput) (*enrollment.ChangeRequestCase, error) {
	return publicCase(s.create(ctx, token, input))
}

// ListPublic lists the change requests of the request behind a token.
func (s *ChangeRequests) ListPublic(ctx context.Context, token string) ([]*enrollment.ChangeRequestCase, error) {
	return publicCases(s.listPublic(ctx, token))
}

// ParentReply answers a staff question on a change request.
func (s *ChangeRequests) ParentReply(ctx context.Context, token string, changeRequestID int64, input enrollment.ChangeRequestMessageInput) (*enrollment.ChangeRequestCase, error) {
	return publicCase(s.parentReply(ctx, token, changeRequestID, input))
}

// ListAdmin lists change requests for staff.
func (s *ChangeRequests) ListAdmin(ctx context.Context, filters enrollment.ChangeRequestFilters) ([]*enrollment.ChangeRequestCase, error) {
	return publicCases(s.listAdmin(ctx, filters))
}

// GetAdmin loads one change request for staff.
func (s *ChangeRequests) GetAdmin(ctx context.Context, changeRequestID int64) (*enrollment.ChangeRequestCase, error) {
	return publicCase(s.loadAggregate(ctx, changeRequestID, true))
}

// AskQuestion asks the family a question and waits for its reply.
func (s *ChangeRequests) AskQuestion(ctx context.Context, changeRequestID int64, input enrollment.ChangeRequestMessageInput) (*enrollment.ChangeRequestCase, error) {
	return publicCase(s.askQuestion(ctx, changeRequestID, input))
}

// Reject rejects a pending change request with a note.
func (s *ChangeRequests) Reject(ctx context.Context, changeRequestID int64, input enrollment.ReviewChangeRequestInput) (*enrollment.ChangeRequestCase, error) {
	return publicCase(s.review(ctx, changeRequestID, input, reviewOutcome{
		noteRequiredMsg: "rejection note is required",
		status:          enrollment.ChangeRequestStatusRejected,
		emailKind:       enrollment.MailKindChangeRequestRejected,
	}))
}

// Approve approves a pending change request and applies it.
func (s *ChangeRequests) Approve(ctx context.Context, changeRequestID int64, input enrollment.ReviewChangeRequestInput) (*enrollment.ChangeRequestCase, error) {
	return publicCase(s.review(ctx, changeRequestID, input, reviewOutcome{
		noteRequiredMsg: "approval note is required",
		status:          enrollment.ChangeRequestStatusApproved,
		emailKind:       enrollment.MailKindChangeRequestApproved,
		apply:           s.applyApprovedChange,
	}))
}

// CorrectApprovedChildData corrects an approved child's identity on the
// enrollment record and the student it created.
func (s *ChangeRequests) CorrectApprovedChildData(ctx context.Context, input enrollment.CorrectApprovedChildDataInput) (*enrollment.ChangeRequestCase, error) {
	return publicCase(s.correctApprovedChildData(ctx, input))
}

func (s *ChangeRequests) create(ctx context.Context, token string, input enrollment.CreateChangeRequestInput) (*changeRequestCase, error) {
	submission, err := decodeSubmitRequest(input.Submission)
	if err != nil {
		return nil, err
	}
	req, tenantID, err := s.requestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	var created *ChangeRequest
	if err := s.deps.Runtime.TenantTx(ctx, tenantID, func(txCtx context.Context) error {
		row, createErr := s.createInTenant(txCtx, token, submission, input)
		created = row
		return createErr
	}); err != nil {
		return nil, err
	}
	s.enqueueAdminNotification(ctx, tenantID, req, created.ID, enrollment.MailKindChangeRequestSubmitted)
	return s.loadAggregateForTenant(ctx, tenantID, created.ID, false)
}

func (s *ChangeRequests) createInTenant(ctx context.Context, token string, submission SubmitRequest, input enrollment.CreateChangeRequestInput) (*ChangeRequest, error) {
	lockedReq, err := decodedRequestByToken(ctx, s.deps.Requests, strings.TrimSpace(token), true)
	if err != nil {
		if !s.deps.Runtime.NotFound(err) {
			return nil, err
		}
		return nil, enrollment.ErrRequestNotFound
	}
	children, err := decodedChildrenOfRequest(ctx, s.deps.Children, lockedReq.ID, true)
	if err != nil {
		return nil, fmt.Errorf("change request: lock children: %w", err)
	}
	if err := s.ensureCanCreate(ctx, lockedReq, children); err != nil {
		return nil, err
	}
	if err := s.ensureNoOpenChangeRequest(ctx, lockedReq.ID); err != nil {
		return nil, err
	}
	// A change request on a rolled-forward renewal is the parent's reaction
	// to the Halbjahreswechsel (#2251): take the children out of the
	// automatic renewal before the base snapshot is captured.
	if err := s.flipRenewalChildrenToSubmitted(ctx, children); err != nil {
		return nil, err
	}
	capabilities, err := s.formCapabilities(ctx, nil)
	if err != nil {
		return nil, err
	}
	prepared, err := s.prepareProposed(ctx, lockedReq, children, submission, capabilities, false, true)
	if err != nil {
		return nil, err
	}
	baseSnapshot, err := s.currentSnapshot(ctx, lockedReq, children)
	if err != nil {
		return nil, err
	}
	proposedSnapshot := submitSnapshot(prepared.request)
	if err := ensureTakenOverChildrenUnchanged(children, baseSnapshot, proposedSnapshot); err != nil {
		return nil, err
	}
	row := &ChangeRequest{
		RequestID: lockedReq.ID, Status: enrollment.ChangeRequestStatusPendingReview,
		ParentNote: trimToNil(input.ParentNote), BaseSnapshot: baseSnapshot, ProposedSnapshot: proposedSnapshot,
		Diff: snapshotDiff(baseSnapshot, proposedSnapshot), CareOfferingsEnabledAtCreation: capabilities.CareOfferingsEnabled,
		CreatedByAccountID: input.CreatedByAccountID,
	}
	if err := createChangeRequest(ctx, s.deps.Requests, row); err != nil {
		return nil, err
	}
	if row.ParentNote != nil {
		if err := s.deps.Requests.InsertChangeRequestMessage(ctx, &enrollment.ChangeRequestMessage{
			ChangeRequestID: row.ID, AuthorType: enrollment.ChangeRequestMessageAuthorParent,
			AuthorAccountID: input.CreatedByAccountID, Body: *row.ParentNote,
		}); err != nil {
			return nil, err
		}
	}
	return row, nil
}

func (s *ChangeRequests) listPublic(ctx context.Context, token string) ([]*changeRequestCase, error) {
	req, tenantID, err := s.requestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	var rows []*ChangeRequest
	if err := s.deps.Runtime.TenantTx(ctx, tenantID, func(txCtx context.Context) error {
		list, listErr := changeRequestsFromOwner(s.deps.Requests.ChangeRequestsForRequest(txCtx, req.ID))
		rows = list
		return listErr
	}); err != nil {
		return nil, err
	}
	out := make([]*changeRequestCase, 0, len(rows))
	for _, row := range rows {
		agg, loadErr := s.loadAggregateForTenant(ctx, tenantID, row.ID, false)
		if loadErr != nil {
			return nil, loadErr
		}
		out = append(out, agg)
	}
	return out, nil
}

func (s *ChangeRequests) parentReply(ctx context.Context, token string, changeRequestID int64, input enrollment.ChangeRequestMessageInput) (*changeRequestCase, error) {
	req, tenantID, err := s.requestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		return nil, fmt.Errorf("%w: message body is required", enrollment.ErrChangeRequestInvalidData)
	}
	if err := s.deps.Runtime.TenantTx(ctx, tenantID, func(txCtx context.Context) error {
		row, err := s.readChangeRequest(txCtx, changeRequestID, true)
		if err != nil && !s.deps.Runtime.NotFound(err) {
			return err
		}
		if err != nil || row == nil || row.RequestID != req.ID {
			return enrollment.ErrChangeRequestNotFound
		}
		if row.Status != enrollment.ChangeRequestStatusNeedsParentResponse {
			return enrollment.ErrChangeRequestInvalidStatus
		}
		if err := s.deps.Requests.InsertChangeRequestMessage(txCtx, &enrollment.ChangeRequestMessage{
			ChangeRequestID: row.ID, AuthorType: enrollment.ChangeRequestMessageAuthorParent, Body: body,
		}); err != nil {
			return err
		}
		return s.deps.Requests.SetChangeRequestStatus(txCtx, row.ID, enrollment.ChangeRequestStatusPendingReview)
	}); err != nil {
		return nil, err
	}
	s.enqueueAdminNotification(ctx, tenantID, req, changeRequestID, enrollment.MailKindChangeRequestParentReply)
	return s.loadAggregateForTenant(ctx, tenantID, changeRequestID, false)
}

func (s *ChangeRequests) listAdmin(ctx context.Context, filters enrollment.ChangeRequestFilters) ([]*changeRequestCase, error) {
	rows, err := changeRequestsFromOwner(s.deps.Requests.ListChangeRequests(ctx, enrollment.ChangeRequestListFilters{
		RequestID: filters.RequestID, Status: filters.Status, Limit: filters.Limit,
	}))
	if err != nil {
		return nil, err
	}
	out := make([]*changeRequestCase, 0, len(rows))
	for _, row := range rows {
		agg, err := s.loadAggregate(ctx, row.ID, true)
		if err != nil {
			return nil, err
		}
		out = append(out, agg)
	}
	return out, nil
}

func (s *ChangeRequests) askQuestion(ctx context.Context, changeRequestID int64, input enrollment.ChangeRequestMessageInput) (*changeRequestCase, error) {
	body := strings.TrimSpace(input.Body)
	if body == "" {
		return nil, fmt.Errorf("%w: message body is required", enrollment.ErrChangeRequestInvalidData)
	}
	var req *enrollmentModels.Request
	if err := s.withLockedChangeRequest(ctx, changeRequestID, func(txCtx context.Context, row *ChangeRequest) error {
		if row.Status != enrollment.ChangeRequestStatusPendingReview {
			return enrollment.ErrChangeRequestInvalidStatus
		}
		loadedReq, err := s.requestByID(txCtx, row.RequestID, false)
		if err != nil {
			return err
		}
		req = loadedReq
		actorID := input.ActorAccountID
		if err := s.deps.Requests.InsertChangeRequestMessage(txCtx, &enrollment.ChangeRequestMessage{
			ChangeRequestID: row.ID, AuthorType: enrollment.ChangeRequestMessageAuthorStaff, AuthorAccountID: &actorID, Body: body,
		}); err != nil {
			return err
		}
		return s.deps.Requests.SetChangeRequestStatus(txCtx, row.ID, enrollment.ChangeRequestStatusNeedsParentResponse)
	}); err != nil {
		return nil, err
	}
	if req != nil {
		s.enqueueParentNotification(ctx, req.TenantID, req, changeRequestID, enrollment.MailKindChangeRequestQuestion)
	}
	return s.loadAggregate(ctx, changeRequestID, true)
}

// reviewOutcome parameterizes the shared approve/reject flow. apply, when
// set, changes the request inside the locked transaction before the change
// request is marked reviewed.
type reviewOutcome struct {
	noteRequiredMsg string
	status          string
	emailKind       string
	apply           func(ctx context.Context, row *ChangeRequest, input enrollment.ReviewChangeRequestInput) error
}

func (s *ChangeRequests) review(ctx context.Context, changeRequestID int64, input enrollment.ReviewChangeRequestInput, outcome reviewOutcome) (*changeRequestCase, error) {
	note := strings.TrimSpace(input.Note)
	if note == "" {
		return nil, fmt.Errorf("%w: %s", enrollment.ErrChangeRequestInvalidData, outcome.noteRequiredMsg)
	}
	var req *enrollmentModels.Request
	if err := s.withLockedChangeRequest(ctx, changeRequestID, func(txCtx context.Context, row *ChangeRequest) error {
		if row.Status != enrollment.ChangeRequestStatusPendingReview {
			return enrollment.ErrChangeRequestInvalidStatus
		}
		loadedReq, err := s.requestByID(txCtx, row.RequestID, false)
		if err != nil {
			return err
		}
		req = loadedReq
		if outcome.apply != nil {
			if err := outcome.apply(txCtx, row, input); err != nil {
				return err
			}
		}
		if err := s.deps.Requests.MarkChangeRequestReviewed(txCtx, row.ID, outcome.status, &note, input.ActorAccountID, time.Now()); err != nil {
			return err
		}
		actorID := input.ActorAccountID
		return s.deps.Requests.InsertChangeRequestMessage(txCtx, &enrollment.ChangeRequestMessage{
			ChangeRequestID: row.ID, AuthorType: enrollment.ChangeRequestMessageAuthorStaff, AuthorAccountID: &actorID, Body: note,
		})
	}); err != nil {
		return nil, err
	}
	if req != nil {
		s.enqueueParentNotification(ctx, req.TenantID, req, changeRequestID, outcome.emailKind)
	}
	return s.loadAggregate(ctx, changeRequestID, true)
}

// requestByID reads a request; a missing one is ErrRequestNotFound.
func (s *ChangeRequests) requestByID(ctx context.Context, id int64, lock bool) (*enrollmentModels.Request, error) {
	req, err := decodedRequestByID(ctx, s.deps.Requests, id, lock)
	if err != nil {
		if !s.deps.Runtime.NotFound(err) {
			return nil, err
		}
		return nil, enrollment.ErrRequestNotFound
	}
	return req, nil
}

// requestByToken resolves the request behind a status token in the
// administrative transaction.
func (s *ChangeRequests) requestByToken(ctx context.Context, token string) (*enrollmentModels.Request, int64, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, 0, enrollment.ErrRequestNotFound
	}
	var req *enrollmentModels.Request
	if err := s.deps.Runtime.AdminTx(ctx, func(adminCtx context.Context) error {
		loaded, err := decodedRequestByToken(adminCtx, s.deps.Requests, token, false)
		if err != nil {
			if !s.deps.Runtime.NotFound(err) {
				return err
			}
			return enrollment.ErrRequestNotFound
		}
		if loaded.StatusTokenExpires != nil && time.Now().After(*loaded.StatusTokenExpires) {
			return enrollment.ErrRequestNotFound
		}
		req = loaded
		return nil
	}); err != nil {
		return nil, 0, err
	}
	return req, req.TenantID, nil
}

// flipRenewalChildrenToSubmitted moves pending_renewal and auto_renewed
// children to submitted and mirrors it on the in-memory rows.
func (s *ChangeRequests) flipRenewalChildrenToSubmitted(ctx context.Context, children []*RequestChild) error {
	for _, child := range children {
		if child.Status != enrollmentModels.ChildStatusPendingRenewal && child.Status != enrollmentModels.ChildStatusAutoRenewed {
			continue
		}
		if err := s.deps.Children.UpdateChildStatus(ctx, child.ID, enrollmentModels.ChildStatusSubmitted, nil, 0); err != nil {
			return fmt.Errorf("change request: promote renewal child %d: %w", child.ID, err)
		}
		child.Status = enrollmentModels.ChildStatusSubmitted
	}
	return nil
}

func (s *ChangeRequests) ensureCanCreate(ctx context.Context, req *enrollmentModels.Request, children []*RequestChild) error {
	if err := s.intake.ensureChangeRequestDraftAvailable(ctx, req, children); err != nil {
		return enrollment.ErrChangeRequestNotAllowed
	}
	phase, err := s.intake.intakePhase(ctx, req.PhaseID)
	if err != nil && !s.deps.Runtime.NotFound(err) {
		return err
	}
	if err != nil || phase == nil || !phase.IsActive {
		return enrollment.ErrChangeRequestNotAllowed
	}
	if !phase.EnrollmentWindowOpen(time.Now()) {
		return enrollment.ErrEnrollmentWindowClosed
	}
	return nil
}

func (s *ChangeRequests) ensureNoOpenChangeRequest(ctx context.Context, requestID int64) error {
	rows, err := changeRequestsFromOwner(s.deps.Requests.ChangeRequestsForRequest(ctx, requestID))
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status == enrollment.ChangeRequestStatusPendingReview || row.Status == enrollment.ChangeRequestStatusNeedsParentResponse {
			return fmt.Errorf("%w: an open change request already exists", enrollment.ErrChangeRequestNotAllowed)
		}
	}
	return nil
}

// formCapabilities resolves the form capabilities of a proposal. An approval
// pins the care-offering capability the proposal was created with.
func (s *ChangeRequests) formCapabilities(ctx context.Context, pinnedCareOfferings *bool) (enrollment.FormCapabilities, error) {
	if pinnedCareOfferings == nil {
		capabilities, err := s.intake.formCapabilities(ctx)
		if err != nil {
			return enrollment.FormCapabilities{}, fmt.Errorf("change request: resolve form capabilities: %w", err)
		}
		return capabilities, nil
	}
	if s.deps.Settings == nil {
		return enrollment.FormCapabilities{}, errSettingsNotConfigured
	}
	collectGrade, err := s.deps.Settings.CollectGradeLevel(ctx)
	if err != nil {
		return enrollment.FormCapabilities{}, fmt.Errorf("change request: resolve %s: %w", settingCollectGradeLevel, err)
	}
	collectClass, err := s.deps.Settings.CollectSchoolClass(ctx)
	if err != nil {
		return enrollment.FormCapabilities{}, fmt.Errorf("change request: resolve %s: %w", settingCollectSchoolClass, err)
	}
	return enrollment.FormCapabilities{
		CollectGradeLevel: collectGrade, CollectSchoolClass: collectGrade && collectClass, CareOfferingsEnabled: *pinnedCareOfferings,
	}, nil
}

// withLockedChangeRequest runs fn on the locked change request in its
// tenant's transaction. It locks the parent request first, the order the
// cleanup and every edit flow take, so the two cannot deadlock.
func (s *ChangeRequests) withLockedChangeRequest(ctx context.Context, id int64, fn func(context.Context, *ChangeRequest) error) error {
	row, err := s.readChangeRequest(ctx, id, false)
	if err != nil && !s.deps.Runtime.NotFound(err) {
		return err
	}
	if err != nil || row == nil {
		return enrollment.ErrChangeRequestNotFound
	}
	return s.deps.Runtime.TenantTx(ctx, row.TenantID, func(txCtx context.Context) error {
		if _, err := s.requestByID(txCtx, row.RequestID, true); err != nil {
			return err
		}
		locked, err := s.readChangeRequest(txCtx, id, true)
		if err != nil && !s.deps.Runtime.NotFound(err) {
			return err
		}
		if err != nil || locked == nil {
			return enrollment.ErrChangeRequestNotFound
		}
		return fn(txCtx, locked)
	})
}

func (s *ChangeRequests) loadAggregate(ctx context.Context, id int64, includeInternal bool) (*changeRequestCase, error) {
	row, err := s.readChangeRequest(ctx, id, false)
	if err != nil && !s.deps.Runtime.NotFound(err) {
		return nil, err
	}
	if err != nil || row == nil {
		return nil, enrollment.ErrChangeRequestNotFound
	}
	return s.aggregateFromRow(ctx, row, includeInternal)
}

func (s *ChangeRequests) loadAggregateForTenant(ctx context.Context, tenantID int64, id int64, includeInternal bool) (*changeRequestCase, error) {
	var agg *changeRequestCase
	err := s.deps.Runtime.TenantTx(ctx, tenantID, func(txCtx context.Context) error {
		loaded, err := s.loadAggregate(txCtx, id, includeInternal)
		agg = loaded
		return err
	})
	return agg, err
}

func (s *ChangeRequests) aggregateFromRow(ctx context.Context, row *ChangeRequest, includeInternal bool) (*changeRequestCase, error) {
	req, err := s.requestByID(ctx, row.RequestID, false)
	if err != nil {
		return nil, err
	}
	children, err := decodedChildrenOfRequest(ctx, s.deps.Children, req.ID, false)
	if err != nil {
		return nil, fmt.Errorf("change request: list aggregate children: %w", err)
	}
	messages, err := s.deps.Requests.ChangeRequestMessages(ctx, []int64{row.ID}, includeInternal)
	if err != nil {
		return nil, err
	}
	var phase *enrollment.Phase
	if s.deps.Catalog != nil {
		phase, err = s.intake.intakePhase(ctx, req.PhaseID)
		if err != nil && !s.deps.Runtime.NotFound(err) {
			return nil, err
		}
	}
	return &changeRequestCase{ChangeRequest: row, Request: req, Children: children, Messages: messages, Phase: phase}, nil
}

// readChangeRequest reads and decodes one change request, optionally
// locked.
func (s *ChangeRequests) readChangeRequest(ctx context.Context, id int64, lock bool) (*ChangeRequest, error) {
	var (
		value *enrollment.ChangeRequest
		err   error
	)
	if lock {
		value, err = s.deps.Requests.ChangeRequestByIDForUpdate(ctx, id)
	} else {
		value, err = s.deps.Requests.ChangeRequestByID(ctx, id)
	}
	if err != nil {
		return nil, err
	}
	return changeRequestFromOwner(value)
}
