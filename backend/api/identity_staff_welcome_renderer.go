package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
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
	mail, recipient, err := identityaccess.StaffWelcomeMail(payload, func() bool {
		replyIdentity := email.ResolveReplyToIdentity(ctx, cfg.MailIdentity, tenantID, logger)
		return strings.TrimSpace(replyIdentity.Address) != ""
	})
	if err != nil {
		return nil, err
	}
	return identityMessage(cfg.DefaultFrom, email.NewEmail("", recipient), mail), nil
}
