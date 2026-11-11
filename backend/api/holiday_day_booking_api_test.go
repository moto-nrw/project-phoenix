// Router-level tests of the booking of days and the bound of care days per
// child in the holidays (#4701, decision E-f A): a week offered by days, the
// capacity on each day, the bound of the period, the contribution per day
// and the host school's care windows on the booked days only.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Monday 4 to Friday 8 January 2027, the week by days of the fixture.
const (
	dayMonday    = "2027-01-04"
	dayTuesday   = "2027-01-05"
	dayWednesday = "2027-01-06"
	dayThursday  = "2027-01-07"
	dayFriday    = "2027-01-08"
)

type dayOfferBody struct {
	MaxDaysPerChild *int `json:"max_days_per_child"`
	Weeks           []struct {
		ID          int64    `json:"id"`
		BookingUnit string   `json:"booking_unit"`
		CareDays    []string `json:"care_days"`
		FullDays    []string `json:"full_days"`
		TimeModels  []struct {
			ID         int64 `json:"id"`
			PriceCents int64 `json:"price_cents"`
		} `json:"time_models"`
	} `json:"weeks"`
	Schools []struct {
		ID int64 `json:"id"`
	} `json:"schools"`
}

type dayPlacesBody struct {
	WeekID      int64  `json:"week_id"`
	BookingUnit string `json:"booking_unit"`
	Days        []struct {
		Date   string `json:"date"`
		Placed int    `json:"placed"`
	} `json:"days"`
	Waitlist []struct {
		Days []string `json:"days"`
	} `json:"waitlist"`
}

// dayFixture is a published period of a Träger with a week by days (Monday
// to Friday, 15 € a day, one place a day) and a whole week (45 €), and a
// bound of three care days per child.
type dayFixture struct {
	holidayRegistrationFixture
	office traegerOffice
	period traegerPeriodLink
	offer  dayOfferBody
}

func newDayFixture(t *testing.T, name string) dayFixture {
	t.Helper()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, name, identityaccess.TraegerRoleAdministration)
	body := map[string]any{
		"name": "Winterferien 2027", "registration_opens_on": "2020-01-01", "registration_closes_on": "2099-12-31",
		"binding": true, "notes": "", "max_days_per_child": 3,
		"weeks": []map[string]any{
			{"starts_on": dayMonday, "ends_on": dayFriday, "host_school_id": office.schools[0], "booking_unit": "day",
				"time_models": []map[string]any{{"name": "bis 13 Uhr", "starts_at": "08:00", "ends_at": "13:00", "price_cents": 1500}}},
			{"starts_on": "2027-01-11", "ends_on": "2027-01-15", "host_school_id": office.schools[1],
				"time_models": []map[string]any{{"name": "bis 13 Uhr", "starts_at": "08:00", "ends_at": "13:00", "price_cents": 4500}}},
		},
	}
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPost, "/traeger/holiday-periods/", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	period := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	d := dayFixture{holidayRegistrationFixture: f, office: office, period: period}
	d.offer = d.readOffer(t)
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayCapacityPath(period.ID, d.offer.Weeks[0].ID), map[string]any{"capacity": 1})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return d
}

func (d dayFixture) readOffer(t *testing.T) dayOfferBody {
	t.Helper()
	rec := publicJSON(t, d.public, http.MethodGet, registrationPath(d.period.PublicToken), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[dayOfferBody](t, rec.Body.Bytes())
}

// registration is a registration of one child with the given bookings.
func (d dayFixture) registration(email, name string, bookings ...map[string]any) map[string]any {
	return map[string]any{
		"guardian": map[string]any{"first_name": "Eva", "last_name": "Tag", "email": email, "phone": "0151 2345678"},
		"children": []map[string]any{{
			"first_name": name, "last_name": "Tag", "birth_date": "2017-05-04", "home_school_id": d.offer.Schools[0].ID, "bookings": bookings,
		}},
		"emergency_contacts": []map[string]any{{"name": "Oma Tag", "phone": "05402 1234", "relationship": "Großmutter"}},
		"privacy_consent":    true, "binding_confirmed": true, "custody_confirmed": true, "declaration_name": "Eva Tag",
		"consent_version": enrollment.HolidayConsentVersionCurrent, "retention_days": holidayDefaultRetentionDays, "captcha_token": "valid-token",
	}
}

func (d dayFixture) days(days ...string) map[string]any {
	return map[string]any{"week_id": d.offer.Weeks[0].ID, "time_model_id": d.offer.Weeks[0].TimeModels[0].ID, "days": days}
}

func (d dayFixture) wholeWeek() map[string]any {
	return map[string]any{"week_id": d.offer.Weeks[1].ID, "time_model_id": d.offer.Weeks[1].TimeModels[0].ID}
}

func (d dayFixture) submit(t *testing.T, body map[string]any) receiptBody {
	t.Helper()
	rec := publicJSON(t, d.public, http.MethodPost, registrationPath(d.period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[receiptBody](t, rec.Body.Bytes())
}

func (d dayFixture) places(t *testing.T) dayPlacesBody {
	t.Helper()
	rec := traegerJSON(t, d.traeger, d.office.token, http.MethodGet, holidayPlacesPath(d.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, week := range decodeSchoolPortalData[[]dayPlacesBody](t, rec.Body.Bytes()) {
		if week.WeekID == d.offer.Weeks[0].ID {
			return week
		}
	}
	t.Fatal("the week by days is missing from the places")
	return dayPlacesBody{}
}

func (d dayFixture) placedOn(t *testing.T) map[string]int {
	t.Helper()
	placed := map[string]int{}
	for _, day := range d.places(t).Days {
		placed[day.Date] = day.Placed
	}
	return placed
}

// TestHolidayDayBookingPlacesEachDayAndBillsItsDays pins the booking of days
// end to end. Each guard goes red without its protection: the capacity on
// each day (a second child on Tuesday), the booking that waits as a whole
// and lets a later one of other days pass, the full days in the offer, the
// days the contribution bills at the price of one day, the waiting booking
// that costs nothing, and the host school's care windows on the booked days
// only (migration 1.15.885).
func TestHolidayDayBookingPlacesEachDayAndBillsItsDays(t *testing.T) {
	t.Parallel()
	d := newDayFixture(t, "Träger Tage Platz")
	week := d.offer.Weeks[0]
	require.Equal(t, "day", week.BookingUnit)
	assert.Equal(t, []string{dayMonday, dayTuesday, dayWednesday, dayThursday, dayFriday}, week.CareDays)
	require.NotNil(t, d.offer.MaxDaysPerChild)
	assert.Equal(t, 3, *d.offer.MaxDaysPerChild)
	assert.Empty(t, week.FullDays)

	tom := d.submit(t, d.registration("tom.tag@example.org", "Tom", d.days(dayTuesday, dayMonday, dayMonday)))
	assert.Equal(t, "placed", tom.Placements[0].Placement)
	ida := d.submit(t, d.registration("ida.tag@example.org", "Ida", d.days(dayTuesday, dayWednesday)))
	assert.Equal(t, "waitlisted", ida.Placements[0].Placement, "Tuesday has no free place, so all of Ida's booking waits")
	max := d.submit(t, d.registration("max.tag@example.org", "Max", d.days(dayWednesday)))
	assert.Equal(t, "placed", max.Placements[0].Placement, "a later booking of a free day passes the waiting one")

	assert.Equal(t, map[string]int{dayMonday: 1, dayTuesday: 1, dayWednesday: 1, dayThursday: 0, dayFriday: 0}, d.placedOn(t),
		"no day holds more than its one place")
	places := d.places(t)
	require.Len(t, places.Waitlist, 1)
	assert.Equal(t, []string{dayTuesday, dayWednesday}, places.Waitlist[0].Days)
	assert.Equal(t, []string{dayMonday, dayTuesday, dayWednesday}, d.readOffer(t).Weeks[0].FullDays)

	rec := traegerJSON(t, d.traeger, d.office.token, http.MethodPost, holidayConfirmPath(d.period.ID, tom.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	confirmed := decodeSchoolPortalData[confirmationBody](t, rec.Body.Bytes())
	assert.Equal(t, int64(3000), confirmed.Contribution.TotalCents, "two days at 15 €; the repeated Monday counts once")
	rec = traegerJSON(t, d.traeger, d.office.token, http.MethodPost, holidayConfirmPath(d.period.ID, ida.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int64(0), decodeSchoolPortalData[confirmationBody](t, rec.Body.Bytes()).Contribution.TotalCents,
		"a waiting booking is not billed")

	var lineDays []int
	require.NoError(t, d.db.NewRaw(`SELECT line.booked_days FROM parent_billing.holiday_contribution_lines AS line
		JOIN parent_billing.holiday_contributions AS contribution ON contribution.id = line.holiday_contribution_id
		WHERE contribution.holiday_registration_id = ? AND line.kind = 'base'`, tom.ID).Scan(context.Background(), &lineDays))
	assert.Equal(t, []int{2}, lineDays, "the base line names its days")

	assert.Equal(t, []string{dayMonday + ".." + dayMonday, dayTuesday + ".." + dayTuesday}, d.hostWindows(t, d.office.schools[0]),
		"the host school's Soll keeps to the booked days")
}

// hostWindows reads the booked care windows the host school sees for the
// week by days.
func (d dayFixture) hostWindows(t *testing.T, school int64) []string {
	t.Helper()
	ctx := context.Background()
	var windows []string
	require.NoError(t, d.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(school)); err != nil {
			return err
		}
		return tx.NewRaw(`SELECT starts_on::text || '..' || ends_on::text FROM enrollment.holiday_host_booked_windows(?, ?)
			WHERE week_id = ? ORDER BY starts_on`, dayMonday, dayFriday, d.offer.Weeks[0].ID).Scan(ctx, &windows)
	}))
	return windows
}

// TestHolidayChildDayLimitRefusesMoreDaysThanThePeriodAllows pins the bound
// of care days per child: a whole week counts its five care days, days are
// counted over all weeks of the period, and a rebooking past the bound is
// refused while one that keeps the days is not. Days outside the week are
// refused on their own. Each refusal goes red without its check.
func TestHolidayChildDayLimitRefusesMoreDaysThanThePeriodAllows(t *testing.T) {
	t.Parallel()
	d := newDayFixture(t, "Träger Tage Grenze")

	rec := publicJSON(t, d.public, http.MethodPost, registrationPath(d.period.PublicToken),
		d.registration("ole.grenze@example.org", "Ole", d.wholeWeek()))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var refusal struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal), rec.Body.String())
	assert.Equal(t, "enrollment.holiday_child_day_limit_public", refusal.Code)
	assert.Equal(t, map[string]any{"limit": float64(3), "child": float64(1)}, refusal.Details)

	rec = publicJSON(t, d.public, http.MethodPost, registrationPath(d.period.PublicToken),
		d.registration("ole.samstag@example.org", "Ole", d.days("2027-01-09")))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_invalid", code)
	assert.Contains(t, fields, "children.0.bookings.0.days")

	ole := d.submit(t, d.registration("ole.tage@example.org", "Ole", d.days(dayThursday, dayFriday)))
	child := d.childOf(t, ole.ID)
	rebook := func(bookings ...map[string]any) int {
		rec := traegerJSON(t, d.traeger, d.office.token, http.MethodPut, holidayRebookPath(d.period.ID, ole.ID, child),
			map[string]any{"bookings": bookings})
		return rec.Code
	}
	assert.Equal(t, http.StatusBadRequest, rebook(d.days(dayThursday, dayFriday), d.wholeWeek()), "seven days over all weeks")
	assert.Equal(t, http.StatusOK, rebook(d.days(dayWednesday, dayThursday, dayFriday)), "three days are allowed")

	// A lower bound keeps the bookings; a rebooking may keep the days.
	_, err := d.db.ExecContext(context.Background(), `UPDATE enrollment.holiday_periods SET max_days_per_child = 1 WHERE id = ?`, d.period.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rebook(d.days(dayWednesday, dayThursday)), "fewer days than before stay possible")
	assert.Equal(t, http.StatusBadRequest, rebook(d.days(dayMonday, dayWednesday, dayThursday)), "more days than before are refused")
}

func (d dayFixture) childOf(t *testing.T, registrationID int64) int64 {
	t.Helper()
	var child int64
	require.NoError(t, d.db.NewRaw(`SELECT id FROM enrollment.holiday_registration_children WHERE holiday_registration_id = ?`,
		registrationID).Scan(context.Background(), &child))
	return child
}

// TestHolidayDayBookingHoldsTheLastPlaceOfADayAgainstAConcurrentRegistration
// runs two transactions at once for the last place on Tuesday. The first
// takes it with Monday and Tuesday while it holds the week's row; the
// registration of Tuesday and Wednesday sent meanwhile waits for that lock
// and then finds Tuesday full. Without SELECT ... FOR UPDATE on the week the
// second counts the day in its own snapshot, sees it free and overbooks it.
func TestHolidayDayBookingHoldsTheLastPlaceOfADayAgainstAConcurrentRegistration(t *testing.T) {
	t.Parallel()
	d := newDayFixture(t, "Träger Tage Gleichzeitig")
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
	_, err = holder.ExecContext(ctx, `SELECT id FROM enrollment.holiday_period_weeks WHERE id = ? FOR UPDATE`, week.ID)
	require.NoError(t, err)
	var registration, child int64
	require.NoError(t, holder.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id,
			guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Erika', 'Erste', 'erika.tage@example.org', '0123', NOW()) RETURNING id`,
		d.office.organization, d.period.ID).Scan(ctx, &registration))
	require.NoError(t, holder.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, 'Ella', 'Erste', '2017-01-02') RETURNING id`, d.office.organization, d.period.ID, registration).Scan(ctx, &child))
	_, err = holder.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement, days)
		VALUES (?, ?, ?, ?, ?, 'placed', ?::date[])`,
		d.office.organization, d.period.ID, child, week.ID, week.TimeModels[0].ID, "{"+dayMonday+","+dayTuesday+"}")
	require.NoError(t, err)

	sent := make(chan receiptBody, 1)
	go func() {
		rec := publicJSON(t, d.public, http.MethodPost, registrationPath(d.period.PublicToken),
			d.registration("tom.gleichzeitig.tage@example.org", "Tom", d.days(dayTuesday, dayWednesday)))
		if rec.Code != http.StatusCreated {
			t.Errorf("the second registration was refused: %d %s", rec.Code, rec.Body.String())
		}
		sent <- decodeSchoolPortalData[receiptBody](t, rec.Body.Bytes())
	}()
	var receipt *receiptBody
	deadline := time.Now().Add(20 * time.Second)
	for receipt == nil && time.Now().Before(deadline) {
		var blocked int
		require.NoError(t, d.db.NewRaw(`SELECT count(*) FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid))`, pid).Scan(ctx, &blocked))
		if blocked > 0 {
			break
		}
		select {
		case done := <-sent:
			receipt = &done
		case <-time.After(20 * time.Millisecond):
		}
	}
	require.NoError(t, holder.Commit())
	committed = true
	if receipt == nil {
		done := <-sent
		receipt = &done
	}

	require.Len(t, receipt.Placements, 1)
	assert.Equal(t, "waitlisted", receipt.Placements[0].Placement, "the second registration finds Tuesday taken")
	assert.Equal(t, 1, d.placedOn(t)[dayTuesday], "Tuesday is not overbooked")
}

// Rules starting midweek must apply to both quotes and stored registrations
// whose first requested day is later in that week (ADR 0115).
func TestHolidayDailyBillingRulesStartOnTheFirstSelectedDay(t *testing.T) {
	t.Parallel()
	d := newDayFixture(t, "Träger Tage Regelbeginn")
	f := reviewFixture{holidayRegistrationFixture: d.holidayRegistrationFixture, office: d.office}
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	for _, rule := range []map[string]any{siblingHalf(), benefitsExemption()} {
		rule["valid_from"] = dayWednesday
		createContributionRule(t, f, billing, rule)
	}
	quote := func(exempt bool) contributionBody {
		rec := publicJSON(t, d.traeger, http.MethodPost, holidayQuotePath(d.period.PublicToken), map[string]any{
			"children": []map[string]any{{"bookings": []map[string]any{d.days(dayFriday, dayThursday)}},
				{"bookings": []map[string]any{d.days(dayThursday)}}}, "benefits_declared": exempt,
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes())
	}
	quoted := quote(false)
	assert.Equal(t, int64(3750), quoted.TotalCents, "3000 plus 1500 at half price")
	assert.True(t, quoted.ExemptionAvailable)
	assert.Equal(t, int64(0), quote(true).TotalCents)
	receipt := d.submit(t, d.registration("rules.days@example.org", "Mia", d.days(dayFriday, dayThursday)))
	rec := traegerJSON(t, d.traeger, d.office.token, http.MethodPost, holidayConfirmPath(d.period.ID, receipt.ID),
		map[string]any{"benefits_exemption": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	confirmed := decodeSchoolPortalData[confirmationBody](t, rec.Body.Bytes()).Contribution
	assert.True(t, confirmed.BenefitsExemption)
	assert.Zero(t, confirmed.TotalCents)
	require.Len(t, confirmed.Children, 1)
	assert.Equal(t, dayMonday, *confirmed.Children[0].Lines[0].WeekStartsOn, "the invoice keeps the offered week range")
}

// The period lock must precede validation: a submission waiting for an
// office change must validate the newly committed booking unit.
func TestHolidaySubmissionValidatesAfterAConcurrentBookingUnitChange(t *testing.T) {
	t.Parallel()
	d := newDayFixture(t, "Träger Buchungsart Gleichzeitig")
	ctx := context.Background()
	week := d.offer.Weeks[0]
	_, err := d.db.ExecContext(ctx, `UPDATE enrollment.holiday_period_weeks SET booking_unit = 'week'
  WHERE organization_id = ? AND id = ?`, d.office.organization, week.ID)
	require.NoError(t, err)
	_, err = d.db.ExecContext(ctx, `UPDATE enrollment.holiday_periods SET max_days_per_child = NULL
  WHERE organization_id = ? AND id = ?`, d.office.organization, d.period.ID)
	require.NoError(t, err)
	holder, err := d.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	var pid int
	require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid))
	_, err = holder.ExecContext(ctx, `SELECT id FROM enrollment.holiday_periods WHERE organization_id = ? AND id = ? FOR UPDATE`,
		d.office.organization, d.period.ID)
	require.NoError(t, err)
	_, err = holder.ExecContext(ctx, `UPDATE enrollment.holiday_period_weeks SET booking_unit = 'day'
  WHERE organization_id = ? AND id = ?`, d.office.organization, week.ID)
	require.NoError(t, err)
	sent := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		sent <- publicJSON(t, d.public, http.MethodPost, registrationPath(d.period.PublicToken),
			d.registration("unit.change@example.org", "Lea", map[string]any{"week_id": week.ID, "time_model_id": week.TimeModels[0].ID}))
	}()
	require.Eventually(t, func() bool {
		var blocked int
		err := d.db.NewRaw(`SELECT count(*) FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid))`, pid).Scan(ctx, &blocked)
		return err == nil && blocked > 0
	}, 20*time.Second, 20*time.Millisecond, "the submission waits for the office transaction")
	require.NoError(t, holder.Commit())
	select {
	case rec := <-sent:
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	case <-time.After(20 * time.Second):
		t.Fatal("the submission did not finish after the office committed")
	}
	var count int
	require.NoError(t, d.db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registrations WHERE organization_id = ? AND holiday_period_id = ?`,
		d.office.organization, d.period.ID).Scan(ctx, &count))
	assert.Zero(t, count, "a daily booking without selected days is never stored")
}
