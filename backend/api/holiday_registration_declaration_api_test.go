// Router-level tests for the declaration of the holiday registration
// (#5626): the parent confirms the custody and types a name to give the
// declarations. The offer serves the German texts of the current version,
// the registration keeps when and with which name and version they were
// given, the receipt mail repeats them, and the Träger-Büro reads them.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The texts of version 4 word for word, with the retention of a year the
// Träger keeps by default (#6691). They bind; a change is a new version, and
// another retention another wording of this one.
const (
	declarationPrivacyText = "Ich bin einverstanden, dass der Träger die Angaben für die Ferienbetreuung nutzt. " +
		"Er löscht sie 365 Tage nach den Ferien. Gesundheitsangaben löscht er schon 92 Tage nach den Ferien. " +
		"Die OGS, die mein Kind in einer Ferienwoche betreut, " +
		"sieht Name und Telefon der zweiten sorgeberechtigten Person. Sie sieht auch die Antworten, " +
		"die der Träger für sie freigibt. Gesundheitsangaben sieht sie nur, wenn ich zustimme. " +
		"Die OGS sieht das schon vor der Ferienwoche. So kann sie sich vorbereiten. " +
		"Die Liste der Kinder sieht sie bis zum letzten Tag der Woche. " +
		"Name, Telefon und Antworten kann sie danach höchstens 7 Tage lesen. Dann werden sie gelöscht."
	declarationHealthText = "Ich bin einverstanden: Der Träger und die OGS sehen die Gesundheitsangaben meiner Kinder. " +
		"Es geht um die OGS, die ein Kind in einer Ferienwoche betreut. " +
		"Sie sieht die Angaben ab der Bestätigung bis zum letzten Tag der Woche."
	declarationBindingText = "Ich weiß, dass die Anmeldung verbindlich ist."
	declarationCustodyText = "Ich darf die Anmeldung allein abgeben: Ich bin allein sorgeberechtigt oder die andere sorgeberechtigte Person ist einverstanden."
	declarationNameText    = "Mit Ihrem Namen geben Sie die Erklärungen oben ab."
)

func TestHolidayOfferServesTheTextsOfTheCurrentVersion(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Erklärung Texte", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)

	rec := publicJSON(t, f.public, http.MethodGet, registrationPath(period.PublicToken), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	offer := decodeSchoolPortalData[struct {
		ConsentTexts enrollment.HolidayConsentTexts `json:"consent_texts"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, enrollment.HolidayConsentTexts{
		Version: 4, RetentionDays: 365, PrivacyConsent: declarationPrivacyText, HealthConsent: declarationHealthText,
		BindingConfirmed: declarationBindingText, CustodyConfirmed: declarationCustodyText, DeclarationName: declarationNameText,
	}, offer.ConsentTexts)
	for _, word := range []string{"Unterschrift", "unterschreib", "schriftlich"} {
		assert.NotContains(t, strings.ToLower(rec.Body.String()), strings.ToLower(word), "no text calls the declaration a signature")
	}
}

func TestHolidayRegistrationRequiresTheCustodyAndTheName(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Erklärung Pflicht", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)

	for i, tc := range []struct {
		name   string
		change func(body map[string]any)
		field  string
	}{
		{"without the custody", func(body map[string]any) { delete(body, "custody_confirmed") }, "custody_confirmed"},
		{"with the custody declined", func(body map[string]any) { body["custody_confirmed"] = false }, "custody_confirmed"},
		{"without a name", func(body map[string]any) { delete(body, "declaration_name") }, "declaration_name"},
		{"with a blank name", func(body map[string]any) { body["declaration_name"] = "   " }, "declaration_name"},
		{"with a name too long", func(body map[string]any) { body["declaration_name"] = strings.Repeat("a", 101) }, "declaration_name"},
		{"under version 2", func(body map[string]any) { body["consent_version"] = 2 }, "consent_version"},
		{"without a version", func(body map[string]any) { delete(body, "consent_version") }, "consent_version"},
		{"under version 3 with its three months", func(body map[string]any) { body["consent_version"] = 3 }, "consent_version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := siblingsRegistration(offer, fmt.Sprintf("erklaerung-%d@example.org", i))
			tc.change(body)
			rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			code, fields := registrationRefusalOf(t, rec)
			assert.Equal(t, "enrollment.holiday_registration_invalid", code)
			assert.Equal(t, []string{tc.field}, fields)
		})
	}
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registrations WHERE holiday_period_id = ?`, period.ID).
		Scan(t.Context(), &count))
	assert.Zero(t, count, "a refused registration stores nothing")

	body := siblingsRegistration(offer, "erklaerung@example.org")
	body["declaration_name"] = "  Eva Muster "
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var stored struct {
		Custody bool   `bun:"custody"`
		SameAt  bool   `bun:"same_at"`
		Name    string `bun:"declaration_name"`
		Version int    `bun:"consent_version"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT custody_confirmed_at IS NOT NULL AS custody, custody_confirmed_at = privacy_consent_at AS same_at,
			declaration_name, consent_version
		FROM enrollment.holiday_registrations WHERE holiday_period_id = ?`, period.ID).Scan(t.Context(), &stored))
	assert.Equal(t, struct {
		Custody bool   `bun:"custody"`
		SameAt  bool   `bun:"same_at"`
		Name    string `bun:"declaration_name"`
		Version int    `bun:"consent_version"`
	}{Custody: true, SameAt: true, Name: "Eva Muster", Version: 4}, stored, "the custody, the trimmed name and version 4 are stored")
}

// declarationMail is the declaration of a receipt mail as Delivery stores it.
type declarationMail struct {
	Declaration *enrollment.HolidayRegistrationMailDeclaration `json:"declaration"`
}

func TestHolidayReceiptAndDetailRepeatTheDeclaration(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Erklärung Kopie", identityaccess.TraegerRoleAdministration)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Erklärung Fremd", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	rec := saveHolidayLegalBlocks(t, f, office, period.ID, holidayLegalBlocks())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	form := decodeSchoolPortalData[enrollment.HolidayPeriodForm](t, rec.Body.Bytes())
	offer := holidayOffer(t, f, period.PublicToken)

	body := siblingsRegistration(offer, "kopie@example.org")
	body["form_version"] = form.Version
	body["legal_acceptances"] = map[string]bool{"foto_ferien": true}
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	registration := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID

	var payload string
	require.NoError(t, f.db.NewRaw(`SELECT payload::text FROM platform.email_outbox WHERE organization_id = ? AND kind = 'holiday_registration_received'`,
		office.organization).Scan(t.Context(), &payload))
	var mail declarationMail
	require.NoError(t, json.Unmarshal([]byte(payload), &mail))
	require.NotNil(t, mail.Declaration, "the receipt carries the parent's copy of the declaration")
	assert.Equal(t, "Eva Muster", mail.Declaration.Name)
	assert.Equal(t, 4, mail.Declaration.ConsentVersion)
	assert.False(t, mail.Declaration.GivenAt.IsZero(), "the receipt names when the declaration was given")
	assert.Equal(t, []enrollment.HolidayDeclarationStatement{
		{Title: "Datenschutz", Text: declarationPrivacyText},
		{Title: "Gesundheitsangaben", Text: declarationHealthText},
		{Title: "Verbindliche Anmeldung", Text: declarationBindingText},
		{Title: "Sorgerecht", Text: declarationCustodyText},
		{Title: "Fotos", Text: "Mein Kind darf fotografiert werden.", Detail: "Fotos bleiben in der OGS."},
	}, mail.Declaration.Statements, "every declaration given, in its words, with the confirmed notice of the Träger")
	assert.NotContains(t, payload, "Erdnüsse", "the receipt carries no health information")
	assert.NotContains(t, payload, "2018-03-01", "the receipt carries no date of birth")

	path := fmt.Sprintf("%s/registrations/%d", holidayPeriodPath(period.ID), registration)
	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[enrollment.HolidayRegistrationDetail](t, rec.Body.Bytes())
	assert.Equal(t, "Eva Muster", detail.DeclarationName)
	require.NotNil(t, detail.CustodyConfirmedAt)
	assert.Equal(t, 4, detail.ConsentVersion)
	require.NotNil(t, detail.ConsentTexts, "the detail names the texts of its version")
	assert.Equal(t, declarationCustodyText, detail.ConsentTexts.CustodyConfirmed)
	assert.Equal(t, declarationPrivacyText, detail.ConsentTexts.PrivacyConsent)
	var reads int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registration_audit WHERE holiday_registration_id = ? AND action = 'read'`,
		registration).Scan(t.Context(), &reads))
	assert.Equal(t, 1, reads, "the read stands in the registration's audit trail")

	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "another Träger sees nothing: %s", rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Eva Muster")

	// A registration from before #5626 keeps no declaration; the detail of
	// version 2 names its texts without the custody.
	_, err := f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registrations
		SET consent_version = 2, custody_confirmed_at = NULL, declaration_name = NULL WHERE id = ?`, registration)
	require.NoError(t, err)
	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail = decodeSchoolPortalData[enrollment.HolidayRegistrationDetail](t, rec.Body.Bytes())
	assert.Empty(t, detail.DeclarationName)
	assert.Nil(t, detail.CustodyConfirmedAt)
	require.NotNil(t, detail.ConsentTexts)
	assert.Equal(t, 2, detail.ConsentTexts.Version)
	assert.Empty(t, detail.ConsentTexts.CustodyConfirmed)
	assert.Contains(t, detail.ConsentTexts.PrivacyConsent, "Er löscht sie drei Monate nach den Ferien.",
		"a registration under an earlier version keeps the wording it agreed to (#6691)")
	assert.Equal(t, 92, detail.ConsentTexts.RetentionDays)
}
