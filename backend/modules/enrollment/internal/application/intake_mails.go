package application

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// enqueueSubmissionEmails enqueues the parent confirmation and the admin
// notifications in the submission transaction, so the request and every
// delivery intent commit together. Without an outbox nothing is sent.
func (s *Intake) enqueueSubmissionEmails(ctx context.Context, tenantID int64, request *enrollmentModels.Request, children []*RequestChild, statusURL string) error {
	if s.deps.Outbox == nil {
		return nil
	}
	schoolName, logoURL := schoolBrand(ctx, s.deps.Notifications, tenantID, s.deps.ParentsURL)
	footerLogoURL := emailbranding.MotoLogoURL(s.deps.ParentsURL)
	childNames := make([]string, 0, len(children))
	for _, c := range children {
		childNames = append(childNames, fmt.Sprintf("%s %s", c.FirstName, c.LastName))
	}
	parentPayload := map[string]any{
		enrollment.EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
		enrollment.EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
		enrollment.EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
		enrollment.EnrollmentPayloadSchoolName:        schoolName,
		enrollment.EnrollmentPayloadStatusURL:         statusURL,
		enrollment.EnrollmentPayloadLogoURL:           logoURL,
		enrollment.EnrollmentPayloadMotoLogoURL:       footerLogoURL,
		enrollment.EnrollmentPayloadChildNames:        childNames,
		enrollment.EnrollmentPayloadRecipientEmail:    request.GuardianEmail,
	}
	if err := s.deps.Outbox.EnqueueMail(ctx, Mail{
		Kind: enrollment.MailKindSubmitted, Payload: parentPayload,
		RelatedEntityType: enrollment.MailRelatedRequest, RelatedEntityID: request.ID,
	}); err != nil {
		return fmt.Errorf("parent confirmation: %w", err)
	}
	for _, admin := range resolveAdminEmails(s.adminNotificationEmails(ctx)) {
		adminPayload := map[string]any{
			enrollment.EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
			enrollment.EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
			enrollment.EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
			enrollment.EnrollmentPayloadSchoolName:        schoolName,
			enrollment.EnrollmentPayloadAdminURL:          fmt.Sprintf("%s/enrollments/%d", s.deps.FrontendURL, request.ID),
			enrollment.EnrollmentPayloadLogoURL:           logoURL,
			enrollment.EnrollmentPayloadMotoLogoURL:       footerLogoURL,
			enrollment.EnrollmentPayloadChildNames:        childNames,
			enrollment.EnrollmentPayloadRecipientEmail:    admin,
		}
		if request.GuardianPhone != nil {
			adminPayload[enrollment.EnrollmentPayloadGuardianPhone] = *request.GuardianPhone
		}
		if err := s.deps.Outbox.EnqueueMail(ctx, Mail{
			Kind: enrollment.MailKindAdminNotification, Payload: adminPayload,
			RelatedEntityType: enrollment.MailRelatedRequest, RelatedEntityID: request.ID,
		}); err != nil {
			return fmt.Errorf("admin notification for %s: %w", admin, err)
		}
	}
	return nil
}

func (s *Intake) adminNotificationEmails(ctx context.Context) string {
	if s.deps.Settings == nil {
		return ""
	}
	return s.deps.Settings.AdminNotificationEmails(ctx)
}

// resolveAdminEmails parses the comma-separated notification_emails setting.
// Invalid entries are dropped: admins should not miss mails over a trailing
// comma in their configuration.
func resolveAdminEmails(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if _, err := mail.ParseAddress(trimmed); err != nil {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
