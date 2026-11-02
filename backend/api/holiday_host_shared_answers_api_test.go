// Router-level tests for what the Träger shares with the school that hosts a
// holiday week (#5202, ADR 0116 and ADR 0120 addenda): the share of a form
// question, which a health question cannot carry; the shared answers and the
// second guardian in the participant list and its export; the second
// guardian as an emergency contact and the shared answers on the sheet of a
// holiday guest, deleted with the contacts after the read-only days and at
// once with a cancellation. Health information, unshared answers and e-mail
// addresses never reach the school.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type hostAnswerBody struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type sharedParticipantListBody struct {
	Questions []struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	} `json:"questions"`
	Participants []struct {
		FirstName      string `json:"first_name"`
		SecondGuardian *struct {
			Name  string `json:"name"`
			Phone string `json:"phone"`
		} `json:"second_guardian"`
		Answers []hostAnswerBody `json:"answers"`
	} `json:"participants"`
}

// sharedQuestions is version 2 of the form: per child the way home and the
// first day of a trip shared, the tetanus choice as health information and
// the favourite meal unshared; per family the pickup shared.
func sharedQuestions() []map[string]any {
	return []map[string]any{
		{"key": "darf_allein_nach_hause", "label": "Darf allein nach Hause gehen", "type": "boolean", "applies_to_child": true, "share_with_host": true},
		{"key": "tetanus_impfschutz", "label": "Tetanus-Impfschutz", "type": "select", "applies_to_child": true, "health": true,
			"options": []map[string]any{{"label": "Ja", "value": "ja"}, {"label": "Nein", "value": "nein"}}},
		{"key": "lieblingsessen", "label": "Lieblingsessen", "type": "text", "applies_to_child": true},
		{"key": "ausflug_ab", "label": "Ausflug ab", "type": "date", "applies_to_child": true, "share_with_host": true},
		{"key": "abholung", "label": "Abholung", "type": "select", "share_with_host": true,
			"options": []map[string]any{{"label": "Mit dem Bus", "value": "bus"}, {"label": "Zu Fuß", "value": "zu_fuss"}}},
	}
}

// A health question can be shared with the hosting school since #4722 (user
// decision of 01.11.2026 "keine Datensilos"): the template shares its health
// questions like the way home, and a save that shares one is accepted. Before
// #4722 the builder refused it with enrollment.holiday_form_field_not_allowed.
func TestTraegerSharesAHealthQuestionWithTheHostingSchool(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Freigabe Formular", identityaccess.TraegerRoleAdministration)
	period := createHolidayPeriod(t, f.traeger, office, januaryHolidays(office))

	rec := traegerJSON(t, f.traeger, office.token, http.MethodGet, holidayFormPath(period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var template struct {
		Data struct {
			Fields []map[string]any `json:"fields"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &template))
	shared := map[string]any{}
	for _, field := range template.Data.Fields {
		shared[field["key"].(string)] = field["share_with_host"]
	}
	assert.Equal(t, true, shared["darf_allein_nach_hause"], "the template shares the way home")
	assert.Equal(t, true, shared["darf_an_ausfluegen_teilnehmen"], "the template shares the trips")
	assert.Equal(t, true, shared["tetanus_impfschutz"], "the template shares its health questions like the fixed health fields")

	questions := sharedQuestions()
	questions[1]["share_with_host"] = true
	saved := saveHolidayForm(t, f, office, period.ID, questions)
	assert.Equal(t, 2, saved.Version, "a shared health question is accepted")
	var sharedHealth bool
	require.NoError(t, f.db.NewRaw(`SELECT (fields->1->'share_with_host')::boolean AND (fields->1->'health')::boolean
		FROM enrollment.holiday_period_forms WHERE holiday_period_id = ? AND version = 2`, period.ID).Scan(t.Context(), &sharedHealth))
	assert.True(t, sharedHealth, "the version keeps the shared health question")

	saved = saveHolidayForm(t, f, office, period.ID, sharedQuestions())
	assert.Equal(t, 3, saved.Version)
	var encoded string
	require.NoError(t, f.db.NewRaw(`SELECT fields::text FROM enrollment.holiday_period_forms
		WHERE holiday_period_id = ? AND version = 3`, period.ID).Scan(t.Context(), &encoded))
	var stored []map[string]any
	require.NoError(t, json.Unmarshal([]byte(encoded), &stored))
	require.Len(t, stored, len(sharedQuestions()))
	for _, field := range stored {
		_, ok := field["share_with_host"]
		assert.True(t, ok, "every stored question names whether the hosting school sees it: %v", field["key"])
	}
}

// submitShared registers Ida (OGS Süd) and Jan (no school of the Träger)
// Sommer on version 2 with a second guardian and answers to every question,
// Ida for week 1, Jan for both weeks, and returns the registration.
func (f holidayHostFixture) submitShared(t *testing.T) int64 {
	t.Helper()
	body := siblingsRegistration(f.offer, "familie.sommer@example.org")
	body["form_version"] = 2
	body["second_guardian"] = map[string]any{"first_name": "Tom", "last_name": "Zweit", "email": "tom.zweit@example.org", "phone": "0170 2222222"}
	body["custom_data"] = map[string]any{"abholung": "bus"}
	children := body["children"].([]map[string]any)
	children[0]["first_name"], children[0]["last_name"], children[0]["home_school_id"] = "Ida", "Sommer", f.offer.Schools[1].ID
	children[0]["custom_data"] = map[string]any{"darf_allein_nach_hause": true, "tetanus_impfschutz": "ja", "lieblingsessen": "Pizza", "ausflug_ab": "2099-07-07"}
	children[1]["first_name"], children[1]["last_name"] = "Jan", "Sommer"
	children[1]["custom_data"] = map[string]any{"darf_allein_nach_hause": false, "tetanus_impfschutz": "nein", "lieblingsessen": "Nudeln"}
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.link), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
}

// sharedPrivate are the values of the Sommer registration that must never
// reach the hosting school: unshared and health answers, allergies and
// e-mail addresses.
var sharedPrivate = []string{"Pizza", "Nudeln", "Erdnüsse", "tetanus", `"ja"`, `"nein"`,
	"familie.sommer@example.org", "tom.zweit@example.org", "eva.muster@example.org"}

func TestHolidayHostSeesSharedAnswersAndTheSecondGuardian(t *testing.T) {
	t.Parallel()
	f := newHolidayHostFixture(t, "Träger Freigabe Liste")
	nord, sued := f.office.schools[0], f.office.schools[1]
	host := f.staffOf(t, nord, permissions.ConfigRead)
	week := f.offer.Weeks[0].ID
	saveHolidayForm(t, f.holidayRegistrationFixture, f.office, f.periodID(t), sharedQuestions())
	sommer := f.submitShared(t)
	f.confirm(t, f.siblings, "confirm")
	f.confirm(t, sommer, "confirm")

	rec := f.get(t, host, fmt.Sprintf("/%d/participants", week))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	list := decodeSchoolPortalData[sharedParticipantListBody](t, rec.Body.Bytes())
	keys := []string{}
	for _, question := range list.Questions {
		keys = append(keys, question.Key)
	}
	assert.Equal(t, []string{"darf_allein_nach_hause", "darf_an_ausfluegen_teilnehmen", "ausflug_ab", "abholung"}, keys,
		"the questions of version 1 (siblings) and version 2 (Sommer), each once")
	require.Len(t, list.Participants, 4)
	byName := map[string]int{}
	for i, participant := range list.Participants {
		byName[participant.FirstName] = i
	}
	ida := list.Participants[byName["Ida"]]
	require.NotNil(t, ida.SecondGuardian)
	assert.Equal(t, "Tom Zweit", ida.SecondGuardian.Name)
	assert.Equal(t, "0170 2222222", ida.SecondGuardian.Phone)
	assert.Equal(t, []hostAnswerBody{
		{Key: "darf_allein_nach_hause", Label: "Darf allein nach Hause gehen", Type: "boolean", Value: true},
		{Key: "ausflug_ab", Label: "Ausflug ab", Type: "date", Value: "2099-07-07"},
		{Key: "abholung", Label: "Abholung", Type: "select", Value: "Mit dem Bus"},
	}, ida.Answers, "only the shared answers without health information, a choice by its label")
	lia := list.Participants[byName["Lia"]]
	assert.Nil(t, lia.SecondGuardian, "a registration without a second guardian names none")
	assert.Equal(t, []hostAnswerBody{
		{Key: "darf_allein_nach_hause", Label: "Darf allein nach Hause gehen", Type: "boolean"},
		{Key: "darf_an_ausfluegen_teilnehmen", Label: "Darf an Ausflügen teilnehmen", Type: "boolean"},
	}, lia.Answers, "a registration without answers keeps its row; the shared questions stay unanswered")
	for _, private := range sharedPrivate {
		assert.NotContains(t, rec.Body.String(), private, "the list keeps %s back", private)
	}

	rec = f.get(t, host, fmt.Sprintf("/%d/participants/export?format=csv", week))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	csv := strings.TrimPrefix(rec.Body.String(), "\xEF\xBB\xBF")
	assert.Equal(t, strings.Join([]string{
		"Nachname;Vorname;Geburtsdatum;Heimatschule;Zeitmodell;Zweite sorgeberechtigte Person;Telefon zweite sorgeberechtigte Person;" +
			"Darf allein nach Hause gehen;Darf an Ausflügen teilnehmen;Ausflug ab;Abholung",
		"Muster;Ben;12.09.2016;Feriengast (Grundschule Melle);bis 13 Uhr;;;;;;",
		"Muster;Lia;01.03.2018;" + f.offer.Schools[0].Name + ";bis 16 Uhr;;;;;;",
		"Sommer;Ida;01.03.2018;" + f.offer.Schools[1].Name + ";bis 16 Uhr;Tom Zweit;0170 2222222;Ja;;07.07.2099;Mit dem Bus",
		"Sommer;Jan;12.09.2016;Feriengast (Grundschule Melle);bis 13 Uhr;Tom Zweit;0170 2222222;Nein;;;Mit dem Bus",
	}, "\n")+"\n", csv, "one column per shared question and the second guardian")
	for _, private := range sharedPrivate {
		assert.NotContains(t, csv, strings.Trim(private, `"`)+";", "the export keeps %s back", private)
	}

	other := f.staffOf(t, sued, permissions.ConfigRead)
	rec = f.get(t, other, fmt.Sprintf("/%d/participants", f.offer.Weeks[1].ID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Jan", "the OGS Süd hosts Jan in its week")
	assert.NotContains(t, rec.Body.String(), "Ida", "Ida is booked for week 1 only")
	assert.Equal(t, http.StatusNotFound, f.get(t, other, fmt.Sprintf("/%d/participants", week)).Code,
		"another school of the Träger reads no answer of a week it does not host")
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Freigabe Fremd", identityaccess.TraegerRoleAdministration)
	stranger := f.staffOf(t, foreign.schools[0], permissions.ConfigRead)
	assert.Equal(t, http.StatusNotFound, f.get(t, stranger, fmt.Sprintf("/%d/participants", week)).Code,
		"a school of another Träger reads no answer")
	noRight := f.staffOf(t, nord, permissions.StudentsRead)
	assert.Equal(t, http.StatusForbidden, f.get(t, noRight, fmt.Sprintf("/%d/participants", week)).Code)

	f.today.Store(calendar.NewDate(2099, 7, 11))
	assert.Equal(t, http.StatusNotFound, f.get(t, host, fmt.Sprintf("/%d/participants", week)).Code,
		"after the last day of the week the list and its answers are gone")
	assert.Equal(t, []string{"holiday_participants::4", "holiday_participants_export:csv:4"}, f.accessLog(t, nord),
		"the access log of the school stays as before")
}

func TestHolidayGuestSheetCarriesTheSecondGuardianAndTheSharedAnswersUntilTheyExpire(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Freigabe Gäste")
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	weekDay := calendar.NewDate(2099, 7, 8)
	saveHolidayForm(t, f.holidayRegistrationFixture, f.office, f.periodID(t), sharedQuestions())
	sommer := f.submitShared(t)
	f.confirm(t, f.siblings, "confirm")
	f.confirm(t, sommer, "confirm")
	rec := f.assign(t, lead, week, f.group)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	f.today.Store(calendar.NewDate(2099, 7, 6))
	f.reconcile(t)
	children := f.roster(t, weekDay)
	assert.Equal(t, []string{"Ben Muster", "Ida Sommer", "Jan Sommer"}, names(children))

	type sheetBody struct {
		holidayGuestSheetBody
		Answers []hostAnswerBody `json:"answers"`
	}
	sheetOf := func(name string) (sheetBody, string) {
		t.Helper()
		rec := f.sheet(t, lead, children[name])
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[sheetBody](t, rec.Body.Bytes()), rec.Body.String()
	}
	ida, raw := sheetOf("Ida Sommer")
	contacts := []string{}
	for _, contact := range ida.Emergency {
		contacts = append(contacts, contact.Name+" "+contact.Phone+" "+contact.Relationship)
	}
	assert.Equal(t, []string{"Eva Muster 0151 2345678 Eltern", "Tom Zweit 0170 2222222 Eltern", "Oma Muster 05402 1234 Großmutter"}, contacts,
		"the second guardian is the second emergency contact")
	assert.Equal(t, []hostAnswerBody{
		{Key: "darf_allein_nach_hause", Label: "Darf allein nach Hause gehen", Type: "boolean", Value: true},
		{Key: "ausflug_ab", Label: "Ausflug ab", Type: "date", Value: "2099-07-07"},
		{Key: "abholung", Label: "Abholung", Type: "select", Value: "Mit dem Bus"},
	}, ida.Answers)
	for _, private := range sharedPrivate {
		assert.NotContains(t, raw, private, "the sheet keeps %s back", private)
	}
	ben, _ := sheetOf("Ben Muster")
	require.Len(t, ben.Emergency, 2, "without a second guardian the sheet keeps its contacts of before")
	assert.Len(t, ben.Answers, 2, "the siblings answered none of the shared questions of version 1")
	for _, answer := range ben.Answers {
		assert.Nil(t, answer.Value)
	}

	storedAnswers := func(name string) string {
		t.Helper()
		var stored string
		require.NoError(t, f.db.NewRaw(`SELECT answers::text FROM enrollment.holiday_guests WHERE tenant_id = ? AND student_id = ?`,
			f.nord, children[name]).Scan(t.Context(), &stored))
		return stored
	}
	f.confirm(t, sommer, "cancel")
	f.reconcile(t)
	assert.Equal(t, http.StatusNotFound, f.sheet(t, lead, children["Ida Sommer"]).Code, "a cancellation ends the copy")
	assert.Equal(t, "[]", storedAnswers("Ida Sommer"), "a cancellation deletes the shared answers with the contacts")
	assert.Equal(t, "[]", storedAnswers("Jan Sommer"))
	assert.NotEqual(t, "[]", storedAnswers("Ben Muster"), "a guest still booked keeps its answers")

	f.today.Store(calendar.NewDate(2099, 7, 17))
	f.reconcile(t)
	ben, _ = sheetOf("Ben Muster")
	assert.True(t, ben.ReadOnly)
	assert.Len(t, ben.Answers, 2, "the answers stay readable for seven days after the week")
	f.today.Store(calendar.NewDate(2099, 7, 18))
	f.reconcile(t)
	assert.Equal(t, http.StatusNotFound, f.sheet(t, lead, children["Ben Muster"]).Code)
	var left int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_guests WHERE tenant_id = ? AND answers <> '[]'::jsonb`, f.nord).
		Scan(t.Context(), &left))
	assert.Zero(t, left, "the answers are deleted with the contacts after the read-only days")
}

func TestHolidayGuestSheetExcludesAnswersReclassifiedAsHealthInformation(t *testing.T) {
	t.Parallel()
	for _, day := range []calendar.Date{calendar.NewDate(2099, 7, 8), calendar.NewDate(2099, 7, 17)} {
		t.Run(day.String(), func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			f := newHolidayGuestFixture(t, "Träger Gesundheitseinstufung Gäste")
			lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
			saveHolidayForm(t, f.holidayRegistrationFixture, f.office, f.periodID(t), sharedQuestions())
			registration := f.submitShared(t)
			f.confirm(t, registration, "confirm")
			rec := f.assign(t, lead, f.offer.Weeks[0].ID, f.group)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			f.today.Store(calendar.NewDate(2099, 7, 6))
			f.reconcile(t)
			student := f.roster(t, calendar.NewDate(2099, 7, 8))["Ida Sommer"]
			require.Positive(t, student)
			type sheetBody struct {
				holidayGuestSheetBody
				Answers []hostAnswerBody `json:"answers"`
			}
			rec = f.sheet(t, lead, student)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Len(t, decodeSchoolPortalData[sheetBody](t, rec.Body.Bytes()).Answers, 3)

			// Reclassify only after copying, also after the week when source
			// reconciliation no longer runs. No reconciliation before this read.
			f.today.Store(day)
			questions := sharedQuestions()
			questions[0]["health"], questions[0]["share_with_host"] = true, false
			saveHolidayForm(t, f.holidayRegistrationFixture, f.office, f.periodID(t), questions)
			for _, reconcile := range []bool{false, true} {
				if reconcile {
					f.reconcile(t)
				}
				rec = f.sheet(t, lead, student)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				sheet := decodeSchoolPortalData[sheetBody](t, rec.Body.Bytes())
				assert.Equal(t, day.After(calendar.NewDate(2099, 7, 10)), sheet.ReadOnly)
				assert.Equal(t, []hostAnswerBody{
					{Key: "ausflug_ab", Label: "Ausflug ab", Type: "date", Value: "2099-07-07"},
					{Key: "abholung", Label: "Abholung", Type: "select", Value: "Mit dem Bus"},
				}, sheet.Answers, "health information must disappear immediately, including during the read-only days")
				assert.NotContains(t, rec.Body.String(), "darf_allein_nach_hause")
				require.Len(t, sheet.Emergency, 3, "ordinary copied contacts remain readable")
			}
		})
	}
}
