package owners

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

const (
	staffWelcomePayloadRecipientEmail = "recipient_email"
	staffWelcomePayloadHelpURL        = "help_url"
	staffWelcomePayloadFirstName      = "first_name"
	staffWelcomePayloadLogoURL        = "logo_url"
	staffWelcomePayloadSchoolName     = "school_name"
)

// staffWelcomePayload stores the welcome Identity & Access composed.
func staffWelcomePayload(welcome identityaccess.StaffWelcome) map[string]any {
	return map[string]any{
		staffWelcomePayloadRecipientEmail: welcome.Recipient,
		staffWelcomePayloadHelpURL:        welcome.HelpURL,
		staffWelcomePayloadFirstName:      welcome.FirstName,
		staffWelcomePayloadLogoURL:        welcome.LogoURL,
		staffWelcomePayloadSchoolName:     welcome.SchoolName,
	}
}

// storedStaffWelcome reads the welcome back; a field of the wrong type reads
// as absent.
func storedStaffWelcome(payload map[string]any) identityaccess.StaffWelcome {
	text := func(key string) string {
		value, _ := payload[key].(string)
		return value
	}
	return identityaccess.StaffWelcome{
		Recipient: text(staffWelcomePayloadRecipientEmail), HelpURL: text(staffWelcomePayloadHelpURL),
		FirstName: text(staffWelcomePayloadFirstName), LogoURL: text(staffWelcomePayloadLogoURL),
		SchoolName: text(staffWelcomePayloadSchoolName),
	}
}

// StaffWelcomeRendererConfig supplies the renderer with process-static mail
// defaults and the delivery state of the preceding invitation.
type StaffWelcomeRendererConfig struct {
	DefaultFrom            email.Email
	MailIdentity           email.ReplyToResolver
	Logger                 *slog.Logger
	InvitationDeliverySent func(context.Context, int64) (bool, error)
}

// NewStaffWelcomeRenderer turns a durable staff_welcome intent into its
// message. It leaves the row pending until the invitation mail is confirmed,
// which keeps the recipient-facing order even after a restart.
func NewStaffWelcomeRenderer(cfg StaffWelcomeRendererConfig) deliveryCompose.MailRendererFunc {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context, intent *deliveryCompose.MailIntent) (*email.Message, error) {
		return renderStaffWelcome(ctx, intent.TenantID, intent.RelatedEntityID, intent.Payload, cfg, logger)
	}
}

func renderStaffWelcome(ctx context.Context, tenantID, invitationID int64, payload map[string]any, cfg StaffWelcomeRendererConfig, logger *slog.Logger) (*email.Message, error) {
	if invitationID <= 0 {
		return nil, errors.New("staff welcome payload missing invitation relation")
	}
	if cfg.InvitationDeliverySent == nil {
		return nil, errors.New("staff welcome invitation delivery status is not configured")
	}
	sent, err := cfg.InvitationDeliverySent(ctx, invitationID)
	if err != nil {
		if identityaccess.StaffWelcomeCancelled(err) {
			return nil, fmt.Errorf("%w: staff invitation is no longer redeemable", deliveryCompose.ErrRenderCancelled)
		}
		return nil, fmt.Errorf("look up staff invitation delivery: %w", err)
	}
	if !sent {
		return nil, fmt.Errorf("%w: staff invitation email has not been accepted", deliveryCompose.ErrRenderDeferred)
	}
	welcome := storedStaffWelcome(payload)
	mail, err := identityaccess.StaffWelcomeMail(welcome, func() bool {
		replyIdentity := email.ResolveReplyToIdentity(ctx, cfg.MailIdentity, tenantID, logger)
		return strings.TrimSpace(replyIdentity.Address) != ""
	})
	if err != nil {
		return nil, err
	}
	return identityMessage(cfg.DefaultFrom, email.NewEmail("", welcome.Recipient), mail), nil
}
