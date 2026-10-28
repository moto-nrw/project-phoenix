// Router-level tests for the form of a holiday period (#5198): the
// Träger-Büro builds it in versions from the standard template, parents
// answer its questions per family and per child, a registration keeps its
// version, and answers with health information reach only the detail of a
// registration.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type holidayFormField struct {
	Key            string         `json:"key"`
	Label          string         `json:"label"`
	Type           string         `json:"type"`
	Required       bool           `json:"required"`
	AppliesToChild bool           `json:"applies_to_child"`
	Health         bool           `json:"health"`
	SortOrder      int            `json:"sort_order"`
	VisibleWhen    map[string]any `json:"visible_when"`
}

type holidayFormBody struct {
	Version  int                `json:"version"`
	Fields   []holidayFormField `json:"fields"`
	Template []holidayFormField `json:"template"`
}

func holidayFormPath(periodID int64) string { return holidayPeriodPath(periodID) + "/form" }

func saveHolidayForm(t *testing.T, f holidayRegistrationFixture, office traegerOffice, periodID int64, fields []map[string]any) holidayFormBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayFormPath(periodID), map[string]any{"fields": fields})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[holidayFormBody](t, rec.Body.Bytes())
}

func formVersions(t *testing.T, f holidayRegistrationFixture, periodID int64) []int {
	t.Helper()
	var versions []int
	require.NoError(t, f.db.NewRaw(`SELECT version FROM enrollment.holiday_period_forms WHERE holiday_period_id = ? ORDER BY version`,
		periodID).Scan(context.Background(), &versions))
	return versions
}

// questionsForm asks per family a required note and per child a required
// tetanus choice (health), whether the child may go home alone, from when
// (required, only when it may) and its favourite meal.
func questionsForm() []map[string]any {
	return []map[string]any{
		{"key": "abholung_hinweis", "label": "Hinweis zur Abholung", "type": "text", "required": true, "help_text": "Wer holt meistens ab?"},
		{"key": "tetanus_impfschutz", "label": "Tetanus-Impfschutz", "type": "select", "required": true, "applies_to_child": true, "health": true,
			"options": []map[string]any{{"label": "Ja", "value": "ja"}, {"label": "Nein", "value": "nein"}, {"label": "Weiß nicht", "value": "weiss_nicht"}}},
		{"key": "darf_allein_nach_hause", "label": "Darf allein nach Hause gehen", "type": "boolean", "applies_to_child": true},
		{"key": "allein_ab", "label": "Ab wann allein", "type": "text", "required": true, "applies_to_child": true,
			"visible_when": map[string]any{"source": "field", "field": "darf_allein_nach_hause", "operator": "eq", "value": true}},
		{"key": "lieblingsessen", "label": "Lieblingsessen", "type": "text", "applies_to_child": true},
	}
}

func TestTraegerBuildsTheFormOfAHolidayPeriodInVersions(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Formular Bau", identityaccess.TraegerRoleAdministration)
	period := createHolidayPeriod(t, f.traeger, office, januaryHolidays(office))

	rec := traegerJSON(t, f.traeger, office.token, http.MethodGet, holidayFormPath(period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	form := decodeSchoolPortalData[holidayFormBody](t, rec.Body.Bytes())
	assert.Equal(t, 1, form.Version, "a new period starts with version 1")
	assert.Equal(t, form.Template, form.Fields, "version 1 is the standard template")
	health := []string{}
	for _, field := range form.Fields {
		assert.True(t, field.AppliesToChild, "every template field asks per child: %s", field.Key)
		assert.False(t, field.Required, "the Träger-Büro decides what is required: %s", field.Key)
		if field.Health {
			health = append(health, field.Key)
		}
	}
	assert.Equal(t, []string{"lebensmittelunvertraeglichkeiten", "tetanus_impfschutz", "foerderbedarf", "koerperliche_einschraenkungen"}, health)
	assert.NotContains(t, rec.Body.String(), "created_by")

	saved := saveHolidayForm(t, f, office, period.ID, questionsForm())
	assert.Equal(t, 2, saved.Version, "saving writes a new version")
	keys := []string{}
	for i, field := range saved.Fields {
		keys = append(keys, field.Key)
		assert.Equal(t, i+1, field.SortOrder, "the order of the request is the order of the form")
	}
	assert.Equal(t, []string{"abholung_hinweis", "tetanus_impfschutz", "darf_allein_nach_hause", "allein_ab", "lieblingsessen"}, keys)
	assert.False(t, saved.Fields[0].AppliesToChild, "a question per family")
	assert.True(t, saved.Fields[0].Required)
	assert.True(t, saved.Fields[1].Health)
	assert.Equal(t, "darf_allein_nach_hause", saved.Fields[3].VisibleWhen["field"])

	reordered := questionsForm()
	reordered[3], reordered[4] = reordered[4], reordered[3]
	saved = saveHolidayForm(t, f, office, period.ID, reordered)
	assert.Equal(t, 3, saved.Version)
	assert.Equal(t, "lieblingsessen", saved.Fields[3].Key)
	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, holidayFormPath(period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 3, decodeSchoolPortalData[holidayFormBody](t, rec.Body.Bytes()).Version)
	assert.Equal(t, []int{1, 2, 3}, formVersions(t, f, period.ID), "every version stays")

	readOnly := newTraegerOffice(t, f.db, f.traeger, "Träger Formular Lesend", identityaccess.TraegerRoleReadOnly)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Formular Fremd", identityaccess.TraegerRoleAdministration)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		body := map[string]any{"fields": []map[string]any{}}
		rec = traegerJSON(t, f.traeger, readOnly.token, method, holidayFormPath(period.ID), body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s without the right: %s", method, rec.Body.String())
		rec = traegerJSON(t, f.traeger, foreign.token, method, holidayFormPath(period.ID), body)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s of another Träger: %s", method, rec.Body.String())
	}
	assert.Equal(t, []int{1, 2, 3}, formVersions(t, f, period.ID), "a refused save writes no version")
}

func TestTraegerHolidayFormRefusesWhatTheHolidayFormDoesNotOffer(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Formular Grenzen", identityaccess.TraegerRoleAdministration)
	period := createHolidayPeriod(t, f.traeger, office, januaryHolidays(office))
	question := map[string]any{"key": "frage", "label": "Frage", "type": "boolean", "applies_to_child": true}

	for _, refused := range []struct {
		name   string
		fields []map[string]any
		code   string
		field  string
	}{
		{"a structured type", []map[string]any{{"key": "telefon", "label": "Telefon", "type": "phone_list", "target": "student.contacts"}},
			"enrollment.holiday_form_field_not_allowed", "fields.0.type"},
		{"a file upload", []map[string]any{{"key": "datei", "label": "Datei", "type": "file"}},
			"enrollment.holiday_form_field_not_allowed", "fields.0.type"},
		{"a target in the master data", []map[string]any{{"key": "gesundheit", "label": "Gesundheit", "type": "textarea", "applies_to_child": true, "target": "student.health_info"}},
			"enrollment.holiday_form_field_not_allowed", "fields.0.target"},
		{"a condition on the grade", []map[string]any{{"key": "klasse", "label": "Klasse", "type": "text", "applies_to_child": true,
			"visible_when": map[string]any{"source": "grade_level", "operator": "eq", "value": 1}}},
			"enrollment.holiday_form_field_not_allowed", "fields.0.visible_when"},
		{"a condition on a care offering", []map[string]any{{"key": "angebot", "label": "Angebot", "type": "text", "applies_to_child": true,
			"visible_when": map[string]any{"source": "care_offering", "operator": "includes", "value": "Mittag"}}},
			"enrollment.holiday_form_field_not_allowed", "fields.0.visible_when"},
		{"health information on an information block", []map[string]any{{"key": "info", "type": "information", "content": "Bitte lesen", "health": true}},
			"enrollment.holiday_form_field_not_allowed", "fields.0.health"},
		{"a duplicate key", []map[string]any{question, question},
			"enrollment.form_field_key_duplicate", "fields.1.key"},
		{"a condition on an unknown field", []map[string]any{{"key": "ab", "label": "Ab", "type": "text", "applies_to_child": true,
			"visible_when": map[string]any{"source": "field", "field": "fehlt", "operator": "eq", "value": true}}},
			"enrollment.form_field_visibility_invalid", "fields.0.visible_when"},
		{"a select without options", []map[string]any{{"key": "wahl", "label": "Wahl", "type": "select"}},
			"enrollment.form_field_options_invalid", "fields.0.options"},
	} {
		rec := traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayFormPath(period.ID), map[string]any{"fields": refused.fields})
		require.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", refused.name, rec.Body.String())
		code, fields := registrationRefusalOf(t, rec)
		assert.Equal(t, refused.code, code, refused.name)
		assert.Equal(t, []string{refused.field}, fields, refused.name)
	}
	assert.Equal(t, []int{1}, formVersions(t, f, period.ID), "no refused form becomes a version")
}

// threeChildren registers the siblings and a third child for the first week,
// without health information in the fixed fields, filled in on version.
func threeChildren(offer holidayOfferBody, email string, version int) map[string]any {
	body := siblingsRegistration(offer, email)
	children := body["children"].([]map[string]any)
	children[0]["allergies"] = ""
	children = append(children, map[string]any{
		"first_name": "Mia", "last_name": "Muster", "birth_date": "2019-06-02", "home_school_id": offer.Schools[1].ID,
		"bookings": []map[string]any{{"week_id": offer.Weeks[0].ID, "time_model_id": offer.Weeks[0].TimeModels[0].ID}},
	})
	body["children"] = children
	body["health_consent"] = false
	body["form_version"] = version
	return body
}

func answerChildren(body map[string]any, answers ...map[string]any) {
	for i, child := range body["children"].([]map[string]any) {
		child["custom_data"] = answers[i]
	}
}

func TestHolidayRegistrationChecksTheAnswersAgainstItsForm(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Formular Antworten", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	saveHolidayForm(t, f, office, period.ID, questionsForm())
	offer := holidayOffer(t, f, period.PublicToken)
	rec := publicJSON(t, f.public, http.MethodGet, registrationPath(period.PublicToken), nil)
	form := decodeSchoolPortalData[struct {
		Form holidayFormBody `json:"form"`
	}](t, rec.Body.Bytes()).Form
	assert.Equal(t, 2, form.Version, "parents get the current version")
	assert.Len(t, form.Fields, 5)
	assert.NotContains(t, rec.Body.String(), "created_by", "the offer names no author of the form")

	missing := threeChildren(offer, "fehlt@example.org", 2)
	answerChildren(missing, map[string]any{"darf_allein_nach_hause": true}, map[string]any{}, map[string]any{"darf_allein_nach_hause": false})
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), missing)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_invalid", code)
	assert.ElementsMatch(t, []string{
		"custom_data.abholung_hinweis",
		"children.0.custom_data.tetanus_impfschutz", "children.0.custom_data.allein_ab",
		"children.1.custom_data.tetanus_impfschutz", "children.2.custom_data.tetanus_impfschutz",
	}, fields, "every missing required answer by its path; a hidden required question blocks nothing")

	unknown := threeChildren(offer, "unbekannt@example.org", 2)
	unknown["custom_data"] = map[string]any{"abholung_hinweis": "Oma"}
	answerChildren(unknown, map[string]any{"tetanus_impfschutz": "ja"}, map[string]any{"tetanus_impfschutz": "vielleicht"},
		map[string]any{"tetanus_impfschutz": "nein", "darf_allein_nach_hause": "ja"})
	unknown["health_consent"] = true
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), unknown)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	_, fields = registrationRefusalOf(t, rec)
	assert.ElementsMatch(t, []string{"children.1.custom_data.tetanus_impfschutz", "children.2.custom_data.darf_allein_nach_hause"}, fields,
		"an unknown choice and a wrong type are refused by their path")

	noConsent := threeChildren(offer, "einwilligung@example.org", 2)
	noConsent["custom_data"] = map[string]any{"abholung_hinweis": "Oma"}
	answerChildren(noConsent, map[string]any{"tetanus_impfschutz": "ja"}, map[string]any{"tetanus_impfschutz": "ja"}, map[string]any{"tetanus_impfschutz": "ja"})
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), noConsent)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	_, fields = registrationRefusalOf(t, rec)
	assert.Equal(t, []string{"health_consent"}, fields, "an answer marked as health information needs the consent")

	stale := threeChildren(offer, "alt@example.org", 1)
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), stale)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_form_changed", code, "a page with an older form learns that the form changed")

	valid := threeChildren(offer, "passt@example.org", 2)
	valid["custom_data"] = map[string]any{"abholung_hinweis": "  Oma Muster  ", "nicht_im_formular": "x"}
	answerChildren(valid,
		map[string]any{"tetanus_impfschutz": "ja", "darf_allein_nach_hause": true, "allein_ab": "15 Uhr", "lieblingsessen": "Nudeln"},
		map[string]any{"tetanus_impfschutz": "nein", "darf_allein_nach_hause": false, "allein_ab": "verworfen"},
		map[string]any{"tetanus_impfschutz": "weiss_nicht", "lieblingsessen": ""})
	valid["health_consent"] = true
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), valid)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	id := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID

	var stored struct {
		FormVersion int             `bun:"form_version"`
		CustomData  json.RawMessage `bun:"custom_data,type:jsonb"`
		Health      bool            `bun:"health"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT form_version, custom_data, health_consent_at IS NOT NULL AS health
		FROM enrollment.holiday_registrations WHERE id = ?`, id).Scan(context.Background(), &stored))
	assert.Equal(t, 2, stored.FormVersion, "the registration keeps the version it was checked against")
	assert.JSONEq(t, `{"abholung_hinweis": "Oma Muster"}`, string(stored.CustomData), "trimmed, and no answer outside the form")
	assert.True(t, stored.Health)
	var children []json.RawMessage
	require.NoError(t, f.db.NewRaw(`SELECT custom_data FROM enrollment.holiday_registration_children
		WHERE holiday_registration_id = ? ORDER BY position`, id).Scan(context.Background(), &children))
	require.Len(t, children, 3)
	assert.JSONEq(t, `{"tetanus_impfschutz": "ja", "darf_allein_nach_hause": true, "allein_ab": "15 Uhr", "lieblingsessen": "Nudeln"}`, string(children[0]))
	assert.JSONEq(t, `{"tetanus_impfschutz": "nein", "darf_allein_nach_hause": false}`, string(children[1]), "the answer to a hidden question is dropped")
	assert.JSONEq(t, `{"tetanus_impfschutz": "weiss_nicht"}`, string(children[2]), "an empty text is no answer")
}

// TestHolidayPeriodWithoutQuestionsServesTheFormOfBefore pins the migration
// path: a period of before #5198 has version 1 without questions, and its
// link takes a registration of the old shape unchanged.
func TestHolidayPeriodWithoutQuestionsServesTheFormOfBefore(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Formular Bestand", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	ctx := context.Background()
	// The state the migration leaves: version 1 without questions.
	_, err := f.db.ExecContext(ctx, `UPDATE enrollment.holiday_period_forms SET fields = '[]'::jsonb WHERE holiday_period_id = ?`, period.ID)
	require.NoError(t, err)

	rec := publicJSON(t, f.public, http.MethodGet, registrationPath(period.PublicToken), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	offer := decodeSchoolPortalData[struct {
		Form holidayFormBody `json:"form"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, 1, offer.Form.Version)
	assert.Empty(t, offer.Form.Fields, "parents see exactly the fixed fields of before")

	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(holidayOffer(t, f, period.PublicToken), "bestand@example.org"))
	require.Equal(t, http.StatusCreated, rec.Code, "a registration without form version and answers is taken as before: %s", rec.Body.String())
	var version int
	require.NoError(t, f.db.NewRaw(`SELECT form_version FROM enrollment.holiday_registrations WHERE holiday_period_id = ?`, period.ID).Scan(ctx, &version))
	assert.Equal(t, 1, version)
}

func TestHolidayRegistrationKeepsItsFormVersionAndHealthAnswersStayInTheDetail(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Formular Einzelansicht")
	fields := []map[string]any{
		{"key": "familie_hinweis", "label": "Hinweis der Familie", "type": "text"},
		{"key": "lebensmittelunvertraeglichkeiten", "label": "Lebensmittelunverträglichkeiten", "type": "textarea", "applies_to_child": true, "health": true},
		{"key": "darf_an_ausfluegen_teilnehmen", "label": "Darf an Ausflügen teilnehmen", "type": "boolean", "applies_to_child": true},
		{"key": "lieblingsessen", "label": "Lieblingsessen", "type": "text", "applies_to_child": true},
	}
	saveHolidayForm(t, f.holidayRegistrationFixture, f.office, f.period.ID, fields)
	body := siblingsRegistration(f.offer, "antworten@example.org")
	body["form_version"] = 2
	body["custom_data"] = map[string]any{"familie_hinweis": "Familienhinweis-FAM"}
	answerChildren(body,
		map[string]any{"lebensmittelunvertraeglichkeiten": "Laktose-GESUND", "darf_an_ausfluegen_teilnehmen": true, "lieblingsessen": "Spaghetti-ESSEN"},
		map[string]any{"darf_an_ausfluegen_teilnehmen": false})
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	id := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID

	var mail string
	require.NoError(t, f.db.NewRaw(`SELECT payload::text FROM platform.email_outbox WHERE organization_id = ? AND related_entity_id = ?`,
		f.office.organization, id).Scan(context.Background(), &mail))
	require.Contains(t, mail, "antworten@example.org", "the receipt of this registration")
	for _, answer := range []string{"Laktose-GESUND", "Spaghetti-ESSEN", "Familienhinweis-FAM"} {
		assert.NotContains(t, mail, answer, "the receipt carries no answer")
	}

	relabelled := []map[string]any{fields[0], fields[2], fields[3]}
	relabelled[2] = map[string]any{"key": "lieblingsessen", "label": "Lieblingsessen (neu)", "type": "text", "applies_to_child": true}
	saveHolidayForm(t, f.holidayRegistrationFixture, f.office, f.period.ID, relabelled)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[struct {
		reviewRegistration
		Form            holidayFormBody `json:"form"`
		CustomData      map[string]any  `json:"custom_data"`
		ChildCustomData []struct {
			ChildID    int64          `json:"child_id"`
			CustomData map[string]any `json:"custom_data"`
		} `json:"child_custom_data"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, 2, detail.Form.Version, "the detail labels the answers with the version the parent filled in")
	labels := []string{}
	for _, field := range detail.Form.Fields {
		labels = append(labels, field.Label)
	}
	assert.Equal(t, []string{"Hinweis der Familie", "Lebensmittelunverträglichkeiten", "Darf an Ausflügen teilnehmen", "Lieblingsessen"}, labels)
	assert.Equal(t, map[string]any{"familie_hinweis": "Familienhinweis-FAM"}, detail.CustomData)
	require.Len(t, detail.ChildCustomData, 2)
	assert.Equal(t, detail.Children[0].ID, detail.ChildCustomData[0].ChildID)
	assert.Equal(t, "Laktose-GESUND", detail.ChildCustomData[0].CustomData["lebensmittelunvertraeglichkeiten"], "the detail shows every answer")
	assert.Equal(t, false, detail.ChildCustomData[1].CustomData["darf_an_ausfluegen_teilnehmen"])

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path(""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, answer := range []string{"Laktose-GESUND", "Spaghetti-ESSEN", "Familienhinweis-FAM", "custom_data"} {
		assert.NotContains(t, rec.Body.String(), answer, "the list carries no answer")
	}

	req := httptest.NewRequest(http.MethodGet, f.path("/export?format=csv"), nil)
	req.Header.Set("Authorization", "Bearer "+f.office.token)
	rec = httptest.NewRecorder()
	f.traeger.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	csv := strings.TrimPrefix(rec.Body.String(), "\xEF\xBB\xBF")
	header := strings.SplitN(csv, "\n", 2)[0]
	assert.True(t, strings.HasSuffix(strings.TrimSpace(header), ";Mögliche Doppelanmeldung;Hinweis der Familie;Darf an Ausflügen teilnehmen;Lieblingsessen;Darf allein nach Hause gehen"),
		"one column per question without health information of the versions the rows keep, newest first: %s", header)
	assert.NotContains(t, csv, "Lebensmittelunverträglichkeiten", "no column for health information")
	assert.NotContains(t, csv, "Laktose-GESUND", "no answer with health information")
	assert.Contains(t, csv, "Spaghetti-ESSEN")
	assert.Contains(t, csv, "Familienhinweis-FAM")
	assert.Contains(t, csv, ";Ja;Spaghetti-ESSEN", "yes and no read as the form shows them")
}

// TestHolidayAnswersStayWithTheTraeger pins that the hosting school reaches
// none of the answers: neither its participant list (ADR 0116) nor the copy
// of a holiday guest (ADR 0120).
func TestHolidayAnswersStayWithTheTraeger(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Formular Schule")
	_, err := f.db.ExecContext(context.Background(), `UPDATE enrollment.holiday_registration_children
		SET custom_data = '{"lebensmittelunvertraeglichkeiten": "Laktose-GAST", "lieblingsessen": "Pizza-GAST"}'::jsonb
		WHERE holiday_registration_id = ?`, f.siblings)
	require.NoError(t, err)
	_, err = f.db.ExecContext(context.Background(), `UPDATE enrollment.holiday_registrations
		SET custom_data = '{"familie_hinweis": "Familie-GAST"}'::jsonb WHERE id = ?`, f.siblings)
	require.NoError(t, err)
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	week := f.offer.Weeks[0].ID
	f.confirm(t, f.siblings, "confirm")

	rec := f.get(t, lead, fmt.Sprintf("/%d/participants", week))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Ben", "the list holds the confirmed children")
	for _, answer := range []string{"Laktose-GAST", "Pizza-GAST", "Familie-GAST"} {
		assert.NotContains(t, rec.Body.String(), answer, "the participant list carries no answer")
	}

	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)
	f.today.Store(calendar.NewDate(2099, 7, 6))
	f.reconcile(t)
	ben := f.roster(t, calendar.NewDate(2099, 7, 8))["Ben Muster"]
	require.NotZero(t, ben, "the confirmed guest joins the group")
	_, _, _, _, health := f.child(t, ben)
	assert.Nil(t, health, "no answer becomes health information of the copy")
	rec = f.sheet(t, lead, ben)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, answer := range []string{"Laktose-GAST", "Pizza-GAST", "Familie-GAST"} {
		assert.NotContains(t, rec.Body.String(), answer, "the guest sheet carries no answer")
	}
}

// TestConcurrentFormSavesWriteOneVersionEach pins invariant 2: two saves of
// one period at the same time wait for each other on the period's row and
// get two versions.
func TestConcurrentFormSavesWriteOneVersionEach(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Formular Gleichzeitig", identityaccess.TraegerRoleAdministration)
	period := createHolidayPeriod(t, f.traeger, office, januaryHolidays(office))

	const saves = 4
	codes := make(chan int, saves)
	versions := make(chan int, saves)
	var wg sync.WaitGroup
	for i := range saves {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fields := []map[string]any{{"key": fmt.Sprintf("frage_%d", i), "label": "Frage", "type": "text"}}
			rec := traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayFormPath(period.ID), map[string]any{"fields": fields})
			codes <- rec.Code
			if rec.Code == http.StatusOK {
				versions <- decodeSchoolPortalData[holidayFormBody](t, rec.Body.Bytes()).Version
			}
		}()
	}
	wg.Wait()
	close(codes)
	close(versions)
	for code := range codes {
		assert.Equal(t, http.StatusOK, code, "no save is lost to another")
	}
	got := []int{}
	for version := range versions {
		got = append(got, version)
	}
	assert.ElementsMatch(t, []int{2, 3, 4, 5}, got, "each save gets a version of its own")
	assert.Equal(t, []int{1, 2, 3, 4, 5}, formVersions(t, f, period.ID))
}
