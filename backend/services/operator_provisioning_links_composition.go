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
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// Identity & Access owns the operator invitation and e-mail change flows
// (#3332). This file binds the seams they need to the retained material the
// root composes — the mail transport, the operator portal host and the
// People Directory address rule — and translates the public outcomes into
// the retained envelope the operator routes classify on. Those routes may
// not name the owner's contract, so the translation lives here.

var operatorLinkEmailBackoff = []time.Duration{
	time.Second,
	5 * time.Second,
	15 * time.Second,
}

// operatorLinkWiring is the configuration the operator link deliveries are
// composed with. dispatcher may be nil: the link is then not mailed, as
// before.
type operatorLinkWiring struct {
	dispatcher  *email.Dispatcher
	defaultFrom email.Email
	// frontendURL is the host of the staff frontend, which serves the
	// operator confirmation page and the logo the mails embed.
	frontendURL string
	// operatorFrontendURL is the operator portal's own host. The invitation
	// link uses it so the mail carries no cross-origin redirect hop, which
	// content scanners treat as a phishing signal.
	operatorFrontendURL string
	invitationExpiry    time.Duration
	emailChangeExpiry   time.Duration
	// backoff spaces the send retries; nil uses the production spacing.
	backoff []time.Duration
	logger  *slog.Logger
}

func operatorProvisioningDependencies(
	wiring *operatorLinkWiring,
	tokens func() identityaccess.OperatorTokens,
	logger *slog.Logger,
) *identityaccessCompose.OperatorProvisioningDependencies {
	if wiring == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	backoff := wiring.backoff
	if backoff == nil {
		backoff = operatorLinkEmailBackoff
	}
	delivery := operatorLinkDelivery{
		dispatcher: wiring.dispatcher, from: wiring.defaultFrom,
		frontendURL: wiring.frontendURL, operatorURL: wiring.operatorFrontendURL,
		backoff: backoff, tokens: tokens, logger: logger,
	}
	return &identityaccessCompose.OperatorProvisioningDependencies{
		Invites:           delivery,
		Changes:           delivery,
		Format:            operatorEmailFormat{},
		InvitationExpiry:  wiring.invitationExpiry,
		EmailChangeExpiry: wiring.emailChangeExpiry,
	}
}

// operatorEmailFormat binds the People Directory contact rule, so the
// module applies the canonical pattern instead of carrying a copy.
type operatorEmailFormat struct{}

func (operatorEmailFormat) IsRoutable(address string) bool {
	return peopledirectory.IsValidEmailFormat(address)
}

// operatorLinkDelivery mails the operator invitation and the three e-mail
// change messages, and records the outcome of the two that carry a link
// back through the module once the send settles.
type operatorLinkDelivery struct {
	dispatcher  *email.Dispatcher
	from        email.Email
	frontendURL string
	operatorURL string
	backoff     []time.Duration
	tokens      func() identityaccess.OperatorTokens
	logger      *slog.Logger
}

func (d operatorLinkDelivery) logoURL() string {
	return fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", d.frontendURL)
}

func (d operatorLinkDelivery) DispatchOperatorInvitation(ctx context.Context, invitation identityaccess.OperatorInvitation, inviterName string, expiry time.Duration) {
	if d.dispatcher == nil {
		d.logger.Warn("email dispatcher unavailable, skipping operator invitation email",
			slog.Int64("invitation_id", invitation.ID))
		return
	}
	if inviterName == "" {
		inviterName = "Ein Operator"
	}
	// The path is /invite, not /operator/invite: the operator subdomain
	// proxy rewrites it internally.
	message := email.Message{
		From:     d.from,
		To:       email.NewEmail("", invitation.Email),
		Subject:  "Einladung als Operator zu moto",
		Template: "operator-invitation.html",
		Content: map[string]any{
			"InvitationURL": fmt.Sprintf("%s/invite?token=%s", d.operatorURL, invitation.Token),
			"ExpiryHours":   int(expiry.Hours()),
			"LogoURL":       d.logoURL(),
			"InviterName":   inviterName,
		},
	}
	meta := email.DeliveryMetadata{
		Type: "operator_invitation", ReferenceID: invitation.ID,
		Token: invitation.Token, Recipient: invitation.Email,
	}
	d.dispatch(ctx, message, meta, invitation.Delivery.RetryCount, d.recordInvitationDelivery)
}

func (d operatorLinkDelivery) DispatchOperatorEmailChangeVerification(ctx context.Context, change identityaccess.OperatorEmailChange, expiry time.Duration) {
	if d.dispatcher == nil {
		d.logger.Warn("email dispatcher unavailable, skipping email change verification",
			slog.Int64("operator_id", change.OperatorID))
		return
	}
	// The link goes through /operator/* → operator subdomain redirect, which
	// preserves the query. The page strips the token from the URL on mount,
	// so no later Referer carries it.
	message := email.Message{
		From:     d.from,
		To:       email.NewEmail("", change.NewEmail),
		Subject:  "E-Mail-Adresse bestätigen",
		Template: "operator-email-change-verify.html",
		Content: map[string]any{
			"VerifyURL":     fmt.Sprintf("%s/operator/email-confirm?token=%s", d.frontendURL, change.Token),
			"ExpiryMinutes": int(expiry.Minutes()),
			"LogoURL":       d.logoURL(),
		},
	}
	meta := email.DeliveryMetadata{
		Type: "operator_email_change_verify", ReferenceID: change.ID,
		Token: change.Token, Recipient: change.NewEmail,
	}
	d.dispatch(ctx, message, meta, change.Delivery.RetryCount, d.recordEmailChangeDelivery)
}

func (d operatorLinkDelivery) DispatchOperatorEmailChangeRequested(ctx context.Context, operator identityaccess.Operator, maskedNewEmail string) {
	if d.dispatcher == nil {
		d.logger.Warn("email dispatcher unavailable, skipping email change notification",
			slog.Int64("operator_id", operator.ID))
		return
	}
	d.dispatch(ctx, email.Message{
		From:     d.from,
		To:       email.NewEmail(operator.DisplayName, operator.Email),
		Subject:  "E-Mail-Änderung angefordert",
		Template: "operator-email-change-notify.html",
		Content: map[string]any{
			"LogoURL":        d.logoURL(),
			"MaskedNewEmail": maskedNewEmail,
			"DisplayName":    operator.DisplayName,
		},
	}, email.DeliveryMetadata{Recipient: operator.Email}, 0, nil)
}

func (d operatorLinkDelivery) DispatchOperatorEmailChangeConfirmed(ctx context.Context, oldEmail, displayName string) {
	if d.dispatcher == nil {
		return
	}
	d.dispatch(ctx, email.Message{
		From:     d.from,
		To:       email.NewEmail(displayName, oldEmail),
		Subject:  "E-Mail-Adresse wurde geändert",
		Template: "operator-email-change-confirmed.html",
		Content: map[string]any{
			"LogoURL":     d.logoURL(),
			"DisplayName": displayName,
		},
	}, email.DeliveryMetadata{Recipient: oldEmail}, 0, nil)
}

func (d operatorLinkDelivery) dispatch(
	ctx context.Context,
	message email.Message,
	meta email.DeliveryMetadata,
	baseRetry int,
	record func(context.Context, email.DeliveryMetadata, int, email.DeliveryResult),
) {
	request := email.DeliveryRequest{
		Message: message, Metadata: meta, BackoffPolicy: d.backoff, MaxAttempts: 3,
	}
	if record != nil {
		request.Callback = func(cbCtx context.Context, result email.DeliveryResult) {
			record(cbCtx, meta, baseRetry, result)
		}
	}
	d.dispatcher.Dispatch(detachedContext(ctx), request)
}

func (d operatorLinkDelivery) recordInvitationDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	delivery := operatorTokenDelivery(baseRetry, result)
	if err := d.tokens().RecordOperatorInvitationDelivery(ctx, meta.ReferenceID, delivery); err != nil {
		d.logger.Error("failed to update invitation delivery status",
			slog.Int64("token_id", meta.ReferenceID),
			slog.Any("error", err),
		)
		return
	}
	d.logPermanentFailure(meta, result, "operator invitation email permanently failed")
}

func (d operatorLinkDelivery) recordEmailChangeDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	delivery := operatorTokenDelivery(baseRetry, result)
	if err := d.tokens().RecordOperatorEmailChangeDelivery(ctx, meta.ReferenceID, delivery); err != nil {
		d.logger.Error("failed to update email change delivery status",
			slog.Int64("token_id", meta.ReferenceID),
			slog.Any("error", err),
		)
		return
	}
	d.logPermanentFailure(meta, result, "email change verification email permanently failed")
}

func (d operatorLinkDelivery) logPermanentFailure(meta email.DeliveryMetadata, result email.DeliveryResult, message string) {
	if !result.Final || result.Status != email.DeliveryStatusFailed {
		return
	}
	d.logger.Error(message,
		slog.Int64("token_id", meta.ReferenceID),
		slog.String("recipient", identityaccess.MaskOperatorEmail(meta.Recipient)),
		slog.Any("error", result.Err),
	)
}

func operatorTokenDelivery(baseRetry int, result email.DeliveryResult) identityaccess.TokenDelivery {
	delivery := identityaccess.TokenDelivery{RetryCount: baseRetry + result.Attempt}
	if result.Status == email.DeliveryStatusSent {
		sentAt := result.SentAt
		delivery.SentAt = &sentAt
	} else if result.Err != nil {
		message := strings.TrimSpace(result.Err.Error())
		delivery.Error = &message
	}
	return delivery
}
