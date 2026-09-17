package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
)

func renderChangeRequestMessage(
	cfg EmailRendererConfig,
	row *emailoutbox.Intent,
	subject string,
	templateName string,
) (*email.Message, error) {
	recipient, _ := row.Payload[EnrollmentPayloadRecipientEmail].(string)
	if recipient == "" {
		return nil, fmt.Errorf("%s payload missing recipient_email", row.Kind)
	}
	schoolName, _ := row.Payload[EnrollmentPayloadSchoolName].(string)
	guardianFirst, _ := row.Payload[EnrollmentPayloadGuardianFirstName].(string)
	guardianLast, _ := row.Payload[EnrollmentPayloadGuardianLastName].(string)
	statusURL, _ := row.Payload[EnrollmentPayloadStatusURL].(string)
	adminURL, _ := row.Payload[EnrollmentPayloadAdminURL].(string)
	logoURL, _ := row.Payload[EnrollmentPayloadLogoURL].(string)
	motoLogoURL, _ := row.Payload[EnrollmentPayloadMotoLogoURL].(string)

	return &email.Message{
		From:     schoolEmailFrom(cfg.DefaultFrom, schoolName),
		To:       email.NewEmail("", recipient),
		Subject:  decisionSubject(subject, schoolName),
		Template: templateName,
		Content: map[string]any{
			"GuardianFirstName": guardianFirst,
			"GuardianLastName":  guardianLast,
			"SchoolName":        schoolName,
			"StatusURL":         statusURL,
			"AdminURL":          adminURL,
			"LogoURL":           logoURL,
			"MotoLogoURL":       motoLogoURL,
		},
	}, nil
}

func NewEnrollmentChangeRequestSubmittedRenderer(cfg EmailRendererConfig) func(context.Context, *emailoutbox.Intent) (*email.Message, error) {
	return func(_ context.Context, row *emailoutbox.Intent) (*email.Message, error) {
		return renderChangeRequestMessage(cfg, row, "Neue Änderungsanfrage", "enrollment-change-request-submitted.html")
	}
}

func NewEnrollmentChangeRequestQuestionRenderer(cfg EmailRendererConfig) func(context.Context, *emailoutbox.Intent) (*email.Message, error) {
	return func(_ context.Context, row *emailoutbox.Intent) (*email.Message, error) {
		return renderChangeRequestMessage(cfg, row, "Rückfrage zu Ihrer Änderungsanfrage", "enrollment-change-request-question.html")
	}
}

func NewEnrollmentChangeRequestParentReplyRenderer(cfg EmailRendererConfig) func(context.Context, *emailoutbox.Intent) (*email.Message, error) {
	return func(_ context.Context, row *emailoutbox.Intent) (*email.Message, error) {
		return renderChangeRequestMessage(cfg, row, "Antwort auf Änderungsanfrage", "enrollment-change-request-parent-reply.html")
	}
}

func NewEnrollmentChangeRequestApprovedRenderer(cfg EmailRendererConfig) func(context.Context, *emailoutbox.Intent) (*email.Message, error) {
	return func(_ context.Context, row *emailoutbox.Intent) (*email.Message, error) {
		return renderChangeRequestMessage(cfg, row, "Änderungsanfrage übernommen", "enrollment-change-request-approved.html")
	}
}

func NewEnrollmentChangeRequestRejectedRenderer(cfg EmailRendererConfig) func(context.Context, *emailoutbox.Intent) (*email.Message, error) {
	return func(_ context.Context, row *emailoutbox.Intent) (*email.Message, error) {
		return renderChangeRequestMessage(cfg, row, "Änderungsanfrage abgelehnt", "enrollment-change-request-rejected.html")
	}
}
