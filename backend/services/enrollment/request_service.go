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
	"github.com/moto-nrw/project-phoenix/models/users"
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
	// ErrCareOfferingMissing is returned when a phase requires at least
	// one care offering per child but a child has no offering selected.
	// Mapped to 400 with a stable code so the parent form can highlight
	// the right child.
	ErrCareOfferingMissing = errors.New("care offering selection is required for every child")
	// ErrCareOfferingExactlyOneRequired is returned when a phase requires
	// exactly one care offering per child but a child selected none or
	// more than one.
	ErrCareOfferingExactlyOneRequired = errors.New("exactly one care offering must be selected for every child")
	// ErrRequiredCareOfferingMissing is returned when a care offering
	// flagged is_required is not selected for one of the children. Unlike
	// ErrCareOfferingMissing (the phase-wide "at least one" gate), this
	// targets a specific mandatory offering. Mapped to 400 with a stable
	// code so the parent form can highlight the right child.
	ErrRequiredCareOfferingMissing = errors.New("a required care offering was not selected for every child")
	// ErrCareOfferingRule wraps ErrInvalidSubmission (so the HTTP layer
	// maps it to 400) and is returned when a child's offering selection
	// violates a group's selection rule (exactly_one / at_least_one /
	// at_most_one). Defense-in-depth: the parent form enforces the same.
	ErrCareOfferingRule     = fmt.Errorf("%w: care offering selection rule not satisfied", ErrInvalidSubmission)
	ErrRateLimited          = errors.New("too many submission attempts; please retry later")
	ErrRequestNotFound      = errors.New("enrollment request not found")
	ErrInvalidGuardianPhone = errors.New("guardian phone number has an invalid format")
	// ErrInvalidGuardianEmail wraps ErrInvalidSubmission so callers that match
	// the broad category keep working, while the HTTP layer maps the specific
	// case to a stable code (enrollment.invalid_email) for per-field marking.
	ErrInvalidGuardianEmail = fmt.Errorf("%w: guardian email has an invalid format", ErrInvalidSubmission)
	ErrEditNotAllowed       = errors.New("request can no longer be edited")
	ErrWithdrawNotAllowed   = errors.New("child cannot be withdrawn in its current state")
	ErrDuplicateEnrollment  = errors.New("an active enrollment already exists for this parent and child in this phase")
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
//
// OfferingDays is the optional per-offering day-selection refinement
// for offerings whose days_of_week_mode is "parent_choice". Entries
// in OfferingDays MUST also appear in OfferingIDs; missing entries
// inherit the offering's default (admin-fixed) day set, written as
// NULL on the resulting request_child_offerings row. The service
// validates subset/non-empty before inserting.
type SubmitChild struct {
	FirstName        string
	LastName         string
	DateOfBirth      time.Time
	TargetGradeLevel *int16
	CustomData       map[string]any
	OfferingIDs      []int64
	OfferingDays     []SubmitOfferingDays
}

// SubmitOfferingDays is one row of SubmitChild.OfferingDays.
type SubmitOfferingDays struct {
	OfferingID   int64
	SelectedDays []string
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

	// ConfirmRenewal transitions every pending_renewal child under
	// this request to submitted, so the admin's regular review queue
	// picks it up. Used by the parent-facing "Anmeldung bestätigen"
	// button in opt-in rollover mode. No-op when the request has no
	// pending_renewal rows (idempotent on double-clicks).
	ConfirmRenewal(ctx context.Context, token string) (int, error)
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
	if err := validateOfferingGroupRules(req.Children, openByID); err != nil {
		return nil, err
	}
	if err := validateRequiredOfferings(req.Children, openByID); err != nil {
		return nil, err
	}
	if err := validateCareOfferingSelectionMode(req.Children, openByID, phase.CareOfferingSelectionMode); err != nil {
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
	if schema != nil {
		if err := validateSubmissionAgainstSchema(req, schema); err != nil {
			return nil, err
		}
	}

	// Defense-in-depth: enforce required custom fields server-side. The
	// client does the same, but a stale or scripted submit must not be
	// able to skip a visible required field. Hidden (conditional) fields
	// are exempt.
	if err := s.validateRequiredCustomFields(schema, req, openByID); err != nil {
		return nil, err
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

			// Index the parent's per-offering day picks (if any) by
			// offering id so we can resolve each offering link in O(1).
			daysByOffering := make(map[int64][]string, len(child.OfferingDays))
			for _, row := range child.OfferingDays {
				daysByOffering[row.OfferingID] = row.SelectedDays
			}

			for _, offeringID := range child.OfferingIDs {
				offering, ok := openByID[offeringID]
				if !ok {
					return fmt.Errorf("submit: offering %d disappeared mid-submit", offeringID)
				}
				selected, err := resolveSelectedDays(offering, daysByOffering[offeringID])
				if err != nil {
					return fmt.Errorf("submit: offering %d: %w", offeringID, err)
				}
				link := &enrollmentModels.RequestChildOffering{
					RequestChildID: row.ID,
					CareOfferingID: offeringID,
					SelectedDays:   selected,
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
	// Validate against the shared canonical pattern (same rule student
	// creation enforces at approval) so a submittable email can never get
	// stuck at the approval step. mail.ParseAddress was too lenient here
	// (it accepts e.g. "a@b" / "test@localhost" that approval rejects).
	if err := users.ValidateOptionalEmail(emailAddr); err != nil {
		return ErrInvalidGuardianEmail
	}
	// Guardian phone is optional, but when present it must match the same
	// canonical format student creation enforces on approval. Validating
	// here stops an invalid value (e.g. "12345") from being stored and
	// then permanently blocking approval with a 500.
	if req.GuardianPhone != nil {
		if err := users.ValidateOptionalPhone(*req.GuardianPhone); err != nil {
			return ErrInvalidGuardianPhone
		}
	}
	for _, key := range enrollmentModels.RequiredConsentKeys {
		accepted, ok := req.ConsentFlags[key].(bool)
		if !ok || !accepted {
			return fmt.Errorf("%w: consent %s is required", ErrInvalidSubmission, key)
		}
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
		if child.TargetGradeLevel == nil {
			return fmt.Errorf("%w: child %d missing target_grade_level", ErrInvalidSubmission, i)
		}
		if *child.TargetGradeLevel < 1 || int(*child.TargetGradeLevel) > gradeMax {
			return fmt.Errorf("%w: child %d grade out of range 1..%d", ErrInvalidSubmission, i, gradeMax)
		}
	}
	return nil
}

func validateSubmissionAgainstSchema(req SubmitRequest, schema *enrollmentModels.FormSchema) error {
	if schema.CoreRequirements.Required(enrollmentModels.CoreRequirementGuardianPhone) {
		if req.GuardianPhone == nil || strings.TrimSpace(*req.GuardianPhone) == "" {
			return fmt.Errorf("%w: guardian phone is required", ErrInvalidSubmission)
		}
	}
	for _, field := range schema.Fields {
		if err := validateRequiredCustomField(field, req); err != nil {
			return err
		}
	}
	return nil
}

// validateRequiredCustomField enforces a single schema field's required
// constraint against the submission. Per-child fields are checked for every
// child; request-level fields once. No-op for optional fields.
func validateRequiredCustomField(field enrollmentModels.FormField, req SubmitRequest) error {
	if !field.Required {
		return nil
	}
	if field.AppliesToCh {
		for i, child := range req.Children {
			if !customValueSatisfiesRequired(field, child.CustomData[field.Key]) {
				return fmt.Errorf("%w: child %d field %s is required", ErrInvalidSubmission, i, field.Key)
			}
		}
		return nil
	}
	if !customValueSatisfiesRequired(field, req.CustomData[field.Key]) {
		return fmt.Errorf("%w: field %s is required", ErrInvalidSubmission, field.Key)
	}
	return nil
}

func customValueSatisfiesRequired(field enrollmentModels.FormField, value any) bool {
	switch field.Type {
	case enrollmentModels.FormFieldBoolean:
		_, ok := value.(bool)
		return ok
	case enrollmentModels.FormFieldPhoneList:
		return phoneListSatisfiesRequired(value)
	case enrollmentModels.FormFieldContactList:
		return contactListSatisfiesRequired(value)
	case enrollmentModels.FormFieldWeekdaySchedule:
		return scheduleHasAnyTime(value)
	case enrollmentModels.FormFieldNumber:
		return numberValueSatisfiesRequired(value)
	default:
		return stringValue(value) != ""
	}
}

func phoneListSatisfiesRequired(value any) bool {
	var entries []enrollmentModels.PhoneEntry
	if err := decodeStructured(value, &entries); err != nil || len(entries) == 0 {
		return false
	}
	for i := range entries {
		if err := entries[i].Validate(); err != nil {
			return false
		}
	}
	return true
}

func contactListSatisfiesRequired(value any) bool {
	var entries []enrollmentModels.ContactEntry
	if err := decodeStructured(value, &entries); err != nil || len(entries) == 0 {
		return false
	}
	for i := range entries {
		if err := entries[i].Validate(); err != nil {
			return false
		}
	}
	return true
}

func numberValueSatisfiesRequired(value any) bool {
	switch v := value.(type) {
	case float64, int:
		return true
	case string:
		return strings.TrimSpace(v) != ""
	default:
		return false
	}
}

func scheduleHasAnyTime(value any) bool {
	raw, ok := value.(map[string]any)
	if !ok {
		if typed, ok := value.(map[string]string); ok {
			for _, v := range typed {
				if strings.TrimSpace(v) != "" {
					return true
				}
			}
		}
		return false
	}
	for _, v := range raw {
		if str, ok := v.(string); ok && strings.TrimSpace(str) != "" {
			return true
		}
	}
	return false
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

// validateRequiredOfferings enforces that every offering flagged
// is_required in the phase's open catalog is selected by every child.
// The day-level requirement for parent_choice offerings is already
// enforced at insert time by resolveSelectedDays, so this only checks
// presence in child.OfferingIDs.
func validateRequiredOfferings(children []SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) error {
	requiredIDs := make([]int64, 0)
	for id, offering := range openByID {
		if offering.IsRequired {
			requiredIDs = append(requiredIDs, id)
		}
	}
	if len(requiredIDs) == 0 {
		return nil
	}
	for i, child := range children {
		selected := make(map[int64]bool, len(child.OfferingIDs))
		for _, id := range child.OfferingIDs {
			selected[id] = true
		}
		for _, requiredID := range requiredIDs {
			if !selected[requiredID] {
				return fmt.Errorf("%w: child %d offering %d", ErrRequiredCareOfferingMissing, i, requiredID)
			}
		}
	}
	return nil
}

// validateCareOfferingSelectionMode enforces the phase's selection mode
// over the *choosable* (non-required) offerings only. Required offerings
// are always-on and orthogonal to the mode: they are forced by
// validateRequiredOfferings and must not count toward "at least one" /
// "exactly one". Counting them would make exactly_one impossible whenever
// the phase also carries a required base offering (e.g. a mandatory care
// package plus a choose-one-time-slot rule): the contradiction this
// function is designed to avoid.
func validateCareOfferingSelectionMode(children []SubmitChild, openByID map[int64]*enrollmentModels.CareOffering, mode string) error {
	if mode == "" || mode == enrollmentModels.PhaseCareOfferingSelectionOptional {
		return nil
	}
	if mode != enrollmentModels.PhaseCareOfferingSelectionAtLeastOne &&
		mode != enrollmentModels.PhaseCareOfferingSelectionExactlyOne {
		return fmt.Errorf("%w: invalid care offering selection mode %q", ErrInvalidSubmission, mode)
	}

	for i, child := range children {
		selected := make(map[int64]bool, len(child.OfferingIDs))
		for _, id := range child.OfferingIDs {
			selected[id] = true
		}
		for _, dayPick := range child.OfferingDays {
			if !selected[dayPick.OfferingID] {
				return fmt.Errorf("%w: child %d offering %d has days but is not selected", ErrInvalidSubmission, i, dayPick.OfferingID)
			}
		}
		// Count only the choosable (non-required) selected offerings. An
		// offering absent from openByID is rejected earlier by
		// validateOfferingSelections, so treat unknown ids as choosable.
		choosableCount := 0
		for _, id := range child.OfferingIDs {
			if o, ok := openByID[id]; !ok || !o.IsRequired {
				choosableCount++
			}
		}
		switch mode {
		case enrollmentModels.PhaseCareOfferingSelectionAtLeastOne:
			if choosableCount == 0 {
				return fmt.Errorf("%w: child %d", ErrCareOfferingMissing, i)
			}
		case enrollmentModels.PhaseCareOfferingSelectionExactlyOne:
			if choosableCount != 1 {
				return fmt.Errorf("%w: child %d", ErrCareOfferingExactlyOneRequired, i)
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

	// status_reason is admin-internal free text. It may only reach a
	// parent when the request's phase opts in via
	// show_status_reason_to_parent. Strip it otherwise so the public
	// status page (and the parents-portal status view that shares this
	// endpoint) never expose an internal rejection/waitlist note. The
	// decision service applies the same gate to the parent email.
	if !s.statusReasonVisibleToParent(tenantCtx, req.PhaseID) {
		for _, c := range children {
			c.StatusReason = nil
		}
	}
	return req, children, nil
}

// statusReasonVisibleToParent reports whether the given phase allows a
// per-child status_reason to be surfaced to the parent. Fail-closed: if
// the phase can't be loaded it returns false, so an internal note is
// redacted rather than risk leaking when the setting can't be confirmed.
func (s *requestService) statusReasonVisibleToParent(ctx context.Context, phaseID int64) bool {
	phase, err := s.phaseRepo.FindByID(ctx, phaseID)
	if err != nil {
		s.logger.Warn("status: phase load failed; redacting status reason",
			slog.Int64("phase_id", phaseID),
			slog.String("error", err.Error()),
		)
		return false
	}
	return phase.ShowStatusReasonToParent
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
		// Same canonical phone check as submit — an admin/parent edit must
		// not be able to reintroduce a value approval would later reject.
		if err := users.ValidateOptionalPhone(v); err != nil {
			return ErrInvalidGuardianPhone
		}
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

// ConfirmRenewal transitions every pending_renewal row under the
// request token to submitted. Idempotent — rows that are no longer
// pending_renewal are skipped, so a parent who double-clicks doesn't
// get an error.
func (s *requestService) ConfirmRenewal(ctx context.Context, token string) (int, error) {
	req, children, err := s.GetByStatusToken(ctx, token)
	if err != nil {
		return 0, err
	}
	tenantID := req.GetTenantID()
	tenantCtx := tenant.WithTenantID(ctx, tenantID)

	var confirmed int
	txErr := tenant.WithTenantTx(tenantCtx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		for _, c := range children {
			if c.Status != enrollmentModels.ChildStatusPendingRenewal {
				continue
			}
			if err := s.requestChildRepo.UpdateStatus(txCtx, c.ID, enrollmentModels.ChildStatusSubmitted, nil, 0); err != nil {
				return err
			}
			confirmed++
		}
		return nil
	})
	if txErr != nil {
		return 0, txErr
	}
	if confirmed > 0 {
		s.logger.Info("renewal confirmed by parent",
			slog.Int64("request_id", req.ID),
			slog.Int("children_confirmed", confirmed),
		)
	}
	return confirmed, nil
}

// enqueueSubmissionEmails fires off the parent confirmation + admin
// notifications. Best-effort - failures log but don't fail the
// submission (the rows are already committed).
func (s *requestService) enqueueSubmissionEmails(ctx context.Context, tenantID int64, request *enrollmentModels.Request, children []*enrollmentModels.RequestChild, statusURL string) {
	if s.outboxEnqueuer == nil {
		return
	}

	schoolName, logoURL := emailBrandForSchool(ctx, s.schoolRepo, tenantID, s.parentsURL)
	footerLogoURL := motoLogoURL(s.parentsURL)
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
		EnrollmentPayloadMotoLogoURL:       footerLogoURL,
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
			EnrollmentPayloadMotoLogoURL:       footerLogoURL,
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

// resolveGradeMax reads the tenant setting and falls back to the current
// registry default when unset or unreadable.
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

// resolveSelectedDays computes the SelectedDays value for a
// request_child_offerings row given the offering's days_of_week_mode
// + available_days and any parent-supplied day picks.
//
//   - fixed offerings: parent picks are ignored and the row stores
//     NULL — semantics "use the offering's current available_days".
//     If the admin later changes the offering's day set, the link
//     reflects the new value automatically. Sending parent picks for
//     a fixed offering is a 400, not a silent overwrite.
//   - parent_choice offerings: picks must be a non-empty subset of
//     the offering's available_days. Missing picks → 400; subset
//     violation → 400.
func resolveSelectedDays(offering *enrollmentModels.CareOffering, picks []string) ([]string, error) {
	switch offering.DaysOfWeekMode {
	case enrollmentModels.DaysOfWeekModeFixed:
		if len(picks) > 0 {
			return nil, fmt.Errorf("offering does not allow parent day selection (days_of_week_mode=fixed)")
		}
		return nil, nil
	case enrollmentModels.DaysOfWeekModeParentChoice:
		if len(picks) == 0 {
			return nil, fmt.Errorf("offering requires the parent to pick at least one day")
		}
		allowed := make(map[string]bool, len(offering.AvailableDays))
		for _, d := range offering.AvailableDays {
			allowed[d] = true
		}
		seen := make(map[string]bool, len(picks))
		dedup := make([]string, 0, len(picks))
		for _, d := range picks {
			if !allowed[d] {
				return nil, fmt.Errorf("day %q is not in the offering's available_days", d)
			}
			if seen[d] {
				continue
			}
			seen[d] = true
			dedup = append(dedup, d)
		}
		return dedup, nil
	default:
		return nil, fmt.Errorf("offering has unknown days_of_week_mode %q", offering.DaysOfWeekMode)
	}
}
