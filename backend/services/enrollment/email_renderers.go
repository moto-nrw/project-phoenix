package enrollment

import (
	"context"
	"fmt"
	"strings"

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
	EnrollmentPayloadMotoLogoURL       = "moto_logo_url"
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
		motoLogoURL, _ := row.Payload[EnrollmentPayloadMotoLogoURL].(string)
		childNames := payloadStringSlice(row.Payload, EnrollmentPayloadChildNames)

		subject := "Anmeldung eingegangen"
		if schoolName != "" {
			subject = fmt.Sprintf("Anmeldung eingegangen - %s", schoolName)
		}

		return &email.Message{
			From:     schoolEmailFrom(cfg.DefaultFrom, schoolName),
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: "enrollment-submitted.html",
			Content: map[string]any{
				"GuardianFirstName": guardianFirst,
				"GuardianLastName":  guardianLast,
				"SchoolName":        schoolName,
				"StatusURL":         statusURL,
				"LogoURL":           logoURL,
				"MotoLogoURL":       motoLogoURL,
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
		motoLogoURL, _ := row.Payload[EnrollmentPayloadMotoLogoURL].(string)
		childNames := payloadStringSlice(row.Payload, EnrollmentPayloadChildNames)

		subject := "Neue Anmeldung eingegangen"
		if schoolName != "" {
			subject = fmt.Sprintf("Neue Anmeldung - %s", schoolName)
		}

		return &email.Message{
			From:     schoolEmailFrom(cfg.DefaultFrom, schoolName),
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
				"MotoLogoURL":       motoLogoURL,
				"ChildNames":        childNames,
			},
		}, nil
	}
}

// Rollover payload key: phase name reuses the existing constant in
// decision_renderers.go (the value is identical: "phase_name").
const (
	EnrollmentPayloadRolloverDeadline = "rollover_deadline"
)

// NewEnrollmentRolloverOptInRenderer builds the renderer for the
// "please confirm next year's enrollment" email. Sent when an admin
// triggers an opt_in rollover; the parent must click through and
// confirm before the deadline or their child is dropped.
func NewEnrollmentRolloverOptInRenderer(cfg EmailRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return newRolloverRenderer(cfg, rolloverRendererSpec{
		errLabel:         "opt_in",
		template:         "enrollment-rollover-opt-in.html",
		bareSubject:      "Bitte bestätigen Sie die Anmeldung",
		phaseSubject:     "Bitte bestätigen: Anmeldung %s",
		phaseFullSubject: "Bitte bestätigen: Anmeldung %s - %s",
	})
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

func newRolloverRenderer(cfg EmailRendererConfig, spec rolloverRendererSpec) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		recipient, _ := row.Payload[EnrollmentPayloadRecipientEmail].(string)
		if recipient == "" {
			return nil, fmt.Errorf("rollover %s payload missing recipient_email", spec.errLabel)
		}
		statusURL, _ := row.Payload[EnrollmentPayloadStatusURL].(string)
		if statusURL == "" {
			return nil, fmt.Errorf("rollover %s payload missing status_url", spec.errLabel)
		}

		schoolName, _ := row.Payload[EnrollmentPayloadSchoolName].(string)
		phaseName, _ := row.Payload[EnrollmentPayloadPhaseName].(string)
		guardianFirst, _ := row.Payload[EnrollmentPayloadGuardianFirstName].(string)
		guardianLast, _ := row.Payload[EnrollmentPayloadGuardianLastName].(string)
		logoURL, _ := row.Payload[EnrollmentPayloadLogoURL].(string)
		motoLogoURL, _ := row.Payload[EnrollmentPayloadMotoLogoURL].(string)
		deadline, _ := row.Payload[EnrollmentPayloadRolloverDeadline].(string)
		childNames := payloadStringSlice(row.Payload, EnrollmentPayloadChildNames)

		subject := spec.bareSubject
		if phaseName != "" {
			subject = fmt.Sprintf(spec.phaseSubject, phaseName)
		}
		if schoolName != "" && phaseName != "" {
			subject = fmt.Sprintf(spec.phaseFullSubject, phaseName, schoolName)
		}

		return &email.Message{
			From:     schoolEmailFrom(cfg.DefaultFrom, schoolName),
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: spec.template,
			Content: map[string]any{
				"GuardianFirstName": guardianFirst,
				"GuardianLastName":  guardianLast,
				"SchoolName":        schoolName,
				"PhaseName":         phaseName,
				"StatusURL":         statusURL,
				"LogoURL":           logoURL,
				"MotoLogoURL":       motoLogoURL,
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
	return newRolloverRenderer(cfg, rolloverRendererSpec{
		errLabel:         "opt_out",
		template:         "enrollment-rollover-opt-out.html",
		bareSubject:      "Anmeldung wurde verlängert",
		phaseSubject:     "Anmeldung verlängert: %s",
		phaseFullSubject: "Anmeldung verlängert: %s - %s",
	})
}

func schoolEmailFrom(defaultFrom email.Email, schoolName string) email.Email {
	schoolName = strings.TrimSpace(schoolName)
	if schoolName == "" {
		return defaultFrom
	}
	return email.NewEmail(schoolName, defaultFrom.Address)
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
