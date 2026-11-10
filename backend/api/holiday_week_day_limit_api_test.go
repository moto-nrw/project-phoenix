// Router-level tests of the bound of care days per child against a change of
// the period (#6506): a booking of a whole week takes the care days of the
// week's current dates, so new dates of a booked week may not take a child
// over the bound. Keeping or lowering the days stays possible, also above a
// bound lowered later.
package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Monday 11 to Thursday 14 January 2027.
const (
	limitMonday    = "2027-01-11"
	limitTuesday   = "2027-01-12"
	limitWednesday = "2027-01-13"
	limitThursday  = "2027-01-14"
)

// weekLimitFixture is a published period with one week booked as a whole
// from Monday to endsOn and a bound of limit care days per child.
type weekLimitFixture struct {
	holidayRegistrationFixture
	office traegerOffice
	period traegerPeriodLink
	offer  dayOfferBody
}

func newWeekLimitFixture(t *testing.T, name, endsOn string, limit int) weekLimitFixture {
	t.Helper()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, name, identityaccess.TraegerRoleAdministration)
	body := map[string]any{
		"name": "Winterferien 2027", "registration_opens_on": "2020-01-01", "registration_closes_on": "2099-12-31",
		"binding": true, "notes": "", "max_days_per_child": limit,
		"weeks": []map[string]any{{
			"starts_on": limitMonday, "ends_on": endsOn, "host_school_id": office.schools[0],
			"time_models": []map[string]any{{"name": "bis 13 Uhr", "starts_at": "08:00", "ends_at": "13:00", "price_cents": 4500}},
		}},
	}
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPost, "/traeger/holiday-periods/", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	w := weekLimitFixture{holidayRegistrationFixture: f, office: office, period: decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())}
	w.offer = dayFixture{holidayRegistrationFixture: f, period: w.period}.readOffer(t)
	return w
}

// register registers one child for the whole week.
func (w weekLimitFixture) register(t *testing.T, email string) receiptBody {
	t.Helper()
	d := dayFixture{holidayRegistrationFixture: w.holidayRegistrationFixture, period: w.period, offer: w.offer}
	return d.submit(t, d.registration(email, "Mia",
		map[string]any{"week_id": w.offer.Weeks[0].ID, "time_model_id": w.offer.Weeks[0].TimeModels[0].ID}))
}

// stored reads the period as the Träger-Büro edits it.
func (w weekLimitFixture) stored(t *testing.T) map[string]any {
	t.Helper()
	rec := traegerJSON(t, w.traeger, w.office.token, http.MethodGet, holidayPeriodPath(w.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[map[string]any](t, rec.Body.Bytes())
}

// write saves the stored period with the week ending on endsOn, the bound
// limit and the price of its time model.
func (w weekLimitFixture) write(t *testing.T, endsOn string, limit int, priceCents int) (int, string, []string) {
	t.Helper()
	period := w.stored(t)
	period["max_days_per_child"] = limit
	week := period["weeks"].([]any)[0].(map[string]any)
	week["ends_on"] = endsOn
	week["time_models"].([]any)[0].(map[string]any)["price_cents"] = priceCents
	rec := traegerJSON(t, w.traeger, w.office.token, http.MethodPut, holidayPeriodPath(w.period.ID), period)
	if rec.Code == http.StatusOK {
		return rec.Code, "", nil
	}
	code, fields := registrationRefusalOf(t, rec)
	return rec.Code, code, fields
}

func (w weekLimitFixture) storedEnd(t *testing.T) string {
	t.Helper()
	return w.stored(t)["weeks"].([]any)[0].(map[string]any)["ends_on"].(string)
}

// TestHolidayWeekChangeCannotRaiseABookedChildOverTheDayLimit is the case of
// the issue: a week booked as a whole from Monday to Monday, a bound of one
// day, one registration. Moving the end to Tuesday gives that child two care
// days; the change is refused and leaves the week as it was.
func TestHolidayWeekChangeCannotRaiseABookedChildOverTheDayLimit(t *testing.T) {
	t.Parallel()
	w := newWeekLimitFixture(t, "Träger Woche Grenze", limitMonday, 1)
	registration := w.register(t, "mia.grenze@example.org")

	status, code, fields := w.write(t, limitTuesday, 1, 4500)

	require.Equal(t, http.StatusConflict, status)
	assert.Equal(t, "enrollment.holiday_week_day_limit", code)
	assert.Equal(t, []string{"weeks.0.ends_on"}, fields)
	assert.Equal(t, limitMonday, w.storedEnd(t), "the refused change leaves the week as it was")

	status, _, _ = w.write(t, limitMonday, 1, 5000)
	assert.Equal(t, http.StatusOK, status, "a change of the price keeps the days")
	status, _, _ = w.write(t, limitTuesday, 2, 5000)
	assert.Equal(t, http.StatusOK, status, "a raised bound admits the longer week")
	status, _, _ = w.write(t, limitWednesday, 2, 5000)
	assert.Equal(t, http.StatusConflict, status, "the written bound holds")

	rec := traegerJSON(t, w.traeger, w.office.token, http.MethodPost, holidayCancelPath(w.period.ID, registration.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	status, _, _ = w.write(t, limitWednesday, 2, 5000)
	assert.Equal(t, http.StatusOK, status, "a cancelled registration takes no day")
}

// TestHolidayWeekChangeKeepsBookingsAboveALoweredLimit: a lower bound takes no
// booking away (#4701); the week may keep or lose days, not gain them.
func TestHolidayWeekChangeKeepsBookingsAboveALoweredLimit(t *testing.T) {
	t.Parallel()
	w := newWeekLimitFixture(t, "Träger Woche Bestand", limitWednesday, 3)
	w.register(t, "mia.bestand@example.org")

	status, _, _ := w.write(t, limitWednesday, 1, 4500)
	assert.Equal(t, http.StatusOK, status, "a lower bound keeps the three booked days")
	status, code, _ := w.write(t, limitThursday, 1, 4500)
	assert.Equal(t, http.StatusConflict, status, "a fourth day is refused")
	assert.Equal(t, "enrollment.holiday_week_day_limit", code)
	status, _, _ = w.write(t, limitTuesday, 1, 4500)
	assert.Equal(t, http.StatusOK, status, "a shorter week lowers the days")
	status, _, _ = w.write(t, limitWednesday, 1, 4500)
	assert.Equal(t, http.StatusConflict, status, "the days lost stay lost under the lower bound")
	assert.Equal(t, limitTuesday, w.storedEnd(t))
}

// TestHolidayWeekChangeWaitsForAConcurrentRegistration holds the lock of the
// period as a registration does while it writes its booking of the whole
// week, and meanwhile moves the end of the week. The change waits for the
// lock and then counts the committed booking. Without the lock of the period
// the change reads the bookings in its own snapshot, sees none and takes the
// child over the bound.
func TestHolidayWeekChangeWaitsForAConcurrentRegistration(t *testing.T) {
	t.Parallel()
	w := newWeekLimitFixture(t, "Träger Woche Gleichzeitig", limitMonday, 1)
	week := w.offer.Weeks[0]
	ctx := context.Background()

	holder, err := w.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	committed := false
	defer func() {
		if !committed {
			require.NoError(t, holder.Rollback())
		}
	}()
	var pid int
	require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid))
	_, err = holder.ExecContext(ctx, `SELECT id FROM enrollment.holiday_periods WHERE id = ? FOR UPDATE`, w.period.ID)
	require.NoError(t, err)
	var registration, child int64
	require.NoError(t, holder.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id,
			guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Erika', 'Erste', 'erika.woche@example.org', '0123', NOW()) RETURNING id`,
		w.office.organization, w.period.ID).Scan(ctx, &registration))
	require.NoError(t, holder.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, 'Ella', 'Erste', '2017-01-02') RETURNING id`, w.office.organization, w.period.ID, registration).Scan(ctx, &child))
	_, err = holder.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement)
		VALUES (?, ?, ?, ?, ?, 'placed')`, w.office.organization, w.period.ID, child, week.ID, week.TimeModels[0].ID)
	require.NoError(t, err)

	period := w.stored(t)
	period["weeks"].([]any)[0].(map[string]any)["ends_on"] = limitTuesday
	sent := make(chan int, 1)
	go func() {
		sent <- traegerJSON(t, w.traeger, w.office.token, http.MethodPut, holidayPeriodPath(w.period.ID), period).Code
	}()
	status := 0
	deadline := time.Now().Add(20 * time.Second)
	for status == 0 && time.Now().Before(deadline) {
		var blocked int
		require.NoError(t, w.db.NewRaw(`SELECT count(*) FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid))`, pid).Scan(ctx, &blocked))
		if blocked > 0 {
			break
		}
		select {
		case status = <-sent:
		case <-time.After(20 * time.Millisecond):
		}
	}
	require.Zero(t, status, "the change of the week waits for the registration")
	require.NoError(t, holder.Commit())
	committed = true
	status = <-sent

	assert.Equal(t, http.StatusConflict, status, "the change counts the committed booking")
	assert.Equal(t, limitMonday, w.storedEnd(t))
}
