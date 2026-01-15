package auth

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
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
		log.Printf("Email dispatcher unavailable; skipping invitation email id=%d", invitation.ID)
		return
	}

	frontend := s.frontendURL
	if frontend == "" {
		frontend = "http://localhost:3000"
	}

	invitationURL := fmt.Sprintf("%s/invite?token=%s", frontend, invitation.Token)
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", frontend)
	expiryHours := int(s.invitationExpiry / time.Hour)

	message := email.Message{
		From:     s.defaultFrom,
		To:       email.NewEmail("", invitation.Email),
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

	meta := email.DeliveryMetadata{
		Type:        "invitation",
		ReferenceID: invitation.ID,
		Token:       invitation.Token,
		Recipient:   invitation.Email,
	}

	baseRetry := invitation.EmailRetryCount

	s.dispatcher.Dispatch(context.Background(), email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: invitationEmailBackoff,
		MaxAttempts:   3,
		Callback: func(cbCtx context.Context, result email.DeliveryResult) {
			s.persistInvitationDelivery(cbCtx, meta, baseRetry, result)
		},
	})
}

// persistInvitationDelivery updates the invitation record with email delivery status.
func (s *invitationService) persistInvitationDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	retryCount := baseRetry + result.Attempt
	var sentAt *time.Time
	var errText *string

	if result.Status == email.DeliveryStatusSent {
		sentTime := result.SentAt
		sentAt = &sentTime
	} else if result.Err != nil {
		msg := sanitizeEmailError(result.Err)
		errText = &msg
	}

	if err := s.invitationRepo.UpdateDeliveryResult(ctx, meta.ReferenceID, sentAt, errText, retryCount); err != nil {
		log.Printf("Failed to update invitation delivery status id=%d err=%v", meta.ReferenceID, err)
		return
	}

	if result.Final && result.Status == email.DeliveryStatusFailed {
		log.Printf("Invitation email permanently failed id=%d recipient=%s err=%v", meta.ReferenceID, meta.Recipient, result.Err)
	}
}

// sanitizeEmailError extracts a clean error message for storage.
func sanitizeEmailError(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}
