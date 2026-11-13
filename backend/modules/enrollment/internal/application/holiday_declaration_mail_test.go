package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The receipt repeats the declaration (#5626): the fixed declarations the
// parent gave, in the words of their version, and each confirmed notice of
// the Träger; a declined or disabled notice and a consent not given stay out.
func TestHolidayMailDeclarationRepeatsEveryDeclarationGiven(t *testing.T) {
	t.Parallel()
	givenAt := time.Date(2026, 11, 16, 8, 30, 0, 0, time.UTC)
	form := enrollment.HolidayPeriodForm{LegalBlocks: []enrollment.FormLegalBlock{
		{Key: "fotos", Title: "Fotos", Label: "Mein Kind darf fotografiert werden.", Text: " Fotos bleiben in der OGS. ", Enabled: true},
		{Key: "ausflug", Title: "Ausflug", Label: "Mein Kind darf mit.", Enabled: true},
		{Key: "alt", Title: "Alt", Label: "Nicht mehr im Formular.", Enabled: false},
	}}
	input := enrollment.HolidayRegistrationInput{
		DeclarationName: "Eva Muster", ConsentVersion: enrollment.HolidayConsentVersionCurrent, RetentionDays: 180,
		PrivacyConsent: true, CustodyConfirmed: true, BindingConfirmed: true,
		LegalAcceptances: map[string]bool{"fotos": true, "ausflug": false, "alt": true},
	}
	texts, ok := enrollment.HolidayConsentTextsOf(enrollment.HolidayConsentVersionCurrent, 180)
	require.True(t, ok)

	declaration := holidayMailDeclaration(enrollment.HolidayPeriod{Binding: false}, form, input, givenAt)
	require.NotNil(t, declaration)
	assert.Equal(t, enrollment.HolidayRegistrationMailDeclaration{
		Name: "Eva Muster", GivenAt: givenAt, ConsentVersion: enrollment.HolidayConsentVersionCurrent,
		Statements: []enrollment.HolidayDeclarationStatement{
			{Title: "Datenschutz", Text: texts.PrivacyConsent},
			{Title: "Sorgerecht", Text: texts.CustodyConfirmed},
			{Title: "Fotos", Text: "Mein Kind darf fotografiert werden.", Detail: "Fotos bleiben in der OGS."},
		},
	}, *declaration, "no health consent given, no binding period: neither is repeated")

	input.HealthConsent = true
	declaration = holidayMailDeclaration(enrollment.HolidayPeriod{Binding: true}, form, input, givenAt)
	require.NotNil(t, declaration)
	titles := []string{}
	for _, statement := range declaration.Statements {
		titles = append(titles, statement.Title)
	}
	assert.Equal(t, []string{"Datenschutz", "Gesundheitsangaben", "Verbindliche Anmeldung", "Sorgerecht", "Fotos"}, titles)
}

func TestHolidayReceiptRendersTheDeclarationInBerlinTime(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(enrollment.HolidayRegistrationMail{
		Recipient: "eva@example.org", GuardianName: "Eva Muster", TraegerName: "Träger Bissendorf", PeriodName: "Winterferien",
		Declaration: &enrollment.HolidayRegistrationMailDeclaration{
			Name: "Eva Muster", GivenAt: time.Date(2026, 11, 16, 8, 30, 0, 0, time.UTC), ConsentVersion: 3,
			Statements: []enrollment.HolidayDeclarationStatement{{Title: "Sorgerecht", Text: "Ich bin allein sorgeberechtigt."}},
		},
	})
	require.NoError(t, err)
	mail, err := renderHolidayRegistrationReceived(context.Background(), enrollment.MailKindHolidayRegistrationReceived, raw, "https://eltern.example.org/")
	require.NoError(t, err)
	content, ok := mail.Content.(map[string]any)
	require.True(t, ok)
	declaration, ok := content["Declaration"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Eva Muster", declaration["Name"])
	assert.Equal(t, "16.11.2026 um 09:30 Uhr", declaration["GivenAt"], "the time reads in Berlin")
	assert.Equal(t, 3, declaration["Version"])
	assert.Equal(t, []map[string]any{{"Title": "Sorgerecht", "Text": "Ich bin allein sorgeberechtigt.", "Detail": ""}}, declaration["Statements"])

	raw, err = json.Marshal(enrollment.HolidayRegistrationMail{Recipient: "eva@example.org"})
	require.NoError(t, err)
	mail, err = renderHolidayRegistrationReceived(context.Background(), enrollment.MailKindHolidayRegistrationReceived, raw, "https://eltern.example.org/")
	require.NoError(t, err)
	content, ok = mail.Content.(map[string]any)
	require.True(t, ok)
	assert.Nil(t, content["Declaration"], "a mail enqueued before #5626 renders without the declaration")
	assert.Empty(t, content["StatusURL"], "a mail enqueued before #6676 carries no token and shows no status link")
}

// TestHolidayReceiptLinksTheStatusPage pins the status link of the receipt
// (#6676): the renderer builds it from the token and the parents URL at
// sending; without a parents URL the mail shows none.
func TestHolidayReceiptLinksTheStatusPage(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(enrollment.HolidayRegistrationMail{Recipient: "eva@example.org", StatusToken: "abc_DEF-123"})
	require.NoError(t, err)
	mail, err := renderHolidayRegistrationReceived(context.Background(), enrollment.MailKindHolidayRegistrationReceived, raw, "https://eltern.example.org/")
	require.NoError(t, err)
	content, ok := mail.Content.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://eltern.example.org/ferienanmeldung/status/abc_DEF-123", content["StatusURL"])

	mail, err = renderHolidayRegistrationReceived(context.Background(), enrollment.MailKindHolidayRegistrationReceived, raw, "")
	require.NoError(t, err)
	content, ok = mail.Content.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, content["StatusURL"])
}
