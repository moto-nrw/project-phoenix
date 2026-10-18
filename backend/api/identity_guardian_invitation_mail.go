package api

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	platformModels "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose/vocabulary"
)

// The guardian invitation mail: this file queues the outbox payload
// Identity & Access writes, which the renderer next door hands back to it at
// dispatch time. The composition root binds it
// as the owner module's delivery seam (#2722). The email_sent_at /
// email_error / email_retry_count columns of auth.guardian_invitations stay
// NULL; delivery state lives in platform.email_outbox, queryable by
// (related_entity_type='guardian_invitation', related_entity_id).

// GuardianMailRecipient is the guardian an invitation mail addresses.
type GuardianMailRecipient struct {
	FirstName string
	LastName  string
	Email     string
}

// GuardianInvitationMailerConfig is what the mail needs beyond the
// invitation: the outbox to queue in and the portal origin the links point
// at. A nil outbox skips the send with a warning.
type GuardianInvitationMailerConfig struct {
	Outbox      platformModels.OutboxResultEnqueuer
	FrontendURL string
	Logger      *slog.Logger
}

// GuardianInvitationMailer queues the two guardian mail variants.
type GuardianInvitationMailer struct {
	outbox      platformModels.OutboxResultEnqueuer
	frontendURL string
	logger      *slog.Logger
}

func NewGuardianInvitationMailer(cfg GuardianInvitationMailerConfig) GuardianInvitationMailer {
	return GuardianInvitationMailer{
		outbox:      cfg.Outbox,
		frontendURL: strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/"),
		logger:      guardianMailLogger(cfg.Logger),
	}
}

// EnqueueInvitation queues the mail carrying the accept link.
func (m GuardianInvitationMailer) EnqueueInvitation(ctx context.Context, invitationID int64, token string, expiresAt time.Time, recipient GuardianMailRecipient, schoolName string) (int64, bool) {
	if m.outbox == nil {
		m.logger.Warn("guardian invitation: outbox enqueuer not configured, email skipped",
			slog.Int64("invitation_id", invitationID))
		return 0, false
	}
	stored, err := m.outbox.EnqueueOutboxWithResult(ctx, platformModels.OutboxEnqueueRequest{
		Kind:              platformModels.EmailKindGuardianInvitation,
		Payload:           identityaccess.GuardianInvitationPayload(identityaccess.GuardianMailRecipient(recipient), m.frontendURL, token, expiresAt, schoolName),
		RelatedEntityType: platformModels.EmailRelatedTypeGuardianInvitation,
		RelatedEntityID:   invitationID,
	})
	if err != nil {
		m.logger.Error("guardian invitation: outbox enqueue failed",
			slog.Int64("invitation_id", invitationID),
			slog.String("error", err.Error()))
		return 0, false
	}
	return stored.ID, true
}

// EnqueueExistingAccount queues the variant for an address that already owns
// an account (#3320).
func (m GuardianInvitationMailer) EnqueueExistingAccount(ctx context.Context, recipient GuardianMailRecipient, schoolName string) (int64, bool) {
	payload := identityaccess.GuardianExistingAccountPayload(identityaccess.GuardianMailRecipient(recipient), m.frontendURL, schoolName)
	if payload == nil {
		return 0, false
	}
	if m.outbox == nil {
		m.logger.Warn("guardian portal access email: outbox enqueuer not configured, email skipped")
		return 0, false
	}
	stored, err := m.outbox.EnqueueOutboxWithResult(ctx, platformModels.OutboxEnqueueRequest{
		Kind:    platformModels.EmailKindGuardianInvitation,
		Payload: payload,
	})
	if err != nil {
		m.logger.Error("guardian portal access email: outbox enqueue failed",
			slog.String("error", err.Error()))
		return 0, false
	}
	return stored.ID, true
}

// EnqueueWelcome queues the welcome that follows a new access (#3534) once
// per guardian contact.
//
// It returns the enqueue failure so the caller can roll back its savepoint;
// a guardian who was already welcomed is not a failure.
func (m GuardianInvitationMailer) EnqueueWelcome(ctx context.Context, guardianProfileID int64, recipient GuardianMailRecipient, schoolName, helpURL string, precedingOutboxID int64) error {
	payload, err := identityaccess.GuardianWelcomePayload(identityaccess.GuardianMailRecipient(recipient), m.frontendURL, schoolName, helpURL,
		identityaccess.MailOrigin(m.frontendURL)+parentInfoPath, precedingOutboxID)
	if err != nil || payload == nil {
		return err
	}
	if m.outbox == nil {
		m.logger.Warn("guardian welcome email: outbox enqueuer not configured, email skipped",
			slog.Int64("guardian_profile_id", guardianProfileID))
		return nil
	}
	err = m.outbox.EnqueueOutbox(ctx, platformModels.OutboxEnqueueRequest{
		Kind:              platformModels.EmailKindGuardianWelcome,
		Payload:           payload,
		RelatedEntityType: platformModels.EmailRelatedTypeGuardianProfile,
		RelatedEntityID:   guardianProfileID,
		IdempotencyKey:    identityaccess.GuardianWelcomeIdempotencyKey(guardianProfileID),
	})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, delivery.ErrIdempotencyConflict):
		// Welcomed before, with other details (a new name or new settings):
		// the guardian already has the mail.
		m.logger.Debug("guardian welcome email already queued",
			slog.Int64("guardian_profile_id", guardianProfileID))
		return nil
	default:
		m.logger.Error("guardian welcome email: outbox enqueue failed",
			slog.Int64("guardian_profile_id", guardianProfileID),
			slog.String("error", err.Error()))
		return err
	}
}

// guardianMailLogger keeps the mailer nil-safe for the bare values tests
// construct.
func guardianMailLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}
