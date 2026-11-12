// Router-level tests for archiving a holiday time model (#5994): a time
// model bookings name cannot be removed, so the Träger-Büro archives it. It
// leaves the offer and takes no new booking; the days booked on it keep it.
package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// archivedTimeModel reads whether the time model is archived and the times
// it keeps.
func archivedTimeModel(t *testing.T, f reviewFixture, id int64) (archived bool, name, startsAt, endsAt string) {
	t.Helper()
	require.NoError(t, f.db.NewRaw(`SELECT archived_at IS NOT NULL, name, to_char(starts_at, 'HH24:MI'), to_char(ends_at, 'HH24:MI')
		FROM enrollment.holiday_week_time_models WHERE id = ?`, id).Scan(context.Background(), &archived, &name, &startsAt, &endsAt))
	return archived, name, startsAt, endsAt
}

// periodWith reads the period as the editor does and changes the time
// model id of its first week with change.
func (f reviewFixture) periodWith(t *testing.T, id int64, change func(*holidayTimeModelBody)) holidayPeriodBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayPeriodPath(f.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	period := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	for i := range period.Weeks[0].TimeModels {
		if period.Weeks[0].TimeModels[i].ID == id {
			change(&period.Weeks[0].TimeModels[i])
		}
	}
	return period
}

// Archiving needs the right "Anmeldung verwalten" and stays within the
// Träger: a reading member and another Träger archive nothing.
func TestArchivingATimeModelNeedsTheRightAndTheTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Archiv Recht")
	booked := f.offer.Weeks[0].TimeModels[0].ID
	archive := f.periodWith(t, booked, func(model *holidayTimeModelBody) { model.Archived = true })

	password := testpkg.NewTestPassword()
	reader := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, reader.ID, identityaccess.TraegerRoleReadOnly)
	code, answer := traegerSignIn(t, f.traeger, reader.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	rec := traegerJSON(t, f.traeger, answer.AccessToken, http.MethodPut, holidayPeriodPath(f.period.ID), archive)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	other := newTraegerOffice(t, f.db, f.traeger, "Träger Archiv Fremd", identityaccess.TraegerRoleAdministration)
	rec = traegerJSON(t, f.traeger, other.token, http.MethodPut, holidayPeriodPath(f.period.ID), archive)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	code2, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_period_not_found", code2)

	archived, _, _, _ := archivedTimeModel(t, f, booked)
	assert.False(t, archived, "neither the reading member nor another Träger archived the time model")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayPeriodPath(f.period.ID), archive)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	archived, _, _, _ = archivedTimeModel(t, f, booked)
	assert.True(t, archived, "the Träger-Büro with the right archives it")
}

// A booked time model is archived instead of removed: it is marked as
// protected, keeps its times, leaves the offer and takes no new booking,
// while a child already booked on it may keep that booking in a rebooking.
func TestArchivedTimeModelIsNoLongerBookable(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Archiv")
	registration := f.list(t, "").Registrations[0]
	lia, ben := registration.Children[0], registration.Children[1]
	f.detail(t, f.act(t, f.siblings, "confirm"))
	week := f.offer.Weeks[0]
	booked := week.TimeModels[0].ID // Ben's "bis 13 Uhr" of the first week

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayPeriodPath(f.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	read := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	require.Equal(t, booked, read.Weeks[0].TimeModels[0].ID)
	assert.True(t, read.Weeks[0].TimeModels[0].Protected, "a booked time model is protected")
	assert.False(t, read.Weeks[0].TimeModels[0].Archived)

	// The editor archives it; a changed name or time does not reach it.
	archive := f.periodWith(t, booked, func(model *holidayTimeModelBody) {
		model.Archived, model.Name, model.EndsAt = true, "Umbenannt", "11:00"
	})
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayPeriodPath(f.period.ID), archive)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	stored := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	require.Equal(t, booked, stored.Weeks[0].TimeModels[0].ID, "the archived time model stays in the period")
	assert.True(t, stored.Weeks[0].TimeModels[0].Archived)
	archived, name, startsAt, endsAt := archivedTimeModel(t, f, booked)
	assert.True(t, archived)
	assert.Equal(t, []string{week.TimeModels[0].Name, "08:00", "13:00"}, []string{name, startsAt, endsAt},
		"the past days keep the times they were booked on")

	offer := holidayOffer(t, f.holidayRegistrationFixture, f.period.PublicToken)
	for _, model := range offer.Weeks[0].TimeModels {
		assert.NotEqual(t, booked, model.ID, "the offer no longer shows the archived time model")
	}
	require.NotEmpty(t, offer.Weeks[0].TimeModels, "the week stays bookable with its other time model")

	body := siblingsRegistration(f.offer, "neu.muster@example.org")
	body["children"] = body["children"].([]map[string]any)[1:]
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), body)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a new registration cannot book the archived time model: %s", rec.Body.String())

	onto := map[string]any{"bookings": []map[string]any{{"week_id": week.ID, "time_model_id": booked}}}
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut,
		holidayRebookPath(f.period.ID, f.siblings, lia.ID), onto)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "nor does a rebooking onto it: %s", rec.Body.String())

	// Ben keeps his booking on it and gives up the second week.
	keep := map[string]any{"bookings": []map[string]any{{"week_id": week.ID, "time_model_id": booked}}}
	f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodPut,
		holidayRebookPath(f.period.ID, f.siblings, ben.ID), keep))

	// A period written without it, or with it no longer archived, keeps it
	// archived: there is no way back.
	without := f.periodWith(t, booked, func(*holidayTimeModelBody) {})
	var rest []holidayTimeModelBody
	for _, model := range without.Weeks[0].TimeModels {
		if model.ID != booked {
			rest = append(rest, model)
		}
	}
	without.Weeks[0].TimeModels = rest
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayPeriodPath(f.period.ID), without)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	restore := f.periodWith(t, booked, func(model *holidayTimeModelBody) { model.Archived = false })
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayPeriodPath(f.period.ID), restore)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	archived, _, _, _ = archivedTimeModel(t, f, booked)
	assert.True(t, archived, "the archived time model is neither removed nor restored")
}
