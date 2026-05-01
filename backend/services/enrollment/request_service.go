package enrollment

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
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
	ErrRequestNotFound        = errors.New("enrollment request not found")
	ErrEditNotAllowed         = errors.New("request can no longer be edited")
	ErrWithdrawNotAllowed     = errors.New("child cannot be withdrawn in its current state")
)

// SubmitRequest is the data the public submission handler hands to the
// service. PR 7's HTTP layer translates the JSON wire shape into this
// struct.
type SubmitRequest struct {
	TenantID         int64
	CalendarPeriodID int64

	GuardianFirstName string
	GuardianLastName  string
	GuardianEmail     string
	GuardianPhone     *string
	ConsentFlags      map[string]any
	CustomData        map[string]any

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
	SchoolRepo               platformModels.SchoolRepository
	OutboxEnqueuer           OutboxEnqueuer
	Settings                 RequestSettingsResolver
	FrontendURL              string
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
	schoolRepo               platformModels.SchoolRepository
	outboxEnqueuer           OutboxEnqueuer
	settings                 RequestSettingsResolver
	frontendURL              string
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
		schoolRepo:               cfg.SchoolRepo,
		outboxEnqueuer:           cfg.OutboxEnqueuer,
		settings:                 cfg.Settings,
		frontendURL:              strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/"),
		db:                       cfg.DB,
		txHandler:                modelBase.NewTxHandler(cfg.DB),
		logger:                   logger,
	}
}

// Submit is the workhorse. One DB transaction writes the request, all
// child rows, all care-offering selections, and enqueues the two
// confirmation emails. Either everything lands or nothing does.
func (s *requestService) Submit(ctx context.Context, req SubmitRequest) (*SubmitResult, error) {
	if err := s.validateSubmission(ctx, req); err != nil {
		return nil, err
	}

	now := time.Now()
	openOfferings, err := s.careOfferingRepo.ListPublicOpenWindow(ctx, now)
	if err != nil {
		return nil, fmt.Errorf("submit: load open offerings: %w", err)
	}
	openByID := make(map[int64]*enrollmentModels.CareOffering, len(openOfferings))
	for _, o := range openOfferings {
		openByID[o.ID] = o
	}
	if err := validateOfferingSelections(req.Children, openByID); err != nil {
		return nil, err
	}

	schema, err := s.formSchemaRepo.FindActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("submit: load active schema: %w", err)
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
	txErr := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		request := &enrollmentModels.Request{
			SchemaID:           schema.ID,
			CalendarPeriodID:   req.CalendarPeriodID,
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
			row := &enrollmentModels.RequestChild{
				RequestID:        request.ID,
				FirstName:        strings.TrimSpace(child.FirstName),
				LastName:         strings.TrimSpace(child.LastName),
				DateOfBirth:      child.DateOfBirth,
				TargetGradeLevel: child.TargetGradeLevel,
				CustomData:       child.CustomData,
				Status:           enrollmentModels.ChildStatusSubmitted,
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
	if !s.isWithinOpenWindow(ctx, time.Now()) {
		return ErrEnrollmentWindowClosed
	}
	if req.CalendarPeriodID <= 0 {
		return fmt.Errorf("%w: calendar_period_id is required", ErrInvalidSubmission)
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
// (token-only auth — RLS would block unprivileged SELECTs).
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
// admin (terminal student records exist) — returns ErrWithdrawNotAllowed.
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
// notifications. Best-effort — failures log but don't fail the
// submission (the rows are already committed).
func (s *requestService) enqueueSubmissionEmails(ctx context.Context, tenantID int64, request *enrollmentModels.Request, children []*enrollmentModels.RequestChild, statusURL string) {
	if s.outboxEnqueuer == nil {
		return
	}

	schoolName := s.lookupSchoolName(ctx, tenantID)
	logoURL := s.frontendURL + "/images/moto_transparent.png"
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

func newStatusToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *requestService) statusURL(token string) string {
	frontend := s.frontendURL
	if frontend == "" {
		frontend = "http://localhost:3000"
	}
	return fmt.Sprintf("%s/enroll/status/%s", frontend, token)
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

func (s *requestService) isWithinOpenWindow(ctx context.Context, now time.Time) bool {
	if s.settings == nil {
		return true
	}
	startStr := s.resolveSettingString(ctx, configModel.KeyEnrollmentOpenWindowStart, "")
	endStr := s.resolveSettingString(ctx, configModel.KeyEnrollmentOpenWindowEnd, "")
	if startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil && now.Before(t) {
			return false
		}
	}
	if endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil && now.After(t.AddDate(0, 0, 1)) {
			// End-of-day inclusive: treat the configured date as
			// "accept until end-of-day on this date".
			return false
		}
	}
	return true
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
// are silently dropped — admins shouldn't receive errors because of a
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
