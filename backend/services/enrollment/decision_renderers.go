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
	schoolName, _ := row.Payload[EnrollmentPayloadSchoolName].(string)
	logoURL, _ := row.Payload[EnrollmentPayloadLogoURL].(string)
	motoLogoURL, _ := row.Payload[EnrollmentPayloadMotoLogoURL].(string)
	phaseName, _ := row.Payload[EnrollmentPayloadPhaseName].(string)
	statusReason, _ := row.Payload[EnrollmentPayloadStatusReason].(string)
	childNames := payloadStringSlice(row.Payload, EnrollmentPayloadChildNames)

	return &email.Message{
		From:     schoolEmailFrom(cfg.DefaultFrom, schoolName),
		To:       email.NewEmail("", recipient),
		Subject:  decisionSubject(subject, schoolName),
		Template: templateName,
		Content: map[string]any{
			"GuardianFirstName": guardianFirst,
			"GuardianLastName":  guardianLast,
			"SchoolName":        schoolName,
			"PhaseName":         phaseName,
			"StatusURL":         statusURL,
			"LogoURL":           logoURL,
			"MotoLogoURL":       motoLogoURL,
			"ChildNames":        childNames,
			"StatusReason":      statusReason,
		},
	}, nil
}

func decisionSubject(subject string, schoolName string) string {
	if schoolName == "" {
		return subject
	}
	return fmt.Sprintf("%s - %s", subject, schoolName)
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

// NewEnrollmentDecisionDigestRenderer renders the single request-level
// summary emitted after every child has a parent-visible final decision.
func NewEnrollmentDecisionDigestRenderer(cfg EmailRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		message, err := renderDecisionMessage(cfg, row, "Entscheidung zu Ihrer Anmeldung", "enrollment-decision-digest.html")
		if err != nil {
			return nil, err
		}
		content, ok := message.Content.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s renderer produced invalid content", row.Kind)
		}
		content["ApprovedNames"] = payloadStringSlice(row.Payload, "approved_names")
		content["WaitlistedNames"] = payloadStringSlice(row.Payload, "waitlisted_names")
		content["RejectedNames"] = payloadStringSlice(row.Payload, "rejected_names")
		content["WithdrawnNames"] = payloadStringSlice(row.Payload, "withdrawn_names")
		return message, nil
	}
}
