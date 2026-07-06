package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Guardian invitation operation names, used in AuthError wrapping for callers
// that match on Op.
const (
	opGuardianInviteCreate   = "create guardian invitation"
	opGuardianInviteValidate = "validate guardian invitation"
	opGuardianInviteAccept   = "accept guardian invitation"
	opGuardianInviteResend   = "resend guardian invitation"
	opGuardianInviteFetch    = "fetch guardian invitation"
)

// guardianRoleBaseName is the name used to look up the system "guardian" role
// row in auth.roles. The role is created by migration 1.7.4 with base_role
// 'guardian'. The accept flow assigns this role to every new guardian account.
const guardianRoleBaseName = "guardian"

// guardianTokenExpiryFallback is used when neither the registry setting nor
// the env var are set. 48 hours matches the staff invitation default and the
// `invitations.guardian_token_expiry_hours` registry default.
const guardianTokenExpiryFallback = 48 * time.Hour

// guardianTokenEnvVar is the legacy env-var fallback path. New deployments
// should configure the registry setting per tenant; the env var stays for
// dev parity.
const guardianTokenEnvVar = "GUARDIAN_INVITATION_TOKEN_EXPIRY_HOURS"

const schoolLogoURLKey = "logoUrl"
const schoolLoginImageURLKey = "loginImageUrl"

// OutboxEnqueuer is the narrow contract the guardian invitation service
// needs from the platform outbox. Defined here so services/auth doesn't
// import services/platform.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, req OutboxEnqueueRequest) error
}

// OutboxEnqueueRequest mirrors platform.EnqueueRequest. The platform
// package owns the canonical type; we redeclare a parallel one here to
// avoid an import cycle.
type OutboxEnqueueRequest struct {
	Kind              string
	Payload           map[string]any
	RelatedEntityType string
	RelatedEntityID   int64
}

// GuardianInvitationServiceConfig is the dependency injection bundle for
// NewGuardianInvitationService. All fields except SettingsResolver,
// Dispatcher, and OutboxEnqueuer are required.
//
// OutboxEnqueuer (PR 5+) replaces the legacy Dispatcher path. When set,
// invitation emails are written to platform.email_outbox and the worker
// dispatches them. When nil, falls back to direct dispatch via
// Dispatcher (kept for tests that don't wire the full outbox stack).
// Production wiring always sets the enqueuer.
type GuardianInvitationServiceConfig struct {
	InvitationRepo      authModels.GuardianInvitationRepository
	AccountRepo         authModels.AccountRepository
	AccountTenantRepo   authModels.AccountTenantRepository
	AccountRoleRepo     authModels.AccountRoleRepository
	RoleRepo            authModels.RoleRepository
	PersonRepo          userModels.PersonRepository
	GuardianProfileRepo userModels.GuardianProfileRepository
	StudentGuardianRepo userModels.StudentGuardianRepository
	StudentRepo         userModels.StudentRepository
	SchoolRepo          platformModels.SchoolRepository
	Mailer              email.Mailer
	Dispatcher          *email.Dispatcher
	OutboxEnqueuer      OutboxEnqueuer
	// EnrollmentBackfiller stamps guardian_account_id onto every
	// pre-account enrollment.requests row matching the guardian's
	// email, so requests submitted before invite acceptance show up in
	// /parent/me/enrollments. nil-safe: the accept flow runs even when
	// the dependency isn't wired (tests, narrow integrations).
	EnrollmentBackfiller EnrollmentBackfiller
	SettingsResolver     GuardianSettingsResolver
	FrontendURL          string
	DefaultFrom          email.Email
	FallbackExpiry       time.Duration
	DB                   *bun.DB
	Logger               *slog.Logger
}

// EnrollmentBackfiller is the narrow contract the accept flow needs
// from the parent enrollment repo. Defined here so this package
// doesn't take a hard dependency on models/parent. Any repo that
// satisfies this method shape works.
type EnrollmentBackfiller interface {
	BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error)
}

type guardianInvitationService struct {
	GuardianInvitationServiceConfig
	txHandler *modelBase.TxHandler
}

// NewGuardianInvitationService builds a guardian invitation service. The
// dispatcher is auto-derived from the mailer when not provided. A nil logger
// falls back to slog.Default().
func NewGuardianInvitationService(cfg GuardianInvitationServiceConfig) GuardianInvitationService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Dispatcher == nil && cfg.Mailer != nil {
		cfg.Dispatcher = email.NewDispatcher(cfg.Mailer, cfg.Logger.With("component", "email"))
	}
	if cfg.FallbackExpiry <= 0 {
		cfg.FallbackExpiry = guardianTokenExpiryFallback
	}
	cfg.FrontendURL = strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/")
	return &guardianInvitationService{
		GuardianInvitationServiceConfig: cfg,
		txHandler:                       modelBase.NewTxHandler(cfg.DB),
	}
}

func (s *guardianInvitationService) getLogger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// resolveTokenExpiry follows the documented HasTenantOverride → ResolveInt →
// env var → fallback chain. Returns a duration in hours.
func (s *guardianInvitationService) resolveTokenExpiry(ctx context.Context) time.Duration {
	hours := 0
	if s.SettingsResolver != nil {
		if has, err := s.SettingsResolver.HasTenantOverride(ctx, configModel.KeyGuardianInvitationTokenExpiryHours); err != nil {
			s.getLogger().Warn("guardian invitation: settings override check failed",
				slog.String("key", configModel.KeyGuardianInvitationTokenExpiryHours),
				slog.String("error", err.Error()),
			)
		} else if has {
			if v, err := s.SettingsResolver.ResolveInt(ctx, configModel.KeyGuardianInvitationTokenExpiryHours); err == nil && v > 0 {
				hours = v
			}
		}
	}
	if hours <= 0 {
		if env := strings.TrimSpace(os.Getenv(guardianTokenEnvVar)); env != "" {
			if parsed, err := strconv.Atoi(env); err == nil && parsed > 0 {
				hours = parsed
			}
		}
	}
	if hours <= 0 {
		return s.FallbackExpiry
	}
	return time.Duration(hours) * time.Hour
}

// Create issues a new guardian invitation row + dispatches the email. Tenant
// context must be present; the GuardianProfile must already exist.
func (s *guardianInvitationService) Create(ctx context.Context, req GuardianInvitationCreateRequest) (*authModels.GuardianInvitation, error) {
	if req.GuardianProfileID <= 0 {
		return nil, &AuthError{Op: opGuardianInviteCreate, Err: fmt.Errorf("guardian profile ID is required")}
	}
	if req.CreatedBy <= 0 {
		return nil, &AuthError{Op: opGuardianInviteCreate, Err: fmt.Errorf("created_by is required")}
	}

	profile, err := s.GuardianProfileRepo.FindByID(ctx, req.GuardianProfileID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, &AuthError{Op: opGuardianInviteCreate, Err: fmt.Errorf("guardian profile not found")}
		}
		return nil, &AuthError{Op: opGuardianInviteCreate, Err: err}
	}
	if profile == nil || profile.Email == nil || strings.TrimSpace(*profile.Email) == "" {
		return nil, &AuthError{Op: opGuardianInviteCreate, Err: fmt.Errorf("guardian has no email on file")}
	}
	if profile.HasAccount {
		return nil, &AuthError{Op: opGuardianInviteCreate, Err: fmt.Errorf("guardian already has an account")}
	}

	invitation := &authModels.GuardianInvitation{
		Token:             uuid.Must(uuid.NewV4()).String(),
		GuardianProfileID: profile.ID,
		CreatedBy:         req.CreatedBy,
		ExpiresAt:         time.Now().Add(s.resolveTokenExpiry(ctx)),
	}
	invitation.SetTenantID(tenant.FromContext(ctx))

	if err := s.InvitationRepo.Create(ctx, invitation); err != nil {
		return nil, &AuthError{Op: opGuardianInviteCreate, Err: err}
	}

	s.getLogger().Info("guardian invitation created",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("guardian_profile_id", profile.ID),
		slog.Int64("created_by", req.CreatedBy),
	)

	schoolName := s.lookupSchoolName(ctx, invitation.TenantID)
	s.enqueueEmail(ctx, invitation, profile, schoolName)

	return invitation, nil
}

// Validate returns the public-safe view of an invitation if its token is
// still usable. Public route, caller is responsible for using WithAdminTx.
func (s *guardianInvitationService) Validate(ctx context.Context, token string) (*GuardianInvitationValidation, error) {
	invitation, err := s.fetchValidInvitation(ctx, token)
	if err != nil {
		return nil, err
	}

	profile, err := s.GuardianProfileRepo.FindByID(ctx, invitation.GuardianProfileID)
	if err != nil {
		return nil, &AuthError{Op: opGuardianInviteValidate, Err: err}
	}

	result := &GuardianInvitationValidation{
		ExpiresAt: invitation.ExpiresAt,
		FirstName: strings.TrimSpace(profile.FirstName),
		LastName:  strings.TrimSpace(profile.LastName),
	}
	if profile.Email != nil {
		result.Email = strings.TrimSpace(*profile.Email)
	}
	s.applySchoolBranding(ctx, invitation.TenantID, result)
	return result, nil
}

func (s *guardianInvitationService) applySchoolBranding(ctx context.Context, tenantID int64, result *GuardianInvitationValidation) {
	if tenantID <= 0 || s.SchoolRepo == nil || result == nil {
		return
	}
	school, err := s.SchoolRepo.FindByID(ctx, tenantID)
	if err != nil || school == nil || school.IsDeleted() {
		return
	}
	result.SchoolName = strings.TrimSpace(school.Name)
	result.TenantSlug = strings.TrimSpace(school.Slug)
	result.SchoolLogoURL = schoolLogoURLFromSettings(school.Settings)
}

func schoolLogoURLFromSettings(settingsJSON string) string {
	settingsJSON = strings.TrimSpace(settingsJSON)
	if settingsJSON == "" {
		return ""
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return ""
	}
	for _, key := range []string{schoolLogoURLKey, schoolLoginImageURLKey} {
		value, ok := settings[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// Accept consumes a token, creating the auth.accounts row, account_tenants
// mapping, and guardian role assignment. Updates the GuardianProfile to
// link to the new account. Public route, caller wraps in WithAdminTx.
func (s *guardianInvitationService) Accept(ctx context.Context, token string, data GuardianInvitationAcceptData) (*authModels.Account, error) {
	if data.Password != data.ConfirmPassword {
		return nil, &AuthError{Op: opGuardianInviteAccept, Err: ErrPasswordMismatch}
	}
	if err := ValidatePasswordStrength(data.Password); err != nil {
		return nil, &AuthError{Op: opGuardianInviteAccept, Err: err}
	}

	invitation, err := s.fetchValidInvitation(ctx, token)
	if err != nil {
		return nil, err
	}

	// Reject invitations for soft-deleted schools. Mirrors staff service.
	if invitation.TenantID > 0 && s.SchoolRepo != nil {
		school, schoolErr := s.SchoolRepo.FindByIDForShare(ctx, invitation.TenantID)
		if schoolErr != nil {
			return nil, &AuthError{Op: opGuardianInviteAccept, Err: schoolErr}
		}
		if school == nil || school.IsDeleted() {
			return nil, &AuthError{Op: opGuardianInviteAccept, Err: ErrInvitationTenantDeleted}
		}
	}

	profile, err := s.GuardianProfileRepo.FindByID(ctx, invitation.GuardianProfileID)
	if err != nil {
		return nil, &AuthError{Op: opGuardianInviteAccept, Err: err}
	}
	if profile == nil || profile.Email == nil || strings.TrimSpace(*profile.Email) == "" {
		return nil, &AuthError{Op: opGuardianInviteAccept, Err: fmt.Errorf("guardian profile missing email")}
	}
	emailAddress := strings.ToLower(strings.TrimSpace(*profile.Email))

	passwordHash, err := HashPassword(data.Password)
	if err != nil {
		return nil, &AuthError{Op: opGuardianInviteAccept, Err: err}
	}

	tenantCtx := tenant.WithTenantID(ctx, invitation.TenantID)

	var account *authModels.Account
	txErr := s.txHandler.RunInTx(tenantCtx, func(txCtx context.Context, _ bun.Tx) error {
		acc, innerErr := s.createOrFindAccount(txCtx, emailAddress, passwordHash)
		if innerErr != nil {
			return innerErr
		}
		if innerErr := s.linkProfileToAccount(txCtx, profile, acc.ID, invitation.TenantID, opGuardianInviteAccept); innerErr != nil {
			return innerErr
		}
		if innerErr := s.InvitationRepo.MarkAsAccepted(txCtx, invitation.ID); innerErr != nil {
			return &AuthError{Op: opGuardianInviteAccept, Err: innerErr}
		}
		account = acc
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	s.getLogger().Info("guardian invitation accepted",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("account_id", account.ID),
		slog.Int64("guardian_profile_id", profile.ID),
	)

	// Backfill guardian_account_id on every pre-account
	// enrollment.requests row matching this email (case-insensitive,
	// cross-tenant). Best-effort, the accept itself stays committed
	// even if this fails. Runs in its own admin tx; the parent's
	// /me/enrollments query reads via guardian_account_id, so without
	// this step legacy submissions wouldn't surface in the dashboard.
	if s.EnrollmentBackfiller != nil {
		backfillErr := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
			n, err := s.EnrollmentBackfiller.BackfillGuardianAccountID(adminCtx, account.ID, emailAddress)
			if err != nil {
				return err
			}
			if n > 0 {
				s.getLogger().Info("guardian invitation accept: claimed pre-account enrollments",
					slog.Int64("account_id", account.ID),
					slog.Int("rows_claimed", n),
				)
			}
			return nil
		})
		if backfillErr != nil {
			s.getLogger().Warn("guardian invitation accept: enrollment backfill failed",
				slog.Int64("account_id", account.ID),
				slog.String("error", backfillErr.Error()),
			)
		}
	}

	return account, nil
}

// createOrFindAccount returns the existing auth.accounts row for this email
// or creates a new one.
func (s *guardianInvitationService) createOrFindAccount(ctx context.Context, emailAddress, passwordHash string) (*authModels.Account, error) {
	existing, err := s.AccountRepo.FindByEmail(ctx, emailAddress)
	if err == nil && existing != nil {
		return existing, nil
	}
	if err != nil && !isNotFoundError(err) {
		return nil, &AuthError{Op: opGuardianInviteAccept, Err: err}
	}

	account := &authModels.Account{
		Email:        emailAddress,
		Active:       true,
		PasswordHash: &passwordHash,
	}
	if err := s.AccountRepo.Create(ctx, account); err != nil {
		return nil, &AuthError{Op: opGuardianInviteAccept, Err: err}
	}
	return account, nil
}

// linkProfileToAccount writes the role assignment + account_tenants mapping +
// guardian profile linkage for this tenant. Splits out so Accept stays under
// gocognit 15.
func (s *guardianInvitationService) linkProfileToAccount(ctx context.Context, profile *userModels.GuardianProfile, accountID, tenantID int64, op string) error {
	role, err := s.RoleRepo.FindByName(ctx, guardianRoleBaseName)
	if err != nil {
		return &AuthError{Op: op, Err: fmt.Errorf("guardian role lookup failed: %w", err)}
	}
	if role == nil {
		return &AuthError{Op: op, Err: fmt.Errorf("guardian role not found")}
	}

	roleAssignment := &authModels.AccountRole{AccountID: accountID, RoleID: role.ID}
	roleAssignment.SetTenantID(tenantID)
	existingRole, err := s.AccountRoleRepo.FindByAccountAndRole(ctx, accountID, role.ID)
	if err != nil && !isNotFoundError(err) {
		return &AuthError{Op: op, Err: fmt.Errorf("find guardian role assignment: %w", err)}
	}
	if existingRole == nil {
		if err := s.AccountRoleRepo.Create(ctx, roleAssignment); err != nil {
			return &AuthError{Op: op, Err: fmt.Errorf("assign guardian role: %w", err)}
		}
	}

	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.AccountTenantRepo.EnsureActive(ctx, mapping); err != nil {
		return &AuthError{Op: op, Err: fmt.Errorf("link account to tenant: %w", err)}
	}

	if err := s.GuardianProfileRepo.LinkAccount(ctx, profile.ID, accountID); err != nil {
		return &AuthError{Op: op, Err: fmt.Errorf("link guardian profile to account: %w", err)}
	}
	return nil
}

// Resend invalidates the email-tracking columns and re-dispatches a fresh
// email. Does NOT issue a new token, same token, same expiry. If the
// invitation has expired, callers should issue a new one via Create.
func (s *guardianInvitationService) Resend(ctx context.Context, invitationID int64, actorAccountID int64) error {
	invitation, err := s.InvitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		if isNotFoundError(err) {
			return &AuthError{Op: opGuardianInviteResend, Err: ErrInvitationNotFound}
		}
		return &AuthError{Op: opGuardianInviteResend, Err: err}
	}
	if invitation.IsAccepted() {
		return &AuthError{Op: opGuardianInviteResend, Err: ErrInvitationUsed}
	}
	if GuardianInvitationExpired(invitation, time.Now()) {
		return &AuthError{Op: opGuardianInviteResend, Err: ErrInvitationExpired}
	}

	profile, err := s.GuardianProfileRepo.FindByID(ctx, invitation.GuardianProfileID)
	if err != nil {
		return &AuthError{Op: opGuardianInviteResend, Err: err}
	}

	invitation.EmailSentAt = nil
	invitation.EmailError = nil
	invitation.UpdatedAt = time.Now()
	if err := s.InvitationRepo.Update(ctx, invitation); err != nil {
		return &AuthError{Op: opGuardianInviteResend, Err: err}
	}

	s.getLogger().Info("guardian invitation resent",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("actor_account_id", actorAccountID),
	)

	schoolName := s.lookupSchoolName(ctx, invitation.TenantID)
	s.enqueueEmail(ctx, invitation, profile, schoolName)
	return nil
}

func (s *guardianInvitationService) fetchValidInvitation(ctx context.Context, token string) (*authModels.GuardianInvitation, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, &AuthError{Op: opGuardianInviteFetch, Err: ErrInvitationNotFound}
	}
	invitation, err := s.InvitationRepo.FindByToken(ctx, token)
	if err != nil {
		if isNotFoundError(err) || errors.Is(err, sql.ErrNoRows) {
			return nil, &AuthError{Op: opGuardianInviteFetch, Err: ErrInvitationNotFound}
		}
		return nil, &AuthError{Op: opGuardianInviteFetch, Err: err}
	}
	if invitation.IsAccepted() {
		return nil, &AuthError{Op: opGuardianInviteFetch, Err: ErrInvitationUsed}
	}
	if GuardianInvitationExpired(invitation, time.Now()) {
		return nil, &AuthError{Op: opGuardianInviteFetch, Err: ErrInvitationExpired}
	}
	if !guardianInvitationTokenConsumable(invitation) {
		return nil, &AuthError{Op: opGuardianInviteFetch, Err: ErrInvitationNotFound}
	}
	return invitation, nil
}

func guardianInvitationTokenConsumable(invitation *authModels.GuardianInvitation) bool {
	if invitation == nil {
		return false
	}
	switch invitation.ApprovalStatus {
	case "", authModels.GuardianInvitationApprovalNotRequired, authModels.GuardianInvitationApprovalApproved:
		return true
	default:
		return false
	}
}

// lookupSchoolName resolves the tenant display name for inclusion in the
// invitation email subject. Best-effort, empty string on failure.
func (s *guardianInvitationService) lookupSchoolName(ctx context.Context, tenantID int64) string {
	if tenantID == 0 || s.SchoolRepo == nil {
		return ""
	}
	school, err := s.SchoolRepo.FindByID(ctx, tenantID)
	if err != nil || school == nil || school.IsDeleted() {
		return ""
	}
	return school.Name
}

var guardianInvitationEmailBackoff = []time.Duration{
	time.Second,
	5 * time.Second,
	15 * time.Second,
}

// enqueueEmail is the unified send path. When an outbox enqueuer is
// wired (production via factory.go since PR 5), the email is written
// to platform.email_outbox and the worker dispatches asynchronously.
// When the enqueuer is nil (older tests, mock setups), falls back to
// the legacy direct-dispatch path through email.Dispatcher.
//
// The legacy path keeps email_sent_at / email_error / email_retry_count
// up to date on auth.guardian_invitations via persistDeliveryResult.
// The outbox path leaves those columns NULL and surfaces delivery
// status from platform.email_outbox instead. Admin UIs that want
// per-invitation delivery state should query the outbox by
// (related_entity_type='guardian_invitation', related_entity_id=invitationID).
func (s *guardianInvitationService) enqueueEmail(ctx context.Context, invitation *authModels.GuardianInvitation, profile *userModels.GuardianProfile, schoolName string) {
	if s.OutboxEnqueuer != nil {
		s.enqueueViaOutbox(ctx, invitation, profile, schoolName)
		return
	}
	s.dispatchEmail(invitation, profile, schoolName)
}

// enqueueViaOutbox writes a guardian_invitation row to the platform
// outbox. The renderer (services/auth/guardian_invitation_renderer.go)
// turns the payload into an email.Message at dispatch time.
func (s *guardianInvitationService) enqueueViaOutbox(ctx context.Context, invitation *authModels.GuardianInvitation, profile *userModels.GuardianProfile, schoolName string) {
	frontend := s.FrontendURL
	if frontend == "" {
		frontend = "http://localhost:3000"
	}
	invitationURL := fmt.Sprintf("%s/accept-guardian-invite/%s", frontend, invitation.Token)
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", frontend)
	expiryHours := int(time.Until(invitation.ExpiresAt) / time.Hour)
	if expiryHours < 1 {
		expiryHours = 1
	}

	recipientEmail := ""
	firstName := ""
	lastName := ""
	if profile != nil {
		if profile.Email != nil {
			recipientEmail = strings.TrimSpace(*profile.Email)
		}
		firstName = strings.TrimSpace(profile.FirstName)
		lastName = strings.TrimSpace(profile.LastName)
	}

	payload := map[string]any{
		guardianPayloadRecipientEmail: recipientEmail,
		guardianPayloadFirstName:      firstName,
		guardianPayloadLastName:       lastName,
		guardianPayloadInvitationURL:  invitationURL,
		guardianPayloadLogoURL:        logoURL,
		guardianPayloadSchoolName:     schoolName,
		guardianPayloadExpiryHours:    expiryHours,
	}

	if err := s.OutboxEnqueuer.Enqueue(ctx, OutboxEnqueueRequest{
		Kind:              platformModels.EmailKindGuardianInvitation,
		Payload:           payload,
		RelatedEntityType: platformModels.EmailRelatedTypeGuardianInvitation,
		RelatedEntityID:   invitation.ID,
	}); err != nil {
		s.getLogger().Error("guardian invitation: outbox enqueue failed",
			slog.Int64("invitation_id", invitation.ID),
			slog.String("error", err.Error()))
	}
}

func (s *guardianInvitationService) dispatchEmail(invitation *authModels.GuardianInvitation, profile *userModels.GuardianProfile, schoolName string) {
	if s.Dispatcher == nil {
		s.getLogger().Warn("guardian invitation: email dispatcher unavailable",
			slog.Int64("invitation_id", invitation.ID),
		)
		return
	}

	frontend := s.FrontendURL
	if frontend == "" {
		frontend = "http://localhost:3000"
	}
	invitationURL := fmt.Sprintf("%s/accept-guardian-invite/%s", frontend, invitation.Token)
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", frontend)
	expiryHours := int(time.Until(invitation.ExpiresAt) / time.Hour)
	if expiryHours < 1 {
		expiryHours = 1
	}

	subject := "Einladung zum Eltern-Portal"
	if schoolName != "" {
		subject = fmt.Sprintf("Einladung zum Eltern-Portal: %s", schoolName)
	}

	recipientEmail := ""
	if profile != nil && profile.Email != nil {
		recipientEmail = strings.TrimSpace(*profile.Email)
	}

	var firstName, lastName string
	if profile != nil {
		firstName = strings.TrimSpace(profile.FirstName)
		lastName = strings.TrimSpace(profile.LastName)
	}

	message := email.Message{
		From:     s.DefaultFrom,
		To:       email.NewEmail("", recipientEmail),
		Subject:  subject,
		Template: "guardian-invitation.html",
		Content: map[string]any{
			"InvitationURL": invitationURL,
			"FirstName":     firstName,
			"LastName":      lastName,
			"ExpiryHours":   expiryHours,
			"LogoURL":       logoURL,
			"SchoolName":    schoolName,
		},
	}

	meta := email.DeliveryMetadata{
		Type:        "guardian_invitation",
		ReferenceID: invitation.ID,
		Token:       invitation.Token,
		Recipient:   recipientEmail,
	}

	baseRetry := invitation.EmailRetryCount

	s.Dispatcher.Dispatch(context.Background(), email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: guardianInvitationEmailBackoff,
		MaxAttempts:   3,
		Callback: func(cbCtx context.Context, result email.DeliveryResult) {
			s.persistDeliveryResult(cbCtx, meta, baseRetry, result)
		},
	})
}

func (s *guardianInvitationService) persistDeliveryResult(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	retryCount := baseRetry + result.Attempt
	var sentAt *time.Time
	var errText *string

	if result.Status == email.DeliveryStatusSent {
		sentTime := result.SentAt
		sentAt = &sentTime
	} else if result.Err != nil {
		msg := strings.TrimSpace(result.Err.Error())
		errText = &msg
	}

	updateCtx := ctx
	if s.DB != nil {
		// Persist via admin tx, the dispatcher's callback runs detached from
		// the request context, so it has no tenant transaction available.
		err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
			return s.InvitationRepo.UpdateEmailStatus(adminCtx, meta.ReferenceID, sentAt, errText, retryCount)
		})
		if err != nil {
			s.getLogger().Error("guardian invitation: persist delivery failed",
				slog.Int64("invitation_id", meta.ReferenceID),
				slog.String("error", err.Error()),
			)
			return
		}
	} else {
		_ = updateCtx
	}

	if result.Final && result.Status == email.DeliveryStatusFailed {
		s.getLogger().Error("guardian invitation email permanently failed",
			slog.Int64("invitation_id", meta.ReferenceID),
			slog.String("recipient", meta.Recipient),
			slog.Any("error", result.Err),
		)
	}
}

// GetTenantSlugForToken resolves the tenant slug from a guardian invitation
// token. Best-effort; returns "" on error.
func (s *guardianInvitationService) GetTenantSlugForToken(ctx context.Context, token string) string {
	var slug string
	_ = tenant.WithAdminTx(ctx, s.DB, func(txCtx context.Context, _ bun.Tx) error {
		invitation, err := s.InvitationRepo.FindByToken(txCtx, token)
		if err != nil {
			return err
		}
		if invitation == nil {
			return nil
		}
		school, err := s.SchoolRepo.FindByID(txCtx, invitation.TenantID)
		if err != nil {
			return err
		}
		if school == nil || school.IsDeleted() {
			return nil
		}
		slug = school.Slug
		return nil
	})
	return slug
}
