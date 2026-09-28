package application

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The change-request mails go out after the change is committed, each in a
// transaction of its own. A failed enqueue is logged, never returned: the
// change itself stands.

func (s *ChangeRequests) emailNotificationsEnabled(ctx context.Context) bool {
	return s.deps.Settings != nil && s.deps.Settings.ChangeRequestMailsEnabled(ctx)
}

// enqueueAdminNotification fans a change-request mail out to every admin
// recipient, with the admin deep link in the payload.
func (s *ChangeRequests) enqueueAdminNotification(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64, kind string) {
	if s.deps.Outbox == nil {
		return
	}
	err := s.deps.Runtime.TenantTx(ctx, tenantID, func(txCtx context.Context) error {
		if !s.emailNotificationsEnabled(txCtx) {
			return nil
		}
		for _, admin := range resolveAdminEmails(s.adminNotificationEmails(txCtx)) {
			payload := s.emailPayload(txCtx, req, changeRequestID, admin)
			payload[enrollment.EnrollmentPayloadAdminURL] = s.adminURL(changeRequestID)
			s.enqueueChangeRequestMail(txCtx, tenantID, req, changeRequestID, kind, payload)
		}
		return nil
	})
	if err != nil {
		s.logChangeRequestNotificationFailure(err, tenantID, req.ID, changeRequestID, kind, "tenant_tx")
	}
}

// enqueueParentNotification sends a change-request mail to the guardian.
func (s *ChangeRequests) enqueueParentNotification(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64, kind string) {
	if s.deps.Outbox == nil {
		return
	}
	err := s.deps.Runtime.TenantTx(ctx, tenantID, func(txCtx context.Context) error {
		if !s.emailNotificationsEnabled(txCtx) {
			return nil
		}
		payload := s.emailPayload(txCtx, req, changeRequestID, req.GuardianEmail)
		s.enqueueChangeRequestMail(txCtx, tenantID, req, changeRequestID, kind, payload)
		return nil
	})
	if err != nil {
		s.logChangeRequestNotificationFailure(err, tenantID, req.ID, changeRequestID, kind, "tenant_tx")
	}
}

func (s *ChangeRequests) enqueueChangeRequestMail(ctx context.Context, tenantID int64, req *enrollmentModels.Request, changeRequestID int64, kind string, payload map[string]any) {
	if err := s.deps.Outbox.EnqueueMail(ctx, Mail{
		Kind: kind, Payload: payload, RelatedEntityType: enrollment.MailRelatedRequest, RelatedEntityID: req.ID,
	}); err != nil {
		s.logChangeRequestNotificationFailure(err, tenantID, req.ID, changeRequestID, kind, "enqueue")
	}
}

func (s *ChangeRequests) adminNotificationEmails(ctx context.Context) string {
	if s.deps.Settings == nil {
		return ""
	}
	return s.deps.Settings.AdminNotificationEmails(ctx)
}

// logChangeRequestNotificationFailure names the request, not the recipient,
// whose address stays out of the log (#2108).
func (s *ChangeRequests) logChangeRequestNotificationFailure(err error, tenantID, requestID, changeRequestID int64, kind, stage string) {
	if err == nil || s.deps.Logger == nil {
		return
	}
	s.deps.Logger.Warn("change request notification enqueue failed",
		slog.String("stage", stage),
		slog.Int64("tenant_id", tenantID),
		slog.Int64("request_id", requestID),
		slog.Int64("change_request_id", changeRequestID),
		slog.String("kind", kind),
		slog.String("error", err.Error()),
	)
}

func (s *ChangeRequests) emailPayload(ctx context.Context, req *enrollmentModels.Request, changeRequestID int64, recipient string) map[string]any {
	schoolName, logoURL := schoolBrand(ctx, s.deps.Notifications, req.TenantID, s.deps.ParentsURL)
	return map[string]any{
		enrollment.EnrollmentPayloadGuardianFirstName: req.GuardianFirstName,
		enrollment.EnrollmentPayloadGuardianLastName:  req.GuardianLastName,
		enrollment.EnrollmentPayloadGuardianEmail:     req.GuardianEmail,
		enrollment.EnrollmentPayloadSchoolName:        schoolName,
		enrollment.EnrollmentPayloadStatusURL:         enrollment.StatusURL(s.deps.ParentsURL, req.StatusToken),
		enrollment.EnrollmentPayloadLogoURL:           logoURL,
		enrollment.EnrollmentPayloadMotoLogoURL:       emailbranding.MotoLogoURL(s.deps.ParentsURL),
		enrollment.EnrollmentPayloadRecipientEmail:    recipient,
		"change_request_id":                           strconv.FormatInt(changeRequestID, 10),
	}
}

func (s *ChangeRequests) adminURL(changeRequestID int64) string {
	if s.deps.FrontendURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/admin/enrollments/change-requests/%d", s.deps.FrontendURL, changeRequestID)
}
