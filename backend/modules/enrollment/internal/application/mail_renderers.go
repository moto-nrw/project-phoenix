package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// NewMailRenderers returns Enrollment's mail renderers. Every field a
// template needs is captured in the payload at enqueue time, so the
// renderers are pure (no DB lookups).
func NewMailRenderers() enrollment.MailRenderers {
	return enrollment.MailRenderers{
		Submitted:                decodingRenderer(renderSubmitted),
		AdminNotification:        decodingRenderer(renderAdminNotification),
		Approved:                 decodingRenderer(decisionRenderer("Anmeldung bestätigt", "enrollment-approved.html")),
		Waitlisted:               decodingRenderer(decisionRenderer("Anmeldung auf Warteliste", "enrollment-waitlisted.html")),
		Rejected:                 decodingRenderer(decisionRenderer("Anmeldung abgelehnt", "enrollment-rejected.html")),
		DecisionDigest:           decodingRenderer(renderDecisionDigest),
		ChangeRequestSubmitted:   decodingRenderer(changeRequestRenderer("Neue Änderungsanfrage", "enrollment-change-request-submitted.html")),
		ChangeRequestQuestion:    decodingRenderer(changeRequestRenderer("Rückfrage zu Ihrer Änderungsanfrage", "enrollment-change-request-question.html")),
		ChangeRequestParentReply: decodingRenderer(changeRequestRenderer("Antwort auf Änderungsanfrage", "enrollment-change-request-parent-reply.html")),
		ChangeRequestApproved:    decodingRenderer(changeRequestRenderer("Änderungsanfrage übernommen", "enrollment-change-request-approved.html")),
		ChangeRequestRejected:    decodingRenderer(changeRequestRenderer("Änderungsanfrage abgelehnt", "enrollment-change-request-rejected.html")),
		RolloverOptIn: decodingRenderer(rolloverRenderer(rolloverRendererSpec{
			errLabel:         "opt_in",
			template:         "enrollment-rollover-opt-in.html",
			bareSubject:      "Bitte bestätigen Sie die Anmeldung",
			phaseSubject:     "Bitte bestätigen: Anmeldung %s",
			phaseFullSubject: "Bitte bestätigen: Anmeldung %s - %s",
		})),
		RolloverOptOut: decodingRenderer(rolloverRenderer(rolloverRendererSpec{
			errLabel:         "opt_out",
			template:         "enrollment-rollover-opt-out.html",
			bareSubject:      "Anmeldung wurde verlängert",
			phaseSubject:     "Anmeldung verlängert: %s",
			phaseFullSubject: "Anmeldung verlängert: %s - %s",
		})),
	}
}

// payloadRenderer renders one mail kind from its decoded payload.
type payloadRenderer func(kind string, payload map[string]any) (*enrollment.RenderedMail, error)

// decodingRenderer decodes the claimed intent's JSON payload, the same
// document the outbox stored, before rendering it.
func decodingRenderer(render payloadRenderer) enrollment.MailRenderer {
	return func(_ context.Context, kind string, raw json.RawMessage) (*enrollment.RenderedMail, error) {
		var payload map[string]any
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &payload); err != nil {
				return nil, fmt.Errorf("%s payload is not a JSON object: %w", kind, err)
			}
		}
		return render(kind, payload)
	}
}

// payloadString reads one string field of a payload; a missing or
// non-string value reads as "".
func payloadString(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}

// payloadStringSlice extracts a string slice from a JSONB roundtrip.
// JSON arrays come back as []any, so we coerce element-wise.
func payloadStringSlice(payload map[string]any, key string) []string {
	v, ok := payload[key]
	if !ok {
		return nil
	}
	if direct, ok := v.([]string); ok {
		return direct
	}
	if anys, ok := v.([]any); ok {
		out := make([]string, 0, len(anys))
		for _, a := range anys {
			if s, ok := a.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// schoolSender is the sender name of a mail: the school's name, or "" for
// the platform's default sender.
func schoolSender(schoolName string) string {
	return strings.TrimSpace(schoolName)
}

// renderSubmitted renders the parent confirmation email.
func renderSubmitted(_ string, payload map[string]any) (*enrollment.RenderedMail, error) {
	recipient := payloadString(payload, enrollment.EnrollmentPayloadRecipientEmail)
	if recipient == "" {
		return nil, fmt.Errorf("enrollment submitted payload missing recipient_email")
	}
	statusURL := payloadString(payload, enrollment.EnrollmentPayloadStatusURL)
	if statusURL == "" {
		return nil, fmt.Errorf("enrollment submitted payload missing status_url")
	}

	schoolName := payloadString(payload, enrollment.EnrollmentPayloadSchoolName)
	subject := "Anmeldung eingegangen"
	if schoolName != "" {
		subject = fmt.Sprintf("Anmeldung eingegangen - %s", schoolName)
	}

	return &enrollment.RenderedMail{
		SenderName: schoolSender(schoolName),
		Recipient:  recipient,
		Subject:    subject,
		Template:   "enrollment-submitted.html",
		Content: map[string]any{
			"GuardianFirstName": payloadString(payload, enrollment.EnrollmentPayloadGuardianFirstName),
			"GuardianLastName":  payloadString(payload, enrollment.EnrollmentPayloadGuardianLastName),
			"SchoolName":        schoolName,
			"StatusURL":         statusURL,
			"LogoURL":           payloadString(payload, enrollment.EnrollmentPayloadLogoURL),
			"MotoLogoURL":       payloadString(payload, enrollment.EnrollmentPayloadMotoLogoURL),
			"ChildNames":        payloadStringSlice(payload, enrollment.EnrollmentPayloadChildNames),
		},
	}, nil
}

// renderAdminNotification renders the admin notification email. Each admin
// in the enrollment.notification_emails setting gets one row; the worker
// dispatches them independently.
func renderAdminNotification(_ string, payload map[string]any) (*enrollment.RenderedMail, error) {
	recipient := payloadString(payload, enrollment.EnrollmentPayloadRecipientEmail)
	if recipient == "" {
		return nil, fmt.Errorf("admin notification payload missing recipient_email")
	}
	adminURL := payloadString(payload, enrollment.EnrollmentPayloadAdminURL)
	if adminURL == "" {
		return nil, fmt.Errorf("admin notification payload missing admin_url")
	}

	schoolName := payloadString(payload, enrollment.EnrollmentPayloadSchoolName)
	subject := "Neue Anmeldung eingegangen"
	if schoolName != "" {
		subject = fmt.Sprintf("Neue Anmeldung - %s", schoolName)
	}

	return &enrollment.RenderedMail{
		SenderName: schoolSender(schoolName),
		Recipient:  recipient,
		Subject:    subject,
		Template:   "enrollment-admin-notification.html",
		Content: map[string]any{
			"GuardianFirstName": payloadString(payload, enrollment.EnrollmentPayloadGuardianFirstName),
			"GuardianLastName":  payloadString(payload, enrollment.EnrollmentPayloadGuardianLastName),
			"GuardianEmail":     payloadString(payload, enrollment.EnrollmentPayloadGuardianEmail),
			"GuardianPhone":     payloadString(payload, enrollment.EnrollmentPayloadGuardianPhone),
			"SchoolName":        schoolName,
			"AdminURL":          adminURL,
			"LogoURL":           payloadString(payload, enrollment.EnrollmentPayloadLogoURL),
			"MotoLogoURL":       payloadString(payload, enrollment.EnrollmentPayloadMotoLogoURL),
			"ChildNames":        payloadStringSlice(payload, enrollment.EnrollmentPayloadChildNames),
		},
	}, nil
}

// rolloverRendererSpec carries the per-variant strings of the two rollover
// emails; everything else (payload extraction, subject fallbacks, message
// assembly) is identical.
type rolloverRendererSpec struct {
	errLabel         string // "opt_in" / "opt_out" in the payload errors
	template         string
	bareSubject      string
	phaseSubject     string // fmt with phase name
	phaseFullSubject string // fmt with phase + school name
}

// rolloverRenderer renders the "please confirm next year's enrollment"
// (opt_in) or "we have pre-registered you" (opt_out) email an admin's
// rollover triggers.
func rolloverRenderer(spec rolloverRendererSpec) payloadRenderer {
	return func(_ string, payload map[string]any) (*enrollment.RenderedMail, error) {
		recipient := payloadString(payload, enrollment.EnrollmentPayloadRecipientEmail)
		if recipient == "" {
			return nil, fmt.Errorf("rollover %s payload missing recipient_email", spec.errLabel)
		}
		statusURL := payloadString(payload, enrollment.EnrollmentPayloadStatusURL)
		if statusURL == "" {
			return nil, fmt.Errorf("rollover %s payload missing status_url", spec.errLabel)
		}

		schoolName := payloadString(payload, enrollment.EnrollmentPayloadSchoolName)
		phaseName := payloadString(payload, enrollment.EnrollmentPayloadPhaseName)
		subject := spec.bareSubject
		if phaseName != "" {
			subject = fmt.Sprintf(spec.phaseSubject, phaseName)
		}
		if schoolName != "" && phaseName != "" {
			subject = fmt.Sprintf(spec.phaseFullSubject, phaseName, schoolName)
		}

		return &enrollment.RenderedMail{
			SenderName: schoolSender(schoolName),
			Recipient:  recipient,
			Subject:    subject,
			Template:   spec.template,
			Content: map[string]any{
				"GuardianFirstName": payloadString(payload, enrollment.EnrollmentPayloadGuardianFirstName),
				"GuardianLastName":  payloadString(payload, enrollment.EnrollmentPayloadGuardianLastName),
				"SchoolName":        schoolName,
				"PhaseName":         phaseName,
				"StatusURL":         statusURL,
				"LogoURL":           payloadString(payload, enrollment.EnrollmentPayloadLogoURL),
				"MotoLogoURL":       payloadString(payload, enrollment.EnrollmentPayloadMotoLogoURL),
				"ChildNames":        payloadStringSlice(payload, enrollment.EnrollmentPayloadChildNames),
				"RolloverDeadline":  payloadString(payload, enrollment.EnrollmentPayloadRolloverDeadline),
			},
		}, nil
	}
}

// renderDecisionMail is the shared body builder for the decision emails.
// They share enough chrome that a single template closure beats
// near-identical ones; the per-status renderer just picks subject +
// template name.
func renderDecisionMail(kind string, payload map[string]any, subject, templateName string) (*enrollment.RenderedMail, error) {
	recipient := payloadString(payload, enrollment.EnrollmentPayloadRecipientEmail)
	if recipient == "" {
		return nil, fmt.Errorf("%s payload missing recipient_email", kind)
	}
	statusURL := payloadString(payload, enrollment.EnrollmentPayloadStatusURL)
	if statusURL == "" {
		return nil, fmt.Errorf("%s payload missing status_url", kind)
	}

	schoolName := payloadString(payload, enrollment.EnrollmentPayloadSchoolName)
	return &enrollment.RenderedMail{
		SenderName: schoolSender(schoolName),
		Recipient:  recipient,
		Subject:    decisionSubject(subject, schoolName),
		Template:   templateName,
		Content: map[string]any{
			"GuardianFirstName": payloadString(payload, enrollment.EnrollmentPayloadGuardianFirstName),
			"GuardianLastName":  payloadString(payload, enrollment.EnrollmentPayloadGuardianLastName),
			"SchoolName":        schoolName,
			"PhaseName":         payloadString(payload, enrollment.EnrollmentPayloadPhaseName),
			"StatusURL":         statusURL,
			"LogoURL":           payloadString(payload, enrollment.EnrollmentPayloadLogoURL),
			"MotoLogoURL":       payloadString(payload, enrollment.EnrollmentPayloadMotoLogoURL),
			"ChildNames":        payloadStringSlice(payload, enrollment.EnrollmentPayloadChildNames),
			"StatusReason":      payloadString(payload, enrollment.EnrollmentPayloadStatusReason),
		},
	}, nil
}

func decisionSubject(subject string, schoolName string) string {
	if schoolName == "" {
		return subject
	}
	return fmt.Sprintf("%s - %s", subject, schoolName)
}

// decisionRenderer renders one standalone decision email. The rejection
// carries its reason only when the phase has show_status_reason_to_parent
// enabled (the notification strips it from the payload otherwise).
func decisionRenderer(subject, templateName string) payloadRenderer {
	return func(kind string, payload map[string]any) (*enrollment.RenderedMail, error) {
		return renderDecisionMail(kind, payload, subject, templateName)
	}
}

// renderDecisionDigest renders the single request-level summary emitted
// after every child has a parent-visible final decision.
func renderDecisionDigest(kind string, payload map[string]any) (*enrollment.RenderedMail, error) {
	mail, err := renderDecisionMail(kind, payload, "Entscheidung zu Ihrer Anmeldung", "enrollment-decision-digest.html")
	if err != nil {
		return nil, err
	}
	content, ok := mail.Content.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s renderer produced invalid content", kind)
	}
	content["ApprovedNames"] = payloadStringSlice(payload, "approved_names")
	content["WaitlistedNames"] = payloadStringSlice(payload, "waitlisted_names")
	content["RejectedNames"] = payloadStringSlice(payload, "rejected_names")
	content["WithdrawnNames"] = payloadStringSlice(payload, "withdrawn_names")
	return mail, nil
}

// changeRequestRenderer renders one of the change-request emails.
func changeRequestRenderer(subject, templateName string) payloadRenderer {
	return func(kind string, payload map[string]any) (*enrollment.RenderedMail, error) {
		recipient := payloadString(payload, enrollment.EnrollmentPayloadRecipientEmail)
		if recipient == "" {
			return nil, fmt.Errorf("%s payload missing recipient_email", kind)
		}
		schoolName := payloadString(payload, enrollment.EnrollmentPayloadSchoolName)
		return &enrollment.RenderedMail{
			SenderName: schoolSender(schoolName),
			Recipient:  recipient,
			Subject:    decisionSubject(subject, schoolName),
			Template:   templateName,
			Content: map[string]any{
				"GuardianFirstName": payloadString(payload, enrollment.EnrollmentPayloadGuardianFirstName),
				"GuardianLastName":  payloadString(payload, enrollment.EnrollmentPayloadGuardianLastName),
				"SchoolName":        schoolName,
				"StatusURL":         payloadString(payload, enrollment.EnrollmentPayloadStatusURL),
				"AdminURL":          payloadString(payload, enrollment.EnrollmentPayloadAdminURL),
				"LogoURL":           payloadString(payload, enrollment.EnrollmentPayloadLogoURL),
				"MotoLogoURL":       payloadString(payload, enrollment.EnrollmentPayloadMotoLogoURL),
			},
		}, nil
	}
}
