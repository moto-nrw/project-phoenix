// Router-level tests for the postal address of a holiday registration
// (#5625): the Träger-Büro switches the building block "Anschrift" per
// version of the form, the public registration is checked against the
// version it comes with, the address reaches the detail and the export of
// the registrations but not the list, the receipt mail, the participant
// list or the guest copy, and a released run copies it into the invoice.
package api

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type holidayAddressFormBody struct {
	Version            int    `json:"version"`
	AddressRequirement string `json:"address_requirement"`
}

func saveHolidayAddressForm(t *testing.T, f holidayRegistrationFixture, office traegerOffice, periodID int64, requirement string) holidayAddressFormBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayFormPath(periodID),
		map[string]any{"fields": []map[string]any{}, "address_requirement": requirement})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[holidayAddressFormBody](t, rec.Body.Bytes())
}

func holidayFormAddressRequirements(t *testing.T, f holidayRegistrationFixture, periodID int64) []string {
	t.Helper()
	var requirements []string
	require.NoError(t, f.db.NewRaw(`SELECT address_requirement FROM enrollment.holiday_period_forms
  WHERE holiday_period_id = ? ORDER BY version`, periodID).Scan(context.Background(), &requirements))
	return requirements
}

func musterAddress() map[string]any {
	return map[string]any{"street": " Musterweg 12 ", "postal_code": " 01234 ", "city": " Bissendorf "}
}

func TestTraegerSwitchesTheAddressPerFormVersion(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Anschrift Schalter", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)

	rec := traegerJSON(t, f.traeger, office.token, http.MethodGet, holidayFormPath(period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "off", decodeSchoolPortalData[holidayAddressFormBody](t, rec.Body.Bytes()).AddressRequirement,
		"a new period asks for no address")

	saved := saveHolidayAddressForm(t, f, office, period.ID, "required")
	assert.Equal(t, holidayAddressFormBody{Version: 2, AddressRequirement: "required"}, saved)
	assert.Equal(t, []string{"off", "required"}, holidayFormAddressRequirements(t, f, period.ID),
		"the new version carries the setting, the older one does not")
	rec = publicJSON(t, f.public, http.MethodGet, registrationPath(period.PublicToken), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"address_requirement":"required"`, "parents see the setting of the current version")

	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayFormPath(period.ID), map[string]any{"fields": []map[string]any{}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "off", decodeSchoolPortalData[holidayAddressFormBody](t, rec.Body.Bytes()).AddressRequirement,
		"a save without the setting asks for no address")

	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayFormPath(period.ID),
		map[string]any{"fields": []map[string]any{}, "address_requirement": "sometimes"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, enrollment.CodeHolidayFormFieldNotAllowed, code)
	assert.Equal(t, []string{"address_requirement"}, fields)
	assert.Equal(t, []string{"off", "required", "off"}, holidayFormAddressRequirements(t, f, period.ID), "a refused save writes no version")

	readOnly := newTraegerOffice(t, f.db, f.traeger, "Anschrift Schalter Lesend", identityaccess.TraegerRoleReadOnly)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Anschrift Schalter Fremd", identityaccess.TraegerRoleAdministration)
	body := map[string]any{"fields": []map[string]any{}, "address_requirement": "optional"}
	assert.Equal(t, http.StatusForbidden, traegerJSON(t, f.traeger, readOnly.token, http.MethodPut, holidayFormPath(period.ID), body).Code)
	assert.Equal(t, http.StatusNotFound, traegerJSON(t, f.traeger, foreign.token, http.MethodPut, holidayFormPath(period.ID), body).Code)
}

// submitWithAddress posts a registration with the address and returns the
// answer; a nil address sends none.
func submitWithAddress(t *testing.T, f holidayRegistrationFixture, token string, offer holidayOfferBody, email string, address map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body := siblingsRegistration(offer, email)
	if address != nil {
		body["guardian_address"] = address
	}
	return publicJSON(t, f.public, http.MethodPost, registrationPath(token), body)
}

func assertAddressRefused(t *testing.T, rec *httptest.ResponseRecorder, want ...string) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_invalid", code)
	assert.ElementsMatch(t, want, fields)
}

func TestHolidayRegistrationChecksTheAddressAgainstItsFormVersion(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	for _, tc := range []struct {
		requirement string
		without     int
	}{
		{"off", http.StatusCreated},
		{"optional", http.StatusCreated},
		{"required", http.StatusBadRequest},
	} {
		t.Run(tc.requirement, func(t *testing.T) {
			t.Parallel()
			office := newTraegerOffice(t, f.db, f.traeger, "Anschrift Prüfung "+tc.requirement, identityaccess.TraegerRoleAdministration)
			period := openHolidays(t, f, office)
			if tc.requirement != "off" {
				saveHolidayAddressForm(t, f, office, period.ID, tc.requirement)
			}
			offer := holidayOffer(t, f, period.PublicToken)
			email := func(name string) string { return fmt.Sprintf("%s.%s@example.org", name, tc.requirement) }

			rec := submitWithAddress(t, f, period.PublicToken, offer, email("ohne"), nil)
			require.Equal(t, tc.without, rec.Code, rec.Body.String())
			if tc.without != http.StatusCreated {
				assertAddressRefused(t, rec, "guardian_address.street", "guardian_address.postal_code", "guardian_address.city")
			}
			rec = submitWithAddress(t, f, period.PublicToken, offer, email("leer"), map[string]any{"street": " ", "postal_code": "", "city": " "})
			assert.Equal(t, tc.without, rec.Code, "a blank address is no address: %s", rec.Body.String())

			if tc.requirement == "off" {
				assertAddressRefused(t, submitWithAddress(t, f, period.PublicToken, offer, email("ungefragt"), musterAddress()),
					"guardian_address.street", "guardian_address.postal_code", "guardian_address.city")
				return
			}
			assertAddressRefused(t, submitWithAddress(t, f, period.PublicToken, offer, email("halb"),
				map[string]any{"street": "Musterweg 12"}), "guardian_address.postal_code", "guardian_address.city")
			assertAddressRefused(t, submitWithAddress(t, f, period.PublicToken, offer, email("plz"),
				map[string]any{"street": "Musterweg 12", "postal_code": "4914", "city": "Bissendorf"}), "guardian_address.postal_code")
			assertAddressRefused(t, submitWithAddress(t, f, period.PublicToken, offer, email("lang"),
				map[string]any{"street": strings.Repeat("ä", 101), "postal_code": "49143", "city": "Bissendorf"}), "guardian_address.street")
			rec = submitWithAddress(t, f, period.PublicToken, offer, email("ganz"), musterAddress())
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		})
	}
}

func TestHolidayAddressIsAuditedAndOnlyInDetailAndExport(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Anschrift Zugriff")
	saveHolidayAddressForm(t, f.holidayRegistrationFixture, f.office, f.period.ID, "optional")
	rec := submitWithAddress(t, f.holidayRegistrationFixture, f.period.PublicToken, f.offer, "anschrift@example.org", musterAddress())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Musterweg")
	id := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[enrollment.HolidayRegistrationDetail](t, rec.Body.Bytes())
	require.NotNil(t, detail.GuardianAddress)
	assert.Equal(t, enrollment.HolidayAddress{Street: "Musterweg 12", PostalCode: "01234", City: "Bissendorf"}, *detail.GuardianAddress)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(f.siblings, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"guardian_address":null`, "a registration without an address names none")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path(""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "guardian_address")
	assert.NotContains(t, rec.Body.String(), "Musterweg")
	var mail string
	require.NoError(t, f.db.NewRaw(`SELECT payload::text FROM platform.email_outbox WHERE organization_id = ? AND related_entity_id = ?
  AND kind = 'holiday_registration_received'`, f.office.organization, id).Scan(context.Background(), &mail))
	assert.Contains(t, mail, "Eva Muster")
	assert.NotContains(t, mail, "Musterweg")
	assert.NotContains(t, mail, "01234")
	assertAddressExports(t, f)
	assert.Contains(t, f.auditActions(t), fmt.Sprintf("read:%d", id))

	for _, role := range []string{identityaccess.TraegerRoleReadOnly, identityaccess.TraegerRoleAdministration} {
		other := newTraegerOffice(t, f.db, f.traeger, "Anschrift Fremdzugriff "+role, role)
		rec = traegerJSON(t, f.traeger, other.token, http.MethodGet, f.registrationPath(id, ""), nil)
		if role == identityaccess.TraegerRoleReadOnly {
			assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		} else {
			assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		}
	}
}

func assertAddressExports(t *testing.T, f reviewFixture) {
	t.Helper()
	for _, format := range []string{"csv", "xlsx"} {
		req := httptest.NewRequest(http.MethodGet, f.path("/export?format="+format), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		if format == "csv" {
			assert.Contains(t, rec.Body.String(), ";Telefon;Straße und Hausnummer;PLZ;Ort;Zweites Elternteil;")
			assert.Contains(t, rec.Body.String(), "anschrift@example.org;0151 2345678;Musterweg 12;01234;Bissendorf;")
			assert.Contains(t, rec.Body.String(), "eva.muster@example.org;0151 2345678;;;;", "a registration without an address keeps the cells empty")
			continue
		}
		content := xlsxText(t, rec.Body.Bytes())
		for _, text := range []string{"Straße und Hausnummer", "PLZ", "Ort", "Musterweg 12", "01234", "Bissendorf"} {
			assert.Contains(t, content, text)
		}
	}
}

// xlsxText is the XML of every part of a workbook, for text checks.
func xlsxText(t *testing.T, workbook []byte) string {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(workbook), int64(len(workbook)))
	require.NoError(t, err)
	var content bytes.Buffer
	for _, file := range archive.File {
		reader, err := file.Open()
		require.NoError(t, err)
		_, err = io.Copy(&content, reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
	}
	return content.String()
}

func TestHolidayAddressStaysOutOfSchoolParticipantsAndGuestCopy(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Anschrift Schulgrenze")
	_, err := f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registrations SET guardian_address_street = 'Musterweg 12',
  guardian_address_postal_code = '01234', guardian_address_city = 'Bissendorf' WHERE id = ?`, f.siblings)
	require.NoError(t, err)
	f.confirm(t, f.siblings, "confirm")
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	rec := f.get(t, lead, fmt.Sprintf("/%d/participants", f.offer.Weeks[0].ID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Ben")
	for _, private := range []string{"Musterweg", "01234", "guardian_address"} {
		assert.NotContains(t, rec.Body.String(), private)
	}
	rec = f.assign(t, lead, f.offer.Weeks[0].ID, f.group)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	f.today.Store(calendar.NewDate(2099, 7, 6))
	f.reconcile(t)
	guest := f.roster(t, calendar.NewDate(2099, 7, 8))["Ben Muster"]
	require.NotZero(t, guest)
	rec = f.sheet(t, lead, guest)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Eva Muster")
	for _, private := range []string{"Musterweg", "01234", "guardian_address"} {
		assert.NotContains(t, rec.Body.String(), private)
	}
}

type billingAddressBody struct {
	Documents []struct {
		RegistrationID   int64 `json:"registration_id"`
		RecipientAddress *struct {
			Street     string `json:"street"`
			PostalCode string `json:"postal_code"`
			City       string `json:"city"`
		} `json:"recipient_address"`
	} `json:"documents"`
}

func TestReleasedBillingRunCopiesTheAddressIntoTheInvoice(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Anschrift Rechnung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	_, err := f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registrations SET guardian_address_street = 'Musterweg 12',
  guardian_address_postal_code = '01234', guardian_address_city = 'Bissendorf' WHERE id = ?`, f.siblings)
	require.NoError(t, err)
	for _, id := range []int64{f.siblings, f.again} {
		require.Equal(t, http.StatusOK, f.act(t, id, "confirm").Code)
	}
	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	addresses := func(body []byte) map[int64]string {
		out := map[int64]string{}
		for _, document := range decodeSchoolPortalData[billingAddressBody](t, body).Documents {
			out[document.RegistrationID] = ""
			if address := document.RecipientAddress; address != nil {
				out[document.RegistrationID] = address.Street + ", " + address.PostalCode + " " + address.City
			}
		}
		return out
	}
	want := map[int64]string{f.siblings: "Musterweg 12, 01234 Bissendorf", f.again: ""}
	assert.Equal(t, want, addresses(rec.Body.Bytes()), "the preview names the address of the registration")

	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, want, addresses(rec.Body.Bytes()), "the release copies the address into the invoice")

	// The invoice keeps its own copy: a later change of the registration
	// changes no issued invoice.
	_, err = f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registrations SET guardian_address_street = NULL,
  guardian_address_postal_code = NULL, guardian_address_city = NULL WHERE id = ?`, f.siblings)
	require.NoError(t, err)
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, want, addresses(rec.Body.Bytes()))

	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=csv"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(rec.Body.String(), "\xEF\xBB\xBF")), "\n")
	assert.Equal(t, "Rechnungsnummer;Empfänger;Straße und Hausnummer;PLZ;Ort;Kind;Position;Betrag in Euro;Zahlungsweg;Fällig am", strings.TrimSpace(lines[0]))
	assert.Contains(t, rec.Body.String(), ";Muster, Eva;Musterweg 12;01234;Bissendorf;")
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=xlsx"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, text := range []string{"Straße und Hausnummer", "PLZ", "Ort", "Musterweg 12", "01234", "Bissendorf"} {
		assert.Contains(t, xlsxText(t, rec.Body.Bytes()), text)
	}
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=pdf"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, strings.HasPrefix(rec.Body.String(), "%PDF"), "the PDF renders the same table")
}
