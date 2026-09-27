package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

const (
	staffWelcomePayloadRecipientEmail = "recipient_email"
	staffWelcomePayloadHelpURL        = "help_url"
	staffWelcomePayloadFirstName      = "first_name"
	staffWelcomePayloadLogoURL        = "logo_url"
	staffWelcomePayloadSchoolName     = "school_name"
)

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
func NewStaffWelcomeRenderer(cfg StaffWelcomeRendererConfig) emailoutbox.RendererFunc {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context, intent *emailoutbox.Intent) (*email.Message, error) {
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
		if errors.Is(err, identityaccess.ErrInvitationUsed) || errors.Is(err, identityaccess.ErrInvitationExpired) {
			return nil, fmt.Errorf("%w: staff invitation is no longer redeemable", emailoutbox.ErrRenderCancelled)
		}
		return nil, fmt.Errorf("look up staff invitation delivery: %w", err)
	}
	if !sent {
		return nil, errors.New("staff invitation email has not been accepted")
	}

	recipient, _ := payload[staffWelcomePayloadRecipientEmail].(string)
	if recipient == "" {
		return nil, errors.New("staff welcome payload missing recipient_email")
	}
	helpURL, _ := payload[staffWelcomePayloadHelpURL].(string)
	if helpURL == "" {
		return nil, errors.New("staff welcome payload missing help_url")
	}
	firstName, _ := payload[staffWelcomePayloadFirstName].(string)
	logoURL, _ := payload[staffWelcomePayloadLogoURL].(string)
	schoolName, _ := payload[staffWelcomePayloadSchoolName].(string)
	subject := "Willkommen bei moto"
	if schoolName != "" {
		subject = fmt.Sprintf("%s – %s", subject, schoolName)
	}
	replyIdentity := email.ResolveReplyToIdentity(ctx, cfg.MailIdentity, tenantID, logger)
	return &email.Message{
		From:     cfg.DefaultFrom,
		To:       email.NewEmail("", recipient),
		Subject:  subject,
		Template: "staff-welcome.html",
		Content: map[string]any{
			"HelpURL":           helpURL,
			"FirstName":         strings.TrimSpace(firstName),
			"LogoURL":           logoURL,
			"SchoolName":        schoolName,
			"ReplyGoesToSchool": strings.TrimSpace(replyIdentity.Address) != "",
		},
	}, nil
}
