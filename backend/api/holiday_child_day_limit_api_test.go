// Router-level tests of the bound of care days per child across the
// registrations of a period (#6446): a second registration of the same
// child, two registrations at once, the rebooking and the cancellation.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// dayLimitRefusal reads a refusal of the bound of care days.
func dayLimitRefusal(t *testing.T, code int, body []byte) map[string]any {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, code, string(body))
	var refusal struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(body, &refusal), string(body))
	require.Equal(t, "enrollment.holiday_child_day_limit", refusal.Code)
	return refusal.Details
}

// TestHolidayChildDayLimitCountsTheChildsOtherRegistrations pins the bound
// of three days over two registrations of the same child: the second asks
// for two more days and is refused, one more day is fine. Another family
// with a child of the same name and date of birth keeps its own days. A
// rebooking of the second registration counts the first as well; after the
// first is cancelled its days are free again. Each refusal goes red without
// the count across registrations.
func TestHolidayChildDayLimitCountsTheChildsOtherRegistrations(t *testing.T) {
	t.Parallel()
	d := newDayFixture(t, "Träger Tage Zweite Anmeldung")

	first := d.submit(t, d.registration("eva.zweite@example.org", "Ole", d.days(dayMonday, dayTuesday)))
	require.Equal(t, "placed", first.Placements[0].Placement)

	rec := publicJSON(t, d.public, http.MethodPost, registrationPath(d.period.PublicToken),
		d.registration(" EVA.Zweite@example.org", "ole", d.days(dayWednesday, dayThursday)))
	assert.Equal(t, map[string]any{"limit": float64(3), "days": float64(4), "elsewhere": float64(2), "child": float64(1)},
		dayLimitRefusal(t, rec.Code, rec.Body.Bytes()), "the days of the first registration count")

	second := d.submit(t, d.registration("eva.zweite@example.org", "Ole", d.days(dayWednesday)))
	other := d.submit(t, d.registration("ina.zweite@example.org", "Ole", d.days(dayThursday, dayFriday)))
	assert.Equal(t, "placed", other.Placements[0].Placement, "another family's child of the same name is another child")

	child := d.childOf(t, second.ID)
	rebook := func(bookings ...map[string]any) *httptest.ResponseRecorder {
		return traegerJSON(t, d.traeger, d.office.token, http.MethodPut, holidayRebookPath(d.period.ID, second.ID, child),
			map[string]any{"bookings": bookings})
	}
	rec = rebook(d.days(dayWednesday, dayThursday))
	assert.Equal(t, map[string]any{"limit": float64(3), "days": float64(4), "elsewhere": float64(2), "child": float64(1)},
		dayLimitRefusal(t, rec.Code, rec.Body.Bytes()), "a rebooking counts the other registration")
	rec = rebook(d.days(dayThursday))
	assert.Equal(t, http.StatusOK, rec.Code, "the same number of days on another day is fine: %s", rec.Body.String())

	rec = traegerJSON(t, d.traeger, d.office.token, http.MethodPost, holidayCancelPath(d.period.ID, first.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = rebook(d.days(dayMonday, dayTuesday, dayThursday))
	assert.Equal(t, http.StatusOK, rec.Code, "a cancelled registration holds no days: %s", rec.Body.String())
}

// TestHolidayChildDayLimitHoldsAgainstAConcurrentRegistrationOfTheSameChild
// runs two registrations of the same child at once. The first holds the
// period's row, as every submission does, and books Monday and Tuesday; the
// second, for Wednesday and Thursday, waits for the lock and then counts
// four days. Without the lock of the period the second counts in its own
// snapshot, finds no other day and passes the bound.
func TestHolidayChildDayLimitHoldsAgainstAConcurrentRegistrationOfTheSameChild(t *testing.T) {
	t.Parallel()
	d := newDayFixture(t, "Träger Tage Grenze Gleichzeitig")
	week := d.offer.Weeks[0]
	ctx := context.Background()

	holder, err := d.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	committed := false
	defer func() {
		if !committed {
			require.NoError(t, holder.Rollback())
		}
	}()
	var pid int
	require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid))
	_, err = holder.ExecContext(ctx, `SELECT id FROM enrollment.holiday_periods WHERE id = ? FOR UPDATE`, d.period.ID)
	require.NoError(t, err)
	var registration, child int64
	require.NoError(t, holder.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id,
			guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Eva', 'Tag', 'eva.gleichzeitig@example.org', '0123', NOW()) RETURNING id`,
		d.office.organization, d.period.ID).Scan(ctx, &registration))
	require.NoError(t, holder.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, 'Ole', 'Tag', '2017-05-04') RETURNING id`, d.office.organization, d.period.ID, registration).Scan(ctx, &child))
	_, err = holder.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement, days)
		VALUES (?, ?, ?, ?, ?, 'placed', ?::date[])`,
		d.office.organization, d.period.ID, child, week.ID, week.TimeModels[0].ID, "{"+dayMonday+","+dayTuesday+"}")
	require.NoError(t, err)

	type answer struct {
		code int
		body []byte
	}
	sent := make(chan answer, 1)
	go func() {
		rec := publicJSON(t, d.public, http.MethodPost, registrationPath(d.period.PublicToken),
			d.registration("eva.gleichzeitig@example.org", "Ole", d.days(dayWednesday, dayThursday)))
		sent <- answer{rec.Code, rec.Body.Bytes()}
	}()
	var done *answer
	deadline := time.Now().Add(20 * time.Second)
	for done == nil && time.Now().Before(deadline) {
		var blocked int
		require.NoError(t, d.db.NewRaw(`SELECT count(*) FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid))`, pid).Scan(ctx, &blocked))
		if blocked > 0 {
			break
		}
		select {
		case a := <-sent:
			done = &a
		case <-time.After(20 * time.Millisecond):
		}
	}
	require.NoError(t, holder.Commit())
	committed = true
	if done == nil {
		a := <-sent
		done = &a
	}
	assert.Equal(t, map[string]any{"limit": float64(3), "days": float64(4), "elsewhere": float64(2), "child": float64(1)},
		dayLimitRefusal(t, done.code, done.body), "the second registration counts the days the first committed")
}

// TestHolidayWeekChangeCountsTheChildsOtherRegistrations: the check of a
// changed period (#6506) counts the same child over its registrations
// (#6446). Two one-day weeks booked as a whole in two registrations of one
// child fill a bound of two; making the second week two days long would
// give the child three. Per registration each child stays within the bound,
// so the change passes without the count across registrations.
func TestHolidayWeekChangeCountsTheChildsOtherRegistrations(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Woche Zwei Anmeldungen", identityaccess.TraegerRoleAdministration)
	model := []map[string]any{{"name": "bis 13 Uhr", "starts_at": "08:00", "ends_at": "13:00", "price_cents": 4500}}
	body := map[string]any{
		"name": "Winterferien 2027", "registration_opens_on": "2020-01-01", "registration_closes_on": "2099-12-31",
		"binding": true, "notes": "", "max_days_per_child": 2,
		"weeks": []map[string]any{
			{"starts_on": "2027-01-11", "ends_on": "2027-01-11", "host_school_id": office.schools[0], "time_models": model},
			{"starts_on": "2027-01-18", "ends_on": "2027-01-18", "host_school_id": office.schools[0], "time_models": model},
		},
	}
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPost, "/traeger/holiday-periods/", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	d := dayFixture{holidayRegistrationFixture: f, office: office, period: decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())}
	d.offer = d.readOffer(t)
	for _, week := range d.offer.Weeks {
		d.submit(t, d.registration("eva.wochen@example.org", "Ole",
			map[string]any{"week_id": week.ID, "time_model_id": week.TimeModels[0].ID}))
	}

	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, holidayPeriodPath(created.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	period := decodeSchoolPortalData[map[string]any](t, rec.Body.Bytes())
	period["weeks"].([]any)[1].(map[string]any)["ends_on"] = "2027-01-19"
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayPeriodPath(created.ID), period)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_week_day_limit", code)
	assert.Equal(t, []string{"weeks.1.ends_on"}, fields)
}
