package api

import (
	"context"
	"log/slog"
	"strings"
	"time"

	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
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

// operatorLinkWiring is the configuration the operator link deliveries are
// composed with. dispatcher may be nil: the link is then not mailed, as
// before.
type operatorLinkWiring struct {
	dispatcher  email.Dispatcher
	durable     DurableMailOutbox
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
	logger *slog.Logger,
) *identityaccessCompose.OperatorProvisioningDependencies {
	if wiring == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	backoff := wiring.backoff
	return &identityaccessCompose.OperatorProvisioningDependencies{
		DeliveryFor: func(tokens identityaccess.OperatorLinkDeliveryRecorder) identityaccessCompose.OperatorLinkDelivery {
			return operatorLinkDelivery{
				dispatcher: wiring.dispatcher, durable: wiring.durable, from: wiring.defaultFrom,
				frontendURL: wiring.frontendURL, operatorURL: wiring.operatorFrontendURL,
				backoff: backoff, tokens: tokens, logger: logger,
			}
		},
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

// operatorLinkRecords records the outcome of the operator mails that carry a
// link in Identity & Access.
type operatorLinkRecords interface {
	RecordOperatorInvitationDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error
	RecordOperatorEmailChangeDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error
}

// operatorLinkDelivery mails the operator invitation and the three e-mail
// change messages, and records the outcome of the two that carry a link
// back through the module once the send settles.
type operatorLinkDelivery struct {
	dispatcher  email.Dispatcher
	durable     DurableMailOutbox
	from        email.Email
	frontendURL string
	operatorURL string
	backoff     []time.Duration
	tokens      operatorLinkRecords
	logger      *slog.Logger
}

func (d operatorLinkDelivery) DispatchOperatorInvitation(ctx context.Context, invitation identityaccess.OperatorInvitation, inviterName string, expiry time.Duration) error {
	if d.dispatcher == nil {
		return nil
	}
	message := *identityMessage(d.from, email.NewEmail("", invitation.Email), identityaccess.OperatorInvitationMail(invitation, inviterName, d.frontendURL, d.operatorURL, expiry))
	return enqueueDurableMail(ctx, d.durable, DurableMail{
		Message: message, Type: "operator_invitation", ReferenceID: invitation.ID,
		Backoff: d.backoff, BaseRetry: invitation.Delivery.RetryCount, ExpiresAt: invitation.ExpiresAt,
	})
}

func (d operatorLinkDelivery) DispatchOperatorEmailChangeVerification(ctx context.Context, change identityaccess.OperatorEmailChange, expiry time.Duration) error {
	if d.dispatcher == nil {
		return nil
	}
	message := *identityMessage(d.from, email.NewEmail("", change.NewEmail), identityaccess.OperatorEmailChangeVerificationMail(change, d.frontendURL, expiry))
	return enqueueDurableMail(ctx, d.durable, DurableMail{
		Message: message, Type: "operator_email_change_verify", ReferenceID: change.ID,
		Backoff: d.backoff, BaseRetry: change.Delivery.RetryCount, ExpiresAt: change.Expiry,
	})
}

func (d operatorLinkDelivery) DispatchOperatorEmailChangeRequested(ctx context.Context, operator identityaccess.Operator, maskedNewEmail string) error {
	if d.dispatcher == nil {
		return nil
	}
	return enqueueDurableMail(ctx, d.durable, DurableMail{
		Message: *identityMessage(d.from, email.NewEmail(operator.DisplayName, operator.Email), identityaccess.OperatorEmailChangeRequestedMail(operator, maskedNewEmail, d.frontendURL)),
		Type:    "operator_email_change_requested", ReferenceID: operator.ID, Backoff: d.backoff,
	})
}

func (d operatorLinkDelivery) DispatchOperatorEmailChangeConfirmed(ctx context.Context, oldEmail, displayName string) error {
	if d.dispatcher == nil {
		return nil
	}
	return enqueueDurableMail(ctx, d.durable, DurableMail{
		Message: *identityMessage(d.from, email.NewEmail(displayName, oldEmail), identityaccess.OperatorEmailChangeConfirmedMail(displayName, d.frontendURL)),
		Type:    "operator_email_change_confirmed", Backoff: d.backoff,
	})
}

func (d operatorLinkDelivery) recordInvitationDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	delivery := operatorTokenDelivery(baseRetry, result)
	if err := d.tokens.RecordOperatorInvitationDelivery(ctx, meta.ReferenceID, delivery); err != nil {
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
	if err := d.tokens.RecordOperatorEmailChangeDelivery(ctx, meta.ReferenceID, delivery); err != nil {
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
		slog.String("recipient", identityaccessCompose.MaskEmail(meta.Recipient)),
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
