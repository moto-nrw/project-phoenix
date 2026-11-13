package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

// The holiday week of these tests runs from Monday 2030-07-08 to Friday
// 2030-07-12.

type holidayClosureImpactBody struct {
	Weeks []struct {
		WeekID       int64 `json:"week_id"`
		HostSchoolID int64 `json:"host_school_id"`
		Children     int   `json:"children"`
		Families     int   `json:"families"`
		ClosedDays   []struct {
			Date string `json:"date"`
		} `json:"closed_days"`
	} `json:"weeks"`
	Families          int    `json:"families"`
	Token             string `json:"token"`
	KeepsContribution bool   `json:"keeps_contribution"`
}

type traegerClosingDaySaveBody struct {
	StartDate            string  `json:"start_date"`
	EndDate              string  `json:"end_date"`
	Reason               string  `json:"reason"`
	SchoolIDs            []int64 `json:"school_ids"`
	HolidayBookingsToken string  `json:"holiday_bookings_token,omitempty"`
}

// bookHolidayWeek books one child of the family email into a week of a
// published period of the Träger at school and returns the week.
func bookHolidayWeek(t *testing.T, organization, school int64, email string) int64 {
	t.Helper()
	ctx := context.Background()
	db := testpkg.SetupTestDB(t)
	var period, week, model, registration, child int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at) VALUES (?, 'Sommerferien', '2030-01-01', '2030-06-01', 'published', NOW())
		RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, '2030-07-08', '2030-07-12', ?) RETURNING id`, organization, period, school).Scan(ctx, &week))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'Ganztag', '08:00', '16:00', 4500) RETURNING id`, organization, week).Scan(ctx, &model))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Eva', 'Ferien', ?, '0123', NOW()) RETURNING id`, organization, period, email).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, 'Mia', 'Ferien', '2019-03-04') RETURNING id`, organization, period, registration).Scan(ctx, &child))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
		organization, period, child, week, model)
	require.NoError(t, err)
	return week
}

// proposeSchoolClosingDay saves a closing day of the school in the booked
// holiday week and the notice it proposes, as School Calendar records it.
func proposeSchoolClosingDay(t *testing.T, db *bun.DB, school int64, day string) {
	t.Helper()
	ctx := context.Background()
	var closingDay int64
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.closing_days (tenant_id, start_date, end_date, reason)
		VALUES (?, ?::date, ?::date, 'Pädagogischer Tag') RETURNING id`, school, day, day).Scan(ctx, &closingDay))
	_, err := db.ExecContext(ctx, `INSERT INTO calendar.holiday_closure_proposals (tenant_id, organization_id, source_kind, source_id,
			operation_id, reason, current_start, current_end, span_from, span_to, revision)
		VALUES (?, 0, 'school_closing_day', ?, gen_random_uuid(), 'Pädagogischer Tag', ?::date, ?::date, ?::date, ?::date, 1)`,
		school, closingDay, day, day, day, day)
	require.NoError(t, err)
}

func closureMails(t *testing.T, db *bun.DB, organization int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.email_outbox WHERE organization_id = ? AND kind = 'holiday_closure_notice'`, organization).
		Scan(context.Background(), &count))
	return count
}

// A Träger closing day in booked holiday weeks of two schools (#6434): the
// Büro sees the affected weeks before it saves, a save that did not confirm
// them is refused and saves nothing, the confirmed save tells each family
// once, and saving it unchanged tells nobody again. A foreign Träger's
// booking at its own school is neither shown nor told.
func TestTraegerClosingDayTellsTheParentsOfBookedHolidayWeeksOnce(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	office := newTraegerOffice(t, db, router, "Träger Ferienschluss", identityaccess.TraegerRoleAdministration)
	foreign := newTraegerOffice(t, db, router, "Träger Ferienschluss Fremd", identityaccess.TraegerRoleAdministration)
	north := bookHolidayWeek(t, office.organization, office.schools[0], "nord@example.org")
	south := bookHolidayWeek(t, office.organization, office.schools[1], "sued@example.org")
	bookHolidayWeek(t, foreign.organization, foreign.schools[0], "fremd@example.org")
	day := traegerClosingDaySaveBody{StartDate: "2030-07-10", EndDate: "2030-07-10", Reason: "Betriebsausflug", SchoolIDs: office.schools}

	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/holiday-closures/preview", day)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	impact := decodeSchoolPortalData[holidayClosureImpactBody](t, rec.Body.Bytes())
	require.Len(t, impact.Weeks, 2, "both schools' weeks")
	assert.ElementsMatch(t, []int64{north, south}, []int64{impact.Weeks[0].WeekID, impact.Weeks[1].WeekID})
	assert.Equal(t, 2, impact.Families)
	assert.True(t, impact.KeepsContribution)
	require.Len(t, impact.Weeks[0].ClosedDays, 1)
	assert.Equal(t, "2030-07-10", impact.Weeks[0].ClosedDays[0].Date)

	rec = traegerJSON(t, router, office.token, http.MethodPost, "/traeger/closing-days/", day)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "traeger.closing_day_bookings_changed")
	rec = traegerJSON(t, router, office.token, http.MethodGet, "/traeger/closing-days/?from=2030-01-01&to=2030-12-31", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[traegerCalendarBody](t, rec.Body.Bytes()).ClosingDays, "a refused save leaves no closing day")
	assert.Zero(t, closureMails(t, db, office.organization))

	day.HolidayBookingsToken = impact.Token
	rec = traegerJSON(t, router, office.token, http.MethodPost, "/traeger/closing-days/", day)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[struct {
		ID               int64 `json:"id"`
		NotifiedFamilies int   `json:"notified_families"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, 2, created.NotifiedFamilies)
	assert.Equal(t, 2, closureMails(t, db, office.organization), "one mail per family")
	var senders []string
	require.NoError(t, db.NewRaw(`SELECT payload->>'traeger_name' FROM platform.email_outbox
		WHERE organization_id = ? AND kind = 'holiday_closure_notice'`, office.organization).Scan(context.Background(), &senders))
	assert.Equal(t, []string{"Träger Ferienschluss", "Träger Ferienschluss"}, senders, "the notice names its Träger (#6437)")

	day.HolidayBookingsToken = ""
	rec = traegerJSON(t, router, office.token, http.MethodPut, traegerClosingDayPath(created.ID), day)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"notified_families":0`)
	assert.Equal(t, 2, closureMails(t, db, office.organization), "an unchanged save tells nobody again")
	assert.Zero(t, closureMails(t, db, foreign.organization), "the foreign Träger's family is not told")

	rec = traegerJSON(t, router, office.token, http.MethodGet, "/traeger/holiday-closures/pending", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[holidayClosureImpactBody](t, rec.Body.Bytes()).Weeks)

	// A school of the foreign Träger is refused before anything is read.
	day.SchoolIDs = foreign.schools[:1]
	rec = traegerJSON(t, router, office.token, http.MethodPost, "/traeger/closing-days/", day)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "traeger.closing_day_school_outside")
}

// The pending notices read the same statements for any number of booked
// weeks (#6434).
func TestTraegerHolidayClosurePendingQueryBudget(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	office := newTraegerOffice(t, db, router, "Träger Budget Ferienschluss", identityaccess.TraegerRoleAdministration)
	bookHolidayWeek(t, office.organization, office.schools[0], "eins@example.org")
	read := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, db)
		req := httptest.NewRequest(http.MethodGet, "/traeger/holiday-closures/pending", nil)
		req.Header.Set("Authorization", "Bearer "+office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, router, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	one := read()
	bookHolidayWeek(t, office.organization, office.schools[1], "zwei@example.org")
	bookHolidayWeek(t, office.organization, office.schools[0], "drei@example.org")
	assert.Len(t, read().Statements, len(one.Statements), "the pending notices read the same statements for any number of weeks")
	querybudgets.AssertQueryBudget(t, "api.traeger.holiday_closures_pending", one)

	// Notices that wait for a click are planned together: one more notice,
	// at another school, reads no further statement.
	proposeSchoolClosingDay(t, db, office.schools[0], "2030-07-09")
	single := read()
	require.Greater(t, len(single.Statements), len(one.Statements), "a waiting notice is planned")
	proposeSchoolClosingDay(t, db, office.schools[1], "2030-07-10")
	proposeSchoolClosingDay(t, db, office.schools[0], "2030-07-11")
	assert.Len(t, read().Statements, len(single.Statements), "the waiting notices are planned in the same statements for any number of notices")
}
