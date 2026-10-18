package api

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// Identity & Access owns the school invitation flows (#2722). This file
// binds the seams they need to the retained material the root composes (the
// role-grant policy, the JWT signer behind the owner check, the mail with
// its tenant reply-to identity). The invitation routes and the staff import
// call the public capability directly (#3332); the operator provisioning
// routes, which may not name it, read the retained envelope this file
// translates into.

// InvitationWiring is the configuration the invitation flows are composed
// with. dispatcher may be nil: the link is then not mailed, as before.
type InvitationWiring struct {
	Dispatcher  *email.Dispatcher
	DefaultFrom email.Email
	// StaffURL and schoolURL are the portal hosts an invitee accepts on.
	StaffURL  string
	SchoolURL string
	// MailIdentity points a reply at the OGS instead of moto (#1936); nil
	// sends without a Reply-To.
	MailIdentity email.ReplyToResolver
	Expiry       time.Duration
	// Backoff spaces the send retries; nil uses the production spacing.
	Backoff []time.Duration
	// Settings resolve the tenant settings the welcome mail's help link
	// carries; nil leaves them out and /help asks.
	Settings tenantSettingsResolver
	// Outbox persists the staff welcome in the invitation transaction.
	Outbox func() MailOutbox
	// WelcomeDelay holds the welcome behind the invitation; zero is immediate.
	WelcomeDelay time.Duration
}

func invitationDependencies(wiring *InvitationWiring, owners identityaccessCompose.InvitationOwnerTokens, invitations func() identityaccess.SchoolInvitations, logger *slog.Logger) *identityaccessCompose.SchoolInvitationDependencies {
	if wiring == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	backoff := wiring.Backoff
	return &identityaccessCompose.SchoolInvitationDependencies{
		Grants: RoleGrantPolicy{},
		Owners: owners,
		Delivery: invitationDelivery{
			dispatcher: wiring.Dispatcher, from: wiring.DefaultFrom, staffURL: wiring.StaffURL, schoolURL: wiring.SchoolURL,
			identity: wiring.MailIdentity, backoff: backoff, invitations: invitations, logger: logger,
			settings: wiring.Settings, outbox: wiring.Outbox, welcomeDelay: wiring.WelcomeDelay,
		},
		Passwords: passwordPolicy{},
		Expiry:    wiring.Expiry,
		Logger:    logger,
	}
}

// RoleGrantPolicy is the Security Runtime decision whether an account may
// hand out the role: the inviting account of a school invitation, and the
// admin who creates or links an account (#2736).
type RoleGrantPolicy struct{}

// CanGrantRole reports whether actorPermissions suffice to grant the role.
func (RoleGrantPolicy) CanGrantRole(role identityaccess.RoleFacts, actorPermissions, rolePermissions []string) bool {
	return securityruntime.CanGrantRole(securityruntime.GrantedRole{
		Name: role.Name, BaseRole: role.BaseRole, IsSystem: role.IsSystem,
		TenantBound: role.TenantID != nil, Permissions: rolePermissions,
	}, actorPermissions)
}

// invitationDelivery mails an invitation and records the outcome through the
// module once the send settles.
type invitationDelivery struct {
	dispatcher   *email.Dispatcher
	from         email.Email
	staffURL     string
	schoolURL    string
	identity     email.ReplyToResolver
	backoff      []time.Duration
	invitations  func() identityaccess.SchoolInvitations
	logger       *slog.Logger
	settings     tenantSettingsResolver
	outbox       func() MailOutbox
	welcomeDelay time.Duration
}

func (d invitationDelivery) portalURL(portal identityaccess.InvitationPortal) string {
	return identityaccess.SchoolInvitationOrigin(portal, d.staffURL, d.schoolURL)
}

func (d invitationDelivery) DispatchSchoolInvitation(ctx context.Context, invitation identityaccess.SchoolInvitation, schoolName string, portal identityaccess.InvitationPortal, expiry time.Duration) {
	d.dispatchSchoolInvitation(ctx, invitation, schoolName, portal, expiry)
}

func (d invitationDelivery) dispatchSchoolInvitation(ctx context.Context, invitation identityaccess.SchoolInvitation, schoolName string, portal identityaccess.InvitationPortal, expiry time.Duration) {
	if d.dispatcher == nil {
		d.logger.Warn("email dispatcher unavailable, skipping invitation email",
			slog.Int64("invitation_id", invitation.ID))
		return
	}
	mail := identityaccess.SchoolInvitationMail(invitation, schoolName, d.portalURL(portal), expiry)
	// An invited Mitarbeiter answering this mail ("wer lädt mich ein?") must
	// reach the OGS, not moto (#1936). The invitation names its school;
	// create queues this after commit on a detached context that has no
	// ambient tenant.
	replyIdentity := email.ResolveReplyToIdentity(ctx, d.identity, invitation.TenantID, d.logger)
	message := *identityMessage(d.from, email.NewEmail("", invitation.Email), mail)
	message.ReplyTo = email.NewEmail(replyIdentity.Name, replyIdentity.Address)
	meta := email.DeliveryMetadata{
		Type:        "invitation",
		ReferenceID: invitation.ID,
		Token:       invitation.Token,
		Recipient:   invitation.Email,
	}
	baseRetry := invitation.Delivery.RetryCount
	d.dispatcher.Dispatch(identityaccessCompose.DetachedMailContext(ctx), email.WithInteractiveRetry(email.DeliveryRequest{
		Message:  message,
		Metadata: meta,
		Callback: func(cbCtx context.Context, result email.DeliveryResult) {
			d.recordDelivery(cbCtx, meta, baseRetry, result)
		},
	}, d.backoff))
}

// QueueSchoolWelcome persists the welcome with the invitation. Its renderer
// waits for the invitation delivery record, so a crash after SMTP accepts the
// invitation cannot lose the follow-up or send it first.
func (d invitationDelivery) QueueSchoolWelcome(ctx context.Context, invitation identityaccess.SchoolInvitation, schoolName string, portal identityaccess.InvitationPortal, rolePermissions []string) error {
	if d.dispatcher == nil {
		d.logger.Warn("email dispatcher unavailable, skipping staff welcome email",
			slog.Int64("invitation_id", invitation.ID))
		return nil
	}
	if d.outbox == nil || d.outbox() == nil {
		return fmt.Errorf("staff welcome outbox is not configured")
	}
	frontend := d.portalURL(portal)
	// A failed read is logged and left out of the link. Run it in a savepoint:
	// PostgreSQL marks the invitation transaction aborted after a failed query,
	// even though the welcome can still be useful without that parameter.
	var helpURL string
	resolveHelpURL := func(savepointCtx context.Context) error {
		var err error
		helpURL, err = welcomeHelpURL(savepointCtx, frontend, staffHelpRole(portal == identityaccess.InvitationPortalSchool, invitation.RoleName, rolePermissions),
			invitation.TenantID, d.welcomeSettings(invitation.TenantID), d.logger)
		return err
	}
	if err := identityaccessCompose.RunMailStepInSavepoint(ctx, resolveHelpURL); err != nil {
		return fmt.Errorf("resolve staff welcome help URL: %w", err)
	}
	payload := staffWelcomePayload(identityaccess.NewStaffWelcome(invitation, schoolName, frontend, helpURL))
	deliverAfter := time.Time{}
	if d.welcomeDelay > 0 {
		deliverAfter = time.Now().Add(d.welcomeDelay)
	}
	if err := d.outbox().EnqueueOutbox(ctx, deliveryCompose.MailEnqueueRequest{
		Kind:              identityaccess.MailKindStaffWelcome,
		Payload:           payload,
		RelatedEntityType: identityaccess.MailRelatedSchoolInvitation,
		RelatedEntityID:   invitation.ID,
		DeliverAfter:      deliverAfter,
	}); err != nil {
		return fmt.Errorf("enqueue staff welcome: %w", err)
	}
	return nil
}

// welcomeSettings binds the help URL to the invitation's school.
func (d invitationDelivery) welcomeSettings(tenantID int64) welcomeHelpSettings {
	if d.settings == nil || tenantID <= 0 {
		return nil
	}
	return schoolSettings{resolver: d.settings, tenantID: tenantID}
}

func (d invitationDelivery) recordDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	delivery := identityaccess.TokenDelivery{RetryCount: baseRetry + result.Attempt}
	if result.Status == email.DeliveryStatusSent {
		sentAt := result.SentAt
		delivery.SentAt = &sentAt
	} else if result.Final && result.Err != nil {
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
			slog.String("recipient", identityaccessCompose.MaskEmail(meta.Recipient)),
			slog.Any("error", result.Err),
		)
	}
}

// InvitationCapability is what the invitation routes consume: the school
// invitation flows the staff screens drive and the guardian invitation flows
// the public accept page drives. One owner serves both (#3332).
type InvitationCapability interface {
	identityaccess.SchoolInvitations
	identityaccess.GuardianInvitations
}
