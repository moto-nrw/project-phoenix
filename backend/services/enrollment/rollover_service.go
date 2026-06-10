package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Sentinel errors for the rollover flow. Mapped to HTTP status codes
// by the handlers in api/enrollment.
var (
	ErrRolloverSourceNotFound = errors.New("rollover source phase not found")
	ErrRolloverInvalidRequest = errors.New("rollover request invalid")
	ErrRolloverReviewNotFound = errors.New("rollover review item not found")
	ErrRolloverReviewInvalid  = errors.New("rollover review decision invalid")
	// ErrRolloverDuplicateName is returned when the new phase name
	// collides with an existing phase for the same tenant. Driven by
	// the enrollment_phases_unique_name UNIQUE(tenant_id, name)
	// constraint — the admin sees a 409 with a clear message instead
	// of a raw Postgres error.
	ErrRolloverDuplicateName = errors.New("phase name already exists")
	// ErrRolloverSourceAlreadyRolled is returned when the source phase
	// has already been rolled forward into another phase. Enforced by
	// the uq_enrollment_request_children_rollover_source partial unique
	// index (migration 1.15.73) — each source child may live in at
	// most one follow-up phase. Service checks up front so the admin
	// sees a clear 409 instead of the raw DB error on the first
	// request_child insert.
	ErrRolloverSourceAlreadyRolled = errors.New("source phase already rolled forward")
)

// phaseNameUniqueConstraint is the Postgres constraint name from
// migration 1.15.67 — kept in sync via grep, not via import, because
// the migration declares it inline.
const phaseNameUniqueConstraint = "enrollment_phases_unique_name"

// rolloverSourceChildUniqueIndex is the Postgres index name from
// migration 1.15.73 that enforces "each source child rolled at most
// once". Kept as a string constant for the same reason as the phase
// name constraint above.
const rolloverSourceChildUniqueIndex = "uq_enrollment_request_children_rollover_source"

// isPhaseDuplicateName reports whether err is a PostgreSQL 23505
// raised by the unique(tenant_id, name) constraint on
// enrollment.phases. Race-safe: we don't pre-check, we just translate
// the DB error into the sentinel so the handler can return 409.
func isPhaseDuplicateName(err error) bool {
	return isUniqueViolationOn(err, phaseNameUniqueConstraint)
}

// isRolloverSourceAlreadyRolled reports whether err is the 23505
// raised by the partial unique index that pins each source child to
// at most one follow-up phase. Fallback for the race between the
// vorab-Check and the actual INSERT.
func isRolloverSourceAlreadyRolled(err error) bool {
	return isUniqueViolationOn(err, rolloverSourceChildUniqueIndex)
}

// isUniqueViolationOn reports whether err is a PostgreSQL 23505 raised
// by the named constraint or unique index. Mirrors the pattern used in
// services/facilities/facility_service.go.
func isUniqueViolationOn(err error, name string) bool {
	if err == nil {
		return false
	}
	var pgErr pgdriver.Error
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Field('C') != "23505" {
		return false
	}
	return pgErr.Field('n') == name
}

// RolloverService creates a new phase from a source phase, carrying
// every approved enrollment forward into the new phase under one of
// two parent-action modes (opt_in / opt_out). Children whose new
// grade would exceed the tenant's `enrollment.grade_level_max` setting
// land in an admin review queue instead of being auto-rolled.
//
// See `.claude/rules/backend-conventions.md` Rule 8 — this service
// orchestrates four repos (phase, request, request_child, request_
// child_offering) inside a single tenant tx, plus the outbox. Pure
// business logic, no SQL.
type RolloverService interface {
	// CreatePhaseFromSource creates the new phase + all carried-forward
	// request rows + child rows + care-offering selections, and
	// enqueues one parent email per carried request. Runs inside a
	// tenant tx so the whole rollover is atomic.
	CreatePhaseFromSource(ctx context.Context, req CreatePhaseFromSourceRequest) (*RolloverResult, error)

	// ListReviewQueue returns every request_children row in the new
	// phase that landed in pending_admin_review, along with enough
	// context (parent + source child) for the admin UI to render.
	ListReviewQueue(ctx context.Context, phaseID int64) ([]*ReviewQueueItem, error)

	// DecideReview applies the admin's decision (keep / drop / defer)
	// to one pending_admin_review row. "Keep" optionally rewrites
	// target_grade_level to handle repeaters.
	DecideReview(ctx context.Context, req DecideReviewRequest) error

	// RunDeadlineWorker scans the current tenant's phases for any
	// whose rollover_deadline has passed, and resolves their pending
	// renewal rows: auto_renewed → submitted (admin still approves
	// through the existing flow), pending_renewal → withdrawn.
	// pending_admin_review rows are intentionally left alone — admin
	// must decide each one through the review queue.
	//
	// Idempotent: re-running after all rows have transitioned is a
	// no-op. Per-phase failures are logged but do not abort the
	// batch — the next tick retries.
	RunDeadlineWorker(ctx context.Context, asOf time.Time) (*DeadlineWorkerSummary, error)
}

// DeadlineWorkerSummary is what the worker returns so callers (the
// scheduler tick log, eventually a CLI cleanup command) can report
// what changed.
type DeadlineWorkerSummary struct {
	PhasesProcessed           int
	AutoRenewedToSubmitted    int
	AutoRenewedToApproved     int // populated only on rollover_auto_approve=true phases
	PendingRenewalToWithdrawn int
	AutoApproveErrors         int
}

// CreatePhaseFromSourceRequest is the admin-facing input for the
// "create next year's phase" form. The tenant comes from context.
type CreatePhaseFromSourceRequest struct {
	SourcePhaseID     int64
	Name              string
	Kind              string
	ServiceStartDate  timezone.Date
	ServiceEndDate    timezone.Date
	EnrollmentOpenAt  *time.Time
	EnrollmentCloseAt *time.Time

	// Copy the source's form_schema_id by default (nil here means
	// "copy"); set to a different schema id to pin a new one.
	FormSchemaID *int64

	RolloverMode        string // opt_in or opt_out
	RolloverAutoApprove bool
	RolloverDeadline    time.Time
	RolloverBumpsGrade  bool

	// AdminAccountID is stamped on reviewed_by for any rolled rows
	// the admin's action causes us to write (the source children are
	// not modified). Optional — zero falls back to "system".
	AdminAccountID int64
}

// RolloverResult summarises what the rollover did so the admin UI can
// confirm "you carried 27 children forward, 2 need review".
type RolloverResult struct {
	Phase             *enrollmentModels.Phase
	SourceChildCount  int            // approved children scanned in the source phase
	RolledCount       int            // child rows created in renewal state
	ReviewCount       int            // child rows created in pending_admin_review
	ReviewByReason    map[string]int // per-reason breakdown for the UI
	RequestCount      int            // distinct parent requests created in the new phase
	EnqueuedEmails    int
	SkippedEmptyEmail int // rows whose parent had no email (no enqueue)
}

// ReviewQueueItem is one row in the admin review UI. SourceChild is
// the previous-year child the rollover tried to roll forward — used
// by the admin to see "this is Lina, last year she was in 4a, now
// she'd be in 5a which is above the cap".
type ReviewQueueItem struct {
	Child       *enrollmentModels.RequestChild
	Request     *enrollmentModels.Request
	SourceChild *enrollmentModels.RequestChild
}

// ReviewDecision values the admin can pick on a queued review row.
const (
	ReviewDecisionKeep  = "keep"
	ReviewDecisionDrop  = "drop"
	ReviewDecisionDefer = "defer"
)

// DecideReviewRequest carries the admin's decision plus the optional
// class override for the "keep" action.
type DecideReviewRequest struct {
	RequestChildID int64
	Decision       string
	// NewGradeLevel — when Decision == keep, optionally rewrites the
	// child's target_grade_level. Set this to keep a repeater at the
	// same grade as last year.
	NewGradeLevel *int16
	// AdminAccountID — stamped on reviewed_by for the audit trail.
	AdminAccountID int64
}

type rolloverService struct {
	phaseRepo                enrollmentModels.PhaseRepository
	requestRepo              enrollmentModels.RequestRepository
	requestChildRepo         enrollmentModels.RequestChildRepository
	requestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	schoolRepo               platformModels.SchoolRepository
	outboxEnqueuer           OutboxEnqueuer
	settings                 RequestSettingsResolver
	// Used only by RunDeadlineWorker when a phase has
	// rollover_auto_approve = true — we route promoted rows through
	// Decide(approved) so applyApprovalRollover updates the existing
	// student instead of stamping out duplicates.
	decisionService DecisionService
	parentsURL      string
	db              *bun.DB
	logger          *slog.Logger
}

// RolloverServiceConfig is the dependency-injection bundle.
type RolloverServiceConfig struct {
	PhaseRepo                enrollmentModels.PhaseRepository
	RequestRepo              enrollmentModels.RequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	SchoolRepo               platformModels.SchoolRepository
	OutboxEnqueuer           OutboxEnqueuer
	Settings                 RequestSettingsResolver
	// DecisionService is consumed by RunDeadlineWorker only when a
	// phase carries rollover_auto_approve = true. Optional — leave
	// nil to disable auto-approve regardless of the phase flag
	// (tests that don't wire the decision service still work).
	DecisionService DecisionService
	ParentsURL      string
	DB              *bun.DB
	Logger          *slog.Logger
}

// NewRolloverService builds the service. Nil logger falls back to
// slog.Default().
func NewRolloverService(cfg RolloverServiceConfig) RolloverService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &rolloverService{
		phaseRepo:                cfg.PhaseRepo,
		requestRepo:              cfg.RequestRepo,
		requestChildRepo:         cfg.RequestChildRepo,
		requestChildOfferingRepo: cfg.RequestChildOfferingRepo,
		schoolRepo:               cfg.SchoolRepo,
		outboxEnqueuer:           cfg.OutboxEnqueuer,
		settings:                 cfg.Settings,
		decisionService:          cfg.DecisionService,
		parentsURL:               cfg.ParentsURL,
		db:                       cfg.DB,
		logger:                   logger,
	}
}

// CreatePhaseFromSource is the workhorse. Pseudocode:
//  1. Load + validate source phase
//  2. Build the new Phase (Validate(), Insert)
//  3. List approved source children + skip ones already rolled
//  4. Group by source request_id and create one new request per group
//  5. Per child: compute new grade, decide status, create row
//  6. Copy care offerings (request_child_offerings) verbatim
//  7. Enqueue one email per new request via the outbox
func (s *rolloverService) CreatePhaseFromSource(ctx context.Context, req CreatePhaseFromSourceRequest) (*RolloverResult, error) {
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("rollover: tenant not in context")
	}

	maxGrade := s.resolveMaxGrade(ctx)

	result := &RolloverResult{
		ReviewByReason: make(map[string]int),
	}

	// Whole rollover runs in one tenant tx. If any insert fails the
	// caller sees "atomic" — either everything carried or nothing.
	txErr := s.db.RunInTx(ctx, nil, func(txCtx context.Context, _ bun.Tx) error {
		// Bridge tx into ctx for the repos.
		// WithTenantTx already does this, but the caller wired the
		// outer tenant tx for us — we just need an inner runtx for
		// commit semantics. Repos that read from ctx are unaffected.
		return s.runCreate(txCtx, tenantID, req, maxGrade, result)
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}

func (s *rolloverService) runCreate(ctx context.Context, tenantID int64, req CreatePhaseFromSourceRequest, maxGrade int, result *RolloverResult) error {
	// 1. Source phase.
	source, err := s.phaseRepo.FindByID(ctx, req.SourcePhaseID)
	if err != nil || source == nil {
		return fmt.Errorf("%w: %d", ErrRolloverSourceNotFound, req.SourcePhaseID)
	}
	if source.TenantID != tenantID {
		// RLS would catch this too, but the message is clearer here.
		return fmt.Errorf("%w: source phase belongs to another tenant", ErrRolloverSourceNotFound)
	}

	// 1b. Refuse a second rollover from the same source. The DB-level
	// partial unique index on rollover_source_child_id will catch this
	// during the request_child insert, but doing the check up front
	// gives a clean 409 before any rows are written and means the
	// admin doesn't see a half-rolled phase.
	exists, err := s.phaseRepo.ExistsByRolloverSourcePhaseID(ctx, source.ID)
	if err != nil {
		return fmt.Errorf("rollover: check existing follow-up: %w", err)
	}
	if exists {
		return fmt.Errorf("%w: source phase %d", ErrRolloverSourceAlreadyRolled, source.ID)
	}

	// 2. New phase. Default form_schema_id to the source's.
	formSchemaID := req.FormSchemaID
	if formSchemaID == nil {
		formSchemaID = source.FormSchemaID
	}

	mode := req.RolloverMode
	deadline := req.RolloverDeadline
	newPhase := &enrollmentModels.Phase{
		Name:                      req.Name,
		Kind:                      req.Kind,
		ServiceStartDate:          req.ServiceStartDate,
		ServiceEndDate:            req.ServiceEndDate,
		EnrollmentOpenAt:          req.EnrollmentOpenAt,
		EnrollmentCloseAt:         req.EnrollmentCloseAt,
		FormSchemaID:              formSchemaID,
		ShowStatusReasonToParent:  source.ShowStatusReasonToParent,
		CareOverflowMode:          source.CareOverflowMode,
		CareOfferingSelectionMode: source.CareOfferingSelectionMode,
		IsActive:                  true,
		RolloverSourcePhaseID:     &source.ID,
		RolloverMode:              &mode,
		RolloverAutoApprove:       req.RolloverAutoApprove,
		RolloverDeadline:          &deadline,
		RolloverBumpsGrade:        req.RolloverBumpsGrade,
	}
	newPhase.SetTenantID(tenantID)
	if err := newPhase.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrRolloverInvalidRequest, err)
	}
	if err := s.phaseRepo.Create(ctx, newPhase); err != nil {
		if isPhaseDuplicateName(err) {
			return fmt.Errorf("%w: %q", ErrRolloverDuplicateName, req.Name)
		}
		return fmt.Errorf("rollover: create phase: %w", err)
	}
	result.Phase = newPhase

	// 3. Approved source children. We skip auto_renewed and pending_*
	// statuses — those are themselves rolled-over rows from an earlier
	// generation and can't seed another rollover. An empty list is fine:
	// the new phase is still created so admins keep one consistent
	// "Anschlussphase erstellen" flow even when nothing carries forward.
	sourceChildren, err := s.requestChildRepo.ListByPhaseAndStatuses(
		ctx, source.ID,
		[]string{enrollmentModels.ChildStatusApproved},
	)
	if err != nil {
		return fmt.Errorf("rollover: list source children: %w", err)
	}
	result.SourceChildCount = len(sourceChildren)

	// 4. Group by request_id so we create exactly one new Request per
	// parent submission, with N children under it.
	bySourceRequest := make(map[int64][]*enrollmentModels.RequestChild, len(sourceChildren))
	sourceRequestOrder := make([]int64, 0)
	for _, c := range sourceChildren {
		if _, seen := bySourceRequest[c.RequestID]; !seen {
			sourceRequestOrder = append(sourceRequestOrder, c.RequestID)
		}
		bySourceRequest[c.RequestID] = append(bySourceRequest[c.RequestID], c)
	}

	for _, sourceRequestID := range sourceRequestOrder {
		sourceReq, err := s.requestRepo.FindByID(ctx, sourceRequestID)
		if err != nil {
			return fmt.Errorf("rollover: load source request %d: %w", sourceRequestID, err)
		}
		if err := s.rollOneRequest(ctx, tenantID, newPhase, sourceReq, bySourceRequest[sourceRequestID], maxGrade, result); err != nil {
			return err
		}
		result.RequestCount++
	}

	return nil
}

// rollOneRequest creates the per-parent renewal envelope (new Request)
// plus per-child rows + their care-offering copies, then enqueues the
// renewal email.
func (s *rolloverService) rollOneRequest(
	ctx context.Context,
	tenantID int64,
	newPhase *enrollmentModels.Phase,
	sourceReq *enrollmentModels.Request,
	sourceChildren []*enrollmentModels.RequestChild,
	maxGrade int,
	result *RolloverResult,
) error {
	statusToken, err := newStatusToken()
	if err != nil {
		return fmt.Errorf("rollover: generate status token: %w", err)
	}

	newReq := &enrollmentModels.Request{
		PhaseID:           newPhase.ID,
		SchemaID:          newPhase.FormSchemaID,
		GuardianFirstName: sourceReq.GuardianFirstName,
		GuardianLastName:  sourceReq.GuardianLastName,
		GuardianEmail:     sourceReq.GuardianEmail,
		GuardianPhone:     sourceReq.GuardianPhone,
		GuardianAccountID: sourceReq.GuardianAccountID,
		ConsentFlags:      sourceReq.ConsentFlags,
		CustomData:        sourceReq.CustomData,
		StatusToken:       statusToken,
		SubmittedAt:       time.Now(),
	}
	newReq.SetTenantID(tenantID)
	if err := s.requestRepo.Create(ctx, newReq); err != nil {
		return fmt.Errorf("rollover: create request: %w", err)
	}

	childNames := make([]string, 0, len(sourceChildren))
	for _, source := range sourceChildren {
		newGradePtr, reviewReason := computeNewGrade(source.TargetGradeLevel, newPhase.RolloverBumpsGrade, maxGrade)

		var status string
		var reviewReasonForRow *string
		if reviewReason != "" {
			status = enrollmentModels.ChildStatusPendingAdminReview
			r := reviewReason
			reviewReasonForRow = &r
			result.ReviewCount++
			result.ReviewByReason[reviewReason]++
		} else {
			status = renewalInitialStatus(*newPhase.RolloverMode)
			result.RolledCount++
		}

		sourceID := source.ID
		child := &enrollmentModels.RequestChild{
			RequestID:             newReq.ID,
			FirstName:             source.FirstName,
			LastName:              source.LastName,
			DateOfBirth:           source.DateOfBirth,
			TargetGradeLevel:      newGradePtr,
			CustomData:            source.CustomData,
			Status:                status,
			ActivationMode:        source.ActivationMode,
			SortOrder:             source.SortOrder,
			RolloverSourceChildID: &sourceID,
			ReviewReason:          reviewReasonForRow,
		}
		child.SetTenantID(tenantID)
		if err := s.requestChildRepo.Create(ctx, child); err != nil {
			if isRolloverSourceAlreadyRolled(err) {
				return fmt.Errorf("%w: source child %d", ErrRolloverSourceAlreadyRolled, source.ID)
			}
			return fmt.Errorf("rollover: create request_child: %w", err)
		}

		// Copy care offerings. The source list is authoritative —
		// admin can edit on the new row through the existing parent /
		// admin flows.
		offerings, err := s.requestChildOfferingRepo.ListByRequestChildID(ctx, source.ID)
		if err != nil {
			return fmt.Errorf("rollover: list source offerings: %w", err)
		}
		for _, off := range offerings {
			copyRow := &enrollmentModels.RequestChildOffering{
				RequestChildID: child.ID,
				CareOfferingID: off.CareOfferingID,
			}
			copyRow.SetTenantID(tenantID)
			if err := s.requestChildOfferingRepo.Create(ctx, copyRow); err != nil {
				return fmt.Errorf("rollover: copy offering: %w", err)
			}
		}

		childNames = append(childNames, fmt.Sprintf("%s %s", source.FirstName, source.LastName))
	}

	// Enqueue the renewal email. Best-effort — failure logs but does
	// not roll back the tx. Without an outbox wired we just skip
	// (test path).
	s.enqueueRenewalEmail(ctx, newPhase, newReq, childNames, result)

	return nil
}

func (s *rolloverService) enqueueRenewalEmail(ctx context.Context, newPhase *enrollmentModels.Phase, req *enrollmentModels.Request, childNames []string, result *RolloverResult) {
	if s.outboxEnqueuer == nil {
		return
	}
	if req.GuardianEmail == "" {
		result.SkippedEmptyEmail++
		return
	}

	kind := platformModels.EmailKindEnrollmentRolloverOptOut
	if *newPhase.RolloverMode == enrollmentModels.PhaseRolloverModeOptIn {
		kind = platformModels.EmailKindEnrollmentRolloverOptIn
	}

	schoolName, logoURL := emailBrandForSchool(ctx, s.schoolRepo, req.TenantID, s.parentsURL)
	footerLogoURL := motoLogoURL(s.parentsURL)
	deadlineStr := ""
	if newPhase.RolloverDeadline != nil {
		deadlineStr = newPhase.RolloverDeadline.Format("02.01.2006")
	}

	payload := map[string]any{
		EnrollmentPayloadGuardianFirstName: req.GuardianFirstName,
		EnrollmentPayloadGuardianLastName:  req.GuardianLastName,
		EnrollmentPayloadGuardianEmail:     req.GuardianEmail,
		EnrollmentPayloadSchoolName:        schoolName,
		EnrollmentPayloadPhaseName:         newPhase.Name,
		EnrollmentPayloadStatusURL:         s.parentStatusURL(req.StatusToken),
		EnrollmentPayloadLogoURL:           logoURL,
		EnrollmentPayloadMotoLogoURL:       footerLogoURL,
		EnrollmentPayloadChildNames:        childNames,
		EnrollmentPayloadRecipientEmail:    req.GuardianEmail,
		EnrollmentPayloadRolloverDeadline:  deadlineStr,
	}
	if err := s.outboxEnqueuer.Enqueue(ctx, OutboxEnqueueRequest{
		Kind:              kind,
		Payload:           payload,
		RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
		RelatedEntityID:   req.ID,
	}); err != nil {
		s.logger.Warn("rollover: enqueue renewal email failed",
			slog.Int64("request_id", req.ID),
			slog.String("kind", kind),
			slog.String("error", err.Error()),
		)
		return
	}
	result.EnqueuedEmails++
}

func (s *rolloverService) parentStatusURL(token string) string {
	host := s.parentsURL
	if host == "" {
		host = "http://localhost:3000"
	}
	return fmt.Sprintf("%s/enroll/status/%s", host, token)
}

func (s *rolloverService) validateCreateRequest(req CreatePhaseFromSourceRequest) error {
	if req.SourcePhaseID <= 0 {
		return fmt.Errorf("%w: source_phase_id is required", ErrRolloverInvalidRequest)
	}
	if req.Name == "" {
		return fmt.Errorf("%w: name is required", ErrRolloverInvalidRequest)
	}
	if req.ServiceStartDate.IsZero() || req.ServiceEndDate.IsZero() {
		return fmt.Errorf("%w: service dates are required", ErrRolloverInvalidRequest)
	}
	if req.ServiceEndDate.Before(req.ServiceStartDate) {
		return fmt.Errorf("%w: service_end_date must be on or after service_start_date", ErrRolloverInvalidRequest)
	}
	if req.RolloverDeadline.IsZero() {
		return fmt.Errorf("%w: rollover_deadline is required", ErrRolloverInvalidRequest)
	}
	if req.RolloverMode != enrollmentModels.PhaseRolloverModeOptIn &&
		req.RolloverMode != enrollmentModels.PhaseRolloverModeOptOut {
		return fmt.Errorf("%w: rollover_mode must be opt_in or opt_out", ErrRolloverInvalidRequest)
	}
	return nil
}

func (s *rolloverService) resolveMaxGrade(ctx context.Context) int {
	// Default mirrors the registry default — the same setting drives
	// both the public form's grade picker and the rollover grade cap.
	const fallback = 4
	if s.settings == nil {
		return fallback
	}
	if has, err := s.settings.HasTenantOverride(ctx, configModel.KeyEnrollmentGradeLevelMax); err == nil && has {
		if v, err := s.settings.ResolveInt(ctx, configModel.KeyEnrollmentGradeLevelMax); err == nil && v > 0 {
			return v
		}
	}
	// No override — pull the registry default through Resolve so a
	// future change to the registry value flows in without a code
	// change.
	if v, err := s.settings.ResolveInt(ctx, configModel.KeyEnrollmentGradeLevelMax); err == nil && v > 0 {
		return v
	}
	return fallback
}

// ListReviewQueue loads admin-review rows + their parent request + the
// source child for context. Tenant RLS scopes the read.
func (s *rolloverService) ListReviewQueue(ctx context.Context, phaseID int64) ([]*ReviewQueueItem, error) {
	if phaseID <= 0 {
		return nil, fmt.Errorf("%w: phase_id is required", ErrRolloverInvalidRequest)
	}
	children, err := s.requestChildRepo.ListByPhaseAndStatuses(
		ctx, phaseID,
		[]string{enrollmentModels.ChildStatusPendingAdminReview},
	)
	if err != nil {
		return nil, fmt.Errorf("rollover: list review queue: %w", err)
	}
	out := make([]*ReviewQueueItem, 0, len(children))
	for _, c := range children {
		req, reqErr := s.requestRepo.FindByID(ctx, c.RequestID)
		if reqErr != nil {
			s.logger.Warn("rollover: review queue request lookup failed",
				slog.Int64("request_child_id", c.ID),
				slog.String("error", reqErr.Error()))
			continue
		}
		item := &ReviewQueueItem{Child: c, Request: req}
		if c.RolloverSourceChildID != nil {
			// Source child is in the previous phase, same tenant, so
			// the RLS-bypass admin tx the handler wraps lets us read
			// it. If we can't, the admin still gets a useful row —
			// just without the prior-year context.
			if src, srcErr := s.requestChildRepo.FindByID(ctx, *c.RolloverSourceChildID); srcErr == nil {
				item.SourceChild = src
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// DecideReview applies the admin's keep/drop/defer action.
func (s *rolloverService) DecideReview(ctx context.Context, req DecideReviewRequest) error {
	if req.RequestChildID <= 0 {
		return fmt.Errorf("%w: request_child_id is required", ErrRolloverReviewInvalid)
	}
	switch req.Decision {
	case ReviewDecisionKeep:
		// "Keep" promotes the row out of review and into the active
		// renewal flow. The downstream parent confirm/decline + the
		// deadline worker then handle it like any other rolled row.
		// To keep things simple, "keep" always lands in
		// auto_renewed: the admin has already implicitly confirmed
		// they want this student carried forward. The deadline
		// worker will promote auto_renewed → submitted, then the
		// admin's decision queue handles final approval. Class
		// override (if any) is applied at the same time.
		return s.requestChildRepo.UpdateRolloverReview(
			ctx,
			req.RequestChildID,
			enrollmentModels.ChildStatusAutoRenewed,
			nil,
			req.NewGradeLevel,
			req.AdminAccountID,
		)
	case ReviewDecisionDrop:
		reason := "rollover_drop"
		return s.requestChildRepo.UpdateRolloverReview(
			ctx,
			req.RequestChildID,
			enrollmentModels.ChildStatusWithdrawn,
			&reason,
			nil,
			req.AdminAccountID,
		)
	case ReviewDecisionDefer:
		// Defer means "I'll come back to it" — leave as-is. We still
		// stamp reviewed_at via UpdateRolloverReview so the admin
		// sees their last touch.
		return s.requestChildRepo.UpdateRolloverReview(
			ctx,
			req.RequestChildID,
			enrollmentModels.ChildStatusPendingAdminReview,
			nil,
			nil,
			req.AdminAccountID,
		)
	default:
		return fmt.Errorf("%w: decision must be keep/drop/defer, got %q",
			ErrRolloverReviewInvalid, req.Decision)
	}
}

// computeNewGrade returns the next grade level and (if positive) the
// review reason explaining why this row needs admin attention.
//
//   - source nil          → no grade on file, needs review
//   - bumpsGrade=false    → keep grade, no review (half-year cadence)
//   - newGrade > maxGrade → above the cap, needs review
//   - otherwise           → bumped grade, no review
func computeNewGrade(sourceGrade *int16, bumpsGrade bool, maxGrade int) (*int16, string) {
	if sourceGrade == nil {
		return nil, enrollmentModels.ReviewReasonNoGradeLevel
	}
	bump := int16(0)
	if bumpsGrade {
		bump = 1
	}
	newGrade := *sourceGrade + bump
	if int(newGrade) > maxGrade {
		// Surface as review with the same grade carried through so
		// the admin can see "this student would be in grade 5".
		return &newGrade, enrollmentModels.ReviewReasonGradeAboveMax
	}
	return &newGrade, ""
}

func renewalInitialStatus(mode string) string {
	if mode == enrollmentModels.PhaseRolloverModeOptIn {
		return enrollmentModels.ChildStatusPendingRenewal
	}
	return enrollmentModels.ChildStatusAutoRenewed
}

// RunDeadlineWorker is the scheduled resolver. Caller (scheduler tick
// or CLI) wraps in WithTenantTx so the bulk updates run as
// phoenix_tenant with RLS scoping to the current tenant.
func (s *rolloverService) RunDeadlineWorker(ctx context.Context, asOf time.Time) (*DeadlineWorkerSummary, error) {
	if s.phaseRepo == nil || s.requestChildRepo == nil {
		return nil, fmt.Errorf("rollover deadline: required repos not wired")
	}

	summary := &DeadlineWorkerSummary{}
	phases, err := s.phaseRepo.ListWithExpiredRolloverDeadline(ctx, asOf)
	if err != nil {
		return summary, fmt.Errorf("rollover deadline: list expired phases: %w", err)
	}
	for _, phase := range phases {
		summary.PhasesProcessed++

		// Opt-out side: auto_renewed transitions to either approved
		// (rollover_auto_approve=true, via DecisionService so the
		// existing student gets updated) or submitted (default, the
		// admin still approves manually through the existing queue).
		autoApproved, autoSubmitted, autoErrs := s.resolveAutoRenewed(ctx, phase)
		summary.AutoRenewedToApproved += autoApproved
		summary.AutoRenewedToSubmitted += autoSubmitted
		summary.AutoApproveErrors += autoErrs

		// Opt-in side: pending_renewal → withdrawn. The parent
		// didn't act before the deadline, so the renewal lapses.
		pendingCount, err := s.requestChildRepo.BulkUpdateStatusByPhaseAndStatus(
			ctx, phase.ID,
			enrollmentModels.ChildStatusPendingRenewal,
			enrollmentModels.ChildStatusWithdrawn,
		)
		if err != nil {
			s.logger.Error("rollover deadline: demote pending_renewal failed",
				slog.Int64("phase_id", phase.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		summary.PendingRenewalToWithdrawn += pendingCount

		if autoApproved > 0 || autoSubmitted > 0 || pendingCount > 0 {
			s.logger.Info("rollover deadline: resolved phase",
				slog.Int64("phase_id", phase.ID),
				slog.Int("auto_to_approved", autoApproved),
				slog.Int("auto_to_submitted", autoSubmitted),
				slog.Int("pending_to_withdrawn", pendingCount),
				slog.Int("auto_approve_errors", autoErrs),
			)
		}
	}
	return summary, nil
}

// resolveAutoRenewed handles the auto_renewed cohort for one phase.
// Returns the counts split by destination status (approved when the
// phase opts in to auto-approve and the decision service is wired,
// otherwise submitted) plus how many per-row Decide() calls errored.
//
// When rollover_auto_approve is on but decisionService is nil (test
// environments that don't wire the full approval pipeline), we fall
// back to the bulk-promotion-to-submitted path so the worker still
// completes — logs a warning so the gap is visible.
func (s *rolloverService) resolveAutoRenewed(ctx context.Context, phase *enrollmentModels.Phase) (approved, submitted, errs int) {
	if !phase.RolloverAutoApprove || s.decisionService == nil {
		if phase.RolloverAutoApprove && s.decisionService == nil {
			s.logger.Warn("rollover deadline: auto_approve=true but DecisionService not wired, falling back to submitted",
				slog.Int64("phase_id", phase.ID))
		}
		count, err := s.requestChildRepo.BulkUpdateStatusByPhaseAndStatus(
			ctx, phase.ID,
			enrollmentModels.ChildStatusAutoRenewed,
			enrollmentModels.ChildStatusSubmitted,
		)
		if err != nil {
			s.logger.Error("rollover deadline: promote auto_renewed failed",
				slog.Int64("phase_id", phase.ID),
				slog.String("error", err.Error()))
			return 0, 0, 0
		}
		return 0, count, 0
	}

	// Auto-approve path: pull each auto_renewed row, call Decide so
	// applyApprovalRollover runs (updates the existing student, fires
	// the approval email, etc.).
	rows, err := s.requestChildRepo.ListByPhaseAndStatuses(
		ctx, phase.ID,
		[]string{enrollmentModels.ChildStatusAutoRenewed},
	)
	if err != nil {
		s.logger.Error("rollover deadline: list auto_renewed failed",
			slog.Int64("phase_id", phase.ID),
			slog.String("error", err.Error()))
		return 0, 0, 0
	}
	for _, row := range rows {
		_, decideErr := s.decisionService.Decide(ctx, DecideInput{
			RequestID: row.RequestID,
			ChildID:   row.ID,
			Status:    DecisionApproved,
		})
		if decideErr != nil {
			errs++
			s.logger.Error("rollover deadline: auto-approve decide failed",
				slog.Int64("phase_id", phase.ID),
				slog.Int64("request_child_id", row.ID),
				slog.String("error", decideErr.Error()))
			continue
		}
		approved++
	}
	return approved, 0, errs
}
