package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// Payload keys specific to the parent-decision emails. Submission
// payload keys (guardian first/last name, school name, status URL,
// child names) are reused via the constants in email_renderers.go.
const (
	EnrollmentPayloadPhaseName    = "phase_name"
	EnrollmentPayloadStatusReason = "status_reason"
)

// renderDecisionMessage is the shared body builder for the three
// decision emails. They share enough chrome that a single template
// closure beats three near-identical ones; the per-status renderer
// just picks subject + template name.
func renderDecisionMessage(
	cfg EmailRendererConfig,
	row *platformModels.EmailOutbox,
	subject string,
	templateName string,
) (*email.Message, error) {
	recipient, _ := row.Payload[EnrollmentPayloadRecipientEmail].(string)
	if recipient == "" {
		return nil, fmt.Errorf("%s payload missing recipient_email", row.Kind)
	}
	statusURL, _ := row.Payload[EnrollmentPayloadStatusURL].(string)
	if statusURL == "" {
		return nil, fmt.Errorf("%s payload missing status_url", row.Kind)
	}

	guardianFirst, _ := row.Payload[EnrollmentPayloadGuardianFirstName].(string)
	guardianLast, _ := row.Payload[EnrollmentPayloadGuardianLastName].(string)
	logoURL, _ := row.Payload[EnrollmentPayloadLogoURL].(string)
	phaseName, _ := row.Payload[EnrollmentPayloadPhaseName].(string)
	statusReason, _ := row.Payload[EnrollmentPayloadStatusReason].(string)
	childNames := payloadStringSlice(row.Payload, EnrollmentPayloadChildNames)

	return &email.Message{
		From:     cfg.DefaultFrom,
		To:       email.NewEmail("", recipient),
		Subject:  subject,
		Template: templateName,
		Content: map[string]any{
			"GuardianFirstName": guardianFirst,
			"GuardianLastName":  guardianLast,
			"PhaseName":         phaseName,
			"StatusURL":         statusURL,
			"LogoURL":           logoURL,
			"ChildNames":        childNames,
			"StatusReason":      statusReason,
		},
	}, nil
}

// NewEnrollmentApprovedRenderer renders the "your child is in" email.
// Triggered by DecisionService when a child transitions to status
// 'approved'. Register at startup with kind
// platform.EmailKindEnrollmentApproved.
func NewEnrollmentApprovedRenderer(cfg EmailRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		return renderDecisionMessage(
			cfg, row,
			"Anmeldung bestätigt",
			"enrollment-approved.html",
		)
	}
}

// NewEnrollmentWaitlistedRenderer renders the "you're on the waitlist"
// email. Phase capacity overflow + admin manual waitlisting both fire
// this renderer.
func NewEnrollmentWaitlistedRenderer(cfg EmailRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		return renderDecisionMessage(
			cfg, row,
			"Anmeldung auf Warteliste",
			"enrollment-waitlisted.html",
		)
	}
}

// NewEnrollmentRejectedRenderer renders the "we couldn't accommodate
// your child" email. Reason text is included only when the phase has
// show_status_reason_to_parent enabled (the service strips it from
// the payload otherwise).
func NewEnrollmentRejectedRenderer(cfg EmailRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		return renderDecisionMessage(
			cfg, row,
			"Anmeldung abgelehnt",
			"enrollment-rejected.html",
		)
	}
}
