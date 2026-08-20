package enrollment

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every enrollment email renderer reads payload values that were
// captured at enqueue time. The two non-negotiable fields per
// renderer are recipient_email (mandatory — without it there's no
// addressee) plus the URL the email's CTA links to (status_url for
// the parent emails, admin_url for the admin notification).

func validSubmittedRow() *platformModels.EmailOutbox {
	return &platformModels.EmailOutbox{
		Kind: "enrollment_submitted",
		Payload: map[string]any{
			EnrollmentPayloadRecipientEmail:    "guardian@example.test",
			EnrollmentPayloadStatusURL:         "https://parents.localhost:3000/status/abc",
			EnrollmentPayloadGuardianFirstName: "Anna",
			EnrollmentPayloadGuardianLastName:  "Beispiel",
			EnrollmentPayloadSchoolName:        "OGS Sonnenschule",
			EnrollmentPayloadLogoURL:           "https://logo",
			EnrollmentPayloadMotoLogoURL:       "https://moto-logo",
			EnrollmentPayloadChildNames:        []string{"Lara", "Tim"},
		},
	}
}

func validAdminRow() *platformModels.EmailOutbox {
	return &platformModels.EmailOutbox{
		Kind: "enrollment_admin_notification",
		Payload: map[string]any{
			EnrollmentPayloadRecipientEmail:    "admin@example.test",
			EnrollmentPayloadAdminURL:          "https://admin.localhost:3000/enrollment/42",
			EnrollmentPayloadGuardianFirstName: "Anna",
			EnrollmentPayloadGuardianLastName:  "Beispiel",
			EnrollmentPayloadGuardianEmail:     "guardian@example.test",
			EnrollmentPayloadGuardianPhone:     "+49 170 1234567",
			EnrollmentPayloadSchoolName:        "OGS Sonnenschule",
			EnrollmentPayloadLogoURL:           "https://logo",
			EnrollmentPayloadMotoLogoURL:       "https://moto-logo",
			EnrollmentPayloadChildNames:        []string{"Lara"},
		},
	}
}

func validRolloverRow() *platformModels.EmailOutbox {
	return &platformModels.EmailOutbox{
		Kind: "enrollment_rollover",
		Payload: map[string]any{
			EnrollmentPayloadRecipientEmail:    "guardian@example.test",
			EnrollmentPayloadStatusURL:         "https://parents.localhost:3000/status/abc",
			EnrollmentPayloadGuardianFirstName: "Anna",
			EnrollmentPayloadGuardianLastName:  "Beispiel",
			EnrollmentPayloadSchoolName:        "OGS Sonnenschule",
			EnrollmentPayloadPhaseName:         "Schuljahr 2027/28",
			EnrollmentPayloadLogoURL:           "https://logo",
			EnrollmentPayloadMotoLogoURL:       "https://moto-logo",
			EnrollmentPayloadRolloverDeadline:  "2027-06-30",
			EnrollmentPayloadChildNames:        []string{"Lara"},
		},
	}
}

func defaultCfg() EmailRendererConfig {
	return EmailRendererConfig{DefaultFrom: email.NewEmail("moto noreply", "noreply@moto-app.de")}
}

// --- NewEnrollmentSubmittedRenderer ---------------------------------------

func TestNewEnrollmentSubmittedRenderer_RejectsMissingRecipient(t *testing.T) {
	t.Parallel()

	row := validSubmittedRow()
	delete(row.Payload, EnrollmentPayloadRecipientEmail)
	_, err := NewEnrollmentSubmittedRenderer(defaultCfg())(context.Background(), row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recipient_email")
}

func TestNewEnrollmentSubmittedRenderer_RejectsMissingStatusURL(t *testing.T) {
	t.Parallel()

	row := validSubmittedRow()
	delete(row.Payload, EnrollmentPayloadStatusURL)
	_, err := NewEnrollmentSubmittedRenderer(defaultCfg())(context.Background(), row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status_url")
}

func TestNewEnrollmentSubmittedRenderer_HappyPath(t *testing.T) {
	t.Parallel()

	msg, err := NewEnrollmentSubmittedRenderer(defaultCfg())(context.Background(), validSubmittedRow())
	require.NoError(t, err)
	assert.Equal(t, "enrollment-submitted.html", msg.Template)
	assert.Equal(t, "Anmeldung eingegangen - OGS Sonnenschule", msg.Subject)
	assert.Equal(t, "OGS Sonnenschule", msg.From.Name)
	assert.Equal(t, "guardian@example.test", msg.To.Address)
}

func TestNewEnrollmentSubmittedRenderer_NoSchoolBareSubject(t *testing.T) {
	t.Parallel()

	row := validSubmittedRow()
	row.Payload[EnrollmentPayloadSchoolName] = ""
	msg, err := NewEnrollmentSubmittedRenderer(defaultCfg())(context.Background(), row)
	require.NoError(t, err)
	assert.Equal(t, "Anmeldung eingegangen", msg.Subject)
}

// --- NewEnrollmentAdminNotificationRenderer -------------------------------

func TestNewEnrollmentAdminNotificationRenderer_RejectsMissingRecipient(t *testing.T) {
	t.Parallel()

	row := validAdminRow()
	delete(row.Payload, EnrollmentPayloadRecipientEmail)
	_, err := NewEnrollmentAdminNotificationRenderer(defaultCfg())(context.Background(), row)
	require.Error(t, err)
}

func TestNewEnrollmentAdminNotificationRenderer_RejectsMissingAdminURL(t *testing.T) {
	t.Parallel()

	row := validAdminRow()
	delete(row.Payload, EnrollmentPayloadAdminURL)
	_, err := NewEnrollmentAdminNotificationRenderer(defaultCfg())(context.Background(), row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "admin_url")
}

func TestNewEnrollmentAdminNotificationRenderer_HappyPath(t *testing.T) {
	t.Parallel()

	msg, err := NewEnrollmentAdminNotificationRenderer(defaultCfg())(context.Background(), validAdminRow())
	require.NoError(t, err)
	assert.Equal(t, "enrollment-admin-notification.html", msg.Template)
	assert.Equal(t, "Neue Anmeldung - OGS Sonnenschule", msg.Subject)
	content, ok := msg.Content.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "guardian@example.test", content["GuardianEmail"])
	assert.Equal(t, "+49 170 1234567", content["GuardianPhone"])
}

func TestNewEnrollmentAdminNotificationRenderer_NoSchoolBareSubject(t *testing.T) {
	t.Parallel()

	row := validAdminRow()
	row.Payload[EnrollmentPayloadSchoolName] = ""
	msg, err := NewEnrollmentAdminNotificationRenderer(defaultCfg())(context.Background(), row)
	require.NoError(t, err)
	assert.Equal(t, "Neue Anmeldung eingegangen", msg.Subject)
}

// --- NewEnrollmentRolloverOptInRenderer -----------------------------------

func TestNewEnrollmentRolloverOptInRenderer_RejectsMissingRecipient(t *testing.T) {
	t.Parallel()

	row := validRolloverRow()
	delete(row.Payload, EnrollmentPayloadRecipientEmail)
	_, err := NewEnrollmentRolloverOptInRenderer(defaultCfg())(context.Background(), row)
	require.Error(t, err)
}

func TestNewEnrollmentRolloverOptInRenderer_RejectsMissingStatusURL(t *testing.T) {
	t.Parallel()

	row := validRolloverRow()
	delete(row.Payload, EnrollmentPayloadStatusURL)
	_, err := NewEnrollmentRolloverOptInRenderer(defaultCfg())(context.Background(), row)
	require.Error(t, err)
}

func TestNewEnrollmentRolloverOptInRenderer_HappyPath(t *testing.T) {
	t.Parallel()

	msg, err := NewEnrollmentRolloverOptInRenderer(defaultCfg())(context.Background(), validRolloverRow())
	require.NoError(t, err)
	assert.Equal(t, "enrollment-rollover-opt-in.html", msg.Template)
	assert.Equal(t, "Bitte bestätigen: Anmeldung Schuljahr 2027/28 - OGS Sonnenschule", msg.Subject)
	content, ok := msg.Content.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2027-06-30", content["RolloverDeadline"])
}

func TestNewEnrollmentRolloverOptInRenderer_PhaseOnlySubject(t *testing.T) {
	t.Parallel()

	row := validRolloverRow()
	row.Payload[EnrollmentPayloadSchoolName] = ""
	msg, err := NewEnrollmentRolloverOptInRenderer(defaultCfg())(context.Background(), row)
	require.NoError(t, err)
	assert.Equal(t, "Bitte bestätigen: Anmeldung Schuljahr 2027/28", msg.Subject)
}

func TestNewEnrollmentRolloverOptInRenderer_BareSubjectWhenNothingSet(t *testing.T) {
	t.Parallel()

	row := validRolloverRow()
	row.Payload[EnrollmentPayloadSchoolName] = ""
	row.Payload[EnrollmentPayloadPhaseName] = ""
	msg, err := NewEnrollmentRolloverOptInRenderer(defaultCfg())(context.Background(), row)
	require.NoError(t, err)
	assert.Equal(t, "Bitte bestätigen Sie die Anmeldung", msg.Subject)
}

// --- NewEnrollmentRolloverOptOutRenderer ----------------------------------

func TestNewEnrollmentRolloverOptOutRenderer_RejectsMissingRecipient(t *testing.T) {
	t.Parallel()

	row := validRolloverRow()
	delete(row.Payload, EnrollmentPayloadRecipientEmail)
	_, err := NewEnrollmentRolloverOptOutRenderer(defaultCfg())(context.Background(), row)
	require.Error(t, err)
}

func TestNewEnrollmentRolloverOptOutRenderer_RejectsMissingStatusURL(t *testing.T) {
	t.Parallel()

	row := validRolloverRow()
	delete(row.Payload, EnrollmentPayloadStatusURL)
	_, err := NewEnrollmentRolloverOptOutRenderer(defaultCfg())(context.Background(), row)
	require.Error(t, err)
}

func TestNewEnrollmentRolloverOptOutRenderer_HappyPath(t *testing.T) {
	t.Parallel()

	msg, err := NewEnrollmentRolloverOptOutRenderer(defaultCfg())(context.Background(), validRolloverRow())
	require.NoError(t, err)
	assert.Equal(t, "enrollment-rollover-opt-out.html", msg.Template)
	assert.Equal(t, "Anmeldung verlängert: Schuljahr 2027/28 - OGS Sonnenschule", msg.Subject)
}

func TestNewEnrollmentRolloverOptOutRenderer_PhaseOnlySubject(t *testing.T) {
	t.Parallel()

	row := validRolloverRow()
	row.Payload[EnrollmentPayloadSchoolName] = ""
	msg, err := NewEnrollmentRolloverOptOutRenderer(defaultCfg())(context.Background(), row)
	require.NoError(t, err)
	assert.Equal(t, "Anmeldung verlängert: Schuljahr 2027/28", msg.Subject)
}

func TestNewEnrollmentRolloverOptOutRenderer_BareSubjectWhenNothingSet(t *testing.T) {
	t.Parallel()

	row := validRolloverRow()
	row.Payload[EnrollmentPayloadSchoolName] = ""
	row.Payload[EnrollmentPayloadPhaseName] = ""
	msg, err := NewEnrollmentRolloverOptOutRenderer(defaultCfg())(context.Background(), row)
	require.NoError(t, err)
	assert.Equal(t, "Anmeldung wurde verlängert", msg.Subject)
}
