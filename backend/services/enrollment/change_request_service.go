package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var (
	ErrChangeRequestNotFound      = errors.New("enrollment change request not found")
	ErrChangeRequestNotAllowed    = errors.New("enrollment change request is not allowed")
	ErrChangeRequestInvalidStatus = errors.New("enrollment change request has invalid status")
	ErrChangeRequestInvalidData   = errors.New("enrollment change request data is invalid")
	ErrChangeRequestConflict      = errors.New("enrollment change request conflicts with newer enrollment data")
)

type CreateChangeRequestInput struct {
	Submission         SubmitRequest
	ParentNote         string
	CreatedByAccountID *int64
}

type ChangeRequestMessageInput struct {
	Body           string
	ActorAccountID int64
}

type ReviewChangeRequestInput struct {
	Note           string
	ActorAccountID int64
	ActorRole      string
}

type CorrectApprovedChildDataInput struct {
	RequestID         int64
	ChildID           int64
	FirstName         string
	LastName          string
	DateOfBirth       timezone.Date
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	Reason            string
	ActorAccountID    int64
}

// CompanionGraphCoordinator carries the two things a write that touches several
// children's "läuft mit" graph at once needs from the student domain: the row
// locks, in the global ascending-id order every companion writer follows, and
// the decision of the stranding verdicts the per-child writes deferred while the
// coordinated edit was still half-applied.
// Implemented by services/users studentService.
type CompanionGraphCoordinator interface {
	LockCompanionGraph(ctx context.Context, subjectIDs []int64, additional []int64) error
	VerifyCompanionStrandingBatch(ctx context.Context) error
}

type ChangeRequestDecisionApplier interface {
	applyApprovedChangeRequestOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*enrollmentModels.RequestChild, error)
	SyncApprovedChildData(ctx context.Context, input SyncApprovedChildDataInput) (*enrollmentModels.RequestChild, error)
}

type ChangeRequestAggregate struct {
	ChangeRequest *enrollmentModels.ChangeRequest
	Request       *enrollmentModels.Request
	Children      []*enrollmentModels.RequestChild
	Messages      []*enrollmentModels.ChangeRequestMessage
	Phase         *enrollmentModels.Phase
}

type ChangeRequestFilters struct {
	RequestID int64
	Status    string
	Limit     int
}

type ChangeRequestService interface {
	Create(ctx context.Context, token string, input CreateChangeRequestInput) (*ChangeRequestAggregate, error)
	ListPublic(ctx context.Context, token string) ([]*ChangeRequestAggregate, error)
	ParentReply(ctx context.Context, token string, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error)
	ListAdmin(ctx context.Context, filters ChangeRequestFilters) ([]*ChangeRequestAggregate, error)
	GetAdmin(ctx context.Context, changeRequestID int64) (*ChangeRequestAggregate, error)
	AskQuestion(ctx context.Context, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error)
	Reject(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error)
	Approve(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error)
	CorrectApprovedChildData(ctx context.Context, input CorrectApprovedChildDataInput) (*ChangeRequestAggregate, error)
}

type ChangeRequestServiceConfig struct {
	ChangeRequestRepo        enrollmentModels.ChangeRequestRepository
	MessageRepo              enrollmentModels.ChangeRequestMessageRepository
	RequestRepo              enrollmentModels.RequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestGuardianRepo      enrollmentModels.RequestGuardianRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	CareOfferingRepo         enrollmentModels.CareOfferingRepository
	FormSchemaRepo           enrollmentModels.FormSchemaRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	SchoolRepo               platformModels.SchoolRepository
	GuardianProfileRepo      userModels.GuardianProfileRepository
	GuardianPhoneRepo        userModels.GuardianPhoneNumberRepository
	// StudentRepo and GuardianAuthorizer back the existing_students pin
	// reconciliation on approval (#1663): an approved change request may edit a
	// child's name/birthday, and the pinned matched student must follow that
	// edit — the same resolution + per-student permission gate Submit and
	// ReplaceEditable run. Both fail CLOSED like the submit path: an unwired
	// authorizer refuses a pinned re-enrollment rather than waving it through,
	// and an unwired StudentRepo cannot re-resolve an edited identity. The
	// factory always wires both; only narrow test setups leave them nil.
	StudentRepo          userModels.StudentRepository
	GuardianAuthorizer   GuardianStudentAuthorizer
	DecisionService      ChangeRequestDecisionApplier
	CompanionGraphLocker CompanionGraphCoordinator
	Settings             RequestSettingsResolver
	OutboxEnqueuer       platformModels.OutboxEnqueuer
	FrontendURL          string
	ParentsURL           string
	DB                   *bun.DB
	Logger               *slog.Logger
}

type changeRequestService struct {
	ChangeRequestServiceConfig
}

func NewChangeRequestService(cfg ChangeRequestServiceConfig) ChangeRequestService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	cfg.ParentsURL = strings.TrimRight(strings.TrimSpace(cfg.ParentsURL), "/")
	if cfg.ParentsURL == "" {
		panic("PARENTS_URL is required")
	}
	cfg.FrontendURL = strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/")
	return &changeRequestService{ChangeRequestServiceConfig: cfg}
}

func (s *changeRequestService) Create(ctx context.Context, token string, input CreateChangeRequestInput) (*ChangeRequestAggregate, error) {
	req, tenantID, err := s.requestByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	var created *enrollmentModels.ChangeRequest
	err = tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		lockedReq, err := s.RequestRepo.FindByStatusTokenForUpdate(txCtx, strings.TrimSpace(token))
		if err != nil {
			return ErrRequestNotFound
		}
		children, err := s.RequestChildRepo.ListByRequestIDForUpdate(txCtx, lockedReq.ID)
		if err != nil {
			return fmt.Errorf("change request: lock children: %w", err)
		}
		if err := s.ensureCanCreate(txCtx, lockedReq, children); err != nil {
			return err
		}
		if err := s.ensureNoOpenChangeRequest(txCtx, lockedReq.ID); err != nil {
			return err
		}
		capabilities, err := s.formCapabilities(txCtx, nil)
		if err != nil {
			return err
		}
		prepared, _, _, _, err := s.prepareProposed(txCtx, lockedReq, children, input.Submission, capabilities, false)
		if err != nil {
			return err
		}
		baseSnapshot, err := s.currentSnapshot(txCtx, lockedReq, children)
		if err != nil {
			return err
		}
		proposedSnapshot := submitSnapshot(prepared)
		note := strings.TrimSpace(input.ParentNote)
		var notePtr *string
		if note != "" {
			notePtr = &note
		}
		row := &enrollmentModels.ChangeRequest{
			RequestID:                      lockedReq.ID,
			Status:                         enrollmentModels.ChangeRequestStatusPendingReview,
			ParentNote:                     notePtr,
			BaseSnapshot:                   baseSnapshot,
			ProposedSnapshot:               proposedSnapshot,
			Diff:                           snapshotDiff(baseSnapshot, proposedSnapshot),
			CareOfferingsEnabledAtCreation: capabilities.CareOfferingsEnabled,
			CreatedByAccountID:             input.CreatedByAccountID,
		}
		if err := s.ChangeRequestRepo.Create(txCtx, row); err != nil {
			return err
		}
		if note != "" {
			if err := s.MessageRepo.Create(txCtx, &enrollmentModels.ChangeRequestMessage{
				ChangeRequestID: row.ID,
				AuthorType:      enrollmentModels.ChangeRequestMessageAuthorParent,
				AuthorAccountID: input.CreatedByAccountID,
				Body:            note,
			}); err != nil {
				return err
			}
		}
		created = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.enqueueChangeRequestSubmitted(ctx, tenantID, req, created)
	return s.loadAggregateForTenant(ctx, tenantID, created.ID, false)
}

func (s *changeRequestService) ListPublic(ctx context.Context, token string) ([]*ChangeRequestAggregate, error) {
	req, tenantID, err := s.requestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	var rows []*enrollmentModels.ChangeRequest
	err = tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		list, listErr := s.ChangeRequestRepo.ListByRequestID(txCtx, req.ID)
		rows = list
		return listErr
	})
	if err != nil {
		return nil, err
	}
	out := make([]*ChangeRequestAggregate, 0, len(rows))
	for _, row := range rows {
		agg, loadErr := s.loadAggregateForTenant(ctx, tenantID, row.ID, false)
		if loadErr != nil {
			return nil, loadErr
		}
		out = append(out, agg)
	}
	return out, nil
}

func (s *changeRequestService) ParentReply(ctx context.Context, token string, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error) {
	req, tenantID, err := s.requestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		return nil, fmt.Errorf("%w: message body is required", ErrChangeRequestInvalidData)
	}
	err = tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		row, err := s.ChangeRequestRepo.FindByIDForUpdate(txCtx, changeRequestID)
		if err != nil || row == nil || row.RequestID != req.ID {
			return ErrChangeRequestNotFound
		}
		if row.Status != enrollmentModels.ChangeRequestStatusNeedsParentResponse {
			return ErrChangeRequestInvalidStatus
		}
		if err := s.MessageRepo.Create(txCtx, &enrollmentModels.ChangeRequestMessage{
			ChangeRequestID: row.ID,
			AuthorType:      enrollmentModels.ChangeRequestMessageAuthorParent,
			AuthorAccountID: nil,
			Body:            body,
		}); err != nil {
			return err
		}
		return s.ChangeRequestRepo.SetStatus(txCtx, row.ID, enrollmentModels.ChangeRequestStatusPendingReview)
	})
	if err != nil {
		return nil, err
	}
	s.enqueueParentReply(ctx, tenantID, req, changeRequestID)
	return s.loadAggregateForTenant(ctx, tenantID, changeRequestID, false)
}

func (s *changeRequestService) ListAdmin(ctx context.Context, filters ChangeRequestFilters) ([]*ChangeRequestAggregate, error) {
	rows, err := s.ChangeRequestRepo.ListAdmin(ctx, enrollmentModels.ChangeRequestListFilters{
		RequestID: filters.RequestID,
		Status:    filters.Status,
		Limit:     filters.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*ChangeRequestAggregate, 0, len(rows))
	for _, row := range rows {
		agg, err := s.loadAggregate(ctx, row.ID, true)
		if err != nil {
			return nil, err
		}
		out = append(out, agg)
	}
	return out, nil
}

func (s *changeRequestService) GetAdmin(ctx context.Context, changeRequestID int64) (*ChangeRequestAggregate, error) {
	return s.loadAggregate(ctx, changeRequestID, true)
}

// CorrectApprovedChildData fixes the authoritative enrollment record and then
// applies the existing enrollment-to-student projection. Offerings, consents,
// guardians, and all other submission data deliberately remain untouched.
func (s *changeRequestService) CorrectApprovedChildData(ctx context.Context, input CorrectApprovedChildDataInput) (*ChangeRequestAggregate, error) {
	firstName, lastName, reason, err := validateAdminCorrectionInput(input)
	if err != nil {
		return nil, err
	}

	req, err := s.RequestRepo.FindByIDForUpdate(ctx, input.RequestID)
	if err != nil || req == nil {
		return nil, ErrRequestNotFound
	}
	children, err := s.RequestChildRepo.ListByRequestIDForUpdate(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("admin data correction: lock children: %w", err)
	}
	child := requestChildByID(children, input.ChildID)
	if child == nil {
		return nil, ErrChangeRequestNotFound
	}
	if err := validateAdminCorrectionChild(child, s.DecisionService); err != nil {
		return nil, err
	}

	targetGradeLevel, targetSchoolClass, err := s.prepareAdminCorrectionTargets(ctx, req, child, input)
	if err != nil {
		return nil, err
	}
	corrected := SubmitChild{
		ID:                child.ID,
		FirstName:         firstName,
		LastName:          lastName,
		DateOfBirth:       input.DateOfBirth,
		TargetGradeLevel:  targetGradeLevel,
		TargetSchoolClass: targetSchoolClass,
	}

	baseSnapshot, err := s.currentSnapshot(ctx, req, children)
	if err != nil {
		return nil, err
	}
	if err := s.rejectOpenChangeRequestsForAdminCorrection(ctx, req, reason, input.ActorAccountID); err != nil {
		return nil, err
	}
	child.FirstName = corrected.FirstName
	child.LastName = corrected.LastName
	child.DateOfBirth = corrected.DateOfBirth
	child.TargetGradeLevel = corrected.TargetGradeLevel
	child.TargetSchoolClass = corrected.TargetSchoolClass
	if err := s.RequestChildRepo.UpdateData(ctx, child); err != nil {
		return nil, fmt.Errorf("admin data correction: update enrollment child: %w", err)
	}
	if _, err := s.DecisionService.SyncApprovedChildData(ctx, SyncApprovedChildDataInput{
		RequestID:           req.ID,
		ChildID:             child.ID,
		ActorAccountID:      input.ActorAccountID,
		ReplaceTargetedData: false,
	}); err != nil {
		return nil, fmt.Errorf("admin data correction: sync student: %w", err)
	}
	proposedSnapshot, err := s.currentSnapshot(ctx, req, children)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	row := &enrollmentModels.ChangeRequest{
		RequestID:           req.ID,
		RequestChildID:      &child.ID,
		Origin:              enrollmentModels.ChangeRequestOriginAdmin,
		Status:              enrollmentModels.ChangeRequestStatusApproved,
		AdminDecisionNote:   &reason,
		BaseSnapshot:        baseSnapshot,
		ProposedSnapshot:    proposedSnapshot,
		Diff:                snapshotDiff(baseSnapshot, proposedSnapshot),
		CreatedByAccountID:  &input.ActorAccountID,
		ReviewedByAccountID: &input.ActorAccountID,
		ReviewedAt:          &now,
	}
	if err := s.ChangeRequestRepo.Create(ctx, row); err != nil {
		return nil, fmt.Errorf("admin data correction: create audit entry: %w", err)
	}
	return s.aggregateFromRow(ctx, row, true)
}

func (s *changeRequestService) rejectOpenChangeRequestsForAdminCorrection(
	ctx context.Context,
	req *enrollmentModels.Request,
	reason string,
	actorAccountID int64,
) error {
	rows, err := s.ChangeRequestRepo.ListOpenByRequestIDForUpdate(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("admin data correction: lock open change requests: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}

	note := "Diese Änderungsanfrage wurde durch eine direkte Korrektur der OGS ersetzt. Grund: " + reason
	now := time.Now()
	for _, row := range rows {
		if err := s.ChangeRequestRepo.MarkReviewed(
			ctx,
			row.ID,
			enrollmentModels.ChangeRequestStatusRejected,
			&note,
			actorAccountID,
			now,
		); err != nil {
			return fmt.Errorf("admin data correction: reject open change request %d: %w", row.ID, err)
		}
		actorID := actorAccountID
		if err := s.MessageRepo.Create(ctx, &enrollmentModels.ChangeRequestMessage{
			ChangeRequestID: row.ID,
			AuthorType:      enrollmentModels.ChangeRequestMessageAuthorStaff,
			AuthorAccountID: &actorID,
			Body:            note,
		}); err != nil {
			return fmt.Errorf("admin data correction: record rejection message for change request %d: %w", row.ID, err)
		}
		s.enqueueReviewed(
			ctx,
			req.GetTenantID(),
			req,
			row.ID,
			platformModels.EmailKindEnrollmentChangeRequestRejected,
		)
	}
	return nil
}

func validateAdminCorrectionInput(input CorrectApprovedChildDataInput) (string, string, string, error) {
	firstName := strings.TrimSpace(input.FirstName)
	lastName := strings.TrimSpace(input.LastName)
	reason := strings.TrimSpace(input.Reason)
	if input.RequestID <= 0 || input.ChildID <= 0 || input.ActorAccountID <= 0 || firstName == "" || lastName == "" || input.DateOfBirth.IsZero() || reason == "" {
		return "", "", "", fmt.Errorf("%w: request, child, actor, child data, and reason are required", ErrChangeRequestInvalidData)
	}
	return firstName, lastName, reason, nil
}

func requestChildByID(children []*enrollmentModels.RequestChild, childID int64) *enrollmentModels.RequestChild {
	for _, child := range children {
		if child != nil && child.ID == childID {
			return child
		}
	}
	return nil
}

func validateAdminCorrectionChild(child *enrollmentModels.RequestChild, decisionService ChangeRequestDecisionApplier) error {
	if child.Status != enrollmentModels.ChildStatusApproved || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 || decisionService == nil {
		return fmt.Errorf("%w: only approved children with a linked student can be corrected", ErrChangeRequestNotAllowed)
	}
	return nil
}

func (s *changeRequestService) prepareAdminCorrectionTargets(
	ctx context.Context,
	req *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	input CorrectApprovedChildDataInput,
) (*int16, *string, error) {
	normalizedSchoolClass := normalizedOptionalString(input.TargetSchoolClass)
	gradeChanged := !sameOptionalInt16(child.TargetGradeLevel, input.TargetGradeLevel)
	classChanged := !sameOptionalString(child.TargetSchoolClass, normalizedSchoolClass)
	if !gradeChanged && !classChanged {
		return child.TargetGradeLevel, child.TargetSchoolClass, nil
	}

	collectGradeLevel, collectSchoolClass, err := s.adminCorrectionCapabilities(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := validateAdminCorrectionTargetPermissions(gradeChanged, classChanged, collectGradeLevel, collectSchoolClass); err != nil {
		return nil, nil, err
	}

	targetGradeLevel, targetSchoolClass, err := s.changedAdminCorrectionTargets(ctx, child, input, normalizedSchoolClass, gradeChanged, classChanged)
	if err != nil {
		return nil, nil, err
	}

	corrected := SubmitChild{TargetGradeLevel: targetGradeLevel, TargetSchoolClass: targetSchoolClass}
	if collectGradeLevel && collectSchoolClass {
		return s.validateAdminCorrectionSchoolClass(ctx, req.PhaseID, corrected)
	}
	if err := validatePreservedSchoolClassGrade(gradeChanged, targetGradeLevel, targetSchoolClass); err != nil {
		return nil, nil, err
	}
	return targetGradeLevel, targetSchoolClass, nil
}

func validateAdminCorrectionTargetPermissions(gradeChanged, classChanged, collectGradeLevel, collectSchoolClass bool) error {
	if gradeChanged && !collectGradeLevel {
		return fmt.Errorf("%w: target_grade_level cannot be changed while grade collection is disabled", ErrChangeRequestInvalidData)
	}
	if classChanged && (!collectGradeLevel || !collectSchoolClass) {
		return fmt.Errorf("%w: target_school_class cannot be changed while school-class collection is disabled", ErrChangeRequestInvalidData)
	}
	return nil
}

func (s *changeRequestService) changedAdminCorrectionTargets(
	ctx context.Context,
	child *enrollmentModels.RequestChild,
	input CorrectApprovedChildDataInput,
	normalizedSchoolClass *string,
	gradeChanged, classChanged bool,
) (*int16, *string, error) {
	targetGradeLevel := child.TargetGradeLevel
	var err error
	if gradeChanged {
		targetGradeLevel, err = s.validateAdminCorrectionGrade(ctx, input.TargetGradeLevel)
		if err != nil {
			return nil, nil, err
		}
	}
	targetSchoolClass := child.TargetSchoolClass
	if classChanged {
		targetSchoolClass = normalizedSchoolClass
	}
	return targetGradeLevel, targetSchoolClass, nil
}

func validatePreservedSchoolClassGrade(gradeChanged bool, targetGradeLevel *int16, targetSchoolClass *string) error {
	if !gradeChanged {
		return nil
	}
	class := trimmedOptionalString(targetSchoolClass)
	grade := strconv.Itoa(int(*targetGradeLevel))
	if prefix := schoolclass.GradePrefix(class); prefix != "" && prefix != grade {
		return fmt.Errorf("%w: existing target_school_class %q does not match target grade %s", ErrChangeRequestInvalidData, class, grade)
	}
	return nil
}

func (s *changeRequestService) adminCorrectionCapabilities(ctx context.Context) (bool, bool, error) {
	if s.Settings == nil {
		return false, false, errors.New("admin data correction: enrollment settings resolver is not configured")
	}
	collectGradeLevel, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectGradeLevel)
	if err != nil {
		return false, false, fmt.Errorf("admin data correction: resolve %s: %w", configModel.KeyEnrollmentCollectGradeLevel, err)
	}
	collectSchoolClass, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectSchoolClass)
	if err != nil {
		return false, false, fmt.Errorf("admin data correction: resolve %s: %w", configModel.KeyEnrollmentCollectSchoolClass, err)
	}
	return collectGradeLevel, collectSchoolClass, nil
}

func (s *changeRequestService) validateAdminCorrectionGrade(ctx context.Context, grade *int16) (*int16, error) {
	if grade == nil {
		return nil, fmt.Errorf("%w: target_grade_level is required", ErrChangeRequestInvalidData)
	}
	rs := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: s.Settings}}
	gradeMax, err := rs.resolveGradeMax(ctx)
	if err != nil {
		return nil, err
	}
	if *grade < 1 || int(*grade) > gradeMax {
		return nil, fmt.Errorf("%w: target_grade_level must be between 1 and %d", ErrChangeRequestInvalidData, gradeMax)
	}
	return grade, nil
}

func (s *changeRequestService) validateAdminCorrectionSchoolClass(ctx context.Context, phaseID int64, child SubmitChild) (*int16, *string, error) {
	phase, err := s.PhaseRepo.FindByID(ctx, phaseID)
	if err != nil {
		return nil, nil, fmt.Errorf("admin data correction: load phase: %w", err)
	}
	if phase == nil {
		return nil, nil, errors.New("admin data correction: phase not found")
	}
	children := []SubmitChild{child}
	rs := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: s.Settings}}
	if err := rs.validateAndNormalizeSchoolClasses(ctx, phase, children); err != nil {
		return nil, nil, err
	}
	return children[0].TargetGradeLevel, children[0].TargetSchoolClass, nil
}

func (s *changeRequestService) AskQuestion(ctx context.Context, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error) {
	body := strings.TrimSpace(input.Body)
	if body == "" {
		return nil, fmt.Errorf("%w: message body is required", ErrChangeRequestInvalidData)
	}
	var req *enrollmentModels.Request
	err := s.withLockedChangeRequest(ctx, changeRequestID, func(txCtx context.Context, row *enrollmentModels.ChangeRequest) error {
		if row.Status != enrollmentModels.ChangeRequestStatusPendingReview {
			return ErrChangeRequestInvalidStatus
		}
		loadedReq, err := s.RequestRepo.FindByID(txCtx, row.RequestID)
		if err != nil {
			return ErrRequestNotFound
		}
		req = loadedReq
		actorID := input.ActorAccountID
		if err := s.MessageRepo.Create(txCtx, &enrollmentModels.ChangeRequestMessage{
			ChangeRequestID: row.ID,
			AuthorType:      enrollmentModels.ChangeRequestMessageAuthorStaff,
			AuthorAccountID: &actorID,
			Body:            body,
		}); err != nil {
			return err
		}
		return s.ChangeRequestRepo.SetStatus(txCtx, row.ID, enrollmentModels.ChangeRequestStatusNeedsParentResponse)
	})
	if err != nil {
		return nil, err
	}
	if req != nil {
		s.enqueueAdminQuestion(ctx, req.GetTenantID(), req, changeRequestID)
	}
	return s.loadAggregate(ctx, changeRequestID, true)
}

func (s *changeRequestService) Reject(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error) {
	return s.review(ctx, changeRequestID, input, reviewOutcome{
		noteRequiredMsg: "rejection note is required",
		status:          enrollmentModels.ChangeRequestStatusRejected,
		emailKind:       platformModels.EmailKindEnrollmentChangeRequestRejected,
	})
}

func (s *changeRequestService) Approve(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error) {
	return s.review(ctx, changeRequestID, input, reviewOutcome{
		noteRequiredMsg: "approval note is required",
		status:          enrollmentModels.ChangeRequestStatusApproved,
		emailKind:       platformModels.EmailKindEnrollmentChangeRequestApproved,
		apply:           s.applyApprovedChange,
	})
}

// reviewOutcome parameterizes the shared Approve/Reject review flow. apply,
// when set, mutates the underlying request inside the locked transaction
// before the change request is marked reviewed (approval path only).
type reviewOutcome struct {
	noteRequiredMsg string
	status          string
	emailKind       string
	apply           func(ctx context.Context, row *enrollmentModels.ChangeRequest, input ReviewChangeRequestInput) error
}

func (s *changeRequestService) review(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput, outcome reviewOutcome) (*ChangeRequestAggregate, error) {
	note := strings.TrimSpace(input.Note)
	if note == "" {
		return nil, fmt.Errorf("%w: %s", ErrChangeRequestInvalidData, outcome.noteRequiredMsg)
	}
	var req *enrollmentModels.Request
	err := s.withLockedChangeRequest(ctx, changeRequestID, func(txCtx context.Context, row *enrollmentModels.ChangeRequest) error {
		if row.Status != enrollmentModels.ChangeRequestStatusPendingReview {
			return ErrChangeRequestInvalidStatus
		}
		loadedReq, err := s.RequestRepo.FindByID(txCtx, row.RequestID)
		if err != nil {
			return ErrRequestNotFound
		}
		req = loadedReq
		if outcome.apply != nil {
			if err := outcome.apply(txCtx, row, input); err != nil {
				return err
			}
		}
		now := time.Now()
		if err := s.ChangeRequestRepo.MarkReviewed(txCtx, row.ID, outcome.status, &note, input.ActorAccountID, now); err != nil {
			return err
		}
		actorID := input.ActorAccountID
		return s.MessageRepo.Create(txCtx, &enrollmentModels.ChangeRequestMessage{
			ChangeRequestID: row.ID,
			AuthorType:      enrollmentModels.ChangeRequestMessageAuthorStaff,
			AuthorAccountID: &actorID,
			Body:            note,
		})
	})
	if err != nil {
		return nil, err
	}
	if req != nil {
		s.enqueueReviewed(ctx, req.GetTenantID(), req, changeRequestID, outcome.emailKind)
	}
	return s.loadAggregate(ctx, changeRequestID, true)
}

func (s *changeRequestService) requestByToken(ctx context.Context, token string) (*enrollmentModels.Request, int64, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, 0, ErrRequestNotFound
	}
	var req *enrollmentModels.Request
	err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		loaded, err := s.RequestRepo.FindByStatusToken(adminCtx, token)
		if err != nil {
			return ErrRequestNotFound
		}
		if loaded.StatusTokenExpires != nil && time.Now().After(*loaded.StatusTokenExpires) {
			return ErrRequestNotFound
		}
		req = loaded
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return req, req.GetTenantID(), nil
}

func (s *changeRequestService) ensureCanCreate(ctx context.Context, req *enrollmentModels.Request, children []*enrollmentModels.RequestChild) error {
	rs := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: s.Settings}}
	if err := rs.ensureChangeRequestDraftAvailable(ctx, req, children); err != nil {
		return ErrChangeRequestNotAllowed
	}
	phase, err := s.PhaseRepo.FindByID(ctx, req.PhaseID)
	if err != nil || phase == nil || !phase.IsActive {
		return ErrChangeRequestNotAllowed
	}
	if !IsEnrollmentWindowOpen(phase, time.Now()) {
		return ErrEnrollmentWindowClosed
	}
	return nil
}

func (s *changeRequestService) formCapabilities(ctx context.Context, pinnedCareOfferings *bool) (FormCapabilities, error) {
	rs := &requestService{RequestServiceConfig: RequestServiceConfig{
		Settings:       s.Settings,
		FormSchemaRepo: s.FormSchemaRepo,
		Logger:         s.Logger,
	}}
	if pinnedCareOfferings != nil {
		if s.Settings == nil {
			return FormCapabilities{}, errors.New("enrollment settings resolver is not configured")
		}
		collectGrade, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectGradeLevel)
		if err != nil {
			return FormCapabilities{}, fmt.Errorf("change request: resolve %s: %w", configModel.KeyEnrollmentCollectGradeLevel, err)
		}
		collectClass, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectSchoolClass)
		if err != nil {
			return FormCapabilities{}, fmt.Errorf("change request: resolve %s: %w", configModel.KeyEnrollmentCollectSchoolClass, err)
		}
		return FormCapabilities{
			CollectGradeLevel:    collectGrade,
			CollectSchoolClass:   collectGrade && collectClass,
			CareOfferingsEnabled: *pinnedCareOfferings,
		}, nil
	}
	capabilities, err := rs.FormCapabilities(ctx)
	if err != nil {
		return FormCapabilities{}, fmt.Errorf("change request: resolve form capabilities: %w", err)
	}
	return capabilities, nil
}

func (s *changeRequestService) prepareProposed(
	ctx context.Context,
	req *enrollmentModels.Request,
	children []*enrollmentModels.RequestChild,
	incoming SubmitRequest,
	capabilities FormCapabilities,
	allowStoredHiddenOfferings bool,
) (SubmitRequest, [][]materializedOfferingSelection, *enrollmentModels.Phase, map[int64]*enrollmentModels.CareOffering, error) {
	editReq := incoming
	editReq.TenantID = req.GetTenantID()
	editReq.PhaseID = req.PhaseID
	editReq.GuardianEmail = req.GuardianEmail
	editReq.GuardianAccountID = req.GuardianAccountID
	if editReq.ConsentFlags == nil {
		editReq.ConsentFlags = map[string]any{}
	}
	if editReq.CustomData == nil {
		editReq.CustomData = map[string]any{}
	}
	if len(editReq.Children) != len(children) {
		return editReq, nil, nil, nil, fmt.Errorf("%w: child count changes are not supported for change requests", ErrChangeRequestInvalidData)
	}
	if err := validateChangeRequestChildIdentity(children, editReq.Children); err != nil {
		return editReq, nil, nil, nil, err
	}
	rs := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: s.Settings, FormSchemaRepo: s.FormSchemaRepo, Logger: s.Logger}}
	if allowStoredHiddenOfferings && !capabilities.CareOfferingsEnabled {
		// A disabled proposal snapshot contains the persisted, hidden links so
		// it stays stable across setting toggles. They are trusted internal
		// state, not fresh parent input; clear them before applying the public
		// disabled-capability validation and restore from the locked rows below.
		for i := range editReq.Children {
			editReq.Children[i].OfferingIDs = nil
			editReq.Children[i].OfferingDays = nil
		}
	}
	phase, err := s.PhaseRepo.FindByID(ctx, req.PhaseID)
	if err != nil || phase == nil || !phase.IsActive {
		return editReq, nil, nil, nil, ErrEnrollmentDisabled
	}
	openOfferings := []*enrollmentModels.CareOffering{}
	if capabilities.CareOfferingsEnabled {
		openOfferings, err = s.CareOfferingRepo.ListActiveByPhase(ctx, phase.ID)
		if err != nil {
			return editReq, nil, nil, nil, fmt.Errorf("change request: load phase offerings: %w", err)
		}
	}
	openByID := make(map[int64]*enrollmentModels.CareOffering, len(openOfferings))
	for _, offering := range openOfferings {
		openByID[offering.ID] = offering
	}
	changeRequestByID := map[int64]*enrollmentModels.CareOffering{}
	var offeringCatalogs []map[int64]*enrollmentModels.CareOffering
	if capabilities.CareOfferingsEnabled {
		var catalogErr error
		offeringCatalogs, changeRequestByID, catalogErr = s.changeRequestOfferingCatalogs(ctx, children, openByID)
		if catalogErr != nil {
			return editReq, nil, nil, nil, catalogErr
		}
	}
	capabilityOfferings := openOfferings
	if len(changeRequestByID) > 0 {
		capabilityOfferings = slices.Collect(maps.Values(changeRequestByID))
	}
	capabilities = EffectiveFormCapabilities(capabilities, capabilityOfferings)
	if err := normalizeSubmissionForCapabilities(&editReq, capabilities); err != nil {
		return editReq, nil, nil, nil, err
	}
	var materializedSelections [][]materializedOfferingSelection
	if capabilities.CareOfferingsEnabled {
		materializedSelections, err = materializeAndValidateChangeRequestChildrenOfferingSelections(
			editReq.Children,
			children,
			offeringCatalogs,
			phase.CareOfferingSelectionMode,
		)
		if err != nil {
			return editReq, nil, nil, nil, err
		}
	} else {
		childIDs := make([]int64, 0, len(children))
		for _, child := range children {
			childIDs = append(childIDs, child.ID)
		}
		existingLinks, linkErr := s.RequestChildOfferingRepo.ListByRequestChildIDs(ctx, childIDs)
		if linkErr != nil {
			return editReq, nil, nil, nil, fmt.Errorf("change request: preserve child offerings: %w", linkErr)
		}
		materializedSelections = preservedOfferingSelections(children, editReq.Children, existingLinks)
		applyPreservedOfferingSelections(editReq.Children, materializedSelections)
	}

	schema, err := s.schemaForRequest(ctx, req)
	if err != nil {
		return editReq, nil, nil, nil, err
	}
	legalBlocks, err := s.legalBlocksForRequest(ctx, schema)
	if err != nil {
		return editReq, nil, nil, nil, err
	}
	if err := normalizeAdditionalGuardians(&editReq); err != nil {
		return editReq, nil, nil, nil, err
	}
	if err := rs.validateSubmission(ctx, editReq, legalBlocks, capabilities); err != nil {
		return editReq, nil, nil, nil, err
	}
	// Concrete-class rules (#1833) hang off the tenant setting + phase, so
	// they run here rather than in validateSubmission - mirroring Submit and
	// ReplaceEditable. Without this a status-linked parent could propose an
	// unoffered target_school_class (or omit a required one for grade >= 2)
	// and staff approval would persist the invalid value.
	if err := rs.validateAndNormalizeSchoolClasses(ctx, phase, editReq.Children); err != nil {
		return editReq, nil, nil, nil, err
	}
	// eligible_school_classes may be a proper SUBSET of
	// available_school_classes, so the normalization above (which validates
	// against the available list) is not enough: without this a parent could
	// propose an available-but-ineligible class once the request is in
	// change-request mode and staff approval would persist it, silently
	// defeating the phase restriction Submit and ReplaceEditable enforce
	// (#1663). Runs on both create and approve — prepareProposed is the single
	// path both take. The trusted-source / rollover exemptions mirror
	// ReplaceEditable: those creation paths bypassed eligibility deliberately
	// and an edit must not newly reject what creation allowed.
	// The grade restriction rides along for the same reason: a phase limited to
	// whole grades must not be defeated by a change request that moves the
	// child into another grade (#1663).
	// validateChangedChildIdentityEligibility closes the third hole: the
	// audience's enrolled-status gate, re-applied to exactly the children whose
	// identity this edit REWRITES.
	if !isTrustedEnrollmentSource(req.SubmissionSource) && !hasRolloverGeneratedChild(children) {
		if err := validateChildGradeEligibility(phase, editReq.Children); err != nil {
			return editReq, nil, nil, nil, err
		}
		if err := validateChildClassEligibility(phase, editReq.Children); err != nil {
			return editReq, nil, nil, nil, err
		}
		if err := s.validateChangedChildIdentityEligibility(ctx, phase, editReq, children); err != nil {
			return editReq, nil, nil, nil, err
		}
	}
	if err := s.validateAccountLinkedGuardianEdits(ctx, req, editReq); err != nil {
		return editReq, nil, nil, nil, err
	}
	editReq.ConsentFlags = filterConsentFlags(editReq.ConsentFlags, legalBlocks)
	if err := rs.validateRequiredCustomFields(schema, editReq, changeRequestByID); err != nil {
		return editReq, nil, nil, nil, err
	}
	if err := rs.validateAccompaniedCompanionNote(schema, editReq, changeRequestByID); err != nil {
		return editReq, nil, nil, nil, err
	}
	if err := rs.validateConstrainedSchedules(schema, editReq, changeRequestByID); err != nil {
		return editReq, nil, nil, nil, err
	}
	byKey := buildFieldsByKey(schema)
	rawGuardian := editReq.CustomData
	existingCustomData := existingChildCustomDataBySubmittedIdentity(children, editReq.Children)
	for i := range editReq.Children {
		childCtx := fieldVisibilityContext{
			guardianAnswers: rawGuardian,
			childAnswers:    editReq.Children[i].CustomData,
			gradeLevel:      editReq.Children[i].TargetGradeLevel,
			offeringNames:   selectedOfferingNames(editReq.Children[i], changeRequestByID),
			fieldsByKey:     byKey,
		}
		sanitizedChild := sanitizeVisibleAnswers(schema, true, editReq.Children[i].CustomData, childCtx)
		pruneChildScheduleAnswers(schema, sanitizedChild, relevantCareDaysForChild(editReq.Children[i], changeRequestByID))
		editReq.Children[i].CustomData = mergeEditableCustomData(existingCustomData[i], sanitizedChild, schema, true)
	}
	editReq.CustomData = mergeEditableCustomData(
		req.CustomData,
		sanitizeVisibleAnswers(schema, false, rawGuardian, fieldVisibilityContext{
			guardianAnswers: rawGuardian,
			fieldsByKey:     byKey,
		}),
		schema,
		false,
	)
	return editReq, materializedSelections, phase, changeRequestByID, nil
}

func (s *changeRequestService) changeRequestOfferingCatalogs(
	ctx context.Context,
	children []*enrollmentModels.RequestChild,
	openByID map[int64]*enrollmentModels.CareOffering,
) ([]map[int64]*enrollmentModels.CareOffering, map[int64]*enrollmentModels.CareOffering, error) {
	childIDs := make([]int64, 0, len(children))
	childIndexByID := make(map[int64]int, len(children))
	for i, child := range children {
		if child == nil {
			continue
		}
		childIDs = append(childIDs, child.ID)
		childIndexByID[child.ID] = i
	}

	links, err := s.RequestChildOfferingRepo.ListByRequestChildIDsAtDate(ctx, childIDs, timezone.TodayDate())
	if err != nil {
		return nil, nil, fmt.Errorf("change request: load current child offerings: %w", err)
	}

	currentIDs := make(map[int64]bool)
	currentByChild := make([]map[int64]bool, len(children))
	for _, link := range links {
		if link == nil {
			continue
		}
		idx, ok := childIndexByID[link.RequestChildID]
		if !ok {
			continue
		}
		if currentByChild[idx] == nil {
			currentByChild[idx] = map[int64]bool{}
		}
		currentByChild[idx][link.CareOfferingID] = true
		if openByID[link.CareOfferingID] == nil {
			currentIDs[link.CareOfferingID] = true
		}
	}

	currentOfferingsByID := map[int64]*enrollmentModels.CareOffering{}
	if len(currentIDs) > 0 {
		ids := slices.Collect(maps.Keys(currentIDs))
		currentOfferings, err := s.CareOfferingRepo.ListByIDs(ctx, ids)
		if err != nil {
			return nil, nil, fmt.Errorf("change request: load current inactive offerings: %w", err)
		}
		for _, offering := range currentOfferings {
			if offering != nil {
				currentOfferingsByID[offering.ID] = offering
			}
		}
	}

	combinedByID := make(map[int64]*enrollmentModels.CareOffering, len(openByID)+len(currentOfferingsByID))
	for id, offering := range openByID {
		combinedByID[id] = offering
	}
	for id, offering := range currentOfferingsByID {
		combinedByID[id] = offering
	}

	catalogs := make([]map[int64]*enrollmentModels.CareOffering, len(children))
	for i := range children {
		catalog := make(map[int64]*enrollmentModels.CareOffering, len(openByID)+len(currentByChild[i]))
		for id, offering := range openByID {
			catalog[id] = offering
		}
		for id := range currentByChild[i] {
			if offering := combinedByID[id]; offering != nil {
				catalog[id] = offering
			}
		}
		catalogs[i] = catalog
	}
	return catalogs, combinedByID, nil
}

func materializeAndValidateChangeRequestChildrenOfferingSelections(
	children []SubmitChild,
	existingChildren []*enrollmentModels.RequestChild,
	catalogs []map[int64]*enrollmentModels.CareOffering,
	selectionMode string,
) ([][]materializedOfferingSelection, error) {
	out := make([][]materializedOfferingSelection, len(children))
	for i := range children {
		catalog := map[int64]*enrollmentModels.CareOffering{}
		if i < len(catalogs) && catalogs[i] != nil {
			catalog = catalogs[i]
		}
		catalogHadEntries := len(catalog) > 0
		if err := validateOfferingSelections([]SubmitChild{children[i]}, catalog); err != nil {
			return nil, fmt.Errorf("child %d: %w", i, err)
		}
		availableCatalog, err := availableCareOfferingsForGrade(catalog, children[i].TargetGradeLevel)
		if err != nil {
			return nil, fmt.Errorf("child %d: %w", i, err)
		}
		if err := validateOfferingSelectionsForChild(children[i], catalog, availableCatalog); err != nil {
			return nil, fmt.Errorf("child %d: %w", i, err)
		}
		catalog = availableCatalog
		manualChild := cloneSubmitChildrenOfferingSelections([]SubmitChild{children[i]})[0]
		selections, err := materializeOfferingSelections(children[i], catalog)
		if err != nil {
			return nil, fmt.Errorf("child %d: %w", i, err)
		}
		children[i].OfferingIDs, children[i].OfferingDays = selectionPayload(selections, catalog)
		if err := validateOfferingGroupRules([]SubmitChild{children[i]}, catalog); err != nil {
			return nil, err
		}
		if err := validateRequiredOfferings([]SubmitChild{children[i]}, catalog); err != nil {
			return nil, err
		}
		if !catalogHadEntries || hasChoosableCareOffering(catalog) {
			if err := validateCareOfferingSelectionMode([]SubmitChild{manualChild}, catalog, selectionMode); err != nil {
				return nil, err
			}
		}
		if i < len(existingChildren) && existingChildren[i] == nil {
			return nil, fmt.Errorf("%w: child %d missing existing row", ErrChangeRequestInvalidData, i)
		}
		out[i] = selections
	}
	return out, nil
}

// applyPreservedOfferingSelections copies hidden persisted links into the
// canonical proposal. This keeps base/proposed snapshots independent of the
// setting's current visibility and prevents a later toggle from looking like
// an offering removal.
func applyPreservedOfferingSelections(children []SubmitChild, selections [][]materializedOfferingSelection) {
	for i := range children {
		children[i].OfferingIDs = nil
		children[i].OfferingDays = nil
		if i >= len(selections) {
			continue
		}
		for _, selection := range selections[i] {
			children[i].OfferingIDs = append(children[i].OfferingIDs, selection.OfferingID)
			if len(selection.SelectedDays) > 0 {
				children[i].OfferingDays = append(children[i].OfferingDays, SubmitOfferingDays{
					OfferingID:   selection.OfferingID,
					SelectedDays: copyDays(selection.SelectedDays),
				})
			}
		}
	}
}

func (s *changeRequestService) schemaForRequest(ctx context.Context, req *enrollmentModels.Request) (*enrollmentModels.FormSchema, error) {
	if req.SchemaID == nil {
		return nil, nil
	}
	schema, err := s.FormSchemaRepo.FindByID(ctx, *req.SchemaID)
	if err != nil {
		return nil, fmt.Errorf("change request: load schema: %w", err)
	}
	return schema, nil
}

func (s *changeRequestService) legalBlocksForRequest(ctx context.Context, schema *enrollmentModels.FormSchema) ([]LegalBlock, error) {
	texts, err := (&requestService{RequestServiceConfig: RequestServiceConfig{Settings: s.Settings}}).LegalTexts(ctx)
	if err != nil {
		return nil, err
	}
	return applyTemplateLegalBlocks(texts, schema).Blocks, nil
}

func (s *changeRequestService) currentSnapshot(ctx context.Context, req *enrollmentModels.Request, children []*enrollmentModels.RequestChild) (map[string]any, error) {
	guardians, err := s.RequestGuardianRepo.ListByRequestID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("change request: list guardians: %w", err)
	}
	childIDs := make([]int64, 0, len(children))
	for _, child := range children {
		childIDs = append(childIDs, child.ID)
	}
	links, err := s.RequestChildOfferingRepo.ListByRequestChildIDsAtDate(ctx, childIDs, timezone.TodayDate())
	if err != nil {
		return nil, fmt.Errorf("change request: list child offerings: %w", err)
	}
	linksByChild := make(map[int64][]*enrollmentModels.RequestChildOffering, len(children))
	for _, link := range links {
		linksByChild[link.RequestChildID] = append(linksByChild[link.RequestChildID], link)
	}
	return persistedSnapshot(req, children, guardians, linksByChild), nil
}

// lockApprovedStudents takes the row locks of every student this approval can
// write — and of their "läuft mit" companions — up front, in the global
// ascending-id order.
//
// The per-child work below locks one student at a time (SyncApprovedChildData
// re-reads it FOR UPDATE, and the departure-plan write path additionally locks
// the far end of every edge it drops). Across several children that acquires
// student locks in enrollment sort order, which is NOT the order the companion
// protocol uses: an approval holding student 100 and waiting for 10 deadlocks
// against a companion edit holding 10 and waiting for 100, and PostgreSQL
// aborts one of them. Taking the whole set in one ordered pass first removes
// the inversion — every later lock in this transaction re-acquires a row it
// already holds.
func (s *changeRequestService) lockApprovedStudents(ctx context.Context, children []*enrollmentModels.RequestChild) error {
	if s.CompanionGraphLocker == nil {
		return nil
	}
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		if child == nil || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 {
			continue
		}
		ids = append(ids, *child.CreatedStudentID)
	}
	if len(ids) == 0 {
		return nil
	}
	if err := s.CompanionGraphLocker.LockCompanionGraph(ctx, ids, nil); err != nil {
		return fmt.Errorf("change request approve: lock students: %w", err)
	}
	return nil
}

// beginCompanionStrandingBatch opens the deferred-verdict scope the per-child
// departure-plan writes below record into, and returns the context they have to
// run with. Without a coordinator there is nothing that could decide the
// deferred verdicts later, so no batch is opened and every write keeps deciding
// its own verdict immediately (the pre-batch behavior).
func (s *changeRequestService) beginCompanionStrandingBatch(ctx context.Context) context.Context {
	if s.CompanionGraphLocker == nil {
		return ctx
	}
	batchCtx, _ := userModels.ContextWithCompanionStrandingBatch(ctx)
	return batchCtx
}

// verifyCompanionStrandingBatch decides the verdicts deferred into the batch on
// ctx. The companion sentinels stay reachable through the wrap, so the handler
// answers the actionable 4xx instead of a blind 500.
func (s *changeRequestService) verifyCompanionStrandingBatch(ctx context.Context) error {
	if s.CompanionGraphLocker == nil {
		return nil
	}
	if err := s.CompanionGraphLocker.VerifyCompanionStrandingBatch(ctx); err != nil {
		return fmt.Errorf("change request approve: verify companion links: %w", err)
	}
	return nil
}

func (s *changeRequestService) applyApprovedChange(ctx context.Context, row *enrollmentModels.ChangeRequest, input ReviewChangeRequestInput) error {
	req, err := s.RequestRepo.FindByIDForUpdate(ctx, row.RequestID)
	if err != nil {
		return ErrRequestNotFound
	}
	children, err := s.RequestChildRepo.ListByRequestIDForUpdate(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("change request approve: lock children: %w", err)
	}
	for _, child := range children {
		if child.Status == enrollmentModels.ChildStatusWithdrawn {
			return ErrChangeRequestNotAllowed
		}
	}
	current, err := s.currentSnapshot(ctx, req, children)
	if err != nil {
		return err
	}
	if !jsonEqual(current, row.BaseSnapshot) {
		return ErrChangeRequestConflict
	}
	proposed, err := snapshotToSubmitRequest(row.ProposedSnapshot)
	if err != nil {
		return err
	}
	capabilities, err := s.formCapabilities(ctx, &row.CareOfferingsEnabledAtCreation)
	if err != nil {
		return err
	}
	prepared, materializedSelections, phase, openByID, err := s.prepareProposed(
		ctx,
		req,
		children,
		proposed,
		capabilities,
		true,
	)
	if err != nil {
		return err
	}
	if err := s.ensureNoActiveDuplicateForApproval(ctx, req, prepared); err != nil {
		return err
	}
	var previousGuardians []*enrollmentModels.RequestGuardian
	if s.RequestGuardianRepo != nil {
		previousGuardians, err = s.RequestGuardianRepo.ListByRequestID(ctx, req.ID)
		if err != nil {
			return fmt.Errorf("change request approve: list previous guardians: %w", err)
		}
	}
	childStatusOverrides := map[int]string{}
	if capabilities.CareOfferingsEnabled {
		childStatusOverrides, err = s.changeRequestCapacityOverrides(ctx, row, children, prepared, phase, openByID)
		if err != nil {
			return err
		}
	}

	req.GuardianFirstName = strings.TrimSpace(prepared.GuardianFirstName)
	req.GuardianLastName = strings.TrimSpace(prepared.GuardianLastName)
	req.GuardianEmail = strings.ToLower(strings.TrimSpace(prepared.GuardianEmail))
	req.GuardianPhone = prepared.GuardianPhone
	req.ConsentFlags = prepared.ConsentFlags
	req.CustomData = prepared.CustomData
	if err := s.RequestRepo.UpdateGuardianDataWithEmail(ctx, req); err != nil {
		return err
	}
	if s.RequestGuardianRepo != nil {
		if err := s.RequestGuardianRepo.DeleteByRequestID(ctx, req.ID); err != nil {
			return err
		}
		for i, guardian := range prepared.AdditionalGuardians {
			if err := s.RequestGuardianRepo.Create(ctx, &enrollmentModels.RequestGuardian{
				RequestID:         req.ID,
				FirstName:         guardian.FirstName,
				LastName:          guardian.LastName,
				Email:             guardian.Email,
				Phone:             guardian.Phone,
				GuardianProfileID: matchingPreviousGuardianProfileID(previousGuardians, guardian),
				SortOrder:         i,
			}); err != nil {
				return fmt.Errorf("change request approve: create guardian %d: %w", i, err)
			}
		}
	}

	if err := s.lockApprovedStudents(ctx, children); err != nil {
		return err
	}

	// One approval is ONE edit of the whole family, applied child by child. A
	// child dropping the accompanied mode drops the "läuft mit" edges that lose
	// their basis — including edges to a SIBLING in the same approval whose own
	// plan change has not been applied yet. Judged per child, that valid
	// coordinated change looks like it strands the sibling and the whole
	// approval is refused. So defer the verdicts and decide them once below,
	// against the final plans of every child (#1694).
	ctx = s.beginCompanionStrandingBatch(ctx)

	// Same exemption set ReplaceEditable uses: trusted creation paths (admin
	// manual / late invite) and generated rollover requests bypassed the
	// self-service eligibility gates deliberately and must keep that override.
	eligibilityEnforced := !isTrustedEnrollmentSource(req.SubmissionSource) && !hasRolloverGeneratedChild(children)

	newlyWaitlisted := make(map[int64]struct{})
	for i, existing := range children {
		next := prepared.Children[i]
		// Before the proposed identity overwrites the row: re-point the
		// existing_students pin at whoever the row now names, and re-run the
		// phase-wide uniqueness guard for anything that ends this approval
		// active (a reopened rejection is newly in competition again) (#1663).
		if err := s.reconcileMatchedStudent(ctx, req, phase, row, existing, next, childStatusOverrides, eligibilityEnforced, i); err != nil {
			return err
		}
		existing.FirstName = strings.TrimSpace(next.FirstName)
		existing.LastName = strings.TrimSpace(next.LastName)
		existing.DateOfBirth = next.DateOfBirth
		existing.TargetGradeLevel = next.TargetGradeLevel
		existing.TargetSchoolClass = next.TargetSchoolClass
		existing.CustomData = next.CustomData
		existing.SortOrder = i
		if err := s.RequestChildRepo.UpdateData(ctx, existing); err != nil {
			return err
		}
		selections := materializedSelections[i]
		if s.approvedChildUsesDecisionSync(existing) {
			// The proposal's offering capability is frozen when the change
			// request is created. A later setting change must not silently
			// discard a valid, already reviewed offering change.
			if row.CareOfferingsEnabledAtCreation {
				offeringInput := UpdateChildOfferingsInput{
					RequestID:      req.ID,
					ChildID:        existing.ID,
					Reason:         input.Note,
					ActorAccountID: input.ActorAccountID,
					ActorRole:      input.ActorRole,
				}
				for _, selection := range selections {
					offeringInput.Offerings = append(offeringInput.Offerings, OfferingAdjustmentSelection{
						OfferingID:   selection.OfferingID,
						SelectedDays: selection.SelectedDays,
					})
				}
				if _, err := s.DecisionService.applyApprovedChangeRequestOfferings(ctx, offeringInput); err != nil {
					return err
				}
			}
			if _, err := s.DecisionService.SyncApprovedChildData(ctx, SyncApprovedChildDataInput{
				RequestID:                req.ID,
				ChildID:                  existing.ID,
				ActorAccountID:           input.ActorAccountID,
				ReplaceTargetedData:      true,
				PreviousSnapshot:         row.BaseSnapshot,
				PreviousRequestGuardians: previousGuardians,
			}); err != nil {
				return err
			}
			continue
		}
		replacement := make([]*enrollmentModels.RequestChildOffering, 0, len(selections))
		for _, selection := range selections {
			replacement = append(replacement, &enrollmentModels.RequestChildOffering{
				RequestChildID:        existing.ID,
				CareOfferingID:        selection.OfferingID,
				SelectedDays:          selection.SelectedDays,
				ManualSelectedDays:    selection.ManualSelectedDays,
				AutomaticSelectedDays: selection.AutomaticSelectedDays,
			})
		}
		if err := s.RequestChildOfferingRepo.ReplaceForRequestChild(ctx, existing.ID, replacement); err != nil {
			return err
		}
		if status, ok := childStatusOverrides[i]; ok {
			if existing.Status == status {
				continue
			}
			if err := s.RequestChildRepo.UpdateStatus(ctx, existing.ID, status, nil, input.ActorAccountID); err != nil {
				return err
			}
			if status == enrollmentModels.ChildStatusWaitlisted {
				newlyWaitlisted[existing.ID] = struct{}{}
			}
			continue
		}
		if existing.Status == enrollmentModels.ChildStatusRejected && childSnapshotChanged(row.BaseSnapshot, row.ProposedSnapshot, existing.ID) {
			if err := s.RequestChildRepo.UpdateStatus(ctx, existing.ID, enrollmentModels.ChildStatusUnderReview, nil, input.ActorAccountID); err != nil {
				return err
			}
		}
	}

	// Every child of the approval is applied — the plans are stored and the
	// edges they no longer allow are gone. Now the deferred verdicts can be
	// decided against that final state; a child genuinely left without a "mit
	// wem" detail still refuses the approval and rolls the transaction back.
	if err := s.verifyCompanionStrandingBatch(ctx); err != nil {
		return err
	}

	if len(newlyWaitlisted) > 0 {
		refreshedChildren, err := s.RequestChildRepo.ListByRequestID(ctx, req.ID)
		if err != nil {
			return fmt.Errorf("change request approve: refresh capacity decisions: %w", err)
		}
		if err := enqueueDecisionNotifications(ctx, decisionNotificationDependencies{
			requests:   s.RequestRepo,
			settings:   s.Settings,
			outbox:     s.OutboxEnqueuer,
			schools:    s.SchoolRepo,
			parentsURL: s.ParentsURL,
		}, req, refreshedChildren, phase, newlyWaitlisted); err != nil {
			return fmt.Errorf("change request approve: notify capacity decisions: %w", err)
		}
	}
	return nil
}

func (s *changeRequestService) changeRequestCapacityOverrides(
	ctx context.Context,
	row *enrollmentModels.ChangeRequest,
	children []*enrollmentModels.RequestChild,
	prepared SubmitRequest,
	phase *enrollmentModels.Phase,
	openByID map[int64]*enrollmentModels.CareOffering,
) (map[int]string, error) {
	overrides := make(map[int]string)
	if s.RequestChildOfferingRepo == nil || len(children) == 0 {
		return overrides, nil
	}

	candidates := make([]SubmitChild, 0, len(children))
	candidateIndexes := make([]int, 0, len(children))
	preservedChildIDs := make([]int64, 0, len(children))
	for i, child := range children {
		if s.approvedChildUsesDecisionSync(child) {
			continue
		}
		reopensRejected := child.Status == enrollmentModels.ChildStatusRejected &&
			childSnapshotChanged(row.BaseSnapshot, row.ProposedSnapshot, child.ID)
		if !childStatusCountsForCapacity(child.Status) && !reopensRejected {
			continue
		}
		candidates = append(candidates, prepared.Children[i])
		candidateIndexes = append(candidateIndexes, i)
		if childStatusCountsForCapacity(child.Status) {
			preservedChildIDs = append(preservedChildIDs, child.ID)
		}
	}
	if len(candidates) == 0 {
		return overrides, nil
	}

	rs := &requestService{RequestServiceConfig: RequestServiceConfig{
		RequestChildOfferingRepo: s.RequestChildOfferingRepo,
		CareOfferingRepo:         s.CareOfferingRepo,
		Settings:                 s.Settings,
	}}
	candidateOverrides, err := rs.applyCapacityOverflowWithReplacedChildren(ctx, phase, candidates, openByID, preservedChildIDs)
	if err != nil {
		return nil, fmt.Errorf("change request approve: capacity overflow: %w", err)
	}
	for candidateIdx, status := range candidateOverrides {
		if candidateIdx < 0 || candidateIdx >= len(candidateIndexes) {
			continue
		}
		overrides[candidateIndexes[candidateIdx]] = status
	}
	return overrides, nil
}

func (s *changeRequestService) approvedChildUsesDecisionSync(child *enrollmentModels.RequestChild) bool {
	return child != nil &&
		child.Status == enrollmentModels.ChildStatusApproved &&
		child.CreatedStudentID != nil &&
		s.DecisionService != nil
}

// matchResolverShim exposes the requestService helpers that own the
// existing_students matching rules (resolve → per-student permission →
// phase-wide uniqueness). The change-request path reuses them verbatim instead
// of growing a second, drifting copy of the same three gates.
func (s *changeRequestService) matchResolverShim() *requestService {
	return &requestService{RequestServiceConfig: RequestServiceConfig{
		RequestRepo:        s.RequestRepo,
		StudentRepo:        s.StudentRepo,
		GuardianAuthorizer: s.GuardianAuthorizer,
		Logger:             s.Logger,
	}}
}

// validateChangedChildIdentityEligibility re-applies the audience's
// enrolled-status gate (new_students / existing_students) to the children whose
// submitted identity a change request actually REWRITES (#1663).
//
// validateChangeRequestChildIdentity pins the child ROW ids, not the human they
// name: an edit may freely change first name, last name and birthday on a
// persisted row. Without this gate a pending or rejected child could be
// retargeted onto an already-enrolled student's name+birthday, and approval
// would then take the fresh-create branch and add a duplicate Person/Student for
// a child the school already has — precisely what the new_students audience
// exists to prevent. Submit and ReplaceEditable both run the same probe; only
// this path was missing it.
//
// Scoped to CHANGED identities on purpose. prepareProposed also runs on
// approval, and a change request may be filed against an already-approved
// request whose own approval created the matching student: probing an unchanged
// child would make it fail the gate against its own record and would reject
// every later change request on it. That is exactly the case the earlier
// "enrolled gates stay out of the change-request path" note was protecting, and
// filtering by identity keeps it working while closing the hole it left.
//
// Children are paired positionally: validateChangeRequestChildIdentity has
// already proven the two slices are the same length and in the same row-id
// order. An unpaired index (defensive: a shorter persisted slice) counts as
// changed and is probed.
func (s *changeRequestService) validateChangedChildIdentityEligibility(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	editReq SubmitRequest,
	children []*enrollmentModels.RequestChild,
) error {
	rs := s.matchResolverShim()
	for i := range editReq.Children {
		if i < len(children) && sameSubmittedIdentity(children[i], editReq.Children[i]) {
			continue
		}
		if err := rs.validateChildEnrolledStatus(ctx, phase, editReq.TenantID, editReq.Children[i], i); err != nil {
			return err
		}
	}
	return nil
}

// postApprovalChildStatus predicts the status a child ends this approval with,
// mirroring the status decisions the write loop makes further down: a capacity
// override wins, otherwise a rejected child whose own data changed is reopened
// to under_review, otherwise the status is unchanged.
func postApprovalChildStatus(row *enrollmentModels.ChangeRequest, existing *enrollmentModels.RequestChild, overrides map[int]string, index int) string {
	if status, ok := overrides[index]; ok {
		return status
	}
	if existing.Status == enrollmentModels.ChildStatusRejected &&
		childSnapshotChanged(row.BaseSnapshot, row.ProposedSnapshot, existing.ID) {
		return enrollmentModels.ChildStatusUnderReview
	}
	return existing.Status
}

// reconcileMatchedStudent keeps an existing_students child's pinned
// matched_student_id honest across an approved change request (#1663).
//
// Two gaps this closes, both invisible to the identity validation (which only
// pins the child ROW id, not the human it names):
//
//  1. An approved change request may rewrite first name, last name and
//     birthday. The row keeps its submission-time pin, so the decision service
//     would renew the ORIGINALLY matched student even though the reviewed
//     request now describes a different child. Re-resolve exactly as
//     ReplaceEditable does — drop the pin on an identity change, re-resolve
//     against the new identity, and re-check the guardian's per-student
//     re-enrollment permission before the new pin can stand.
//  2. HasActiveRequestForMatchedStudent ignores rejected rows, so while this
//     request sat rejected another guardian could legitimately pin the same
//     student. Approving a change request can reopen it to under_review; the
//     uniqueness guard must therefore run again for every child that ends this
//     approval in an active state, not just at insert time.
//
// Skipped for children that already materialized a student (approval created or
// renewed it — SyncApprovedChildData owns that live record now, and the pin is
// history) and for rollover children (the rollover source chain takes
// precedence at approval, so the pin is dead data there).
//
// Must be called BEFORE the loop copies the proposed identity onto `existing`,
// while the row still carries its persisted name/birthday.
func (s *changeRequestService) reconcileMatchedStudent(
	ctx context.Context,
	req *enrollmentModels.Request,
	phase *enrollmentModels.Phase,
	row *enrollmentModels.ChangeRequest,
	existing *enrollmentModels.RequestChild,
	next SubmitChild,
	overrides map[int]string,
	eligibilityEnforced bool,
	index int,
) error {
	if existing.CreatedStudentID != nil || existing.RolloverSourceChildID != nil {
		return nil
	}
	if phase.Audience != enrollmentModels.PhaseAudienceExistingStudents && existing.MatchedStudentID == nil {
		return nil
	}

	rs := s.matchResolverShim()
	pin := existing.MatchedStudentID
	// An edited identity invalidates the carried pin: keeping it would renew a
	// student the reviewed request no longer describes. An unchanged identity
	// keeps it (dropping it would send approval down the fresh-create branch and
	// duplicate the very student this audience exists to renew).
	if pin != nil && !sameSubmittedIdentity(existing, next) {
		pin = nil
	}
	if phase.Audience == enrollmentModels.PhaseAudienceExistingStudents {
		resolved, err := rs.resolveMatchedStudentID(ctx, req.GetTenantID(), phase, index, next)
		if err != nil {
			return err
		}
		if resolved != nil {
			pin = resolved
		}
	}
	if err := assertExistingStudentMatchResolved(phase, pin, eligibilityEnforced, index); err != nil {
		return err
	}
	if err := rs.assertGuardianMayReEnrollStudent(ctx,
		reEnrollmentSubmitterFor(req.SubmissionSource, req.GuardianAccountID, req.GuardianEmail),
		pin, req.GetTenantID(), index); err != nil {
		return err
	}
	if childStatusCountsForCapacity(postApprovalChildStatus(row, existing, overrides, index)) {
		if err := rs.guardMatchedStudentUnique(ctx, phase.ID, pin, existing.ID, index); err != nil {
			return err
		}
	}
	if ptrInt64Equal(pin, existing.MatchedStudentID) {
		return nil
	}
	if err := s.RequestChildRepo.UpdateMatchedStudent(ctx, existing.ID, pin); err != nil {
		return fmt.Errorf("change request approve: update matched student for child %d: %w", index, err)
	}
	s.Logger.Info("change request approve: re-pinned existing-student match",
		slog.Int64("request_child_id", existing.ID),
		slog.Bool("had_pin", existing.MatchedStudentID != nil),
		slog.Bool("has_pin", pin != nil),
	)
	existing.MatchedStudentID = pin
	return nil
}

func ptrInt64Equal(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func childStatusCountsForCapacity(status string) bool {
	return status != enrollmentModels.ChildStatusRejected &&
		status != enrollmentModels.ChildStatusWithdrawn
}

func (s *changeRequestService) ensureNoActiveDuplicateForApproval(ctx context.Context, req *enrollmentModels.Request, prepared SubmitRequest) error {
	emailLC := strings.ToLower(strings.TrimSpace(req.GuardianEmail))
	if err := s.RequestRepo.AcquireSubmissionDedupLock(ctx, req.PhaseID, fnvHash64(emailLC)); err != nil {
		return fmt.Errorf("change request approve: acquire duplicate lock: %w", err)
	}

	dupKeys := make([]enrollmentModels.DuplicateChildKey, 0, len(prepared.Children))
	for _, child := range prepared.Children {
		dupKeys = append(dupKeys, enrollmentModels.DuplicateChildKey{
			FirstName: child.FirstName,
			LastName:  child.LastName,
		})
	}
	dupes, err := s.RequestRepo.FindActiveDuplicateExcludingRequest(ctx, req.PhaseID, req.GuardianEmail, dupKeys, req.ID)
	if err != nil {
		return fmt.Errorf("change request approve: duplicate check: %w", err)
	}
	if len(dupes) > 0 {
		if s.Settings == nil {
			return errors.New("enrollment settings resolver is not configured")
		}
		policy, resolveErr := s.Settings.ResolveString(ctx, configModel.KeyEnrollmentDuplicateHandling)
		if resolveErr != nil {
			return fmt.Errorf("change request approve: resolve duplicate handling: %w", resolveErr)
		}
		switch policy {
		case configModel.EnrollmentDuplicateHandlingBlock:
			return ErrDuplicateEnrollment
		case configModel.EnrollmentDuplicateHandlingWarn:
			s.Logger.WarnContext(ctx, "change request approved with active duplicate", slog.Int64("request_id", req.ID))
		case configModel.EnrollmentDuplicateHandlingIgnore:
		default:
			return fmt.Errorf("change request approve: unsupported duplicate handling %q", policy)
		}
	}
	return nil
}

func matchingPreviousGuardianProfileID(previous []*enrollmentModels.RequestGuardian, guardian SubmitGuardian) *int64 {
	key := submitGuardianProfileCarryKey(guardian)
	for _, row := range previous {
		if row == nil || row.GuardianProfileID == nil || *row.GuardianProfileID <= 0 {
			continue
		}
		if requestGuardianProfileCarryKey(row) == key {
			return row.GuardianProfileID
		}
	}
	return nil
}

func submitGuardianProfileCarryKey(guardian SubmitGuardian) string {
	return strings.Join([]string{
		normalizedProfileCarryValue(guardian.FirstName),
		normalizedProfileCarryValue(guardian.LastName),
		normalizedProfileCarryPtr(guardian.Email),
		normalizedProfileCarryPtr(guardian.Phone),
	}, "\x00")
}

func requestGuardianProfileCarryKey(guardian *enrollmentModels.RequestGuardian) string {
	return strings.Join([]string{
		normalizedProfileCarryValue(guardian.FirstName),
		normalizedProfileCarryValue(guardian.LastName),
		normalizedProfileCarryPtr(guardian.Email),
		normalizedProfileCarryPtr(guardian.Phone),
	}, "\x00")
}

func normalizedProfileCarryPtr(value *string) string {
	if value == nil {
		return ""
	}
	return normalizedProfileCarryValue(*value)
}

func normalizedProfileCarryValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (s *changeRequestService) ensureNoOpenChangeRequest(ctx context.Context, requestID int64) error {
	rows, err := s.ChangeRequestRepo.ListByRequestID(ctx, requestID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status == enrollmentModels.ChangeRequestStatusPendingReview ||
			row.Status == enrollmentModels.ChangeRequestStatusNeedsParentResponse {
			return fmt.Errorf("%w: an open change request already exists", ErrChangeRequestNotAllowed)
		}
	}
	return nil
}

func validateChangeRequestChildIdentity(existing []*enrollmentModels.RequestChild, incoming []SubmitChild) error {
	if len(existing) != len(incoming) {
		return fmt.Errorf("%w: child count changes are not supported for change requests", ErrChangeRequestInvalidData)
	}
	for i, child := range existing {
		if incoming[i].ID <= 0 {
			return fmt.Errorf("%w: child %d id is required", ErrChangeRequestInvalidData, i)
		}
		if incoming[i].ID != child.ID {
			return fmt.Errorf("%w: child order or identity changes are not supported for change requests", ErrChangeRequestInvalidData)
		}
	}
	return nil
}

func childSnapshotChanged(base, proposed map[string]any, childID int64) bool {
	baseChild := snapshotChildByID(base, childID)
	proposedChild := snapshotChildByID(proposed, childID)
	if baseChild == nil || proposedChild == nil {
		return false
	}
	baseComparable := copySnapshotMapWithoutStatus(baseChild)
	proposedComparable := copySnapshotMapWithoutStatus(proposedChild)
	return !jsonEqual(baseComparable, proposedComparable)
}

func snapshotChildByID(snapshot map[string]any, childID int64) map[string]any {
	for _, raw := range sliceFromAny(snapshot["children"]) {
		row := mapFromAny(raw)
		if int64FromAny(row["id"]) == childID {
			return row
		}
	}
	return nil
}

func copySnapshotMapWithoutStatus(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		if key == "status" {
			continue
		}
		out[key] = value
	}
	return out
}

func (s *changeRequestService) validateAccountLinkedGuardianEdits(ctx context.Context, req *enrollmentModels.Request, editReq SubmitRequest) error {
	if s.GuardianProfileRepo == nil {
		return nil
	}
	profile, err := s.primaryGuardianProfile(ctx, req)
	if err != nil {
		return fmt.Errorf("change request: load primary guardian profile for account guardrail: %w", err)
	}
	if profile != nil && profile.HasPortalAccount() {
		if !sameTrimmedString(editReq.GuardianFirstName, req.GuardianFirstName) ||
			!sameTrimmedString(editReq.GuardianLastName, req.GuardianLastName) ||
			!sameOptionalString(editReq.GuardianPhone, req.GuardianPhone) {
			return fmt.Errorf("%w: account-linked guardian profile details must be changed in the parent portal", ErrChangeRequestInvalidData)
		}
	}
	if len(editReq.AdditionalGuardians) == 0 || s.RequestGuardianRepo == nil {
		return nil
	}
	existing, err := s.RequestGuardianRepo.ListByRequestID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("change request: list guardians for account guardrail: %w", err)
	}
	for i, guardian := range editReq.AdditionalGuardians {
		email := optionalLowerEmail(guardian.Email)
		if email == "" {
			continue
		}
		profile, err := s.GuardianProfileRepo.FindByEmail(ctx, email)
		if err != nil {
			if errors.Is(err, userModels.ErrGuardianProfileNotFound) {
				continue
			}
			return fmt.Errorf("change request: load co-guardian profile for account guardrail: %w", err)
		}
		if profile == nil || !profile.HasPortalAccount() {
			continue
		}
		if old := matchingRequestGuardian(existing, profile.ID, email); old != nil {
			if !sameTrimmedString(guardian.FirstName, old.FirstName) ||
				!sameTrimmedString(guardian.LastName, old.LastName) ||
				!sameOptionalString(guardian.Phone, old.Phone) {
				return fmt.Errorf("%w: account-linked co-guardian %d details must be changed in the parent portal", ErrChangeRequestInvalidData, i)
			}
			continue
		}
		if !sameTrimmedString(guardian.FirstName, profile.FirstName) ||
			!sameTrimmedString(guardian.LastName, profile.LastName) ||
			!s.submittedPhoneMatchesProfile(ctx, profile.ID, guardian.Phone) {
			return fmt.Errorf("%w: account-linked co-guardian %d details must match the parent portal profile", ErrChangeRequestInvalidData, i)
		}
	}
	return nil
}

func (s *changeRequestService) primaryGuardianProfile(ctx context.Context, req *enrollmentModels.Request) (*userModels.GuardianProfile, error) {
	if s.GuardianProfileRepo == nil || req == nil {
		return nil, nil
	}
	if req.GuardianAccountID != nil && *req.GuardianAccountID > 0 {
		profile, err := s.GuardianProfileRepo.FindByAccountID(ctx, *req.GuardianAccountID)
		if err == nil && profile != nil {
			return profile, nil
		}
		if err != nil && !errors.Is(err, userModels.ErrGuardianProfileNotFound) {
			return nil, err
		}
	}
	email := strings.TrimSpace(strings.ToLower(req.GuardianEmail))
	if email == "" {
		return nil, nil
	}
	profile, err := s.GuardianProfileRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, userModels.ErrGuardianProfileNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return profile, nil
}

func matchingRequestGuardian(rows []*enrollmentModels.RequestGuardian, profileID int64, email string) *enrollmentModels.RequestGuardian {
	email = strings.TrimSpace(strings.ToLower(email))
	for _, row := range rows {
		if row == nil {
			continue
		}
		if row.GuardianProfileID != nil && *row.GuardianProfileID == profileID {
			return row
		}
		if email != "" && optionalLowerEmail(row.Email) == email {
			return row
		}
	}
	return nil
}

func (s *changeRequestService) submittedPhoneMatchesProfile(ctx context.Context, profileID int64, submitted *string) bool {
	phone := trimmedOptionalString(submitted)
	if phone == "" {
		return true
	}
	if s.GuardianPhoneRepo == nil {
		return false
	}
	rows, err := s.GuardianPhoneRepo.FindByGuardianID(ctx, profileID)
	if err != nil {
		return false
	}
	for _, row := range rows {
		if strings.TrimSpace(row.PhoneNumber) == phone {
			return true
		}
	}
	return false
}

func sameTrimmedString(left, right string) bool {
	return strings.TrimSpace(left) == strings.TrimSpace(right)
}

func sameOptionalString(left, right *string) bool {
	return trimmedOptionalString(left) == trimmedOptionalString(right)
}

func sameOptionalInt16(left, right *int16) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func normalizedOptionalString(value *string) *string {
	trimmed := trimmedOptionalString(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func trimmedOptionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func optionalLowerEmail(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(strings.ToLower(*value))
}

func (s *changeRequestService) withLockedChangeRequest(ctx context.Context, id int64, fn func(context.Context, *enrollmentModels.ChangeRequest) error) error {
	row, err := s.ChangeRequestRepo.FindByID(ctx, id)
	if err != nil || row == nil {
		return ErrChangeRequestNotFound
	}
	return tenant.WithTenantTx(ctx, s.DB, row.GetTenantID(), func(txCtx context.Context, _ bun.Tx) error {
		// Enrollment cleanup and every request-edit flow lock the aggregate
		// parent before its children. Take the parent before the change-request
		// row as well: cleanup's cascading delete eventually locks this row, so
		// the reverse order would create a parent/change-request deadlock.
		if _, err := s.RequestRepo.FindByIDForUpdate(txCtx, row.RequestID); err != nil {
			return ErrRequestNotFound
		}
		locked, err := s.ChangeRequestRepo.FindByIDForUpdate(txCtx, id)
		if err != nil || locked == nil {
			return ErrChangeRequestNotFound
		}
		return fn(txCtx, locked)
	})
}

func (s *changeRequestService) loadAggregate(ctx context.Context, id int64, includeInternal bool) (*ChangeRequestAggregate, error) {
	row, err := s.ChangeRequestRepo.FindByID(ctx, id)
	if err != nil || row == nil {
		return nil, ErrChangeRequestNotFound
	}
	return s.aggregateFromRow(ctx, row, includeInternal)
}

func (s *changeRequestService) loadAggregateForTenant(ctx context.Context, tenantID int64, id int64, includeInternal bool) (*ChangeRequestAggregate, error) {
	var agg *ChangeRequestAggregate
	err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		loaded, err := s.loadAggregate(txCtx, id, includeInternal)
		agg = loaded
		return err
	})
	return agg, err
}

func (s *changeRequestService) aggregateFromRow(ctx context.Context, row *enrollmentModels.ChangeRequest, includeInternal bool) (*ChangeRequestAggregate, error) {
	req, err := s.RequestRepo.FindByID(ctx, row.RequestID)
	if err != nil {
		return nil, ErrRequestNotFound
	}
	children, err := s.RequestChildRepo.ListByRequestID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("change request: list aggregate children: %w", err)
	}
	messages, err := s.MessageRepo.ListByChangeRequestID(ctx, row.ID, includeInternal)
	if err != nil {
		return nil, err
	}
	var phase *enrollmentModels.Phase
	if s.PhaseRepo != nil {
		phase, _ = s.PhaseRepo.FindByID(ctx, req.PhaseID)
	}
	return &ChangeRequestAggregate{
		ChangeRequest: row,
		Request:       req,
		Children:      children,
		Messages:      messages,
		Phase:         phase,
	}, nil
}

func submitSnapshot(req SubmitRequest) map[string]any {
	children := make([]any, 0, len(req.Children))
	for _, child := range req.Children {
		offeringIDs := make([]any, 0, len(child.OfferingIDs))
		for _, id := range child.OfferingIDs {
			offeringIDs = append(offeringIDs, strconv.FormatInt(id, 10))
		}
		offeringDays := make([]any, 0, len(child.OfferingDays))
		for _, row := range child.OfferingDays {
			offeringDays = append(offeringDays, map[string]any{
				"offering_id":   strconv.FormatInt(row.OfferingID, 10),
				"selected_days": copyDays(row.SelectedDays),
			})
		}
		row := map[string]any{
			"first_name":         child.FirstName,
			"last_name":          child.LastName,
			"date_of_birth":      child.DateOfBirth.String(),
			"target_grade_level": child.TargetGradeLevel,
			"custom_data":        child.CustomData,
			"offering_ids":       offeringIDs,
			"offering_days":      offeringDays,
		}
		// Only emit target_school_class when a concrete class is actually
		// set. Snapshots persisted before #1833 have no such key at all, so
		// currentSnapshot() must omit it too when unset — otherwise
		// applyApprovedChange() compares a recomputed snapshot carrying
		// "target_school_class": null against a stored base snapshot missing
		// the key, and jsonEqual treats missing != null, wrongly reporting a
		// conflict that blocks approval of every pre-existing pending change
		// request. Round-trips cleanly: snapshotToSubmitRequest reads a
		// missing key back as nil.
		if class := trimmedOptionalString(child.TargetSchoolClass); class != "" {
			row["target_school_class"] = class
		}
		if child.ID > 0 {
			row["id"] = strconv.FormatInt(child.ID, 10)
		}
		children = append(children, row)
	}
	guardians := make([]any, 0, len(req.AdditionalGuardians))
	for _, guardian := range req.AdditionalGuardians {
		guardians = append(guardians, map[string]any{
			"first_name": guardian.FirstName,
			"last_name":  guardian.LastName,
			"email":      ptrStringValue(guardian.Email),
			"phone":      ptrStringValue(guardian.Phone),
		})
	}
	return map[string]any{
		"phase_id":             strconv.FormatInt(req.PhaseID, 10),
		"guardian_first_name":  req.GuardianFirstName,
		"guardian_last_name":   req.GuardianLastName,
		"guardian_email":       req.GuardianEmail,
		"guardian_phone":       ptrStringValue(req.GuardianPhone),
		"consent_flags":        req.ConsentFlags,
		"custom_data":          req.CustomData,
		"additional_guardians": guardians,
		"children":             children,
	}
}

func persistedSnapshot(
	req *enrollmentModels.Request,
	children []*enrollmentModels.RequestChild,
	guardians []*enrollmentModels.RequestGuardian,
	linksByChild map[int64][]*enrollmentModels.RequestChildOffering,
) map[string]any {
	submit := SubmitRequest{
		TenantID:          req.GetTenantID(),
		PhaseID:           req.PhaseID,
		GuardianFirstName: req.GuardianFirstName,
		GuardianLastName:  req.GuardianLastName,
		GuardianEmail:     req.GuardianEmail,
		GuardianPhone:     req.GuardianPhone,
		ConsentFlags:      req.ConsentFlags,
		CustomData:        req.CustomData,
	}
	for _, guardian := range guardians {
		submit.AdditionalGuardians = append(submit.AdditionalGuardians, SubmitGuardian{
			FirstName: guardian.FirstName,
			LastName:  guardian.LastName,
			Email:     guardian.Email,
			Phone:     guardian.Phone,
		})
	}
	for _, child := range children {
		next := SubmitChild{
			FirstName:         child.FirstName,
			LastName:          child.LastName,
			DateOfBirth:       child.DateOfBirth,
			TargetGradeLevel:  child.TargetGradeLevel,
			TargetSchoolClass: child.TargetSchoolClass,
			CustomData:        child.CustomData,
		}
		for _, link := range linksByChild[child.ID] {
			next.OfferingIDs = append(next.OfferingIDs, link.CareOfferingID)
			if len(link.SelectedDays) > 0 {
				next.OfferingDays = append(next.OfferingDays, SubmitOfferingDays{
					OfferingID:   link.CareOfferingID,
					SelectedDays: copyDays(link.SelectedDays),
				})
			}
		}
		submit.Children = append(submit.Children, next)
	}
	snapshot := submitSnapshot(submit)
	rows := snapshot["children"].([]any)
	for i, child := range children {
		if i >= len(rows) {
			continue
		}
		if row, ok := rows[i].(map[string]any); ok {
			row["id"] = strconv.FormatInt(child.ID, 10)
			row["status"] = child.Status
		}
	}
	return snapshot
}

func snapshotDiff(base, proposed map[string]any) map[string]any {
	changed := make([]string, 0)
	keys := make(map[string]bool)
	for key := range base {
		keys[key] = true
	}
	for key := range proposed {
		keys[key] = true
	}
	for key := range keys {
		if !jsonEqual(base[key], proposed[key]) {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return map[string]any{
		"changed": changed,
	}
}

func jsonEqual(left, right any) bool {
	normalize := func(v any) any {
		raw, err := json.Marshal(v)
		if err != nil {
			return v
		}
		var out any
		if err := json.Unmarshal(raw, &out); err != nil {
			return v
		}
		return out
	}
	return reflect.DeepEqual(normalize(left), normalize(right))
}

func snapshotToSubmitRequest(snapshot map[string]any) (SubmitRequest, error) {
	var out SubmitRequest
	out.PhaseID = int64FromAny(snapshot["phase_id"])
	out.GuardianFirstName = stringFromAny(snapshot["guardian_first_name"])
	out.GuardianLastName = stringFromAny(snapshot["guardian_last_name"])
	out.GuardianEmail = stringFromAny(snapshot["guardian_email"])
	out.GuardianPhone = optionalStringFromAny(snapshot["guardian_phone"])
	out.ConsentFlags = mapFromAny(snapshot["consent_flags"])
	out.CustomData = mapFromAny(snapshot["custom_data"])
	for _, raw := range sliceFromAny(snapshot["additional_guardians"]) {
		row := mapFromAny(raw)
		out.AdditionalGuardians = append(out.AdditionalGuardians, SubmitGuardian{
			FirstName: stringFromAny(row["first_name"]),
			LastName:  stringFromAny(row["last_name"]),
			Email:     optionalStringFromAny(row["email"]),
			Phone:     optionalStringFromAny(row["phone"]),
		})
	}
	for i, raw := range sliceFromAny(snapshot["children"]) {
		row := mapFromAny(raw)
		dob, err := timezone.ParseDate(stringFromAny(row["date_of_birth"]))
		if err != nil {
			return out, fmt.Errorf("%w: child %d date_of_birth", ErrChangeRequestInvalidData, i)
		}
		child := SubmitChild{
			ID:                int64FromAny(row["id"]),
			FirstName:         stringFromAny(row["first_name"]),
			LastName:          stringFromAny(row["last_name"]),
			DateOfBirth:       dob,
			TargetGradeLevel:  int16PtrFromAny(row["target_grade_level"]),
			TargetSchoolClass: optionalStringFromAny(row["target_school_class"]),
			CustomData:        mapFromAny(row["custom_data"]),
		}
		for _, rawID := range sliceFromAny(row["offering_ids"]) {
			if id := int64FromAny(rawID); id > 0 {
				child.OfferingIDs = append(child.OfferingIDs, id)
			}
		}
		for _, rawDay := range sliceFromAny(row["offering_days"]) {
			day := mapFromAny(rawDay)
			id := int64FromAny(day["offering_id"])
			if id <= 0 {
				continue
			}
			child.OfferingDays = append(child.OfferingDays, SubmitOfferingDays{
				OfferingID:   id,
				SelectedDays: stringSliceFromAny(day["selected_days"]),
			})
		}
		out.Children = append(out.Children, child)
	}
	return out, nil
}

func ptrStringValue(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func optionalStringFromAny(v any) *string {
	s := strings.TrimSpace(stringFromAny(v))
	if s == "" {
		return nil
	}
	return &s
}

func int64FromAny(v any) int64 {
	switch typed := v.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		n, _ := typed.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return n
	default:
		return 0
	}
}

func int16PtrFromAny(v any) *int16 {
	n := int64FromAny(v)
	if n == 0 {
		return nil
	}
	out := int16(n)
	return &out
}

func mapFromAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok && m != nil {
		return m
	}
	return map[string]any{}
}

func sliceFromAny(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	if stringsSlice, ok := v.([]string); ok {
		out := make([]any, 0, len(stringsSlice))
		for _, value := range stringsSlice {
			out = append(out, value)
		}
		return out
	}
	return nil
}

func stringSliceFromAny(v any) []string {
	out := make([]string, 0)
	for _, raw := range sliceFromAny(v) {
		if s := stringFromAny(raw); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (s *changeRequestService) emailNotificationsEnabled(ctx context.Context) bool {
	if s.Settings == nil {
		return false
	}
	enabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentChangeRequestEmailNotificationsEnabled)
	return err == nil && enabled
}

func (s *changeRequestService) enqueueChangeRequestSubmitted(ctx context.Context, tenantID int64, req *enrollmentModels.Request, cr *enrollmentModels.ChangeRequest) {
	if s.OutboxEnqueuer == nil {
		return
	}
	s.enqueueAdminNotification(ctx, tenantID, req, cr.ID, platformModels.EmailKindEnrollmentChangeRequestSubmitted)
}

func (s *changeRequestService) enqueueParentReply(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64) {
	s.enqueueAdminNotification(ctx, tenantID, req, changeRequestID, platformModels.EmailKindEnrollmentChangeRequestParentReply)
}

// enqueueAdminNotification fans a change-request email out to every resolved
// admin recipient (with the admin deep link in the payload). Counterpart of
// enqueueParentNotification, which addresses the single guardian instead.
func (s *changeRequestService) enqueueAdminNotification(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64, kind string) {
	if s.OutboxEnqueuer == nil {
		return
	}
	err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if !s.emailNotificationsEnabled(txCtx) {
			return nil
		}
		for _, admin := range (&requestService{RequestServiceConfig: RequestServiceConfig{Settings: s.Settings}}).resolveAdminEmails(txCtx) {
			payload := s.emailPayload(txCtx, req, changeRequestID, admin)
			payload[EnrollmentPayloadAdminURL] = s.adminURL(changeRequestID)
			if enqueueErr := s.OutboxEnqueuer.EnqueueOutbox(txCtx, platformModels.OutboxEnqueueRequest{
				Kind:              kind,
				Payload:           payload,
				RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
				RelatedEntityID:   req.ID,
			}); enqueueErr != nil {
				s.logChangeRequestNotificationFailure(enqueueErr, tenantID, req.ID, changeRequestID, kind, admin, "enqueue")
			}
		}
		return nil
	})
	if err != nil {
		s.logChangeRequestNotificationFailure(err, tenantID, req.ID, changeRequestID, kind, "", "tenant_tx")
	}
}

func (s *changeRequestService) enqueueAdminQuestion(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64) {
	s.enqueueParentNotification(ctx, tenantID, req, changeRequestID, platformModels.EmailKindEnrollmentChangeRequestQuestion)
}

func (s *changeRequestService) enqueueReviewed(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64, kind string) {
	s.enqueueParentNotification(ctx, tenantID, req, changeRequestID, kind)
}

func (s *changeRequestService) enqueueParentNotification(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64, kind string) {
	if s.OutboxEnqueuer == nil {
		return
	}
	err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if !s.emailNotificationsEnabled(txCtx) {
			return nil
		}
		payload := s.emailPayload(txCtx, req, changeRequestID, req.GuardianEmail)
		if enqueueErr := s.OutboxEnqueuer.EnqueueOutbox(txCtx, platformModels.OutboxEnqueueRequest{
			Kind:              kind,
			Payload:           payload,
			RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
			RelatedEntityID:   req.ID,
		}); enqueueErr != nil {
			s.logChangeRequestNotificationFailure(enqueueErr, tenantID, req.ID, changeRequestID, kind, req.GuardianEmail, "enqueue")
		}
		return nil
	})
	if err != nil {
		s.logChangeRequestNotificationFailure(err, tenantID, req.ID, changeRequestID, kind, req.GuardianEmail, "tenant_tx")
	}
}

func (s *changeRequestService) logChangeRequestNotificationFailure(err error, tenantID, requestID, changeRequestID int64, kind, recipient, stage string) {
	if err == nil || s.Logger == nil {
		return
	}
	s.Logger.Warn("change request notification enqueue failed",
		slog.String("stage", stage),
		slog.Int64("tenant_id", tenantID),
		slog.Int64("request_id", requestID),
		slog.Int64("change_request_id", changeRequestID),
		slog.String("kind", kind),
		slog.String("recipient", recipient),
		slog.String("error", err.Error()),
	)
}

func (s *changeRequestService) emailPayload(ctx context.Context, req *enrollmentModels.Request, changeRequestID int64, recipient string) map[string]any {
	schoolName, logoURL := emailBrandForSchool(ctx, s.SchoolRepo, req.GetTenantID(), s.ParentsURL)
	return map[string]any{
		EnrollmentPayloadGuardianFirstName: req.GuardianFirstName,
		EnrollmentPayloadGuardianLastName:  req.GuardianLastName,
		EnrollmentPayloadGuardianEmail:     req.GuardianEmail,
		EnrollmentPayloadSchoolName:        schoolName,
		EnrollmentPayloadStatusURL:         fmt.Sprintf("%s/enroll/status/%s", s.ParentsURL, req.StatusToken),
		EnrollmentPayloadLogoURL:           logoURL,
		EnrollmentPayloadMotoLogoURL:       motoLogoURL(s.ParentsURL),
		EnrollmentPayloadRecipientEmail:    recipient,
		"change_request_id":                strconv.FormatInt(changeRequestID, 10),
	}
}

func (s *changeRequestService) adminURL(changeRequestID int64) string {
	if s.FrontendURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/admin/enrollments/change-requests/%d", s.FrontendURL, changeRequestID)
}
