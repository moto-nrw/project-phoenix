package enrollment

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// RequestService sentinel errors. The HTTP layer maps these to status
// codes; tests assert on them via errors.Is.
var (
	ErrEnrollmentDisabled     = errors.New("enrollment is not enabled for this tenant")
	ErrEnrollmentWindowClosed = errors.New("enrollment window is closed")
	ErrInvalidSubmission      = errors.New("invalid submission")
	ErrCareOfferingClosed     = errors.New("one or more selected care offerings are not currently accepting applications")
	ErrCareOfferingFull       = errors.New("one or more selected care offerings are at capacity")
	ErrRateLimited            = errors.New("too many submission attempts; please retry later")
	ErrRequestNotFound        = errors.New("enrollment request not found")
	ErrEditNotAllowed         = errors.New("request can no longer be edited")
	ErrWithdrawNotAllowed     = errors.New("child cannot be withdrawn in its current state")
	ErrDuplicateEnrollment    = errors.New("an active enrollment already exists for this parent and child in this phase")
)

// Rate-limit thresholds. Hardcoded for now - if individual schools
// need different limits we can promote these to settings, but the
// defaults need to work for "small school with families of 3 kids
// submitting once" without ever needing tuning.
const (
	rateLimitWindowIP        = time.Hour
	rateLimitWindowEmail     = 24 * time.Hour
	rateLimitMaxAttemptsIP   = 10
	rateLimitMaxAttemptsMail = 5
)

// SubmitRequest is the data the public submission handler hands to the
// service. PR 7's HTTP layer translates the JSON wire shape into this
// struct.
type SubmitRequest struct {
	TenantID int64
	PhaseID  int64

	// RemoteIP is the parent's source IP (handler resolves X-Forwarded-For
	// when set, falls back to RemoteAddr). Empty disables IP-based rate
	// limiting for that submission - emit it from the handler when at
	// all possible.
	RemoteIP string

	GuardianFirstName string
	GuardianLastName  string
	GuardianEmail     string
	GuardianPhone     *string
	ConsentFlags      map[string]any
	CustomData        map[string]any

	// GuardianAccountID is set when the submission comes from an
	// authenticated parent on the parents portal. The handler verifies
	// the account has access to req.TenantID before passing it through.
	// Stamped onto the request row so PR 11/4 can skip the invitation
	// when an account already exists. nil = anonymous public submission.
	GuardianAccountID *int64

	Children []SubmitChild
}

// SubmitChild is one child within a SubmitRequest.
type SubmitChild struct {
	FirstName        string
	LastName         string
	DateOfBirth      time.Time
	TargetGradeLevel *int16
	CustomData       map[string]any
	OfferingIDs      []int64
}

// SubmitResult bundles what the handler needs after Submit returns.
type SubmitResult struct {
	Request   *enrollmentModels.Request
	Children  []*enrollmentModels.RequestChild
	StatusURL string
}

// EditPatch carries the fields the parent can edit between submission
// and the first admin decision. Pointer fields = "leave alone unless
// set"; map fields replace wholesale.
type EditPatch struct {
	GuardianFirstName *string
	GuardianLastName  *string
	GuardianPhone     *string
	ConsentFlags      map[string]any
	CustomData        map[string]any
}

// RequestService manages the parent-facing submission lifecycle. PR 7
// ships Submit / GetByStatusToken / Edit / Withdraw; PR 8's decision
// service builds on the same data.
type RequestService interface {
	Submit(ctx context.Context, req SubmitRequest) (*SubmitResult, error)
	GetByStatusToken(ctx context.Context, token string) (*enrollmentModels.Request, []*enrollmentModels.RequestChild, error)
	Edit(ctx context.Context, token string, patch EditPatch) error
	Withdraw(ctx context.Context, token string, childID int64) error
	// IsEnrollmentEnabled reports whether the per-tenant master toggle
	// (enrollment.enabled setting) is on for the tenant in ctx. Public
	// form-load endpoints call this so a deactivated tenant returns a
	// 404 at the picker/schema step instead of letting the parent fill
	// in a whole form before being rejected at submit. Caller must
	// already be inside a tenant-tx so the settings repo can read the
	// per-tenant override.
	IsEnrollmentEnabled(ctx context.Context) bool
}

// RequestSettingsResolver is the narrow contract the service needs from
// the platform settings service.
type RequestSettingsResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// RequestServiceConfig is the dep-injection bundle.
type RequestServiceConfig struct {
	RequestRepo              enrollmentModels.RequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	CareOfferingRepo         enrollmentModels.CareOfferingRepository
	FormSchemaRepo           enrollmentModels.FormSchemaRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	SchoolRepo               platformModels.SchoolRepository
	RateLimitRepo            enrollmentModels.SubmissionRateLimitRepository
	OutboxEnqueuer           OutboxEnqueuer
	Settings                 RequestSettingsResolver
	FrontendURL              string // staff/admin URLs only (admin notification email link)
	ParentsURL               string // parent-facing URLs (status link, logo). Falls back to FrontendURL when empty.
	DB                       *bun.DB
	Logger                   *slog.Logger
}

// OutboxEnqueuer mirrors the same shape used by services/auth so this
// package doesn't import services/platform. Adapter wiring lives in
// services/platform alongside the existing auth adapter.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, req OutboxEnqueueRequest) error
}

// OutboxEnqueueRequest mirrors platform.EnqueueRequest.
type OutboxEnqueueRequest struct {
	Kind              string
	Payload           map[string]any
	RelatedEntityType string
	RelatedEntityID   int64
}

type requestService struct {
	requestRepo              enrollmentModels.RequestRepository
	requestChildRepo         enrollmentModels.RequestChildRepository
	requestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	careOfferingRepo         enrollmentModels.CareOfferingRepository
	formSchemaRepo           enrollmentModels.FormSchemaRepository
	phaseRepo                enrollmentModels.PhaseRepository
	schoolRepo               platformModels.SchoolRepository
	rateLimitRepo            enrollmentModels.SubmissionRateLimitRepository
	outboxEnqueuer           OutboxEnqueuer
	settings                 RequestSettingsResolver
	frontendURL              string
	parentsURL               string
	db                       *bun.DB
	txHandler                *modelBase.TxHandler
	logger                   *slog.Logger
}

// NewRequestService builds the service. A nil logger falls back to
// slog.Default().
func NewRequestService(cfg RequestServiceConfig) RequestService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &requestService{
		requestRepo:              cfg.RequestRepo,
		requestChildRepo:         cfg.RequestChildRepo,
		requestChildOfferingRepo: cfg.RequestChildOfferingRepo,
		careOfferingRepo:         cfg.CareOfferingRepo,
		formSchemaRepo:           cfg.FormSchemaRepo,
		phaseRepo:                cfg.PhaseRepo,
		schoolRepo:               cfg.SchoolRepo,
		rateLimitRepo:            cfg.RateLimitRepo,
		outboxEnqueuer:           cfg.OutboxEnqueuer,
		settings:                 cfg.Settings,
		frontendURL:              strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/"),
		parentsURL: func() string {
			parents := strings.TrimRight(strings.TrimSpace(cfg.ParentsURL), "/")
			if parents != "" {
				return parents
			}
			return strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/")
		}(),
		db:        cfg.DB,
		txHandler: modelBase.NewTxHandler(cfg.DB),
		logger:    logger,
	}
}

// Submit is the workhorse. One DB transaction writes the request, all
// child rows, all care-offering selections, and enqueues the two
// confirmation emails. Either everything lands or nothing does.
//
// Phase-model: req.PhaseID identifies the parent's chosen phase. The
// phase carries its own enrollment window, form-schema reference, and
// care-overflow mode - all the things that used to be tenant-wide.
func (s *requestService) Submit(ctx context.Context, req SubmitRequest) (*SubmitResult, error) {
	if err := s.enforceRateLimit(ctx, req); err != nil {
		return nil, err
	}
	if err := s.validateSubmission(ctx, req); err != nil {
		return nil, err
	}

	phase, err := s.loadPhaseForSubmission(ctx, req.PhaseID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if !phase.IsEnrollmentWindowOpen(now) {
		return nil, ErrEnrollmentWindowClosed
	}

	// Build the dedup key list once; the actual check + insert happens
	// inside the write tx below, after we've taken an advisory lock to
	// serialize concurrent submits for the same (phase, email).
	dupKeys := make([]enrollmentModels.DuplicateChildKey, 0, len(req.Children))
	for _, c := range req.Children {
		dupKeys = append(dupKeys, enrollmentModels.DuplicateChildKey{
			FirstName: c.FirstName,
			LastName:  c.LastName,
		})
	}

	openOfferings, err := s.careOfferingRepo.ListActiveByPhase(ctx, phase.ID)
	if err != nil {
		return nil, fmt.Errorf("submit: load phase offerings: %w", err)
	}
	openByID := make(map[int64]*enrollmentModels.CareOffering, len(openOfferings))
	for _, o := range openOfferings {
		openByID[o.ID] = o
	}
	if err := validateOfferingSelections(req.Children, openByID); err != nil {
		return nil, err
	}

	// Decide per-child overflow status before opening the write tx.
	// childStatusOverrides[i] is set when capacity logic forces a
	// non-default status (e.g. waitlisted under mode=waitlist). When the
	// mode is 'reject' an over-capacity offering aborts the whole
	// submission with ErrCareOfferingFull. Mode comes from the phase row.
	childStatusOverrides, err := s.applyCapacityOverflow(ctx, phase, req.Children, openByID)
	if err != nil {
		return nil, err
	}

	// Pin the schema version to whichever schema the phase points at,
	// or the tenant's currently-active schema if the phase has no
	// override. When neither resolves (Basis phase + no tenant schema
	// ever published), we submit without a schema_id - the column is
	// nullable since migration 1.15.69.
	schema, err := s.resolveSubmissionSchema(ctx, phase)
	if err != nil {
		return nil, fmt.Errorf("submit: load schema: %w", err)
	}

	statusToken, err := newStatusToken()
	if err != nil {
		return nil, fmt.Errorf("submit: generate status token: %w", err)
	}
	statusExpiry := s.resolveStatusTokenExpiry(ctx)
	statusExpiresAt := time.Now().Add(statusExpiry)

	var (
		createdRequest  *enrollmentModels.Request
		createdChildren []*enrollmentModels.RequestChild
	)
	txErr := s.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		// Serialize concurrent submits for the same (phase, guardian
		// email) so two parallel requests can't both pass the dedup
		// check and then both insert. The lock auto-releases at tx
		// commit/rollback. Phase ID is the first key, FNV-64 hash of
		// the lowercased email is the second - pg_advisory_xact_lock
		// takes two int4s OR one int8.
		emailLC := strings.ToLower(strings.TrimSpace(req.GuardianEmail))
		emailHash := fnvHash64(emailLC)
		if _, err := tx.ExecContext(txCtx, `SELECT pg_advisory_xact_lock(?, ?)`, int32(phase.ID&0x7fffffff), int32(emailHash&0x7fffffff)); err != nil {
			return fmt.Errorf("submit: acquire dedup lock: %w", err)
		}

		// Dedup check runs inside the lock so the result is stable for
		// the rest of the tx. Different parents or different child
		// names slip past untouched; rejected/withdrawn rows are
		// ignored, so a parent can re-apply after a denial.
		dupes, dupErr := s.requestRepo.FindActiveDuplicate(txCtx, phase.ID, req.GuardianEmail, dupKeys)
		if dupErr != nil {
			return fmt.Errorf("submit: duplicate check: %w", dupErr)
		}
		if len(dupes) > 0 {
			return ErrDuplicateEnrollment
		}

		var schemaID *int64
		if schema != nil {
			id := schema.ID
			schemaID = &id
		}
		request := &enrollmentModels.Request{
			SchemaID:           schemaID,
			PhaseID:            phase.ID,
			GuardianAccountID:  req.GuardianAccountID,
			GuardianFirstName:  strings.TrimSpace(req.GuardianFirstName),
			GuardianLastName:   strings.TrimSpace(req.GuardianLastName),
			GuardianEmail:      strings.ToLower(strings.TrimSpace(req.GuardianEmail)),
			GuardianPhone:      req.GuardianPhone,
			ConsentFlags:       req.ConsentFlags,
			CustomData:         req.CustomData,
			StatusToken:        statusToken,
			StatusTokenExpires: &statusExpiresAt,
			SubmittedAt:        time.Now(),
		}
		if err := s.requestRepo.Create(txCtx, request); err != nil {
			return fmt.Errorf("submit: create request: %w", err)
		}
		createdRequest = request

		for i, child := range req.Children {
			status := enrollmentModels.ChildStatusSubmitted
			if override, ok := childStatusOverrides[i]; ok {
				status = override
			}
			row := &enrollmentModels.RequestChild{
				RequestID:        request.ID,
				FirstName:        strings.TrimSpace(child.FirstName),
				LastName:         strings.TrimSpace(child.LastName),
				DateOfBirth:      child.DateOfBirth,
				TargetGradeLevel: child.TargetGradeLevel,
				CustomData:       child.CustomData,
				Status:           status,
				ActivationMode:   enrollmentModels.ChildActivationScheduled,
				SortOrder:        i,
			}
			if err := s.requestChildRepo.Create(txCtx, row); err != nil {
				return fmt.Errorf("submit: create request child %d: %w", i, err)
			}

			for _, offeringID := range child.OfferingIDs {
				if _, ok := openByID[offeringID]; !ok {
					return fmt.Errorf("submit: offering %d disappeared mid-submit", offeringID)
				}
				link := &enrollmentModels.RequestChildOffering{
					RequestChildID: row.ID,
					CareOfferingID: offeringID,
				}
				if err := s.requestChildOfferingRepo.Create(txCtx, link); err != nil {
					return fmt.Errorf("submit: create child-offering link: %w", err)
				}
			}
			createdChildren = append(createdChildren, row)
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	statusURL := s.statusURL(statusToken)
	s.enqueueSubmissionEmails(ctx, req.TenantID, createdRequest, createdChildren, statusURL)

	s.logger.Info("enrollment request submitted",
		slog.Int64("request_id", createdRequest.ID),
		slog.Int64("tenant_id", req.TenantID),
		slog.Int("children", len(createdChildren)))

	return &SubmitResult{
		Request:   createdRequest,
		Children:  createdChildren,
		StatusURL: statusURL,
	}, nil
}

func (s *requestService) validateSubmission(ctx context.Context, req SubmitRequest) error {
	if !s.isEnrollmentEnabled(ctx) {
		return ErrEnrollmentDisabled
	}
	// Phase-window enforcement happens after we load the phase row in
	// Submit(). The here check used to enforce a tenant-wide window -
	// that setting is gone in the phase model.
	if req.PhaseID <= 0 {
		return fmt.Errorf("%w: phase_id is required", ErrInvalidSubmission)
	}
	if strings.TrimSpace(req.GuardianFirstName) == "" {
		return fmt.Errorf("%w: guardian first name is required", ErrInvalidSubmission)
	}
	if strings.TrimSpace(req.GuardianLastName) == "" {
		return fmt.Errorf("%w: guardian last name is required", ErrInvalidSubmission)
	}
	emailAddr := strings.TrimSpace(req.GuardianEmail)
	if emailAddr == "" {
		return fmt.Errorf("%w: guardian email is required", ErrInvalidSubmission)
	}
	if _, err := mail.ParseAddress(emailAddr); err != nil {
		return fmt.Errorf("%w: invalid email address", ErrInvalidSubmission)
	}
	if len(req.Children) == 0 {
		return fmt.Errorf("%w: at least one child is required", ErrInvalidSubmission)
	}
	gradeMax := s.resolveGradeMax(ctx)
	for i, child := range req.Children {
		if strings.TrimSpace(child.FirstName) == "" || strings.TrimSpace(child.LastName) == "" {
			return fmt.Errorf("%w: child %d missing name", ErrInvalidSubmission, i)
		}
		if child.DateOfBirth.IsZero() {
			return fmt.Errorf("%w: child %d missing date_of_birth", ErrInvalidSubmission, i)
		}
		if child.TargetGradeLevel != nil {
			if *child.TargetGradeLevel < 1 || int(*child.TargetGradeLevel) > gradeMax {
				return fmt.Errorf("%w: child %d grade out of range 1..%d", ErrInvalidSubmission, i, gradeMax)
			}
		}
	}
	return nil
}

// validateOfferingSelections cross-checks every offering id against the
// live open-window catalog. Defense-in-depth against stale clients.
func validateOfferingSelections(children []SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) error {
	for _, child := range children {
		for _, offeringID := range child.OfferingIDs {
			if _, ok := openByID[offeringID]; !ok {
				return ErrCareOfferingClosed
			}
		}
	}
	return nil
}

// GetByStatusToken loads a request + its children for the public
// status page. Caller is responsible for setting an admin-tx context
// (token-only auth - RLS would block unprivileged SELECTs).
func (s *requestService) GetByStatusToken(ctx context.Context, token string) (*enrollmentModels.Request, []*enrollmentModels.RequestChild, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil, ErrRequestNotFound
	}
	req, err := s.requestRepo.FindByStatusToken(ctx, token)
	if err != nil {
		return nil, nil, ErrRequestNotFound
	}
	if req.StatusTokenExpires != nil && time.Now().After(*req.StatusTokenExpires) {
		return nil, nil, ErrRequestNotFound
	}

	tenantCtx := tenant.WithTenantID(ctx, req.GetTenantID())
	children, err := s.requestChildRepo.ListByRequestID(tenantCtx, req.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("status: list children: %w", err)
	}
	return req, children, nil
}

// Edit applies the patch only when every child is still `submitted`.
// PR 7 ships a minimal in-place mutation via raw bun on the tenant
// transaction; PR 8's admin update path may extend the repo with a
// dedicated Update method when it ships its own admin-edit endpoint.
func (s *requestService) Edit(ctx context.Context, token string, patch EditPatch) error {
	req, children, err := s.GetByStatusToken(ctx, token)
	if err != nil {
		return err
	}
	if !s.allowSubmissionEdit(ctx) {
		return ErrEditNotAllowed
	}
	for _, c := range children {
		if c.Status != enrollmentModels.ChildStatusSubmitted {
			return ErrEditNotAllowed
		}
	}

	if patch.GuardianFirstName != nil {
		req.GuardianFirstName = strings.TrimSpace(*patch.GuardianFirstName)
	}
	if patch.GuardianLastName != nil {
		req.GuardianLastName = strings.TrimSpace(*patch.GuardianLastName)
	}
	if patch.GuardianPhone != nil {
		v := strings.TrimSpace(*patch.GuardianPhone)
		req.GuardianPhone = &v
	}
	if patch.ConsentFlags != nil {
		req.ConsentFlags = patch.ConsentFlags
	}
	if patch.CustomData != nil {
		req.CustomData = patch.CustomData
	}

	tenantID := req.GetTenantID()
	tenantCtx := tenant.WithTenantID(ctx, tenantID)
	return tenant.WithTenantTx(tenantCtx, s.db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
		_, err := tx.NewUpdate().
			Model(req).
			ModelTableExpr(`enrollment.requests AS "request"`).
			Set("guardian_first_name = ?", req.GuardianFirstName).
			Set("guardian_last_name = ?", req.GuardianLastName).
			Set("guardian_phone = ?", req.GuardianPhone).
			Set("consent_flags = ?", req.ConsentFlags).
			Set("custom_data = ?", req.CustomData).
			Set("updated_at = NOW()").
			Where(`"request".id = ?`, req.ID).
			Exec(txCtx)
		return err
	})
}

// Withdraw transitions a child to `withdrawn` (or every non-terminal
// child when childID is 0). Approved children must go through the
// admin (terminal student records exist) - returns ErrWithdrawNotAllowed.
func (s *requestService) Withdraw(ctx context.Context, token string, childID int64) error {
	req, children, err := s.GetByStatusToken(ctx, token)
	if err != nil {
		return err
	}

	tenantID := req.GetTenantID()
	tenantCtx := tenant.WithTenantID(ctx, tenantID)
	return tenant.WithTenantTx(tenantCtx, s.db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
		anyWithdrawn := false
		for _, c := range children {
			if childID != 0 && c.ID != childID {
				continue
			}
			if c.Status == enrollmentModels.ChildStatusApproved {
				if childID != 0 {
					return ErrWithdrawNotAllowed
				}
				continue
			}
			if c.IsTerminal() {
				continue // already approved/rejected/withdrawn
			}
			if err := s.requestChildRepo.UpdateStatus(txCtx, c.ID, enrollmentModels.ChildStatusWithdrawn, nil, 0); err != nil {
				return err
			}
			anyWithdrawn = true
		}
		if childID == 0 && anyWithdrawn {
			now := time.Now()
			_, err := tx.NewUpdate().
				Model((*enrollmentModels.Request)(nil)).
				ModelTableExpr(`enrollment.requests AS "request"`).
				Set("withdrawn_at = ?", now).
				Set("updated_at = NOW()").
				Where(`"request".id = ?`, req.ID).
				Exec(txCtx)
			return err
		}
		return nil
	})
}

// enqueueSubmissionEmails fires off the parent confirmation + admin
// notifications. Best-effort - failures log but don't fail the
// submission (the rows are already committed).
func (s *requestService) enqueueSubmissionEmails(ctx context.Context, tenantID int64, request *enrollmentModels.Request, children []*enrollmentModels.RequestChild, statusURL string) {
	if s.outboxEnqueuer == nil {
		return
	}

	schoolName := s.lookupSchoolName(ctx, tenantID)
	logoURL := s.parentsURL + "/images/moto_transparent.png"
	childNames := make([]string, 0, len(children))
	for _, c := range children {
		childNames = append(childNames, fmt.Sprintf("%s %s", c.FirstName, c.LastName))
	}

	parentPayload := map[string]any{
		EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
		EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
		EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
		EnrollmentPayloadSchoolName:        schoolName,
		EnrollmentPayloadStatusURL:         statusURL,
		EnrollmentPayloadLogoURL:           logoURL,
		EnrollmentPayloadChildNames:        childNames,
		EnrollmentPayloadRecipientEmail:    request.GuardianEmail,
	}
	if err := s.outboxEnqueuer.Enqueue(ctx, OutboxEnqueueRequest{
		Kind:              platformModels.EmailKindEnrollmentSubmitted,
		Payload:           parentPayload,
		RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
		RelatedEntityID:   request.ID,
	}); err != nil {
		s.logger.Error("submit: enqueue parent confirmation failed",
			slog.Int64("request_id", request.ID),
			slog.String("error", err.Error()))
	}

	for _, admin := range s.resolveAdminEmails(ctx) {
		adminPayload := map[string]any{
			EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
			EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
			EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
			EnrollmentPayloadSchoolName:        schoolName,
			EnrollmentPayloadAdminURL:          fmt.Sprintf("%s/enrollments/%d", s.frontendURL, request.ID),
			EnrollmentPayloadLogoURL:           logoURL,
			EnrollmentPayloadChildNames:        childNames,
			EnrollmentPayloadRecipientEmail:    admin,
		}
		if request.GuardianPhone != nil {
			adminPayload[EnrollmentPayloadGuardianPhone] = *request.GuardianPhone
		}
		if err := s.outboxEnqueuer.Enqueue(ctx, OutboxEnqueueRequest{
			Kind:              platformModels.EmailKindEnrollmentAdminNotify,
			Payload:           adminPayload,
			RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
			RelatedEntityID:   request.ID,
		}); err != nil {
			s.logger.Error("submit: enqueue admin notification failed",
				slog.Int64("request_id", request.ID),
				slog.String("admin", admin),
				slog.String("error", err.Error()))
		}
	}
}

// --- helpers ---

// loadPhaseForSubmission fetches the phase the parent selected and
// validates it's enabled. Returns ErrEnrollmentDisabled when the phase
// is inactive (admin marked it hidden) and ErrInvalidSubmission when
// the id is unknown. The window check happens in Submit() - kept
// separate so error mapping in the handler stays specific.
func (s *requestService) loadPhaseForSubmission(ctx context.Context, phaseID int64) (*enrollmentModels.Phase, error) {
	if s.phaseRepo == nil {
		return nil, fmt.Errorf("submit: phase repo not wired")
	}
	phase, err := s.phaseRepo.FindByID(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("%w: phase %d not found", ErrInvalidSubmission, phaseID)
	}
	if !phase.IsActive {
		return nil, ErrEnrollmentDisabled
	}
	return phase, nil
}

// resolveSubmissionSchema returns the phase's pinned form schema, or
// nil when the phase is "Basis" (form_schema_id IS NULL). Returning
// nil writes the request with a NULL schema_id, which is allowed
// since migration 1.15.69. We deliberately do NOT fall back to the
// tenant's currently-active schema for Basis phases - the admin UI
// promises "nur die Standardfelder", so silently inheriting the
// latest custom schema would leak its fields into every Basis phase.
func (s *requestService) resolveSubmissionSchema(ctx context.Context, phase *enrollmentModels.Phase) (*enrollmentModels.FormSchema, error) {
	if phase.FormSchemaID == nil {
		return nil, nil
	}
	schema, err := s.formSchemaRepo.FindByID(ctx, *phase.FormSchemaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Pinned schema was deleted out from under the phase.
			// Treat as Basis rather than 500 - submission still
			// succeeds with NULL schema_id and the admin can repin.
			s.logger.Warn("phase form_schema_id pointed at missing schema; submitting as Basis",
				slog.Int64("phase_id", phase.ID),
				slog.Int64("form_schema_id", *phase.FormSchemaID))
			return nil, nil
		}
		return nil, err
	}
	return schema, nil
}

func newStatusToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *requestService) statusURL(token string) string {
	// Status link is parent-facing - sent in the submitted/approved/
	// waitlisted/rejected emails. Routes to the parents portal.
	host := s.parentsURL
	if host == "" {
		host = "http://localhost:3000"
	}
	return fmt.Sprintf("%s/enroll/status/%s", host, token)
}

// IsEnrollmentEnabled is the public counterpart of isEnrollmentEnabled
// so HTTP handlers can gate their form-load endpoints without reaching
// into the settings package directly.
func (s *requestService) IsEnrollmentEnabled(ctx context.Context) bool {
	return s.isEnrollmentEnabled(ctx)
}

// fnvHash64 returns a 64-bit FNV-1a hash of the input. Used to derive
// a stable advisory-lock key from the lowercased guardian email.
func fnvHash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func (s *requestService) isEnrollmentEnabled(ctx context.Context) bool {
	if s.settings == nil {
		return false
	}
	if has, err := s.settings.HasTenantOverride(ctx, configModel.KeyEnrollmentEnabled); err == nil && has {
		v, err := s.settings.ResolveBool(ctx, configModel.KeyEnrollmentEnabled)
		if err == nil {
			return v
		}
	}
	return false
}

func (s *requestService) resolveGradeMax(ctx context.Context) int {
	if s.settings == nil {
		return 4
	}
	if has, err := s.settings.HasTenantOverride(ctx, configModel.KeyEnrollmentGradeLevelMax); err == nil && has {
		if v, err := s.settings.ResolveInt(ctx, configModel.KeyEnrollmentGradeLevelMax); err == nil && v > 0 {
			return v
		}
	}
	return 4
}

func (s *requestService) allowSubmissionEdit(ctx context.Context) bool {
	if s.settings == nil {
		return true
	}
	if has, err := s.settings.HasTenantOverride(ctx, configModel.KeyEnrollmentAllowSubmissionEdit); err == nil && has {
		v, err := s.settings.ResolveBool(ctx, configModel.KeyEnrollmentAllowSubmissionEdit)
		if err == nil {
			return v
		}
	}
	return true
}

func (s *requestService) resolveStatusTokenExpiry(ctx context.Context) time.Duration {
	const defaultDays = 365
	if s.settings == nil {
		return time.Duration(defaultDays) * 24 * time.Hour
	}
	if has, err := s.settings.HasTenantOverride(ctx, configModel.KeyEnrollmentStatusTokenTTLDays); err == nil && has {
		if v, err := s.settings.ResolveInt(ctx, configModel.KeyEnrollmentStatusTokenTTLDays); err == nil && v > 0 {
			return time.Duration(v) * 24 * time.Hour
		}
	}
	return time.Duration(defaultDays) * 24 * time.Hour
}

func (s *requestService) resolveSettingString(ctx context.Context, key, fallback string) string {
	if has, err := s.settings.HasTenantOverride(ctx, key); err == nil && has {
		if v, err := s.settings.ResolveString(ctx, key); err == nil && v != "" {
			return v
		}
	}
	return fallback
}

// resolveAdminEmails parses the comma-separated notification_emails
// setting and returns a list of valid email addresses. Invalid entries
// are silently dropped - admins shouldn't receive errors because of a
// trailing comma in their config.
func (s *requestService) resolveAdminEmails(ctx context.Context) []string {
	csv := s.resolveSettingString(ctx, configModel.KeyEnrollmentNotificationEmails, "")
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if _, err := mail.ParseAddress(trimmed); err != nil {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func (s *requestService) lookupSchoolName(ctx context.Context, tenantID int64) string {
	if s.schoolRepo == nil || tenantID == 0 {
		return ""
	}
	school, err := s.schoolRepo.FindByID(ctx, tenantID)
	if err != nil || school == nil || school.IsDeleted() {
		return ""
	}
	return school.Name
}

// applyCapacityOverflow inspects each selected offering's capacity vs
// current claimants. Per the phase's care_overflow_mode it either
// rejects the whole submission, returns per-child status overrides
// (waitlist), or lets everything pass through unchanged (allow).
//
// The map[childIndex]status return is intentionally sparse - entries
// only appear for children that need a non-default status. Callers
// fall back to ChildStatusSubmitted when the index isn't in the map.
//
// Counting model: a child counts when EITHER it already has a
// non-terminal status in the DB, OR it appears earlier in the same
// submission selecting the same offering. The earlier-in-submission
// case prevents one large family from sneaking past a tight capacity
// because the per-offering DB count was taken once at the top of the
// loop.
func (s *requestService) applyCapacityOverflow(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	children []SubmitChild,
	openByID map[int64]*enrollmentModels.CareOffering,
) (map[int]string, error) {
	overrides := make(map[int]string)
	if s.requestChildOfferingRepo == nil || len(children) == 0 {
		return overrides, nil
	}

	mode := phase.CareOverflowMode
	if mode == "" {
		mode = enrollmentModels.PhaseCareOverflowWaitlist
	}

	// Cache per-offering current count + capacity. Avoid hitting the DB
	// once per (child, offering) pair when one offering is shared.
	type slot struct {
		capacity *int // nil = unlimited
		current  int  // pre-existing claimants (DB)
		queued   int  // count from earlier children in this submission
	}
	slots := make(map[int64]*slot)

	getSlot := func(offeringID int64) (*slot, error) {
		if cached, ok := slots[offeringID]; ok {
			return cached, nil
		}
		offering, ok := openByID[offeringID]
		if !ok {
			// Should be impossible (validateOfferingSelections ran first).
			return nil, fmt.Errorf("submit: offering %d not in open catalog", offeringID)
		}
		count, err := s.requestChildOfferingRepo.CountActiveByCareOffering(ctx, offeringID)
		if err != nil {
			return nil, fmt.Errorf("submit: count offering %d: %w", offeringID, err)
		}
		s := &slot{capacity: offering.Capacity, current: count}
		slots[offeringID] = s
		return s, nil
	}

	for childIdx, child := range children {
		childOver := false
		for _, offeringID := range child.OfferingIDs {
			sl, err := getSlot(offeringID)
			if err != nil {
				return nil, err
			}
			if sl.capacity == nil {
				sl.queued++
				continue
			}
			if sl.current+sl.queued+1 > *sl.capacity {
				childOver = true
				if mode == enrollmentModels.PhaseCareOverflowReject {
					return nil, fmt.Errorf("%w: offering %d", ErrCareOfferingFull, offeringID)
				}
			}
			sl.queued++
		}
		if childOver && mode == enrollmentModels.PhaseCareOverflowWaitlist {
			overrides[childIdx] = enrollmentModels.ChildStatusWaitlisted
		}
	}

	return overrides, nil
}

// enforceRateLimit increments the per-IP and per-email buckets and
// returns ErrRateLimited as soon as either crosses its threshold. The
// repository is optional - when not wired, this is a no-op (mainly for
// tests that don't care about rate limiting). Tenant-scoped: each
// school owns its own counters.
func (s *requestService) enforceRateLimit(ctx context.Context, req SubmitRequest) error {
	if s.rateLimitRepo == nil || req.TenantID <= 0 {
		return nil
	}

	ip := strings.TrimSpace(req.RemoteIP)
	email := strings.ToLower(strings.TrimSpace(req.GuardianEmail))

	if ip != "" {
		state, err := s.rateLimitRepo.IncrementAttempts(ctx, req.TenantID, enrollmentModels.SubmissionRateLimitKeyTypeIP, ip, rateLimitWindowIP)
		if err != nil {
			s.logger.Warn("enrollment submit: rate-limit IP increment failed; allowing through",
				slog.String("error", err.Error()))
		} else if state.Attempts > rateLimitMaxAttemptsIP {
			s.logger.Info("enrollment submit rate-limited",
				slog.String("key_type", enrollmentModels.SubmissionRateLimitKeyTypeIP),
				slog.Int("attempts", state.Attempts),
				slog.Int64("tenant_id", req.TenantID))
			return ErrRateLimited
		}
	}

	if email != "" {
		state, err := s.rateLimitRepo.IncrementAttempts(ctx, req.TenantID, enrollmentModels.SubmissionRateLimitKeyTypeEmail, email, rateLimitWindowEmail)
		if err != nil {
			s.logger.Warn("enrollment submit: rate-limit email increment failed; allowing through",
				slog.String("error", err.Error()))
		} else if state.Attempts > rateLimitMaxAttemptsMail {
			s.logger.Info("enrollment submit rate-limited",
				slog.String("key_type", enrollmentModels.SubmissionRateLimitKeyTypeEmail),
				slog.Int("attempts", state.Attempts),
				slog.Int64("tenant_id", req.TenantID))
			return ErrRateLimited
		}
	}

	return nil
}
