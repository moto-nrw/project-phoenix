package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Identity & Access owns the school invitation flows (#2722). This file
// binds the seams they need to the retained material the root composes (the
// role-grant policy, the JWT signer behind the owner check, the mail with
// its tenant reply-to identity). The invitation routes and the staff import
// call the public capability directly (#3332); the operator provisioning
// routes, which may not name it, read the retained envelope this file
// translates into.

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
	return securityruntime.CanGrantRole(securityruntime.GrantedRole{
		Name: role.Name, BaseRole: role.BaseRole, IsSystem: role.IsSystem,
		TenantBound: role.TenantID != nil, Permissions: rolePermissions,
	}, actorPermissions)
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

// InvitationCapability is what the invitation routes consume: the school
// invitation flows the staff screens drive and the guardian invitation flows
// the public accept page drives. One owner serves both (#3332).
type InvitationCapability interface {
	identityaccess.SchoolInvitations
	identityaccess.GuardianInvitations
}
