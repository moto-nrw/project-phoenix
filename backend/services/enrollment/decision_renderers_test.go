package enrollment

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decisionSubject appends the school name (if present) so parents
// recognise *which* school the decision concerns when they're juggling
// multiple kids across schools. Without a school name we send the bare
// subject so it still works for tenants that left the name blank.

func TestDecisionSubject_EmptySchoolReturnsBareSubject(t *testing.T) {
	assert.Equal(t, "Anmeldung bestätigt",
		decisionSubject("Anmeldung bestätigt", ""))
}

func TestDecisionSubject_WithSchoolAppendsName(t *testing.T) {
	assert.Equal(t, "Anmeldung bestätigt - OGS Sonnenschule",
		decisionSubject("Anmeldung bestätigt", "OGS Sonnenschule"))
}

// renderDecisionMessage is shared by all three decision-email
// renderers. The contract is: missing recipient_email or status_url
// must error out (loud broken payload = visible bug), every other
// field falls back to empty string in the template.

func validDecisionRow(kind string) *platformModels.EmailOutbox {
	return &platformModels.EmailOutbox{
		Kind: kind,
		Payload: map[string]any{
			EnrollmentPayloadRecipientEmail:    "guardian@example.test",
			EnrollmentPayloadStatusURL:         "https://parents.localhost:3000/status/abc",
			EnrollmentPayloadGuardianFirstName: "Anna",
			EnrollmentPayloadGuardianLastName:  "Beispiel",
			EnrollmentPayloadSchoolName:        "OGS Sonnenschule",
			EnrollmentPayloadLogoURL:           "https://logo",
			EnrollmentPayloadMotoLogoURL:       "https://moto-logo",
			EnrollmentPayloadPhaseName:         "Schuljahr 2026/27",
			EnrollmentPayloadStatusReason:      "Kapazität erschöpft",
			EnrollmentPayloadChildNames:        []string{"Lara", "Tim"},
		},
	}
}

func TestRenderDecisionMessage_RejectsMissingRecipient(t *testing.T) {
	row := validDecisionRow("enrollment_approved")
	delete(row.Payload, EnrollmentPayloadRecipientEmail)

	_, err := renderDecisionMessage(EmailRendererConfig{}, row, "Subj", "tpl.html")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recipient_email")
}

func TestRenderDecisionMessage_RejectsBlankRecipient(t *testing.T) {
	row := validDecisionRow("enrollment_approved")
	row.Payload[EnrollmentPayloadRecipientEmail] = ""

	_, err := renderDecisionMessage(EmailRendererConfig{}, row, "Subj", "tpl.html")
	require.Error(t, err, "blank recipient must error — same as missing")
}

func TestRenderDecisionMessage_RejectsMissingStatusURL(t *testing.T) {
	row := validDecisionRow("enrollment_approved")
	delete(row.Payload, EnrollmentPayloadStatusURL)

	_, err := renderDecisionMessage(EmailRendererConfig{}, row, "Subj", "tpl.html")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status_url")
}

func TestRenderDecisionMessage_PopulatesAllContent(t *testing.T) {
	row := validDecisionRow("enrollment_approved")
	cfg := EmailRendererConfig{
		DefaultFrom: email.NewEmail("moto noreply", "noreply@moto-app.de"),
	}

	msg, err := renderDecisionMessage(cfg, row, "Anmeldung bestätigt", "enrollment-approved.html")
	require.NoError(t, err)
	assert.Equal(t, "OGS Sonnenschule", msg.From.Name, "From name is the school")
	assert.Equal(t, "noreply@moto-app.de", msg.From.Address, "address stays moto's for DMARC")
	assert.Equal(t, "Anmeldung bestätigt - OGS Sonnenschule", msg.Subject)
	assert.Equal(t, "enrollment-approved.html", msg.Template)
	content, ok := msg.Content.(map[string]any)
	require.True(t, ok, "Content should be a map[string]any from the renderer")
	assert.Equal(t, "Anna", content["GuardianFirstName"])
	assert.Equal(t, "Beispiel", content["GuardianLastName"])
	assert.Equal(t, "OGS Sonnenschule", content["SchoolName"])
	assert.Equal(t, "Schuljahr 2026/27", content["PhaseName"])
	assert.Equal(t, "Kapazität erschöpft", content["StatusReason"])
	assert.Equal(t, []string{"Lara", "Tim"}, content["ChildNames"])
}

func TestRenderDecisionMessage_TolerantOfMissingOptionalFields(t *testing.T) {
	// Only recipient + status_url are mandatory; everything else
	// defaults to "" in the template. School-less tenants and
	// status_reason-suppressed phases still need to render.
	row := &platformModels.EmailOutbox{
		Kind: "enrollment_approved",
		Payload: map[string]any{
			EnrollmentPayloadRecipientEmail: "guardian@example.test",
			EnrollmentPayloadStatusURL:      "https://parents.localhost:3000/status/abc",
		},
	}
	cfg := EmailRendererConfig{
		DefaultFrom: email.NewEmail("moto noreply", "noreply@moto-app.de"),
	}

	msg, err := renderDecisionMessage(cfg, row, "Anmeldung bestätigt", "enrollment-approved.html")
	require.NoError(t, err)
	assert.Equal(t, "moto noreply", msg.From.Name, "no school name → fall back to default")
	assert.Equal(t, "Anmeldung bestätigt", msg.Subject, "no school name → bare subject")
	content, ok := msg.Content.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "", content["StatusReason"])
}

// Renderer factories are thin closures. Verify each picks the right
// subject + template to guard against future copy-paste swaps.

func TestNewEnrollmentApprovedRenderer_UsesApprovedSubjectAndTemplate(t *testing.T) {
	r := NewEnrollmentApprovedRenderer(EmailRendererConfig{})
	msg, err := r(context.Background(), validDecisionRow("enrollment_approved"))
	require.NoError(t, err)
	assert.Equal(t, "enrollment-approved.html", msg.Template)
	assert.Contains(t, msg.Subject, "bestätigt")
}

func TestNewEnrollmentWaitlistedRenderer_UsesWaitlistedSubjectAndTemplate(t *testing.T) {
	r := NewEnrollmentWaitlistedRenderer(EmailRendererConfig{})
	msg, err := r(context.Background(), validDecisionRow("enrollment_waitlisted"))
	require.NoError(t, err)
	assert.Equal(t, "enrollment-waitlisted.html", msg.Template)
	assert.Contains(t, msg.Subject, "Warteliste")
}

func TestNewEnrollmentRejectedRenderer_UsesRejectedSubjectAndTemplate(t *testing.T) {
	r := NewEnrollmentRejectedRenderer(EmailRendererConfig{})
	msg, err := r(context.Background(), validDecisionRow("enrollment_rejected"))
	require.NoError(t, err)
	assert.Equal(t, "enrollment-rejected.html", msg.Template)
	assert.Contains(t, msg.Subject, "abgelehnt")
}

func TestNewEnrollmentDecisionDigestRenderer_PopulatesStatusBuckets(t *testing.T) {
	row := validDecisionRow(platformModels.EmailKindEnrollmentDecisionDigest)
	row.Payload["approved_names"] = []any{"Lara"}
	row.Payload["waitlisted_names"] = []string{"Tim"}
	row.Payload["rejected_names"] = []string{"Mina"}
	row.Payload["withdrawn_names"] = []string{"Noah"}

	msg, err := NewEnrollmentDecisionDigestRenderer(EmailRendererConfig{})(context.Background(), row)
	require.NoError(t, err)
	assert.Equal(t, "enrollment-decision-digest.html", msg.Template)
	content := msg.Content.(map[string]any)
	assert.Equal(t, []string{"Lara"}, content["ApprovedNames"])
	assert.Equal(t, []string{"Tim"}, content["WaitlistedNames"])
	assert.Equal(t, []string{"Mina"}, content["RejectedNames"])
	assert.Equal(t, []string{"Noah"}, content["WithdrawnNames"])
}
