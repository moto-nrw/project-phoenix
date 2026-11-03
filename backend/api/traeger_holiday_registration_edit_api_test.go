// Router-level tests for the Träger-Büro's edit of the details of a holiday
// registration (#5629): each section changes and shows in the detail, the
// checks of the public form with their field paths, the state of the
// registration, the history entry without values, the refused health answer,
// the hosting school's view and two edits at the same time.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type editedRegistration struct {
	GuardianFirstName string `json:"guardian_first_name"`
	GuardianEmail     string `json:"guardian_email"`
	GuardianPhone     string `json:"guardian_phone"`
	SecondGuardian    *struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
	} `json:"second_guardian"`
	Children []struct {
		ID              int64  `json:"id"`
		FirstName       string `json:"first_name"`
		LastName        string `json:"last_name"`
		BirthDate       string `json:"birth_date"`
		HomeSchoolID    *int64 `json:"home_school_id"`
		GuestSchoolName string `json:"guest_school_name"`
	} `json:"children"`
	Contacts []struct {
		Kind  string `json:"kind"`
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"contacts"`
	CustomData      map[string]any `json:"custom_data"`
	ChildCustomData []struct {
		ChildID    int64          `json:"child_id"`
		CustomData map[string]any `json:"custom_data"`
	} `json:"child_custom_data"`
	History []struct {
		Action  string         `json:"action"`
		Details map[string]any `json:"details"`
	} `json:"history"`
}

// newEditFixture is the review fixture over a period whose form asks the
// questions of questionsForm; the siblings answered them. Lia's tetanus
// answer is health information.
func newEditFixture(t *testing.T, name string) reviewFixture {
	t.Helper()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, name, identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	saveHolidayForm(t, f, office, period.ID, questionsForm())
	offer := holidayOffer(t, f, period.PublicToken)
	body := siblingsRegistration(offer, "eva.muster@example.org")
	body["form_version"] = 2
	body["custom_data"] = map[string]any{"abholung_hinweis": "Oma holt ab"}
	answerChildren(body,
		map[string]any{"tetanus_impfschutz": "ja", "lieblingsessen": "Nudeln"},
		map[string]any{"tetanus_impfschutz": "nein", "darf_allein_nach_hause": false})
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	siblings := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
	return reviewFixture{holidayRegistrationFixture: f, office: office, period: period, offer: offer, siblings: siblings}
}

func (f reviewFixture) editDetails(t *testing.T, id int64, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, f.office.token, http.MethodPut, f.registrationPath(id, "/details"), body)
}

func (f reviewFixture) edited(t *testing.T, rec *httptest.ResponseRecorder) editedRegistration {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[editedRegistration](t, rec.Body.Bytes())
}

func (f reviewFixture) readDetail(t *testing.T, id int64) editedRegistration {
	t.Helper()
	return f.edited(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(id, ""), nil))
}

// editEntries are the details of the edit entries of a registration's audit
// trail, oldest first, as stored.
func (f reviewFixture) editEntries(t *testing.T, id int64) []string {
	t.Helper()
	var details []string
	require.NoError(t, f.db.NewRaw(`SELECT details::text FROM enrollment.holiday_registration_audit
		WHERE holiday_registration_id = ? AND action = 'edit' ORDER BY id`, id).Scan(context.Background(), &details))
	return details
}

// sealedHealth is the sealed health information of the family and of each
// child of a registration, as stored.
func (f reviewFixture) sealedHealth(t *testing.T, id int64) [][]byte {
	t.Helper()
	var family [][]byte
	require.NoError(t, f.db.NewRaw(`SELECT health_data FROM enrollment.holiday_registrations WHERE id = ?`, id).Scan(context.Background(), &family))
	var children [][]byte
	require.NoError(t, f.db.NewRaw(`SELECT health_data FROM enrollment.holiday_registration_children
		WHERE holiday_registration_id = ? ORDER BY position`, id).Scan(context.Background(), &children))
	return append(family, children...)
}

func editedGuardians(phone string, second map[string]any) map[string]any {
	return map[string]any{
		"guardian":        map[string]any{"first_name": "Eva", "last_name": "Muster", "email": "eva.muster@example.org", "phone": phone},
		"second_guardian": second,
	}
}

func TestTraegerEditsEachSectionOfAHolidayRegistration(t *testing.T) {
	t.Parallel()
	f := newEditFixture(t, "Träger Bearbeiten Ferien")
	before := f.readDetail(t, f.siblings)
	lia, ben := before.Children[0], before.Children[1]
	health := f.sealedHealth(t, f.siblings)

	guardians := f.edited(t, f.editDetails(t, f.siblings, editedGuardians("0151 9999999",
		map[string]any{"first_name": "Max", "last_name": "Muster", "email": " Max.Muster@Example.org "})))
	assert.Equal(t, "0151 9999999", guardians.GuardianPhone)
	require.NotNil(t, guardians.SecondGuardian, "the second guardian is added")
	assert.Equal(t, "max.muster@example.org", guardians.SecondGuardian.Email, "normalized like the public form")
	last := guardians.History[len(guardians.History)-1]
	assert.Equal(t, "edit", last.Action)
	assert.Equal(t, []any{"guardian.phone", "second_guardian"}, last.Details["fields"])

	removed := f.edited(t, f.editDetails(t, f.siblings, editedGuardians("0151 9999999", nil)))
	assert.Nil(t, removed.SecondGuardian, "an absent second guardian is removed")

	children := f.edited(t, f.editDetails(t, f.siblings, map[string]any{"children": []map[string]any{{
		"id": ben.ID, "first_name": "Ben", "last_name": "Mustermann", "birth_date": "2016-09-13",
		"home_school_id": f.offer.Schools[0].ID, "guest_school_name": "Grundschule Melle",
		"custom_data": map[string]any{"darf_allein_nach_hause": true, "allein_ab": "15 Uhr"},
	}}}))
	editedBen := children.Children[1]
	assert.Equal(t, "Mustermann", editedBen.LastName)
	assert.Equal(t, "2016-09-13", editedBen.BirthDate)
	require.NotNil(t, editedBen.HomeSchoolID, "the holiday guest now attends a school of the Träger")
	assert.Equal(t, f.offer.Schools[0].ID, *editedBen.HomeSchoolID)
	assert.Empty(t, editedBen.GuestSchoolName, "a child of a school of the Träger names no own school")
	assert.Equal(t, lia, children.Children[0], "a child the edit does not name stays as it is")
	assert.Equal(t, map[string]any{"darf_allein_nach_hause": true, "allein_ab": "15 Uhr"}, children.ChildCustomData[1].CustomData)
	assert.Equal(t, []any{"children.1.last_name", "children.1.birth_date", "children.1.home_school_id",
		"children.1.guest_school_name", "children.1.custom_data.allein_ab", "children.1.custom_data.darf_allein_nach_hause"},
		children.History[len(children.History)-1].Details["fields"])

	contacts := f.edited(t, f.editDetails(t, f.siblings, map[string]any{
		"emergency_contacts": []map[string]any{
			{"name": "Oma Muster", "phone": "05402 9999", "relationship": "Großmutter"},
			{"name": "Tante Muster", "phone": "05402 5555"},
		},
		"pickup_persons": []map[string]any{},
	}))
	kinds := []string{}
	for _, contact := range contacts.Contacts {
		kinds = append(kinds, contact.Kind+":"+contact.Name+":"+contact.Phone)
	}
	assert.Equal(t, []string{"emergency:Oma Muster:05402 9999", "emergency:Tante Muster:05402 5555"}, kinds, "the pickup person is removed")
	assert.Equal(t, []any{"emergency_contacts.0.phone", "emergency_contacts.1", "pickup_persons.0"},
		contacts.History[len(contacts.History)-1].Details["fields"])

	answers := f.edited(t, f.editDetails(t, f.siblings, map[string]any{"custom_data": map[string]any{"abholung_hinweis": "Opa holt ab"}}))
	assert.Equal(t, map[string]any{"abholung_hinweis": "Opa holt ab"}, answers.CustomData)
	assert.Equal(t, []any{"custom_data.abholung_hinweis"}, answers.History[len(answers.History)-1].Details["fields"])

	assert.Equal(t, health, f.sealedHealth(t, f.siblings), "the sealed health information stays as it was")
	shown := f.readDetail(t, f.siblings)
	assert.Equal(t, "Mustermann", shown.Children[1].LastName, "the detail shows the new values")
	assert.Equal(t, "0151 9999999", shown.GuardianPhone)

	entries := f.editEntries(t, f.siblings)
	require.Len(t, entries, 5, "each saved change writes exactly one entry")
	for _, entry := range entries {
		var details map[string]any
		require.NoError(t, json.Unmarshal([]byte(entry), &details))
		assert.Equal(t, []string{"fields"}, mapKeys(details), "the entry names the fields only")
		for _, value := range []string{"0151 9999999", "Mustermann", "Opa holt ab", "05402 9999", "Max"} {
			assert.NotContains(t, entry, value, "the entry carries no value")
		}
	}

	unchanged := f.edited(t, f.editDetails(t, f.siblings, map[string]any{"custom_data": map[string]any{"abholung_hinweis": " Opa holt ab "}}))
	assert.Len(t, unchanged.History, len(answers.History), "an edit without a change writes no entry")
	assert.Len(t, f.editEntries(t, f.siblings), 5)
}

func mapKeys(values map[string]any) []string {
	keys := []string{}
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func TestTraegerEditRefusesWhatThePublicFormRefuses(t *testing.T) {
	t.Parallel()
	f := newEditFixture(t, "Träger Bearbeiten Prüfung")
	detail := f.readDetail(t, f.siblings)
	lia := detail.Children[0]
	child := func(changes map[string]any) map[string]any {
		body := map[string]any{"id": lia.ID, "first_name": "Lia", "last_name": "Muster", "birth_date": "2018-03-01",
			"home_school_id": f.offer.Schools[0].ID, "custom_data": map[string]any{"lieblingsessen": "Nudeln"}}
		for key, value := range changes {
			body[key] = value
		}
		return map[string]any{"children": []map[string]any{body}}
	}
	for _, refusal := range []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"invalid e-mail", map[string]any{"guardian": map[string]any{"first_name": "Eva", "last_name": "Muster", "email": "eva@", "phone": "0151 1"}}, "guardian.email"},
		{"missing last name", child(map[string]any{"last_name": " "}), "children.0.last_name"},
		{"implausible date of birth", child(map[string]any{"birth_date": "2099-01-01"}), "children.0.birth_date"},
		{"unknown school", child(map[string]any{"home_school_id": 999999999}), "children.0.home_school_id"},
		{"no emergency contact", map[string]any{"emergency_contacts": []map[string]any{}}, "emergency_contacts"},
		{"required question without answer", map[string]any{"custom_data": map[string]any{}}, "custom_data.abholung_hinweis"},
		{"answer with health information", child(map[string]any{"custom_data": map[string]any{"tetanus_impfschutz": "nein"}}), "children.0.custom_data.tetanus_impfschutz"},
	} {
		rec := f.editDetails(t, f.siblings, refusal.body)
		require.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", refusal.name, rec.Body.String())
		code, fields := registrationRefusalOf(t, rec)
		assert.Equal(t, "enrollment.holiday_registration_invalid", code, refusal.name)
		assert.Equal(t, []string{refusal.field}, fields, refusal.name)
	}
	assert.Empty(t, f.editEntries(t, f.siblings), "a refused edit writes no entry")
	assert.Equal(t, detail.Children, f.readDetail(t, f.siblings).Children, "a refused edit changes nothing")

	rec := f.editDetails(t, f.siblings, map[string]any{"children": []map[string]any{{"id": 999999999, "first_name": "X", "last_name": "Y", "birth_date": "2018-03-01"}}})
	require.Equal(t, http.StatusNotFound, rec.Code, "a child of no registration: %s", rec.Body.String())
}

func TestTraegerEditFollowsTheStateAndKeepsContributionAndInvoice(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Bearbeiten Zustand")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())

	frozen := f.detail(t, f.act(t, f.siblings, "freeze"))
	require.True(t, frozen.Frozen)
	f.edited(t, f.editDetails(t, f.siblings, editedGuardians("0151 1111111", nil)))
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := startBillingRun(t, f, billing, f.period.ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(billingRun(t, f, billing, run.ID)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	invoice := billingRun(t, f, billing, run.ID)
	require.Len(t, invoice.Documents, 1)
	require.NotEmpty(t, invoice.Documents[0].Number)
	contribution := f.contribution(t, f.siblings)

	ben := f.readDetail(t, f.siblings).Children[1]
	confirmed := f.edited(t, f.editDetails(t, f.siblings, map[string]any{
		"guardian": map[string]any{"first_name": "Evelyn", "last_name": "Muster", "email": "evelyn.muster@example.org", "phone": "0151 1111111"},
		"children": []map[string]any{{"id": ben.ID, "first_name": "Benjamin", "last_name": "Muster", "birth_date": "2016-09-12", "home_school_id": 0}},
	}))
	assert.Equal(t, "Evelyn", confirmed.GuardianFirstName, "a confirmed registration can be changed")
	assert.Equal(t, contribution, f.contribution(t, f.siblings), "no detail changes the contribution")
	assert.Equal(t, invoice, billingRun(t, f, billing, run.ID), "an issued invoice keeps its own copy")

	assert.Equal(t, "cancelled", f.detail(t, f.act(t, f.again, "cancel")).Status)
	rec = f.editDetails(t, f.again, editedGuardians("0151 2222222", nil))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_state_conflict", code)
	assert.Empty(t, f.editEntries(t, f.again))
}

func TestTraegerEditNeedsTheRightAndStaysWithItsTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Bearbeiten Grenze")
	readOnly := newTraegerOffice(t, f.db, f.traeger, "Träger Bearbeiten Lesend", identityaccess.TraegerRoleReadOnly)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Bearbeiten Fremd", identityaccess.TraegerRoleAdministration)
	body := editedGuardians("0151 3333333", nil)
	rec := traegerJSON(t, f.traeger, readOnly.token, http.MethodPut, f.registrationPath(f.siblings, "/details"), body)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodPut, f.registrationPath(f.siblings, "/details"), body)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Empty(t, f.editEntries(t, f.siblings))
	assert.Equal(t, "0151 2345678", f.readDetail(t, f.siblings).GuardianPhone)
}

// Two edits of one registration at the same time run one after the other
// through the lock of the registration: neither is lost.
func TestTraegerEditsOfOneRegistrationRunOneAfterTheOther(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Bearbeiten Sperre")
	bodies := []map[string]any{
		editedGuardians("0151 4444444", nil),
		{"emergency_contacts": []map[string]any{{"name": "Nachbarin Muster", "phone": "05402 7777"}}, "pickup_persons": []map[string]any{}},
	}
	var wg sync.WaitGroup
	codes := make([]int, len(bodies))
	for i, body := range bodies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = f.editDetails(t, f.siblings, body).Code
		}()
	}
	wg.Wait()
	assert.Equal(t, []int{http.StatusOK, http.StatusOK}, codes)
	detail := f.readDetail(t, f.siblings)
	assert.Equal(t, "0151 4444444", detail.GuardianPhone)
	require.Len(t, detail.Contacts, 1)
	assert.Equal(t, "Nachbarin Muster", detail.Contacts[0].Name)
	assert.Len(t, f.editEntries(t, f.siblings), 2)
}

// The hosting school reads its participant list live: a changed name shows
// at once. The copy of a holiday guest follows at the next reconciliation.
func TestHolidayHostSeesAnEditedNameAtOnceAndTheGuestCopyAfterTheReconciliation(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Bearbeiten Ausrichtung")
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)
	f.confirm(t, f.siblings, "confirm")
	f.today.Store(calendar.NewDate(2099, 7, 6))
	f.reconcile(t)
	weekDay := calendar.NewDate(2099, 7, 8)
	require.Equal(t, []string{"Ben Muster"}, names(f.roster(t, weekDay)))

	path := fmt.Sprintf("%s/registrations/%d/details", holidayPeriodPath(f.periodID(t)), f.siblings)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, fmt.Sprintf("%s/registrations/%d", holidayPeriodPath(f.periodID(t)), f.siblings), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	ben := decodeSchoolPortalData[editedRegistration](t, rec.Body.Bytes()).Children[1]
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, map[string]any{"children": []map[string]any{{
		"id": ben.ID, "first_name": "Ben", "last_name": "Mustermann", "birth_date": "2016-09-12", "home_school_id": 0,
		"guest_school_name": "Grundschule Melle", "custom_data": map[string]any{},
	}}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	lastNames := []string{}
	for _, participant := range f.participants(t, lead, week).Participants {
		lastNames = append(lastNames, participant.FirstName+" "+participant.LastName)
	}
	assert.Contains(t, lastNames, "Ben Mustermann", "the participant list reads the registration live")
	assert.Equal(t, []string{"Ben Muster"}, names(f.roster(t, weekDay)), "the guest copy waits for the reconciliation")
	f.reconcile(t)
	assert.Equal(t, []string{"Ben Mustermann"}, names(f.roster(t, weekDay)), "the reconciliation follows the new name")
}

// dependentAnswersForm asks per family whether trips are possible and per
// child the trip destination (only when trips are possible), the tetanus
// protection (health information) and its date (only with protection).
func dependentAnswersForm() []map[string]any {
	return []map[string]any{
		{"key": "ausfluege_moeglich", "label": "Ausflüge möglich", "type": "boolean"},
		{"key": "ausflug_ziel", "label": "Lieblingsausflug", "type": "text", "applies_to_child": true,
			"visible_when": map[string]any{"source": "field", "field": "ausfluege_moeglich", "operator": "eq", "value": true}},
		{"key": "tetanus_impfschutz", "label": "Tetanus-Impfschutz", "type": "select", "applies_to_child": true, "health": true,
			"options": []map[string]any{{"label": "Ja", "value": "ja"}, {"label": "Nein", "value": "nein"}}},
		{"key": "tetanus_datum", "label": "Letzte Tetanus-Impfung", "type": "text", "applies_to_child": true,
			"visible_when": map[string]any{"source": "field", "field": "tetanus_impfschutz", "operator": "eq", "value": "ja"}},
	}
}

// newDependentAnswersFixture registers the siblings with Lia's answers to
// dependentAnswersForm: trips possible, Zoo, tetanus protection from May 2024.
func newDependentAnswersFixture(t *testing.T, name string) (reviewFixture, int64) {
	t.Helper()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, name, identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	saveHolidayForm(t, f, office, period.ID, dependentAnswersForm())
	offer := holidayOffer(t, f, period.PublicToken)
	body := siblingsRegistration(offer, "eva.muster@example.org")
	body["form_version"] = 2
	body["custom_data"] = map[string]any{"ausfluege_moeglich": true}
	answerChildren(body,
		map[string]any{"ausflug_ziel": "Zoo", "tetanus_impfschutz": "ja", "tetanus_datum": "Mai 2024"},
		map[string]any{"ausflug_ziel": "Museum"})
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	siblings := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
	f2 := reviewFixture{holidayRegistrationFixture: f, office: office, period: period, offer: offer, siblings: siblings}
	return f2, f2.readDetail(t, siblings).Children[0].ID
}

func childAnswersOf(detail editedRegistration, childID int64) map[string]any {
	for _, child := range detail.ChildCustomData {
		if child.ChildID == childID {
			return child.CustomData
		}
	}
	return nil
}

// An answer whose question depends on a question with health information
// stays when the Träger-Büro changes the child's name: the edit cannot see
// the sealed controlling answer, so it does not decide on the visibility.
func TestTraegerEditKeepsAnAnswerThatDependsOnAHealthQuestion(t *testing.T) {
	t.Parallel()
	f, lia := newDependentAnswersFixture(t, "Träger Bearbeiten Gesundheitsbedingung")
	health := f.sealedHealth(t, f.siblings)

	edited := f.edited(t, f.editDetails(t, f.siblings, map[string]any{"children": []map[string]any{{
		"id": lia, "first_name": "Lia", "last_name": "Mustermann", "birth_date": "2018-03-01",
		"home_school_id": f.offer.Schools[0].ID,
		"custom_data":    map[string]any{"ausflug_ziel": "Zoo", "tetanus_datum": "Mai 2024"},
	}}}))
	assert.Equal(t, map[string]any{"ausflug_ziel": "Zoo", "tetanus_datum": "Mai 2024"}, childAnswersOf(edited, lia),
		"the answer behind the health question is kept")
	assert.Equal(t, []any{"children.0.last_name"}, edited.History[len(edited.History)-1].Details["fields"])
	assert.Equal(t, health, f.sealedHealth(t, f.siblings), "the sealed health information stays as it was")
}

// A change of a family answer reaches the answers of every child: an answer
// whose question the change hides goes, and the history names it.
func TestTraegerEditOfAFamilyAnswerUpdatesTheChildAnswers(t *testing.T) {
	t.Parallel()
	f, lia := newDependentAnswersFixture(t, "Träger Bearbeiten Familienantwort")
	ben := f.readDetail(t, f.siblings).Children[1].ID

	edited := f.edited(t, f.editDetails(t, f.siblings, map[string]any{"custom_data": map[string]any{"ausfluege_moeglich": false}}))
	assert.Equal(t, map[string]any{"tetanus_datum": "Mai 2024"}, childAnswersOf(edited, lia),
		"the hidden trip answer goes, the answer behind the health question stays")
	assert.Empty(t, childAnswersOf(edited, ben))
	assert.Equal(t, []any{"children.0.custom_data.ausflug_ziel", "children.1.custom_data.ausflug_ziel", "custom_data.ausfluege_moeglich"},
		edited.History[len(edited.History)-1].Details["fields"])
}
