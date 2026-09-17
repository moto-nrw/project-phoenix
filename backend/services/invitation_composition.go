package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Identity & Access owns the school invitation flows (#2722). This file
// binds the seams they need to the retained material the root composes (the
// role-grant policy, the JWT signer behind the owner check, the mail with
// its tenant reply-to identity) and serves the retained auth service's
// consumer-owned InvitationService port over the public module.

var invitationEmailBackoff = []time.Duration{
	time.Second,
	5 * time.Second,
	15 * time.Second,
}

// systemRoleTranslations maps English system role names to the German
// display names the invitation mail shows.
var systemRoleTranslations = map[string]string{
	"admin":     "Administrator",
	"user":      "Betreuer",
	"guest":     "Gast",
	"guardian":  "Erziehungsberechtigter",
	"lehrkraft": "Lehrkraft",
}

func translateRoleNameToGerman(roleName string) string {
	if translated, ok := systemRoleTranslations[strings.ToLower(roleName)]; ok {
		return translated
	}
	return roleName
}

// invitationWiring is the configuration the invitation flows are composed
// with. dispatcher may be nil: the link is then not mailed, as before.
type invitationWiring struct {
	dispatcher  *email.Dispatcher
	defaultFrom email.Email
	// staffURL and schoolURL are the portal hosts an invitee accepts on.
	staffURL  string
	schoolURL string
	// mailIdentity points a reply at the OGS instead of moto (#1936); nil
	// sends without a Reply-To.
	mailIdentity email.ReplyToResolver
	tokenAuth    *authjwt.TokenAuth
	expiry       time.Duration
	// backoff spaces the send retries; nil uses the production spacing.
	backoff []time.Duration
}

func invitationDependencies(wiring *invitationWiring, invitations func() identityaccess.SchoolInvitations, logger *slog.Logger) *identityaccessCompose.SchoolInvitationDependencies {
	if wiring == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	backoff := wiring.backoff
	if backoff == nil {
		backoff = invitationEmailBackoff
	}
	return &identityaccessCompose.SchoolInvitationDependencies{
		Grants: invitationGrantPolicy{},
		Owners: invitationOwnerTokens{tokenAuth: wiring.tokenAuth},
		Delivery: invitationDelivery{
			dispatcher: wiring.dispatcher, from: wiring.defaultFrom, staffURL: wiring.staffURL, schoolURL: wiring.schoolURL,
			identity: wiring.mailIdentity, backoff: backoff, invitations: invitations, logger: logger,
		},
		Passwords: passwordPolicy{},
		Expiry:    wiring.expiry,
		Logger:    logger,
	}
}

// invitationGrantPolicy is the Security Runtime decision whether the
// inviting account may hand out the role.
type invitationGrantPolicy struct{}

func (invitationGrantPolicy) CanGrantRole(role identityaccess.RoleFacts, actorPermissions, rolePermissions []string) bool {
	return auth.CanGrantRole(auth.RoleFacts(role), actorPermissions, rolePermissions)
}

// invitationOwnerTokens verifies that the caller holds a live session of the
// invited account. A preview, read-only, unfinished-MFA or operator-scope
// token proves nothing.
type invitationOwnerTokens struct{ tokenAuth *authjwt.TokenAuth }

func (t invitationOwnerTokens) AccountOfAccessToken(token string) (int64, error) {
	if t.tokenAuth == nil || token == "" {
		return 0, nil
	}
	claims, err := t.tokenAuth.ParseAccessJWT(token)
	if err != nil || claims.ID <= 0 || claims.ExpiresAt <= time.Now().Unix() ||
		claims.ReadOnly || claims.ActingAdminID != 0 || claims.PreviewID != "" {
		return 0, nil
	}
	switch claims.Scope {
	case "", "tenant", "org", "school":
		if claims.TenantID <= 0 {
			return 0, nil
		}
	case "parent":
	default:
		return 0, nil
	}
	return int64(claims.ID), nil
}

// invitationDelivery mails an invitation and records the outcome through the
// module once the send settles.
type invitationDelivery struct {
	dispatcher  *email.Dispatcher
	from        email.Email
	staffURL    string
	schoolURL   string
	identity    email.ReplyToResolver
	backoff     []time.Duration
	invitations func() identityaccess.SchoolInvitations
	logger      *slog.Logger
}

func (d invitationDelivery) DispatchSchoolInvitation(ctx context.Context, invitation identityaccess.SchoolInvitation, schoolName string, portal identityaccess.InvitationPortal, expiry time.Duration) {
	if d.dispatcher == nil {
		d.logger.Warn("email dispatcher unavailable, skipping invitation email",
			slog.Int64("invitation_id", invitation.ID))
		return
	}
	frontend := d.staffURL
	if frontend == "" {
		frontend = "http://localhost:3000"
	}
	// A school-portal role accepts on the school portal (#2207): the link
	// must land where its login lives, otherwise the Lehrkraft sets a
	// password in the staff portal and then cannot use it there.
	if portal == identityaccess.InvitationPortalSchool && d.schoolURL != "" {
		frontend = d.schoolURL
	}
	subject := "Einladung zu moto"
	if schoolName != "" {
		subject = fmt.Sprintf("Einladung zu moto – %s", schoolName)
	}
	// An invited Mitarbeiter answering this mail ("wer lädt mich ein?") must
	// reach the OGS, not moto (#1936). The invitation names its school;
	// create queues this after commit on a detached context that has no
	// ambient tenant.
	replyIdentity := email.ResolveReplyToIdentity(ctx, d.identity, invitation.TenantID, d.logger)
	message := email.Message{
		From:     d.from,
		ReplyTo:  email.NewEmail(replyIdentity.Name, replyIdentity.Address),
		To:       email.NewEmail("", invitation.Email),
		Subject:  subject,
		Template: "invitation.html",
		Content: map[string]any{
			"InvitationURL": fmt.Sprintf("%s/invite?token=%s", frontend, invitation.Token),
			"RoleName":      translateRoleNameToGerman(invitation.RoleName),
			"FirstName":     invitation.FirstName,
			"LastName":      invitation.LastName,
			"ExpiryHours":   int(expiry / time.Hour),
			"LogoURL":       fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", frontend),
			"SchoolName":    schoolName,
		},
	}
	meta := email.DeliveryMetadata{
		Type:        "invitation",
		ReferenceID: invitation.ID,
		Token:       invitation.Token,
		Recipient:   invitation.Email,
	}
	baseRetry := invitation.Delivery.RetryCount
	d.dispatcher.Dispatch(detachedContext(ctx), email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: d.backoff,
		MaxAttempts:   3,
		Callback: func(cbCtx context.Context, result email.DeliveryResult) {
			d.recordDelivery(cbCtx, meta, baseRetry, result)
		},
	})
}

func (d invitationDelivery) recordDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	delivery := identityaccess.TokenDelivery{RetryCount: baseRetry + result.Attempt}
	if result.Status == email.DeliveryStatusSent {
		sentAt := result.SentAt
		delivery.SentAt = &sentAt
	} else if result.Err != nil {
		message := strings.TrimSpace(result.Err.Error())
		delivery.Error = &message
	}
	if err := d.invitations().RecordSchoolInvitationDelivery(ctx, meta.ReferenceID, delivery); err != nil {
		d.logger.Error("failed to update invitation delivery status",
			slog.Int64("invitation_id", meta.ReferenceID),
			slog.Any("error", err),
		)
		return
	}
	if result.Final && result.Status == email.DeliveryStatusFailed {
		d.logger.Error("invitation email permanently failed",
			slog.Int64("invitation_id", meta.ReferenceID),
			slog.String("recipient", meta.Recipient),
			slog.Any("error", result.Err),
		)
	}
}

// detachedContext isolates asynchronous work from the request transaction
// and its commit hooks while keeping the tenant and the runtime.
func detachedContext(ctx context.Context) context.Context {
	return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(context.WithoutCancel(ctx)))
}

// --- the retained invitation service's consumer-owned port -----------------

// NewInvitationService serves the retained invitation contract over the
// public module, so the invitation routes and the staff import keep calling
// the same methods.
func NewInvitationService(module *identityaccess.Module) auth.InvitationService {
	return auth.NewInvitationService(schoolInvitations{module: module})
}

type schoolInvitations struct{ module *identityaccess.Module }

func (s schoolInvitations) CreateInvitation(ctx context.Context, request auth.InvitationRequest) (auth.InvitationRecord, error) {
	invitation, err := s.module.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: request.Email, RoleID: request.RoleID, TenantID: request.TenantID, FirstName: request.FirstName,
		LastName: request.LastName, Position: request.Position, CaregiverEnabled: request.CaregiverEnabled,
		PersonID: request.PersonID, CreatedBy: request.CreatedBy, SchoolName: request.SchoolName,
		ActorPermissions: request.ActorPermissions, OperatorGrant: request.OperatorGrant,
	})
	if err != nil {
		return auth.InvitationRecord{}, invitationServiceError(err)
	}
	return invitationRecord(invitation), nil
}

func invitationRecord(invitation identityaccess.SchoolInvitation) auth.InvitationRecord {
	return auth.InvitationRecord{
		ID: invitation.ID, TenantID: invitation.TenantID, Email: invitation.Email, Token: invitation.Token,
		RoleID: invitation.RoleID, RoleName: invitation.RoleName, ExpiresAt: invitation.ExpiresAt, UsedAt: invitation.UsedAt,
		CreatedBy: invitation.CreatedBy, CreatorEmail: invitation.CreatorEmail, FirstName: invitation.FirstName, LastName: invitation.LastName,
		Position: invitation.Position, CaregiverEnabled: invitation.CaregiverEnabled, PersonID: invitation.PersonID,
		EmailSentAt: invitation.Delivery.SentAt, EmailError: invitation.Delivery.Error,
		EmailRetryCount: invitation.Delivery.RetryCount, CreatedAt: invitation.CreatedAt,
	}
}

func (s schoolInvitations) AcceptInvitation(ctx context.Context, token string, userData auth.UserRegistrationData) (auth.InvitationAccount, error) {
	account, err := s.module.AcceptSchoolInvitation(ctx, token, identityaccess.InvitationRegistration{
		OwnerAccessToken: userData.OwnerAccessToken, FirstName: userData.FirstName, LastName: userData.LastName,
		Password: userData.Password, ConfirmPassword: userData.ConfirmPassword,
	})
	if err != nil {
		return auth.InvitationAccount{}, invitationServiceError(err)
	}
	return auth.InvitationAccount{ID: account.ID, Email: account.Email}, nil
}

func (s schoolInvitations) ListPendingInvitations(ctx context.Context) ([]auth.InvitationRecord, error) {
	invitations, err := s.module.ListPendingSchoolInvitations(ctx)
	if err != nil {
		return nil, invitationServiceError(err)
	}
	records := make([]auth.InvitationRecord, 0, len(invitations))
	for _, invitation := range invitations {
		records = append(records, invitationRecord(invitation))
	}
	return records, nil
}

func (s schoolInvitations) ValidateInvitation(ctx context.Context, token string) (*auth.InvitationValidationResult, error) {
	preview, err := s.module.ValidateSchoolInvitation(ctx, token)
	if err != nil {
		return nil, invitationServiceError(err)
	}
	return &auth.InvitationValidationResult{
		TargetPortal: string(preview.Portal), RequiresAccountLogin: preview.RequiresAccountLogin, Email: preview.Email,
		RoleName: preview.RoleName, FirstName: preview.FirstName, LastName: preview.LastName, Position: preview.Position,
		CaregiverEnabled: preview.CaregiverEnabled, ExpiresAt: preview.ExpiresAt,
	}, nil
}

func (s schoolInvitations) ResendInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	return invitationServiceError(s.module.ResendSchoolInvitation(ctx, invitationID, actorAccountID))
}

func (s schoolInvitations) RevokeInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	return invitationServiceError(s.module.RevokeSchoolInvitation(ctx, invitationID, actorAccountID))
}

func (s schoolInvitations) InvalidatePendingInvitationsByTenantID(ctx context.Context, tenantID int64) (int, error) {
	revoked, err := s.module.RevokeTenantSchoolInvitations(ctx, tenantID)
	return revoked, invitationServiceError(err)
}

func (s schoolInvitations) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	deleted, err := s.module.DeleteExpiredSchoolInvitations(ctx)
	return deleted, invitationServiceError(err)
}

func (s schoolInvitations) GetTenantSubdomainForToken(ctx context.Context, token string) string {
	return s.module.SchoolInvitationSubdomain(ctx, token)
}

var invitationRetainedSentinels = []retainedSentinel{
	{identityaccess.ErrInvitationNotFound, auth.ErrInvitationNotFound},
	{identityaccess.ErrInvitationExpired, auth.ErrInvitationExpired},
	{identityaccess.ErrInvitationUsed, auth.ErrInvitationUsed},
	{identityaccess.ErrInvitationTenantDeleted, auth.ErrInvitationTenantDeleted},
	{identityaccess.ErrInvitationNameRequired, auth.ErrInvitationNameRequired},
	{identityaccess.ErrInvitationOwnerRequired, auth.ErrInvitationOwnerRequired},
	{identityaccess.ErrInvitationOwnerMismatch, auth.ErrInvitationOwnerMismatch},
	{identityaccess.ErrAccountAlreadyHasTenantAccess, auth.ErrAccountAlreadyHasTenantAccess},
	{identityaccess.ErrInvitationPasswordMismatch, auth.ErrPasswordMismatch},
	{identityaccess.ErrRoleGrantNotPermitted, auth.ErrRoleGrantNotPermitted},
	{identityaccess.ErrLehrkraftNoCaregiver, auth.ErrLehrkraftNoCaregiver},
	{identityaccess.ErrSchoolInvitationUnavailable, auth.ErrInvitationsUnavailable},
}

// invitationServiceError translates the public contract into the retained
// envelope the invitation routes switch on.
func invitationServiceError(err error) error {
	if err == nil {
		return nil
	}
	var operation *identityaccess.AuthenticationError
	if errors.As(err, &operation) && operation == err {
		return &auth.AuthError{Op: operation.Op, Err: invitationServiceError(operation.Err)}
	}
	for _, sentinel := range invitationRetainedSentinels {
		if !errors.Is(err, sentinel.public) {
			continue
		}
		if err == sentinel.public {
			return sentinel.retained
		}
		return &retainedError{text: err.Error(), sentinel: sentinel.retained, cause: err}
	}
	return authServiceError(err)
}
