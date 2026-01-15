package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/mailer"
	authModels "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	"github.com/moto-nrw/project-phoenix/internal/core/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

// systemRoleTranslations maps English system role names to German display names.
// Used for user-facing content like emails.
var systemRoleTranslations = map[string]string{
	"admin": "Administrator",
	"user":  "Nutzer",
	"guest": "Gast",
}

// translateRoleNameToGerman translates system role names to German.
// Falls back to the original name if no translation exists.
func translateRoleNameToGerman(roleName string) string {
	if translated, ok := systemRoleTranslations[strings.ToLower(roleName)]; ok {
		return translated
	}
	return roleName
}

// invitationEmailBackoff defines retry intervals for invitation emails.
var invitationEmailBackoff = []time.Duration{
	time.Second,
	5 * time.Second,
	15 * time.Second,
}

// sendInvitationEmail queues an invitation email for async delivery.
func (s *invitationService) sendInvitationEmail(invitation *authModels.InvitationToken, roleName string) {
	if s.dispatcher == nil {
		if logger.Logger != nil {
			logger.Logger.WithField("invitation_id", invitation.ID).Warn("Email dispatcher unavailable; skipping invitation email")
		}
		return
	}

	invitationURL := fmt.Sprintf("%s/invite?token=%s", s.frontendURL, invitation.Token)
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", s.frontendURL)
	expiryHours := int(s.invitationExpiry / time.Hour)

	message := port.EmailMessage{
		From:     s.defaultFrom,
		To:       port.EmailAddress{Address: invitation.Email},
		Subject:  "Einladung zu moto",
		Template: "invitation.html",
		Content: map[string]any{
			"InvitationURL": invitationURL,
			"RoleName":      translateRoleNameToGerman(roleName),
			"FirstName":     invitation.FirstName,
			"LastName":      invitation.LastName,
			"ExpiryHours":   expiryHours,
			"LogoURL":       logoURL,
		},
	}

	meta := mailer.DeliveryMetadata{
		Type:        "invitation",
		ReferenceID: invitation.ID,
		Token:       invitation.Token,
		Recipient:   invitation.Email,
	}

	baseRetry := invitation.EmailRetryCount

	s.dispatcher.Dispatch(context.Background(), mailer.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: invitationEmailBackoff,
		MaxAttempts:   3,
		Callback: func(cbCtx context.Context, result mailer.DeliveryResult) {
			s.persistInvitationDelivery(cbCtx, meta, baseRetry, result)
		},
	})
}

// persistInvitationDelivery updates the invitation record with email delivery status.
func (s *invitationService) persistInvitationDelivery(ctx context.Context, meta mailer.DeliveryMetadata, baseRetry int, result mailer.DeliveryResult) {
	retryCount := baseRetry + result.Attempt
	var sentAt *time.Time
	var errText *string

	if result.Status == mailer.DeliveryStatusSent {
		sentTime := result.SentAt
		sentAt = &sentTime
	} else if result.Err != nil {
		msg := sanitizeEmailError(result.Err)
		errText = &msg
	}

	if err := s.invitationRepo.UpdateDeliveryResult(ctx, meta.ReferenceID, sentAt, errText, retryCount); err != nil {
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"invitation_id": meta.ReferenceID,
				"error":         err.Error(),
			}).Error("Failed to update invitation delivery status")
		}
		return
	}

	if result.Final && result.Status == mailer.DeliveryStatusFailed {
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"invitation_id": meta.ReferenceID,
				"recipient":     meta.Recipient,
				"error":         result.Err.Error(),
			}).Error("Invitation email permanently failed")
		}
	}
}

// sanitizeEmailError extracts a clean error message for storage.
func sanitizeEmailError(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}
