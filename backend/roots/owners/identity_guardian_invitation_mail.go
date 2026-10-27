package owners

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// GuardianMailRecipient is the guardian an invitation mail addresses.
type GuardianMailRecipient struct {
	FirstName string
	LastName  string
	Email     string
}

// GuardianInvitationMailerConfig is what the mail needs beyond the
// invitation: the outbox to queue in and the portal origin the links point
// at. The outbox is required; the composition refuses to start without it.
type GuardianInvitationMailerConfig struct {
	Outbox      MailResultOutbox
	FrontendURL string
	Logger      *slog.Logger
}

// GuardianInvitationMailer queues the two guardian mail variants.
type GuardianInvitationMailer struct {
	outbox      MailResultOutbox
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

// errGuardianMailOutboxMissing reports a mailer composed without the outbox.
// The composition refuses to start without one, so only a bare value reaches
// it; the mail is never skipped silently.
var errGuardianMailOutboxMissing = errors.New("guardian mail: outbox enqueuer not configured")

// EnqueueInvitation queues the mail carrying the accept link. A failure is
// returned so the invitation rolls back with it (#4602).
func (m GuardianInvitationMailer) EnqueueInvitation(ctx context.Context, invitationID int64, token string, expiresAt time.Time, recipient GuardianMailRecipient, schoolName string) (int64, error) {
	if m.outbox == nil {
		return 0, errGuardianMailOutboxMissing
	}
	stored, err := m.outbox.EnqueueOutboxWithResult(ctx, deliveryCompose.MailEnqueueRequest{
		Kind:              identityaccess.MailKindGuardianInvitation,
		Payload:           guardianInvitationPayload(identityaccess.GuardianInvitationMailOf(identityaccess.GuardianMailRecipient(recipient), m.frontendURL, token, expiresAt, schoolName)),
		RelatedEntityType: identityaccess.MailRelatedGuardianInvitation,
		RelatedEntityID:   invitationID,
	})
	if err != nil {
		return 0, fmt.Errorf("guardian invitation %d: outbox enqueue: %w", invitationID, err)
	}
	return stored.ID, nil
}

// EnqueueExistingAccount queues the variant for an address that already owns
// an account (#3320). queued is false only when the recipient has no
// address; an enqueue failure is returned so the access rolls back (#4602).
func (m GuardianInvitationMailer) EnqueueExistingAccount(ctx context.Context, recipient GuardianMailRecipient, schoolName string) (outboxID int64, queued bool, err error) {
	mail, addressed := identityaccess.GuardianExistingAccountMailOf(identityaccess.GuardianMailRecipient(recipient), m.frontendURL, schoolName)
	if !addressed {
		return 0, false, nil
	}
	if m.outbox == nil {
		return 0, false, errGuardianMailOutboxMissing
	}
	stored, err := m.outbox.EnqueueOutboxWithResult(ctx, deliveryCompose.MailEnqueueRequest{
		Kind:    identityaccess.MailKindGuardianInvitation,
		Payload: guardianExistingAccountPayload(mail),
	})
	if err != nil {
		return 0, false, fmt.Errorf("guardian portal access email: outbox enqueue: %w", err)
	}
	return stored.ID, true, nil
}

// EnqueueWelcome queues the welcome that follows a new access (#3534) once
// per guardian contact.
//
// It returns the enqueue failure so the caller can roll back its savepoint;
// a guardian who was already welcomed is not a failure.
func (m GuardianInvitationMailer) EnqueueWelcome(ctx context.Context, guardianProfileID int64, recipient GuardianMailRecipient, schoolName, helpURL string, precedingOutboxID int64) error {
	mail, addressed, err := identityaccess.GuardianWelcomeMailOf(identityaccess.GuardianMailRecipient(recipient), m.frontendURL, schoolName, helpURL,
		identityaccess.ParentInfoURL(m.frontendURL), precedingOutboxID)
	if err != nil || !addressed {
		return err
	}
	if m.outbox == nil {
		return errGuardianMailOutboxMissing
	}
	err = m.outbox.EnqueueOutbox(ctx, deliveryCompose.MailEnqueueRequest{
		Kind:              identityaccess.MailKindGuardianWelcome,
		Payload:           guardianWelcomePayload(mail),
		RelatedEntityType: identityaccess.MailRelatedGuardianProfile,
		RelatedEntityID:   guardianProfileID,
		IdempotencyKey:    identityaccess.GuardianWelcomeIdempotencyKey(guardianProfileID),
	})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, deliveryContract.ErrIdempotencyConflict):
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

// guardianPayload stores the fields every guardian mail shares.
func guardianPayload(mail identityaccess.GuardianMail) map[string]any {
	return map[string]any{
		guardianPayloadRecipientEmail: mail.Recipient,
		guardianPayloadFirstName:      mail.FirstName,
		guardianPayloadLastName:       mail.LastName,
		guardianPayloadInvitationURL:  mail.InvitationURL,
		guardianPayloadLogoURL:        mail.LogoURL,
		guardianPayloadSchoolName:     mail.SchoolName,
	}
}

func guardianInvitationPayload(mail identityaccess.GuardianMail) map[string]any {
	payload := guardianPayload(mail)
	payload[guardianPayloadExpiryHours] = mail.ExpiryHours
	return payload
}

func guardianExistingAccountPayload(mail identityaccess.GuardianMail) map[string]any {
	payload := guardianPayload(mail)
	payload[guardianPayloadExistingAccount] = true
	return payload
}

// guardianWelcomePayload carries no accept link.
func guardianWelcomePayload(mail identityaccess.GuardianMail) map[string]any {
	payload := guardianPayload(mail)
	delete(payload, guardianPayloadInvitationURL)
	payload[guardianPayloadHelpURL] = mail.HelpURL
	payload[guardianPayloadParentInfoURL] = mail.ParentInfoURL
	payload[guardianPayloadPrecedingOutboxID] = mail.PrecedingOutboxID
	return payload
}

// guardianMailLogger keeps the mailer nil-safe for the bare values tests
// construct.
func guardianMailLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}
