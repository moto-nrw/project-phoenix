package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every enrollment email renderer reads payload values that were
// captured at enqueue time. The two non-negotiable fields per
// renderer are recipient_email (mandatory — without it there's no
// addressee) plus the URL the email's CTA links to (status_url for
// the parent emails, admin_url for the admin notification).

// outboxRow is one claimed outbox intent as a renderer receives it.
type outboxRow struct {
	Kind    string
	Payload map[string]any
}

func render(renderer enrollment.MailRenderer, row *outboxRow) (*enrollment.RenderedMail, error) {
	payload, err := json.Marshal(row.Payload)
	if err != nil {
		return nil, err
	}
	return renderer(context.Background(), row.Kind, payload)
}

// contentOf is the template document a renderer produced.
func contentOf(t *testing.T, mail *enrollment.RenderedMail) map[string]any {
	t.Helper()
	content, ok := mail.Content.(map[string]any)
	require.True(t, ok, "the renderer produces a template map")
	return content
}

func TestRenderersRejectAPayloadThatIsNotAnObject(t *testing.T) {
	t.Parallel()
	_, err := NewMailRenderers().Submitted(context.Background(), "enrollment_submitted", json.RawMessage(`[1,2]`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enrollment_submitted payload is not a JSON object")
}

func validSubmittedRow() *outboxRow {
	return &outboxRow{
		Kind: "enrollment_submitted",
		Payload: map[string]any{
			enrollment.EnrollmentPayloadRecipientEmail:    "guardian@example.test",
			enrollment.EnrollmentPayloadStatusURL:         "https://parents.localhost:3000/status/abc",
			enrollment.EnrollmentPayloadGuardianFirstName: "Anna",
			enrollment.EnrollmentPayloadGuardianLastName:  "Beispiel",
			enrollment.EnrollmentPayloadSchoolName:        "OGS Sonnenschule",
			enrollment.EnrollmentPayloadLogoURL:           "https://logo",
			enrollment.EnrollmentPayloadMotoLogoURL:       "https://moto-logo",
			enrollment.EnrollmentPayloadChildNames:        []string{"Lara", "Tim"},
		},
	}
}

func validAdminRow() *outboxRow {
	return &outboxRow{
		Kind: "enrollment_admin_notification",
		Payload: map[string]any{
			enrollment.EnrollmentPayloadRecipientEmail:    "admin@example.test",
			enrollment.EnrollmentPayloadAdminURL:          "https://admin.localhost:3000/enrollment/42",
			enrollment.EnrollmentPayloadGuardianFirstName: "Anna",
			enrollment.EnrollmentPayloadGuardianLastName:  "Beispiel",
			enrollment.EnrollmentPayloadGuardianEmail:     "guardian@example.test",
			enrollment.EnrollmentPayloadGuardianPhone:     "+49 170 1234567",
			enrollment.EnrollmentPayloadSchoolName:        "OGS Sonnenschule",
			enrollment.EnrollmentPayloadLogoURL:           "https://logo",
			enrollment.EnrollmentPayloadMotoLogoURL:       "https://moto-logo",
			enrollment.EnrollmentPayloadChildNames:        []string{"Lara"},
		},
	}
}

func validRolloverRow() *outboxRow {
	return &outboxRow{
		Kind: "enrollment_rollover",
		Payload: map[string]any{
			enrollment.EnrollmentPayloadRecipientEmail:    "guardian@example.test",
			enrollment.EnrollmentPayloadStatusURL:         "https://parents.localhost:3000/status/abc",
			enrollment.EnrollmentPayloadGuardianFirstName: "Anna",
			enrollment.EnrollmentPayloadGuardianLastName:  "Beispiel",
			enrollment.EnrollmentPayloadSchoolName:        "OGS Sonnenschule",
			enrollment.EnrollmentPayloadPhaseName:         "Schuljahr 2027/28",
			enrollment.EnrollmentPayloadLogoURL:           "https://logo",
			enrollment.EnrollmentPayloadMotoLogoURL:       "https://moto-logo",
			enrollment.EnrollmentPayloadRolloverDeadline:  "2027-06-30",
			enrollment.EnrollmentPayloadChildNames:        []string{"Lara"},
		},
	}
}

func validDecisionRow(kind string) *outboxRow {
	return &outboxRow{
		Kind: kind,
		Payload: map[string]any{
			enrollment.EnrollmentPayloadRecipientEmail:    "guardian@example.test",
			enrollment.EnrollmentPayloadStatusURL:         "https://parents.localhost:3000/status/abc",
			enrollment.EnrollmentPayloadGuardianFirstName: "Anna",
			enrollment.EnrollmentPayloadGuardianLastName:  "Beispiel",
			enrollment.EnrollmentPayloadSchoolName:        "OGS Sonnenschule",
			enrollment.EnrollmentPayloadLogoURL:           "https://logo",
			enrollment.EnrollmentPayloadMotoLogoURL:       "https://moto-logo",
			enrollment.EnrollmentPayloadPhaseName:         "Schuljahr 2026/27",
			enrollment.EnrollmentPayloadStatusReason:      "Kapazität erschöpft",
			enrollment.EnrollmentPayloadChildNames:        []string{"Lara", "Tim"},
		},
	}
}

// --- submitted ------------------------------------------------------------

func TestSubmittedRendererRejectsMissingRecipient(t *testing.T) {
	t.Parallel()
	row := validSubmittedRow()
	delete(row.Payload, enrollment.EnrollmentPayloadRecipientEmail)
	_, err := render(NewMailRenderers().Submitted, row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recipient_email")
}

func TestSubmittedRendererRejectsMissingStatusURL(t *testing.T) {
	t.Parallel()
	row := validSubmittedRow()
	delete(row.Payload, enrollment.EnrollmentPayloadStatusURL)
	_, err := render(NewMailRenderers().Submitted, row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status_url")
}

func TestSubmittedRendererHappyPath(t *testing.T) {
	t.Parallel()
	mail, err := render(NewMailRenderers().Submitted, validSubmittedRow())
	require.NoError(t, err)
	assert.Equal(t, "enrollment-submitted.html", mail.Template)
	assert.Equal(t, "Anmeldung eingegangen - OGS Sonnenschule", mail.Subject)
	assert.Equal(t, "OGS Sonnenschule", mail.SenderName)
	assert.Equal(t, "guardian@example.test", mail.Recipient)
	assert.Equal(t, []string{"Lara", "Tim"}, contentOf(t, mail)["ChildNames"])
}

func TestSubmittedRendererNoSchoolBareSubject(t *testing.T) {
	t.Parallel()
	row := validSubmittedRow()
	row.Payload[enrollment.EnrollmentPayloadSchoolName] = ""
	mail, err := render(NewMailRenderers().Submitted, row)
	require.NoError(t, err)
	assert.Equal(t, "Anmeldung eingegangen", mail.Subject)
	assert.Empty(t, mail.SenderName, "no school name keeps the default sender")
}

// --- admin notification ---------------------------------------------------

func TestAdminNotificationRendererRejectsMissingRecipient(t *testing.T) {
	t.Parallel()
	row := validAdminRow()
	delete(row.Payload, enrollment.EnrollmentPayloadRecipientEmail)
	_, err := render(NewMailRenderers().AdminNotification, row)
	require.Error(t, err)
}

func TestAdminNotificationRendererRejectsMissingAdminURL(t *testing.T) {
	t.Parallel()
	row := validAdminRow()
	delete(row.Payload, enrollment.EnrollmentPayloadAdminURL)
	_, err := render(NewMailRenderers().AdminNotification, row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "admin_url")
}

func TestAdminNotificationRendererHappyPath(t *testing.T) {
	t.Parallel()
	mail, err := render(NewMailRenderers().AdminNotification, validAdminRow())
	require.NoError(t, err)
	assert.Equal(t, "enrollment-admin-notification.html", mail.Template)
	assert.Equal(t, "Neue Anmeldung - OGS Sonnenschule", mail.Subject)
	assert.Equal(t, "guardian@example.test", contentOf(t, mail)["GuardianEmail"])
	assert.Equal(t, "+49 170 1234567", contentOf(t, mail)["GuardianPhone"])
}

func TestAdminNotificationRendererNoSchoolBareSubject(t *testing.T) {
	t.Parallel()
	row := validAdminRow()
	row.Payload[enrollment.EnrollmentPayloadSchoolName] = ""
	mail, err := render(NewMailRenderers().AdminNotification, row)
	require.NoError(t, err)
	assert.Equal(t, "Neue Anmeldung eingegangen", mail.Subject)
}

// --- rollover ---------------------------------------------------------------

func TestRolloverRenderersRejectMissingRecipientAndStatusURL(t *testing.T) {
	t.Parallel()
	renderers := NewMailRenderers()
	for name, renderer := range map[string]enrollment.MailRenderer{"opt_in": renderers.RolloverOptIn, "opt_out": renderers.RolloverOptOut} {
		for _, key := range []string{enrollment.EnrollmentPayloadRecipientEmail, enrollment.EnrollmentPayloadStatusURL} {
			row := validRolloverRow()
			delete(row.Payload, key)
			_, err := render(renderer, row)
			require.Error(t, err, "%s without %s", name, key)
			assert.Contains(t, err.Error(), "rollover "+name+" payload missing")
		}
	}
}

func TestRolloverOptInRendererHappyPath(t *testing.T) {
	t.Parallel()
	mail, err := render(NewMailRenderers().RolloverOptIn, validRolloverRow())
	require.NoError(t, err)
	assert.Equal(t, "enrollment-rollover-opt-in.html", mail.Template)
	assert.Equal(t, "Bitte bestätigen: Anmeldung Schuljahr 2027/28 - OGS Sonnenschule", mail.Subject)
	assert.Equal(t, "2027-06-30", contentOf(t, mail)["RolloverDeadline"])
}

func TestRolloverOptInRendererSubjectFallbacks(t *testing.T) {
	t.Parallel()
	row := validRolloverRow()
	row.Payload[enrollment.EnrollmentPayloadSchoolName] = ""
	mail, err := render(NewMailRenderers().RolloverOptIn, row)
	require.NoError(t, err)
	assert.Equal(t, "Bitte bestätigen: Anmeldung Schuljahr 2027/28", mail.Subject)

	row.Payload[enrollment.EnrollmentPayloadPhaseName] = ""
	mail, err = render(NewMailRenderers().RolloverOptIn, row)
	require.NoError(t, err)
	assert.Equal(t, "Bitte bestätigen Sie die Anmeldung", mail.Subject)
}

func TestRolloverOptOutRendererHappyPath(t *testing.T) {
	t.Parallel()
	mail, err := render(NewMailRenderers().RolloverOptOut, validRolloverRow())
	require.NoError(t, err)
	assert.Equal(t, "enrollment-rollover-opt-out.html", mail.Template)
	assert.Equal(t, "Anmeldung verlängert: Schuljahr 2027/28 - OGS Sonnenschule", mail.Subject)
}

func TestRolloverOptOutRendererSubjectFallbacks(t *testing.T) {
	t.Parallel()
	row := validRolloverRow()
	row.Payload[enrollment.EnrollmentPayloadSchoolName] = ""
	mail, err := render(NewMailRenderers().RolloverOptOut, row)
	require.NoError(t, err)
	assert.Equal(t, "Anmeldung verlängert: Schuljahr 2027/28", mail.Subject)

	row.Payload[enrollment.EnrollmentPayloadPhaseName] = ""
	mail, err = render(NewMailRenderers().RolloverOptOut, row)
	require.NoError(t, err)
	assert.Equal(t, "Anmeldung wurde verlängert", mail.Subject)
}

// --- decisions ------------------------------------------------------------

// decisionSubject appends the school name (if present) so parents
// recognise *which* school the decision concerns when they're juggling
// multiple kids across schools. Without a school name the bare subject
// still works for tenants that left the name blank.
func TestDecisionSubject(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Anmeldung bestätigt", decisionSubject("Anmeldung bestätigt", ""))
	assert.Equal(t, "Anmeldung bestätigt - OGS Sonnenschule", decisionSubject("Anmeldung bestätigt", "OGS Sonnenschule"))
}

// renderDecisionMail is shared by all decision-email renderers. The
// contract is: missing recipient_email or status_url must error out (loud
// broken payload = visible bug), every other field falls back to empty
// string in the template.
func TestRenderDecisionMailRejectsMissingOrBlankRecipient(t *testing.T) {
	t.Parallel()
	row := validDecisionRow("enrollment_approved")
	delete(row.Payload, enrollment.EnrollmentPayloadRecipientEmail)
	_, err := renderDecisionMail(row.Kind, row.Payload, "Subj", "tpl.html")
	require.Error(t, err)
	assert.Equal(t, "enrollment_approved payload missing recipient_email", err.Error())

	row.Payload[enrollment.EnrollmentPayloadRecipientEmail] = ""
	_, err = renderDecisionMail(row.Kind, row.Payload, "Subj", "tpl.html")
	require.Error(t, err, "blank recipient must error — same as missing")
}

func TestRenderDecisionMailRejectsMissingStatusURL(t *testing.T) {
	t.Parallel()
	row := validDecisionRow("enrollment_approved")
	delete(row.Payload, enrollment.EnrollmentPayloadStatusURL)
	_, err := renderDecisionMail(row.Kind, row.Payload, "Subj", "tpl.html")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status_url")
}

func TestRenderDecisionMailPopulatesAllContent(t *testing.T) {
	t.Parallel()
	row := validDecisionRow("enrollment_approved")
	mail, err := renderDecisionMail(row.Kind, row.Payload, "Anmeldung bestätigt", "enrollment-approved.html")
	require.NoError(t, err)
	assert.Equal(t, "OGS Sonnenschule", mail.SenderName, "the sender name is the school")
	assert.Equal(t, "Anmeldung bestätigt - OGS Sonnenschule", mail.Subject)
	assert.Equal(t, "enrollment-approved.html", mail.Template)
	assert.Equal(t, "Anna", contentOf(t, mail)["GuardianFirstName"])
	assert.Equal(t, "Beispiel", contentOf(t, mail)["GuardianLastName"])
	assert.Equal(t, "OGS Sonnenschule", contentOf(t, mail)["SchoolName"])
	assert.Equal(t, "Schuljahr 2026/27", contentOf(t, mail)["PhaseName"])
	assert.Equal(t, "Kapazität erschöpft", contentOf(t, mail)["StatusReason"])
	assert.Equal(t, []string{"Lara", "Tim"}, contentOf(t, mail)["ChildNames"])
}

func TestRenderDecisionMailToleratesMissingOptionalFields(t *testing.T) {
	t.Parallel()
	// Only recipient + status_url are mandatory; everything else defaults
	// to "" in the template. School-less tenants and status_reason-
	// suppressed phases still need to render.
	mail, err := renderDecisionMail("enrollment_approved", map[string]any{
		enrollment.EnrollmentPayloadRecipientEmail: "guardian@example.test",
		enrollment.EnrollmentPayloadStatusURL:      "https://parents.localhost:3000/status/abc",
	}, "Anmeldung bestätigt", "enrollment-approved.html")
	require.NoError(t, err)
	assert.Empty(t, mail.SenderName, "no school name → default sender")
	assert.Equal(t, "Anmeldung bestätigt", mail.Subject, "no school name → bare subject")
	assert.Equal(t, "", contentOf(t, mail)["StatusReason"])
}

// Renderer factories are thin closures. Verify each picks the right
// subject + template to guard against future copy-paste swaps.
func TestDecisionRenderersUseTheirSubjectAndTemplate(t *testing.T) {
	t.Parallel()
	renderers := NewMailRenderers()
	for _, tt := range []struct {
		renderer enrollment.MailRenderer
		kind     string
		template string
		subject  string
	}{
		{renderers.Approved, "enrollment_approved", "enrollment-approved.html", "bestätigt"},
		{renderers.Waitlisted, "enrollment_waitlisted", "enrollment-waitlisted.html", "Warteliste"},
		{renderers.Rejected, "enrollment_rejected", "enrollment-rejected.html", "abgelehnt"},
	} {
		mail, err := render(tt.renderer, validDecisionRow(tt.kind))
		require.NoError(t, err)
		assert.Equal(t, tt.template, mail.Template)
		assert.Contains(t, mail.Subject, tt.subject)
	}
}

func TestDecisionDigestRendererPopulatesStatusBuckets(t *testing.T) {
	t.Parallel()
	row := validDecisionRow("enrollment_decision_digest")
	row.Payload["approved_names"] = []any{"Lara"}
	row.Payload["waitlisted_names"] = []string{"Tim"}
	row.Payload["rejected_names"] = []string{"Mina"}
	row.Payload["withdrawn_names"] = []string{"Noah"}

	mail, err := render(NewMailRenderers().DecisionDigest, row)
	require.NoError(t, err)
	assert.Equal(t, "enrollment-decision-digest.html", mail.Template)
	assert.Equal(t, []string{"Lara"}, contentOf(t, mail)["ApprovedNames"])
	assert.Equal(t, []string{"Tim"}, contentOf(t, mail)["WaitlistedNames"])
	assert.Equal(t, []string{"Mina"}, contentOf(t, mail)["RejectedNames"])
	assert.Equal(t, []string{"Noah"}, contentOf(t, mail)["WithdrawnNames"])
}

// --- change requests --------------------------------------------------------

func TestChangeRequestRenderersRequireOnlyTheRecipient(t *testing.T) {
	t.Parallel()
	renderers := NewMailRenderers()
	_, err := render(renderers.ChangeRequestSubmitted, &outboxRow{Kind: "enrollment_change_request_submitted", Payload: map[string]any{}})
	require.Error(t, err)
	assert.Equal(t, "enrollment_change_request_submitted payload missing recipient_email", err.Error())

	mail, err := render(renderers.ChangeRequestQuestion, &outboxRow{Kind: "enrollment_change_request_question", Payload: map[string]any{
		enrollment.EnrollmentPayloadRecipientEmail: "guardian@example.test",
		enrollment.EnrollmentPayloadSchoolName:     "OGS Sonnenschule",
		enrollment.EnrollmentPayloadAdminURL:       "https://admin",
	}})
	require.NoError(t, err)
	assert.Equal(t, "enrollment-change-request-question.html", mail.Template)
	assert.Equal(t, "Rückfrage zu Ihrer Änderungsanfrage - OGS Sonnenschule", mail.Subject)
	assert.Equal(t, "https://admin", contentOf(t, mail)["AdminURL"])
}

// --- payloadStringSlice -------------------------------------------------------

// payloadStringSlice handles the JSON-roundtrip case where an outbox
// payload column comes back as []any after json.Unmarshal. The renderer
// needs []string, so we coerce element-by-element and skip non-string
// values rather than panicking.
func TestPayloadStringSlice(t *testing.T) {
	t.Parallel()
	assert.Nil(t, payloadStringSlice(map[string]any{"other": "x"}, "names"))
	assert.Nil(t, payloadStringSlice(nil, "names"))
	assert.Equal(t, []string{"Anna", "Bert"}, payloadStringSlice(map[string]any{"names": []string{"Anna", "Bert"}}, "names"))
	assert.Equal(t, []string{"Anna", "Bert"}, payloadStringSlice(map[string]any{"names": []any{"Anna", "Bert"}}, "names"))
	assert.Equal(t, []string{"Anna", "Bert"}, payloadStringSlice(map[string]any{"names": []any{"Anna", 42, nil, "Bert"}}, "names"))
	assert.Nil(t, payloadStringSlice(map[string]any{"names": "just one string"}, "names"), "non-slice value must return nil rather than panic")
	empty := payloadStringSlice(map[string]any{"names": []any{}}, "names")
	assert.Empty(t, empty)
	assert.NotNil(t, empty, "empty []any must yield a (zero-len) []string, not nil")
}

// schoolSender controls who "the email is from" in the parent's inbox. A
// blank school name keeps the default sender.
func TestSchoolSender(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", schoolSender(""))
	assert.Equal(t, "", schoolSender("   "))
	assert.Equal(t, "OGS Sonnenschule", schoolSender("OGS Sonnenschule"))
	assert.Equal(t, "Schule", schoolSender("  Schule  "))
}
