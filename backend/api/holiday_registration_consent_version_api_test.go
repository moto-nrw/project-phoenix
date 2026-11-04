package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHolidayLegalAcceptancesRequireTheExplicitPublishedVersion(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Zustimmungsfassung", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	document := holidayPDF(t, f, office, period.ID)
	rec := saveHolidayLegalBlocks(t, f, office, period.ID, holidayLegalBlocks(document))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	form := decodeSchoolPortalData[enrollment.HolidayPeriodForm](t, rec.Body.Bytes())
	offer := holidayOffer(t, f, period.PublicToken)

	// Consent evidence must never be silently assigned to a version the
	// caller did not name. Refusals must not persist any registration.
	for i, tc := range []struct {
		name    string
		version any
		decline bool
	}{
		{name: "omitted"},
		{name: "zero", version: 0},
		{name: "previous", version: form.Version - 1},
		{name: "future", version: form.Version + 1},
		{name: "negative", version: -1},
		{name: "only a declined consent", decline: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := siblingsRegistration(offer, fmt.Sprintf("consent-version-%d@example.org", i))
			body["legal_acceptances"] = map[string]bool{"hinweis_0": true}
			if tc.decline {
				body["legal_acceptances"] = map[string]bool{"foto_ferien": false}
			}
			if tc.version != nil {
				body["form_version"] = tc.version
			}
			rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			code, fields := registrationRefusalOf(t, rec)
			assert.Equal(t, common.CodeEnrollmentHolidayRegistrationFormChanged, code)
			assert.Equal(t, []string{"form_version"}, fields)
		})
	}
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registrations WHERE holiday_period_id = ?`, period.ID).Scan(context.Background(), &count))
	assert.Zero(t, count)

	body := siblingsRegistration(offer, "consent-current@example.org")
	body["form_version"] = form.Version
	body["legal_acceptances"] = map[string]bool{"hinweis_0": true}
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestHolidayRegistrationWithoutAcceptancesKeepsLegacyVersionDefault(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Ohne Zustimmungen", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	for i, acceptances := range []any{nil, map[string]bool{}} {
		body := siblingsRegistration(offer, fmt.Sprintf("legacy-consent-%d@example.org", i))
		if acceptances != nil {
			body["legal_acceptances"] = acceptances
		}
		rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
}
