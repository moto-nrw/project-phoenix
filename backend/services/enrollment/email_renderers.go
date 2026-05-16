package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// Payload keys used by the enrollment outbox rows. Every field needed
// by the email template is captured at enqueue time so the renderer is
// pure (no DB lookups). Keeping the keys named here keeps the
// service-side enqueue and the renderer-side read in lockstep -
// renaming a key requires editing both files in the same diff.
const (
	EnrollmentPayloadGuardianFirstName = "guardian_first_name"
	EnrollmentPayloadGuardianLastName  = "guardian_last_name"
	EnrollmentPayloadGuardianEmail     = "guardian_email"
	EnrollmentPayloadGuardianPhone     = "guardian_phone"
	EnrollmentPayloadSchoolName        = "school_name"
	EnrollmentPayloadStatusURL         = "status_url"
	EnrollmentPayloadAdminURL          = "admin_url"
	EnrollmentPayloadLogoURL           = "logo_url"
	EnrollmentPayloadChildNames        = "child_names"
	EnrollmentPayloadRecipientEmail    = "recipient_email"
)

// EmailRendererConfig captures the closure-state for the enrollment
// renderer functions. Same shape as the guardian invitation renderer.
type EmailRendererConfig struct {
	DefaultFrom email.Email
}

// NewEnrollmentSubmittedRenderer builds the renderer for the parent
// confirmation email. Register at startup with kind
// platform.EmailKindEnrollmentSubmitted.
func NewEnrollmentSubmittedRenderer(cfg EmailRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		recipient, _ := row.Payload[EnrollmentPayloadRecipientEmail].(string)
		if recipient == "" {
			return nil, fmt.Errorf("enrollment submitted payload missing recipient_email")
		}
		statusURL, _ := row.Payload[EnrollmentPayloadStatusURL].(string)
		if statusURL == "" {
			return nil, fmt.Errorf("enrollment submitted payload missing status_url")
		}

		schoolName, _ := row.Payload[EnrollmentPayloadSchoolName].(string)
		guardianFirst, _ := row.Payload[EnrollmentPayloadGuardianFirstName].(string)
		guardianLast, _ := row.Payload[EnrollmentPayloadGuardianLastName].(string)
		logoURL, _ := row.Payload[EnrollmentPayloadLogoURL].(string)
		childNames := payloadStringSlice(row.Payload, EnrollmentPayloadChildNames)

		subject := "Anmeldung eingegangen"
		if schoolName != "" {
			subject = fmt.Sprintf("Anmeldung eingegangen - %s", schoolName)
		}

		return &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: "enrollment-submitted.html",
			Content: map[string]any{
				"GuardianFirstName": guardianFirst,
				"GuardianLastName":  guardianLast,
				"SchoolName":        schoolName,
				"StatusURL":         statusURL,
				"LogoURL":           logoURL,
				"ChildNames":        childNames,
			},
		}, nil
	}
}

// NewEnrollmentAdminNotificationRenderer builds the renderer for the
// admin notification email. Each admin in the
// `enrollment.notification_emails` setting gets one row; the worker
// dispatches them independently.
func NewEnrollmentAdminNotificationRenderer(cfg EmailRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		recipient, _ := row.Payload[EnrollmentPayloadRecipientEmail].(string)
		if recipient == "" {
			return nil, fmt.Errorf("admin notification payload missing recipient_email")
		}
		adminURL, _ := row.Payload[EnrollmentPayloadAdminURL].(string)
		if adminURL == "" {
			return nil, fmt.Errorf("admin notification payload missing admin_url")
		}

		schoolName, _ := row.Payload[EnrollmentPayloadSchoolName].(string)
		guardianFirst, _ := row.Payload[EnrollmentPayloadGuardianFirstName].(string)
		guardianLast, _ := row.Payload[EnrollmentPayloadGuardianLastName].(string)
		guardianEmail, _ := row.Payload[EnrollmentPayloadGuardianEmail].(string)
		guardianPhone, _ := row.Payload[EnrollmentPayloadGuardianPhone].(string)
		logoURL, _ := row.Payload[EnrollmentPayloadLogoURL].(string)
		childNames := payloadStringSlice(row.Payload, EnrollmentPayloadChildNames)

		subject := "Neue Anmeldung eingegangen"
		if schoolName != "" {
			subject = fmt.Sprintf("Neue Anmeldung - %s", schoolName)
		}

		return &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: "enrollment-admin-notification.html",
			Content: map[string]any{
				"GuardianFirstName": guardianFirst,
				"GuardianLastName":  guardianLast,
				"GuardianEmail":     guardianEmail,
				"GuardianPhone":     guardianPhone,
				"SchoolName":        schoolName,
				"AdminURL":          adminURL,
				"LogoURL":           logoURL,
				"ChildNames":        childNames,
			},
		}, nil
	}
}

// Rollover payload key — phase name reuses the existing constant in
// decision_renderers.go (the value is identical: "phase_name").
const (
	EnrollmentPayloadRolloverDeadline = "rollover_deadline"
)

// NewEnrollmentRolloverOptInRenderer builds the renderer for the
// "please confirm next year's enrollment" email. Sent when an admin
// triggers an opt_in rollover; the parent must click through and
// confirm before the deadline or their child is dropped.
//
// Slice 1: uses the existing enrollment-submitted.html template as a
// placeholder — proper opt-in branding lands in a follow-up.
func NewEnrollmentRolloverOptInRenderer(cfg EmailRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		recipient, _ := row.Payload[EnrollmentPayloadRecipientEmail].(string)
		if recipient == "" {
			return nil, fmt.Errorf("rollover opt_in payload missing recipient_email")
		}
		statusURL, _ := row.Payload[EnrollmentPayloadStatusURL].(string)
		if statusURL == "" {
			return nil, fmt.Errorf("rollover opt_in payload missing status_url")
		}

		schoolName, _ := row.Payload[EnrollmentPayloadSchoolName].(string)
		phaseName, _ := row.Payload[EnrollmentPayloadPhaseName].(string)
		guardianFirst, _ := row.Payload[EnrollmentPayloadGuardianFirstName].(string)
		guardianLast, _ := row.Payload[EnrollmentPayloadGuardianLastName].(string)
		logoURL, _ := row.Payload[EnrollmentPayloadLogoURL].(string)
		deadline, _ := row.Payload[EnrollmentPayloadRolloverDeadline].(string)
		childNames := payloadStringSlice(row.Payload, EnrollmentPayloadChildNames)

		subject := "Bitte bestätigen Sie die Anmeldung"
		if phaseName != "" {
			subject = fmt.Sprintf("Bitte bestätigen: Anmeldung %s", phaseName)
		}
		if schoolName != "" && phaseName != "" {
			subject = fmt.Sprintf("Bitte bestätigen: Anmeldung %s – %s", phaseName, schoolName)
		}

		return &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: "enrollment-rollover-opt-in.html",
			Content: map[string]any{
				"GuardianFirstName": guardianFirst,
				"GuardianLastName":  guardianLast,
				"SchoolName":        schoolName,
				"PhaseName":         phaseName,
				"StatusURL":         statusURL,
				"LogoURL":           logoURL,
				"ChildNames":        childNames,
				"RolloverDeadline":  deadline,
			},
		}, nil
	}
}

// NewEnrollmentRolloverOptOutRenderer builds the renderer for the
// "we have pre-registered you" email. Sent when an admin triggers an
// opt_out rollover; the parent only acts if they want to decline.
func NewEnrollmentRolloverOptOutRenderer(cfg EmailRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		recipient, _ := row.Payload[EnrollmentPayloadRecipientEmail].(string)
		if recipient == "" {
			return nil, fmt.Errorf("rollover opt_out payload missing recipient_email")
		}
		statusURL, _ := row.Payload[EnrollmentPayloadStatusURL].(string)
		if statusURL == "" {
			return nil, fmt.Errorf("rollover opt_out payload missing status_url")
		}

		schoolName, _ := row.Payload[EnrollmentPayloadSchoolName].(string)
		phaseName, _ := row.Payload[EnrollmentPayloadPhaseName].(string)
		guardianFirst, _ := row.Payload[EnrollmentPayloadGuardianFirstName].(string)
		guardianLast, _ := row.Payload[EnrollmentPayloadGuardianLastName].(string)
		logoURL, _ := row.Payload[EnrollmentPayloadLogoURL].(string)
		deadline, _ := row.Payload[EnrollmentPayloadRolloverDeadline].(string)
		childNames := payloadStringSlice(row.Payload, EnrollmentPayloadChildNames)

		subject := "Anmeldung wurde verlängert"
		if phaseName != "" {
			subject = fmt.Sprintf("Anmeldung verlängert: %s", phaseName)
		}
		if schoolName != "" && phaseName != "" {
			subject = fmt.Sprintf("Anmeldung verlängert: %s – %s", phaseName, schoolName)
		}

		return &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: "enrollment-rollover-opt-out.html",
			Content: map[string]any{
				"GuardianFirstName": guardianFirst,
				"GuardianLastName":  guardianLast,
				"SchoolName":        schoolName,
				"PhaseName":         phaseName,
				"StatusURL":         statusURL,
				"LogoURL":           logoURL,
				"ChildNames":        childNames,
				"RolloverDeadline":  deadline,
			},
		}, nil
	}
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
