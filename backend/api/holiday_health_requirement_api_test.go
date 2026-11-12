// Router-level tests for the building block "Gesundheit" of a holiday form
// (#6686): the Träger-Büro makes allergies and medication optional or
// required per version of the form, and the public registration is checked
// against the version it comes with. A required field takes "Keine" as its
// answer; every answer is health information and needs the health consent.
package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

type holidayHealthFormBody struct {
	Version           int    `json:"version"`
	HealthRequirement string `json:"health_requirement"`
}

func saveHolidayHealthForm(t *testing.T, f holidayRegistrationFixture, office traegerOffice, periodID int64, requirement string) holidayHealthFormBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayFormPath(periodID),
		map[string]any{"fields": []map[string]any{}, "health_requirement": requirement})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[holidayHealthFormBody](t, rec.Body.Bytes())
}

func holidayFormHealthRequirements(t *testing.T, f holidayRegistrationFixture, periodID int64) []string {
	t.Helper()
	var requirements []string
	require.NoError(t, f.db.NewRaw(`SELECT health_requirement FROM enrollment.holiday_period_forms
  WHERE holiday_period_id = ? ORDER BY version`, periodID).Scan(context.Background(), &requirements))
	return requirements
}

func TestTraegerSwitchesTheHealthRequirementPerFormVersion(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Gesundheit Schalter", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)

	rec := traegerJSON(t, f.traeger, office.token, http.MethodGet, holidayFormPath(period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "optional", decodeSchoolPortalData[holidayHealthFormBody](t, rec.Body.Bytes()).HealthRequirement,
		"a new period keeps the health fields optional")

	saved := saveHolidayHealthForm(t, f, office, period.ID, "required")
	assert.Equal(t, holidayHealthFormBody{Version: 2, HealthRequirement: "required"}, saved)
	assert.Equal(t, []string{"optional", "required"}, holidayFormHealthRequirements(t, f, period.ID),
		"the new version carries the setting, the older one does not")
	rec = publicJSON(t, f.public, http.MethodGet, registrationPath(period.PublicToken), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"health_requirement":"required"`, "parents see the setting of the current version")

	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayFormPath(period.ID), map[string]any{"fields": []map[string]any{}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "optional", decodeSchoolPortalData[holidayHealthFormBody](t, rec.Body.Bytes()).HealthRequirement,
		"a save without the setting keeps the health fields optional")

	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayFormPath(period.ID),
		map[string]any{"fields": []map[string]any{}, "health_requirement": "sometimes"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, enrollment.CodeHolidayFormFieldNotAllowed, code)
	assert.Equal(t, []string{"health_requirement"}, fields)
	assert.Equal(t, []string{"optional", "required", "optional"}, holidayFormHealthRequirements(t, f, period.ID),
		"a refused save writes no version")
}

func TestHolidayRegistrationChecksTheHealthFieldsAgainstItsFormVersion(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	for _, requirement := range []string{"optional", "required"} {
		t.Run(requirement, func(t *testing.T) {
			t.Parallel()
			office := newTraegerOffice(t, f.db, f.traeger, "Gesundheit Prüfung "+requirement, identityaccess.TraegerRoleAdministration)
			period := openHolidays(t, f, office)
			saveHolidayHealthForm(t, f, office, period.ID, requirement)
			offer := holidayOffer(t, f, period.PublicToken)
			email := func(name string) string { return fmt.Sprintf("%s.gesundheit.%s@example.org", name, requirement) }
			submit := func(name string, change func(children []map[string]any), consent bool) (int, []string) {
				body := siblingsRegistration(offer, email(name))
				children := body["children"].([]map[string]any)
				change(children)
				body["health_consent"] = consent
				rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
				if rec.Code == http.StatusCreated {
					return rec.Code, nil
				}
				require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				_, fields := registrationRefusalOf(t, rec)
				return rec.Code, fields
			}

			// Lia names her allergy, Ben gives nothing.
			status, fields := submit("ohne", func([]map[string]any) {}, true)
			if requirement == "optional" {
				assert.Equal(t, http.StatusCreated, status, "optional health fields may stay empty: %v", fields)
				return
			}
			assert.Equal(t, http.StatusBadRequest, status)
			assert.ElementsMatch(t, []string{"children.0.medication", "children.1.allergies", "children.1.medication"}, fields,
				"a required form needs allergies and medication of every child")

			status, fields = submit("leer", func(children []map[string]any) {
				for _, child := range children {
					child["allergies"], child["medication"] = "  ", " "
				}
			}, true)
			assert.Equal(t, http.StatusBadRequest, status)
			assert.ElementsMatch(t, []string{"children.0.allergies", "children.0.medication", "children.1.allergies", "children.1.medication"}, fields,
				"blank answers are no answers")

			keine := func(children []map[string]any) {
				children[0]["medication"] = "Keine"
				children[1]["allergies"], children[1]["medication"] = "Keine", "Keine"
			}
			status, fields = submit("ohne-zustimmung", keine, false)
			assert.Equal(t, http.StatusBadRequest, status)
			assert.Equal(t, []string{"health_consent"}, fields, "the answer Keine is health information too")

			status, fields = submit("keine", keine, true)
			assert.Equal(t, http.StatusCreated, status, "Keine answers a required health field: %v", fields)
		})
	}
}
