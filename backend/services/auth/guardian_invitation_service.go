package auth

import (
	"context"
	"database/sql"
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

// Guardian invitation operation names — used in AuthError wrapping for callers
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

// GuardianInvitationServiceConfig is the dependency injection bundle for
// NewGuardianInvitationService. All fields except SettingsResolver and
// Dispatcher are required.
type GuardianInvitationServiceConfig struct {
	InvitationRepo      authModels.GuardianInvitationRepository
	AccountRepo         authModels.AccountRepository
	AccountTenantRepo   authModels.AccountTenantRepository
	AccountRoleRepo     authModels.AccountRoleRepository
	RoleRepo            authModels.RoleRepository
	PersonRepo          userModels.PersonRepository
	GuardianProfileRepo userModels.GuardianProfileRepository
	SchoolRepo          platformModels.SchoolRepository
	Mailer              email.Mailer
	Dispatcher          *email.Dispatcher
	SettingsResolver    GuardianSettingsResolver
	FrontendURL         string
	DefaultFrom         email.Email
	FallbackExpiry      time.Duration
	DB                  *bun.DB
	Logger              *slog.Logger
}

type guardianInvitationService struct {
	invitationRepo      authModels.GuardianInvitationRepository
	accountRepo         authModels.AccountRepository
	accountTenantRepo   authModels.AccountTenantRepository
	accountRoleRepo     authModels.AccountRoleRepository
	roleRepo            authModels.RoleRepository
	personRepo          userModels.PersonRepository
	guardianProfileRepo userModels.GuardianProfileRepository
	schoolRepo          platformModels.SchoolRepository
	dispatcher          *email.Dispatcher
	settingsResolver    GuardianSettingsResolver
	frontendURL         string
	defaultFrom         email.Email
	fallbackExpiry      time.Duration
	db                  *bun.DB
	txHandler           *modelBase.TxHandler
	logger              *slog.Logger
}

// NewGuardianInvitationService builds a guardian invitation service. The
// dispatcher is auto-derived from the mailer when not provided. A nil logger
// falls back to slog.Default().
func NewGuardianInvitationService(cfg GuardianInvitationServiceConfig) GuardianInvitationService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	dispatcher := cfg.Dispatcher
	if dispatcher == nil && cfg.Mailer != nil {
		dispatcher = email.NewDispatcher(cfg.Mailer, logger.With("component", "email"))
	}
	expiry := cfg.FallbackExpiry
	if expiry <= 0 {
		expiry = guardianTokenExpiryFallback
	}
	return &guardianInvitationService{
		invitationRepo:      cfg.InvitationRepo,
		accountRepo:         cfg.AccountRepo,
		accountTenantRepo:   cfg.AccountTenantRepo,
		accountRoleRepo:     cfg.AccountRoleRepo,
		roleRepo:            cfg.RoleRepo,
		personRepo:          cfg.PersonRepo,
		guardianProfileRepo: cfg.GuardianProfileRepo,
		schoolRepo:          cfg.SchoolRepo,
		dispatcher:          dispatcher,
		settingsResolver:    cfg.SettingsResolver,
		frontendURL:         strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/"),
		defaultFrom:         cfg.DefaultFrom,
		fallbackExpiry:      expiry,
		db:                  cfg.DB,
		txHandler:           modelBase.NewTxHandler(cfg.DB),
		logger:              logger,
	}
}

func (s *guardianInvitationService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// resolveTokenExpiry follows the documented HasTenantOverride → ResolveInt →
// env var → fallback chain. Returns a duration in hours.
func (s *guardianInvitationService) resolveTokenExpiry(ctx context.Context) time.Duration {
	hours := 0
	if s.settingsResolver != nil {
		if has, err := s.settingsResolver.HasTenantOverride(ctx, configModel.KeyGuardianInvitationTokenExpiryHours); err != nil {
			s.getLogger().Warn("guardian invitation: settings override check failed",
				slog.String("key", configModel.KeyGuardianInvitationTokenExpiryHours),
				slog.String("error", err.Error()),
			)
		} else if has {
			if v, err := s.settingsResolver.ResolveInt(ctx, configModel.KeyGuardianInvitationTokenExpiryHours); err == nil && v > 0 {
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
		return s.fallbackExpiry
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

	profile, err := s.guardianProfileRepo.FindByID(ctx, req.GuardianProfileID)
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

	if err := s.invitationRepo.Create(ctx, invitation); err != nil {
		return nil, &AuthError{Op: opGuardianInviteCreate, Err: err}
	}

	s.getLogger().Info("guardian invitation created",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("guardian_profile_id", profile.ID),
		slog.Int64("created_by", req.CreatedBy),
	)

	schoolName := s.lookupSchoolName(ctx, invitation.TenantID)
	s.dispatchEmail(invitation, profile, schoolName)

	return invitation, nil
}

// Validate returns the public-safe view of an invitation if its token is
// still usable. Public route — caller is responsible for using WithAdminTx.
func (s *guardianInvitationService) Validate(ctx context.Context, token string) (*GuardianInvitationValidation, error) {
	invitation, err := s.fetchValidInvitation(ctx, token)
	if err != nil {
		return nil, err
	}

	profile, err := s.guardianProfileRepo.FindByID(ctx, invitation.GuardianProfileID)
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
	return result, nil
}

// Accept consumes a token, creating the auth.accounts row, account_tenants
// mapping, and guardian role assignment. Updates the GuardianProfile to
// link to the new account. Public route — caller wraps in WithAdminTx.
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
	if invitation.TenantID > 0 && s.schoolRepo != nil {
		school, schoolErr := s.schoolRepo.FindByIDForShare(ctx, invitation.TenantID)
		if schoolErr != nil {
			return nil, &AuthError{Op: opGuardianInviteAccept, Err: schoolErr}
		}
		if school == nil || school.IsDeleted() {
			return nil, &AuthError{Op: opGuardianInviteAccept, Err: ErrInvitationTenantDeleted}
		}
	}

	profile, err := s.guardianProfileRepo.FindByID(ctx, invitation.GuardianProfileID)
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
		if innerErr := s.linkProfileToAccount(txCtx, profile, acc.ID, invitation.TenantID); innerErr != nil {
			return innerErr
		}
		if innerErr := s.invitationRepo.MarkAsAccepted(txCtx, invitation.ID); innerErr != nil {
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

	return account, nil
}

// createOrFindAccount returns the existing auth.accounts row for this email
// (cross-tenant — guardians may be invited to multiple schools) or creates a
// new one. When reusing an existing account we update the password hash so
// the parent gets to set their own.
func (s *guardianInvitationService) createOrFindAccount(ctx context.Context, emailAddress, passwordHash string) (*authModels.Account, error) {
	existing, err := s.accountRepo.FindByEmail(ctx, emailAddress)
	if err == nil && existing != nil {
		if updateErr := s.accountRepo.UpdatePassword(ctx, existing.ID, passwordHash); updateErr != nil {
			return nil, &AuthError{Op: opGuardianInviteAccept, Err: updateErr}
		}
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
	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, &AuthError{Op: opGuardianInviteAccept, Err: err}
	}
	return account, nil
}

// linkProfileToAccount writes the role assignment + account_tenants mapping +
// guardian profile linkage for this tenant. Splits out so Accept stays under
// gocognit 15.
func (s *guardianInvitationService) linkProfileToAccount(ctx context.Context, profile *userModels.GuardianProfile, accountID, tenantID int64) error {
	role, err := s.roleRepo.FindByName(ctx, guardianRoleBaseName)
	if err != nil {
		return &AuthError{Op: opGuardianInviteAccept, Err: fmt.Errorf("guardian role lookup failed: %w", err)}
	}
	if role == nil {
		return &AuthError{Op: opGuardianInviteAccept, Err: fmt.Errorf("guardian role not found")}
	}

	roleAssignment := &authModels.AccountRole{AccountID: accountID, RoleID: role.ID}
	roleAssignment.SetTenantID(tenantID)
	if err := s.accountRoleRepo.Create(ctx, roleAssignment); err != nil {
		return &AuthError{Op: opGuardianInviteAccept, Err: fmt.Errorf("assign guardian role: %w", err)}
	}

	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.accountTenantRepo.Create(ctx, mapping); err != nil {
		return &AuthError{Op: opGuardianInviteAccept, Err: fmt.Errorf("link account to tenant: %w", err)}
	}

	if err := s.guardianProfileRepo.LinkAccount(ctx, profile.ID, accountID); err != nil {
		return &AuthError{Op: opGuardianInviteAccept, Err: fmt.Errorf("link guardian profile to account: %w", err)}
	}
	return nil
}

// Resend invalidates the email-tracking columns and re-dispatches a fresh
// email. Does NOT issue a new token — same token, same expiry. If the
// invitation has expired, callers should issue a new one via Create.
func (s *guardianInvitationService) Resend(ctx context.Context, invitationID int64, actorAccountID int64) error {
	invitation, err := s.invitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		if isNotFoundError(err) {
			return &AuthError{Op: opGuardianInviteResend, Err: ErrInvitationNotFound}
		}
		return &AuthError{Op: opGuardianInviteResend, Err: err}
	}
	if invitation.IsAccepted() {
		return &AuthError{Op: opGuardianInviteResend, Err: ErrInvitationUsed}
	}
	if invitation.IsExpired() {
		return &AuthError{Op: opGuardianInviteResend, Err: ErrInvitationExpired}
	}

	profile, err := s.guardianProfileRepo.FindByID(ctx, invitation.GuardianProfileID)
	if err != nil {
		return &AuthError{Op: opGuardianInviteResend, Err: err}
	}

	invitation.EmailSentAt = nil
	invitation.EmailError = nil
	invitation.UpdatedAt = time.Now()
	if err := s.invitationRepo.Update(ctx, invitation); err != nil {
		return &AuthError{Op: opGuardianInviteResend, Err: err}
	}

	s.getLogger().Info("guardian invitation resent",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("actor_account_id", actorAccountID),
	)

	schoolName := s.lookupSchoolName(ctx, invitation.TenantID)
	s.dispatchEmail(invitation, profile, schoolName)
	return nil
}

// CleanupExpired removes accepted-or-expired guardian invitations. Scheduler-
// callable. Mirrors staff InvitationService.CleanupExpiredInvitations.
func (s *guardianInvitationService) CleanupExpired(ctx context.Context) (int, error) {
	count, err := s.invitationRepo.DeleteExpired(ctx)
	if err != nil {
		return 0, &AuthError{Op: "cleanup guardian invitations", Err: err}
	}
	if count > 0 {
		s.getLogger().Info("guardian invitation cleanup completed",
			slog.Int("records_deleted", count),
		)
	}
	return count, nil
}

func (s *guardianInvitationService) fetchValidInvitation(ctx context.Context, token string) (*authModels.GuardianInvitation, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, &AuthError{Op: opGuardianInviteFetch, Err: ErrInvitationNotFound}
	}
	invitation, err := s.invitationRepo.FindByToken(ctx, token)
	if err != nil {
		if isNotFoundError(err) || errors.Is(err, sql.ErrNoRows) {
			return nil, &AuthError{Op: opGuardianInviteFetch, Err: ErrInvitationNotFound}
		}
		return nil, &AuthError{Op: opGuardianInviteFetch, Err: err}
	}
	if invitation.IsAccepted() {
		return nil, &AuthError{Op: opGuardianInviteFetch, Err: ErrInvitationUsed}
	}
	if invitation.IsExpired() {
		return nil, &AuthError{Op: opGuardianInviteFetch, Err: ErrInvitationExpired}
	}
	return invitation, nil
}

// lookupSchoolName resolves the tenant display name for inclusion in the
// invitation email subject. Best-effort — empty string on failure.
func (s *guardianInvitationService) lookupSchoolName(ctx context.Context, tenantID int64) string {
	if tenantID == 0 || s.schoolRepo == nil {
		return ""
	}
	school, err := s.schoolRepo.FindByID(ctx, tenantID)
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

func (s *guardianInvitationService) dispatchEmail(invitation *authModels.GuardianInvitation, profile *userModels.GuardianProfile, schoolName string) {
	if s.dispatcher == nil {
		s.getLogger().Warn("guardian invitation: email dispatcher unavailable",
			slog.Int64("invitation_id", invitation.ID),
		)
		return
	}

	frontend := s.frontendURL
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
		subject = fmt.Sprintf("Einladung zum Eltern-Portal – %s", schoolName)
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
		From:     s.defaultFrom,
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

	s.dispatcher.Dispatch(context.Background(), email.DeliveryRequest{
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
	if s.db != nil {
		// Persist via admin tx — the dispatcher's callback runs detached from
		// the request context, so it has no tenant transaction available.
		err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
			return s.invitationRepo.UpdateEmailStatus(adminCtx, meta.ReferenceID, sentAt, errText, retryCount)
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
