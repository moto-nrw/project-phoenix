package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The privacy text of a new registration says that the Träger keeps the
// customer number of the Gemeinde for later registrations and when it goes
// (#6692): a new wording, so a new version that every new registration
// agrees to again. The wording of version 4 stays as the parents agreed to
// it.
func TestConsentTextsNameTheKeptDebtorNumberFromVersionFive(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 5, enrollment.HolidayConsentVersionCurrent, "the changed privacy text is a new version")

	current, ok := enrollment.HolidayConsentTextsOf(enrollment.HolidayConsentVersionCurrent, 365)
	require.True(t, ok)
	assert.Equal(t, 5, current.Version)
	assert.Equal(t, 365, current.RetentionDays)
	assert.Contains(t, current.PrivacyConsent, "Er löscht sie 365 Tage nach den Ferien.")
	assert.Contains(t, current.PrivacyConsent, "Kundennummer der Gemeinde (Debitorennummer)")
	assert.Contains(t, current.PrivacyConsent, "Er löscht sie am Ende des dritten Jahres nach der letzten Anmeldung.")
	assert.NotEmpty(t, current.CustodyConfirmed)

	previous, ok := enrollment.HolidayConsentTextsOf(4, 365)
	require.True(t, ok)
	assert.Equal(t, 4, previous.Version)
	assert.NotContains(t, previous.PrivacyConsent, "Debitorennummer", "version 4 keeps its wording")
	assert.Equal(t, previous.PrivacyConsent, current.PrivacyConsent[:len(previous.PrivacyConsent)],
		"version 5 adds to the text of version 4")
}
