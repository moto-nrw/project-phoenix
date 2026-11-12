package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
)

// NewMailRenderers returns Enrollment's mail renderers. Only holiday acceptance
// mails look up account eligibility at delivery time; other fields are captured
// in the outbox payload. Without a lookup or parents URL, no app link is shown.
func NewMailRenderers(accounts enrollment.HolidayAcceptanceAccounts, parentsURL string) enrollment.MailRenderers {
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
		HolidayRegistrationReceived: renderHolidayRegistrationReceived,
		HolidayPlaceGranted:         renderHolidayPlaceGranted,
		HolidayClosureNotice:        renderHolidayClosureNotice,
		HolidayRegistrationAccepted: func(ctx context.Context, kind string, raw json.RawMessage) (*enrollment.RenderedMail, error) {
			return renderHolidayRegistrationAccepted(ctx, kind, raw, accounts, parentsURL)
		},
		HolidayParentAccountOffer: func(ctx context.Context, kind string, raw json.RawMessage) (*enrollment.RenderedMail, error) {
			return renderHolidayParentAccountOffer(ctx, kind, raw, accounts, parentsURL)
		},
	}
}

// renderHolidayRegistrationReceived renders the receipt of a public holiday
// registration from its typed payload. The Träger is the sender.
func renderHolidayRegistrationReceived(_ context.Context, kind string, raw json.RawMessage) (*enrollment.RenderedMail, error) {
	var payload enrollment.HolidayRegistrationMail
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%s payload is not a holiday registration mail: %w", kind, err)
	}
	if strings.TrimSpace(payload.Recipient) == "" {
		return nil, fmt.Errorf("%s payload missing recipient_email", kind)
	}
	subject := "Ferienanmeldung eingegangen"
	if payload.PeriodName != "" {
		subject = "Ferienanmeldung eingegangen: " + payload.PeriodName
	}
	children := make([]map[string]any, 0, len(payload.Children))
	waitlisted, pending := false, false
	for _, child := range payload.Children {
		children = append(children, map[string]any{
			"Name": child.Name, "Guest": child.Guest, "Weeks": child.Weeks, "WaitlistWeeks": child.WaitlistWeeks,
			"PendingWeeks": child.PendingWeeks,
		})
		waitlisted = waitlisted || len(child.WaitlistWeeks) > 0
		pending = pending || len(child.PendingWeeks) > 0
	}
	return &enrollment.RenderedMail{
		SenderName: schoolSender(payload.TraegerName),
		Recipient:  payload.Recipient,
		Subject:    subject,
		Template:   "holiday-registration-received.html",
		Content: map[string]any{
			"GuardianName": payload.GuardianName,
			"SchoolName":   payload.TraegerName,
			"PeriodName":   payload.PeriodName,
			"Binding":      payload.Binding,
			"Children":     children,
			"Waitlisted":   waitlisted,
			"Pending":      pending,
			"Declaration":  holidayDeclarationContent(payload.Declaration),
		},
	}, nil
}

// renderHolidayPlaceGranted renders the mail that a child moved up from the
// waiting list of a holiday week (#4699). The Träger is the sender.
func renderHolidayPlaceGranted(_ context.Context, kind string, raw json.RawMessage) (*enrollment.RenderedMail, error) {
	var payload enrollment.HolidayPlaceGrantedMail
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%s payload is not a holiday place mail: %w", kind, err)
	}
	if strings.TrimSpace(payload.Recipient) == "" {
		return nil, fmt.Errorf("%s payload missing recipient_email", kind)
	}
	subject := "Ferienplatz frei geworden"
	if payload.Allocated {
		// The Büro decided on the places by criteria (#4700).
		subject = "Ferienplätze vergeben"
	}
	if payload.PeriodName != "" {
		subject += ": " + payload.PeriodName
	}
	children := make([]map[string]any, 0, len(payload.Children))
	waitlisted := false
	for _, child := range payload.Children {
		children = append(children, map[string]any{"Name": child.Name, "Guest": child.Guest, "Weeks": child.Weeks, "WaitlistWeeks": child.WaitlistWeeks})
		waitlisted = waitlisted || len(child.WaitlistWeeks) > 0
	}
	return &enrollment.RenderedMail{
		SenderName: schoolSender(payload.TraegerName),
		Recipient:  payload.Recipient,
		Subject:    subject,
		Template:   "holiday-place-granted.html",
		Content: map[string]any{
			"GuardianName": payload.GuardianName,
			"SchoolName":   payload.TraegerName,
			"PeriodName":   payload.PeriodName,
			"Binding":      payload.Binding,
			"Children":     children,
			"Allocated":    payload.Allocated,
			"Waitlisted":   waitlisted,
		},
	}, nil
}

// renderHolidayClosureNotice renders the mail that a booked holiday week has
// days without care (#6434) or days that open again (#6457), in two
// sections when a closing day moved: one mail, not two (ADR 0196). The
// Träger is the sender.
func renderHolidayClosureNotice(_ context.Context, kind string, raw json.RawMessage) (*enrollment.RenderedMail, error) {
	var payload enrollment.HolidayClosureMail
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%s payload is not a holiday closure mail: %w", kind, err)
	}
	if strings.TrimSpace(payload.Recipient) == "" {
		return nil, fmt.Errorf("%s payload missing recipient_email", kind)
	}
	payload.ContainsPastDays = payload.ContainsPastDays || holidayClosureMailHasPastDays(payload, calendar.TodayDate())
	closed, closedWaitlisted, closedPending := holidayClosureMailChildren(payload.Children)
	reopened, reopenedWaitlisted, reopenedPending := holidayClosureMailChildren(payload.ReopenedChildren)
	subject := "Ferienbetreuung: An einem gebuchten Tag ist geschlossen"
	switch {
	case payload.ContainsPastDays:
		subject = "Ferienbetreuung: Hinweis zu vergangenen Schließtagen"
	case len(closed) > 0 && len(reopened) > 0:
		subject = "Ferienbetreuung: Die Schließtage haben sich geändert"
	case len(reopened) > 0:
		subject = "Ferienbetreuung: Die Betreuung findet doch statt"
	}
	return &enrollment.RenderedMail{
		SenderName: schoolSender(payload.TraegerName),
		Recipient:  payload.Recipient,
		Subject:    subject,
		Template:   "holiday-closure-notice.html",
		Content: map[string]any{
			"GuardianName":      payload.GuardianName,
			"SchoolName":        payload.TraegerName,
			"Reasons":           payload.Reasons,
			"Children":          closed,
			"ReopenedChildren":  reopened,
			"ContainsPastDays":  payload.ContainsPastDays,
			"Waitlisted":        closedWaitlisted || reopenedWaitlisted,
			"Pending":           closedPending || reopenedPending,
			"KeepsContribution": payload.KeepsContribution,
		},
	}, nil
}

// holidayClosureMailChildren lays the children of one section of the
// closing day mail out for the template.
func holidayClosureMailChildren(mailChildren []enrollment.HolidayClosureMailChild) ([]map[string]any, bool, bool) {
	children := make([]map[string]any, 0, len(mailChildren))
	waitlisted, pending := false, false
	for _, child := range mailChildren {
		weeks := make([]map[string]any, 0, len(child.Weeks))
		for _, week := range child.Weeks {
			weeks = append(weeks, map[string]any{"Week": week.Week, "ClosedDays": week.ClosedDays, "Waitlisted": week.Waitlisted, "Pending": week.Pending})
			waitlisted = waitlisted || week.Waitlisted
			pending = pending || week.Pending
		}
		children = append(children, map[string]any{"Name": child.Name, "Guest": child.Guest, "Weeks": weeks})
	}
	return children, waitlisted, pending
}

// renderHolidayRegistrationAccepted renders the acceptance mail of a
// confirmed holiday registration (#6672). The Träger is the sender; its
// text is plain text, which the template escapes and keeps the lines of.
func renderHolidayRegistrationAccepted(ctx context.Context, kind string, raw json.RawMessage, accounts enrollment.HolidayAcceptanceAccounts, parentsURL string) (*enrollment.RenderedMail, error) {
	var payload enrollment.HolidayRegistrationAcceptedMail
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%s payload is not a holiday acceptance mail: %w", kind, err)
	}
	if strings.TrimSpace(payload.Recipient) == "" {
		return nil, fmt.Errorf("%s payload missing recipient_email", kind)
	}
	var err error
	if payload.ParentAppURL, payload.SetupURL, err = holidayAcceptanceLinks(ctx, accounts, parentsURL, payload); err != nil {
		return nil, fmt.Errorf("%s: %w", kind, err)
	}
	subject := "Ferienanmeldung bestätigt"
	if payload.PeriodName != "" {
		subject += ": " + payload.PeriodName
	}
	children := make([]map[string]any, 0, len(payload.Children))
	waitlisted := false
	for _, child := range payload.Children {
		children = append(children, map[string]any{
			"Name": child.Name, "Guest": child.Guest, "Weeks": child.Weeks, "WaitlistWeeks": child.WaitlistWeeks,
			"PendingWeeks": child.PendingWeeks,
		})
		waitlisted = waitlisted || len(child.WaitlistWeeks) > 0
	}
	return &enrollment.RenderedMail{
		SenderName: schoolSender(payload.TraegerName),
		Recipient:  payload.Recipient,
		Subject:    subject,
		Template:   "holiday-registration-accepted.html",
		Content: map[string]any{
			"GuardianName": payload.GuardianName,
			"SchoolName":   payload.TraegerName,
			"PeriodName":   payload.PeriodName,
			"Binding":      payload.Binding,
			"Children":     children,
			"Waitlisted":   waitlisted,
			"TraegerText":  payload.TraegerText,
			"ParentAppURL": payload.ParentAppURL,
			"SetupURL":     payload.SetupURL,
		},
	}, nil
}

// holidayAcceptanceLinks are the links of the acceptance mail, decided at
// sending: the parents portal for an address with a parent account, the
// setup of a parent account at the Träger for every other address (ADR 0216
// point 11). Links from the outbox are never trusted. Account eligibility is
// checked at delivery time, after the confirmation's Träger transaction has
// ended. Without a lookup or a parents URL the mail has no link.
func holidayAcceptanceLinks(ctx context.Context, accounts enrollment.HolidayAcceptanceAccounts, parentsURL string,
	payload enrollment.HolidayRegistrationAcceptedMail) (appURL, setupURL string, err error) {
	if accounts == nil || strings.TrimSpace(parentsURL) == "" {
		return "", "", nil
	}
	parent, err := accounts.HasParentAccount(ctx, payload.Recipient)
	if err != nil {
		return "", "", fmt.Errorf("find the parent account: %w", err)
	}
	if parent {
		return strings.TrimRight(strings.TrimSpace(parentsURL), "/") + "/", "", nil
	}
	if payload.OrganizationID <= 0 {
		return "", "", nil
	}
	setupURL, err = accounts.HolidayParentSetupURL(payload.Recipient, payload.OrganizationID)
	if err != nil {
		return "", "", fmt.Errorf("issue the setup link of a parent account: %w", err)
	}
	return "", setupURL, nil
}

// renderHolidayParentAccountOffer renders the offer of a parent account a
// holiday registration mails (ADR 0216 point 4). The Träger is the sender.
// The mail is decided when it is sent, by the rule of the acceptance mail:
// an address with a parent account gets the sign-in of the parents portal
// only, every other address the setup link, issued now and never taken from
// the outbox, with the sign-in as a quiet second way. The mail goes to the
// owner of the address alone; the form never tells.
func renderHolidayParentAccountOffer(ctx context.Context, kind string, raw json.RawMessage, accounts enrollment.HolidayAcceptanceAccounts, parentsURL string) (*enrollment.RenderedMail, error) {
	var payload enrollment.HolidayParentAccountOfferMail
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%s payload is not a holiday parent account offer: %w", kind, err)
	}
	if strings.TrimSpace(payload.Recipient) == "" {
		return nil, fmt.Errorf("%s payload missing recipient_email", kind)
	}
	parentsURL = strings.TrimRight(strings.TrimSpace(parentsURL), "/")
	if accounts == nil || parentsURL == "" {
		return nil, fmt.Errorf("%s: the parents portal and the account lookup are required", kind)
	}
	parent, err := accounts.HasParentAccount(ctx, payload.Recipient)
	if err != nil {
		return nil, fmt.Errorf("%s: find the parent account: %w", kind, err)
	}
	setupURL := ""
	if !parent && payload.OrganizationID > 0 {
		if setupURL, err = accounts.HolidayParentSetupURL(payload.Recipient, payload.OrganizationID); err != nil {
			return nil, fmt.Errorf("%s: issue the setup link of a parent account: %w", kind, err)
		}
	}
	return &enrollment.RenderedMail{
		SenderName: schoolSender(payload.TraegerName),
		Recipient:  payload.Recipient,
		Subject:    "Ihr Elternkonto für die Ferienbetreuung",
		Template:   "holiday-parent-account-offer.html",
		Content: map[string]any{
			"GuardianName": payload.GuardianName,
			"SchoolName":   payload.TraegerName,
			"PeriodName":   payload.PeriodName,
			"SetupURL":     setupURL,
			"LoginURL":     parentsURL + "/login",
		},
	}, nil
}

// holidayDeclarationContent is the section "Ihre Erklärung" of the receipt
// (#5626): the typed name, the Berlin time it was given, the version of the
// texts and every declaration in its words. A mail enqueued before has none.
func holidayDeclarationContent(declaration *enrollment.HolidayRegistrationMailDeclaration) map[string]any {
	if declaration == nil {
		return nil
	}
	statements := make([]map[string]any, 0, len(declaration.Statements))
	for _, statement := range declaration.Statements {
		statements = append(statements, map[string]any{"Title": statement.Title, "Text": statement.Text, "Detail": statement.Detail})
	}
	return map[string]any{
		"Name":       declaration.Name,
		"GivenAt":    declaration.GivenAt.In(calendar.Berlin).Format("02.01.2006 um 15:04 Uhr"),
		"Version":    declaration.ConsentVersion,
		"Statements": statements,
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

// careFacilityBadge is the text of the 48 px logo stand-in in the mail
// header (#6042); the parent message mail shares the rule (#6082).
func careFacilityBadge(term string) string { return textnorm.Badge(term) }

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
			"CareFacilityTerm":  payloadString(payload, enrollment.EnrollmentPayloadCareFacilityTerm),
			"CareFacilityBadge": careFacilityBadge(payloadString(payload, enrollment.EnrollmentPayloadCareFacilityTerm)),
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
			"CareFacilityTerm":  payloadString(payload, enrollment.EnrollmentPayloadCareFacilityTerm),
			"CareFacilityBadge": careFacilityBadge(payloadString(payload, enrollment.EnrollmentPayloadCareFacilityTerm)),
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
				"CareFacilityTerm":  payloadString(payload, enrollment.EnrollmentPayloadCareFacilityTerm),
				"CareFacilityBadge": careFacilityBadge(payloadString(payload, enrollment.EnrollmentPayloadCareFacilityTerm)),
				"ChildNames":        payloadStringSlice(payload, enrollment.EnrollmentPayloadChildNames),
				"RolloverDeadline":  payloadString(payload, enrollment.EnrollmentPayloadRolloverDeadline),
				// A payload enqueued before #6456 has no flag and keeps the
				// old wording.
				"RenewalTermsRequired": payload[enrollment.EnrollmentPayloadRenewalTermsRequired] == true,
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
			"CareFacilityTerm":  payloadString(payload, enrollment.EnrollmentPayloadCareFacilityTerm),
			"CareFacilityBadge": careFacilityBadge(payloadString(payload, enrollment.EnrollmentPayloadCareFacilityTerm)),
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
				"CareFacilityTerm":  payloadString(payload, enrollment.EnrollmentPayloadCareFacilityTerm),
				"CareFacilityBadge": careFacilityBadge(payloadString(payload, enrollment.EnrollmentPayloadCareFacilityTerm)),
			},
		}, nil
	}
}
