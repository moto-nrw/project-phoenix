package application

import (
	"context"
	"fmt"
	"log/slog"
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
	admins, err := s.adminNotificationRecipients(ctx)
	if err != nil {
		return fmt.Errorf("admin notification recipients: %w", err)
	}
	for _, admin := range admins {
		adminPayload := map[string]any{
			enrollment.EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
			enrollment.EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
			enrollment.EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
			enrollment.EnrollmentPayloadSchoolName:        schoolName,
			enrollment.EnrollmentPayloadAdminURL:          fmt.Sprintf("%s/admin/enrollments/%d", s.deps.FrontendURL, request.ID),
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

// adminNotificationRecipients is the configured address list plus, when
// bound, the staff who switched the mail on.
func (s *Intake) adminNotificationRecipients(ctx context.Context) ([]string, error) {
	configured := resolveAdminEmails(s.adminNotificationEmails(ctx))
	if s.deps.AdminSubscribers == nil {
		return configured, nil
	}
	var recipients []string
	err := s.deps.Runtime.Savepoint(ctx, func(txCtx context.Context) error {
		var lookupErr error
		recipients, lookupErr = s.deps.AdminSubscribers.AdminNotificationRecipients(txCtx, configured)
		return lookupErr
	})
	if err == nil {
		return recipients, nil
	}
	if s.deps.Runtime.IsSavepointControl(err) {
		return nil, err
	}
	s.logger().Warn("enrollment admin subscriber lookup failed", slog.String("error", err.Error()))
	return deduplicateAdminEmails(configured), nil
}

func deduplicateAdminEmails(addresses []string) []string {
	seen := make(map[string]bool, len(addresses))
	unique := make([]string, 0, len(addresses))
	for _, address := range addresses {
		key := strings.ToLower(address)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, address)
		}
	}
	return unique
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
