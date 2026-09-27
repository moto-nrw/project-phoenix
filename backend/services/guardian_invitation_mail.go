package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/delivery"
)

// The guardian invitation mail: this file writes the outbox payload the
// renderer next door reads at dispatch time. The composition root binds it
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

// EnqueueInvitation queues the mail carrying the accept link. expiresAt
// becomes the "valid for N hours" the mail states, floored at one hour so a
// link that is about to expire never advertises zero.
func (m GuardianInvitationMailer) EnqueueInvitation(ctx context.Context, invitationID int64, token string, expiresAt time.Time, recipient GuardianMailRecipient, schoolName string) (int64, bool) {
	if m.outbox == nil {
		m.logger.Warn("guardian invitation: outbox enqueuer not configured, email skipped",
			slog.Int64("invitation_id", invitationID))
		return 0, false
	}
	expiryHours := int(time.Until(expiresAt) / time.Hour)
	if expiryHours < 1 {
		expiryHours = 1
	}
	payload := m.payload(recipient, "/accept-guardian-invite/"+token, schoolName)
	payload[guardianPayloadExpiryHours] = expiryHours

	stored, err := m.outbox.EnqueueOutboxWithResult(ctx, platformModels.OutboxEnqueueRequest{
		Kind:              platformModels.EmailKindGuardianInvitation,
		Payload:           payload,
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
// an account (#3320): no registration and no token, just the parents portal
// login and a hint to use the existing credentials.
func (m GuardianInvitationMailer) EnqueueExistingAccount(ctx context.Context, recipient GuardianMailRecipient, schoolName string) (int64, bool) {
	if strings.TrimSpace(recipient.Email) == "" {
		return 0, false
	}
	if m.outbox == nil {
		m.logger.Warn("guardian portal access email: outbox enqueuer not configured, email skipped")
		return 0, false
	}
	payload := m.payload(recipient, "/login", schoolName)
	payload[guardianPayloadExistingAccount] = true
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

// EnqueueWelcome queues the welcome that follows a new access (#3534): the
// help article for parents and the Elterninfo. The idempotency key names the
// guardian contact, which belongs to one school, so a guardian is welcomed
// once per school however often they are invited or linked to a child.
//
// It returns the enqueue failure so the caller can roll back its savepoint;
// a guardian who was already welcomed is not a failure.
func (m GuardianInvitationMailer) EnqueueWelcome(ctx context.Context, guardianProfileID int64, recipient GuardianMailRecipient, schoolName, helpURL string, precedingOutboxID int64) error {
	if precedingOutboxID <= 0 {
		return errors.New("guardian welcome email: preceding outbox ID is required")
	}
	if strings.TrimSpace(recipient.Email) == "" {
		return nil
	}
	if m.outbox == nil {
		m.logger.Warn("guardian welcome email: outbox enqueuer not configured, email skipped",
			slog.Int64("guardian_profile_id", guardianProfileID))
		return nil
	}
	payload := m.payload(recipient, "/login", schoolName)
	delete(payload, guardianPayloadInvitationURL)
	payload[guardianPayloadHelpURL] = helpURL
	payload[guardianPayloadParentInfoURL] = portalOrigin(m.frontendURL) + parentInfoPath
	payload[guardianPayloadPrecedingOutboxID] = precedingOutboxID
	err := m.outbox.EnqueueOutbox(ctx, platformModels.OutboxEnqueueRequest{
		Kind:              platformModels.EmailKindGuardianWelcome,
		Payload:           payload,
		RelatedEntityType: platformModels.EmailRelatedTypeGuardianProfile,
		RelatedEntityID:   guardianProfileID,
		IdempotencyKey:    fmt.Sprintf("guardian_welcome:%d", guardianProfileID),
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

// payload builds the fields both variants share. linkPath is appended to the
// parents portal origin.
func (m GuardianInvitationMailer) payload(recipient GuardianMailRecipient, linkPath, schoolName string) map[string]any {
	frontend := portalOrigin(m.frontendURL)
	return map[string]any{
		guardianPayloadRecipientEmail: strings.TrimSpace(recipient.Email),
		guardianPayloadFirstName:      strings.TrimSpace(recipient.FirstName),
		guardianPayloadLastName:       strings.TrimSpace(recipient.LastName),
		guardianPayloadInvitationURL:  frontend + linkPath,
		guardianPayloadLogoURL:        frontend + "/images/moto-logo-mit-schriftzug.png",
		guardianPayloadSchoolName:     schoolName,
	}
}

// GuardianTokenExpiryFallback applies when neither the registry setting nor
// the env var name a guardian token lifetime. 48 hours matches the staff
// invitation default and the registry default.
const GuardianTokenExpiryFallback = 48 * time.Hour

// guardianMailLogger keeps the mailer nil-safe for the bare values tests
// construct.
func guardianMailLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}
