// Router-level tests of the lunch in the holiday care (#5631, ADR 0115
// addendum): the Träger-Büro offers lunch per week, parents book it per
// child and week, the Elternabrechnung bills it as a position of its own
// outside the rule chain, and the hosting school sees who eats.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

type lunchOfferBody struct {
	Weeks []struct {
		ID              int64  `json:"id"`
		LunchPriceCents *int64 `json:"lunch_price_cents"`
		LunchDays       int    `json:"lunch_days"`
		TimeModels      []struct {
			ID int64 `json:"id"`
		} `json:"time_models"`
	} `json:"weeks"`
}

type lunchLineBody struct {
	Kind         string  `json:"kind"`
	AmountCents  int64   `json:"amount_cents"`
	WeekStartsOn *string `json:"week_starts_on"`
	BookedDays   *int    `json:"booked_days"`
}

type lunchContributionBody struct {
	Children []struct {
		Lines      []lunchLineBody `json:"lines"`
		TotalCents int64           `json:"total_cents"`
	} `json:"children"`
	TotalCents int64 `json:"total_cents"`
}

// lunchFixture is a published period with a week by days (15 € a day) and a
// whole week (45 €), both with lunch at 4,30 €, and a whole week without
// lunch (45 €).
type lunchFixture struct {
	dayFixture
	review reviewFixture
	lunch  lunchOfferBody
}

func lunchWeeks(office traegerOffice, lunchPrice any) []map[string]any {
	model := func(price int) []map[string]any {
		return []map[string]any{{"name": "bis 13 Uhr", "starts_at": "08:00", "ends_at": "13:00", "price_cents": price}}
	}
	return []map[string]any{
		{"starts_on": dayMonday, "ends_on": dayFriday, "host_school_id": office.schools[0], "booking_unit": "day",
			"lunch_price_cents": lunchPrice, "time_models": model(1500)},
		{"starts_on": "2027-01-11", "ends_on": "2027-01-15", "host_school_id": office.schools[0],
			"lunch_price_cents": lunchPrice, "time_models": model(4500)},
		{"starts_on": "2027-01-18", "ends_on": "2027-01-22", "host_school_id": office.schools[0], "time_models": model(4500)},
	}
}

func newLunchFixture(t *testing.T, name string) lunchFixture {
	t.Helper()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, name, identityaccess.TraegerRoleAdministration)
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPost, "/traeger/holiday-periods/", map[string]any{
		"name": "Winterferien 2027", "registration_opens_on": "2020-01-01", "registration_closes_on": "2099-12-31",
		"binding": true, "notes": "", "weeks": lunchWeeks(office, 430),
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	period := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	l := lunchFixture{dayFixture: dayFixture{holidayRegistrationFixture: f, office: office, period: period}}
	l.offer = l.readOffer(t)
	l.review = reviewFixture{holidayRegistrationFixture: f, office: office, period: period}
	rec = publicJSON(t, f.public, http.MethodGet, registrationPath(period.PublicToken), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	l.lunch = decodeSchoolPortalData[lunchOfferBody](t, rec.Body.Bytes())
	return l
}

// booking books week (by its position in the period) with or without lunch;
// days name the days of the week by days.
func (l lunchFixture) booking(week int, lunch bool, days ...string) map[string]any {
	booking := map[string]any{"week_id": l.lunch.Weeks[week].ID, "time_model_id": l.lunch.Weeks[week].TimeModels[0].ID, "lunch": lunch}
	if len(days) > 0 {
		booking["days"] = days
	}
	return booking
}

func (l lunchFixture) quote(t *testing.T, benefits bool, children ...[]map[string]any) (lunchContributionBody, *httptest.ResponseRecorder) {
	t.Helper()
	body := make([]map[string]any, len(children))
	for i, bookings := range children {
		body[i] = map[string]any{"bookings": bookings}
	}
	rec := publicJSON(t, l.traeger, http.MethodPost, holidayQuotePath(l.period.PublicToken), map[string]any{"children": body, "benefits_declared": benefits})
	if rec.Code != http.StatusOK {
		return lunchContributionBody{}, rec
	}
	return decodeSchoolPortalData[lunchContributionBody](t, rec.Body.Bytes()), rec
}

func (l lunchFixture) confirm(t *testing.T, registrationID int64) lunchContributionBody {
	t.Helper()
	rec := traegerJSON(t, l.traeger, l.office.token, http.MethodPost, holidayConfirmPath(l.period.ID, registrationID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		Contribution lunchContributionBody `json:"contribution"`
	}](t, rec.Body.Bytes()).Contribution
}

func lunchKinds(lines []lunchLineBody) []string {
	kinds := make([]string, len(lines))
	for i, line := range lines {
		kinds[i] = line.Kind
	}
	return kinds
}

// A booking with lunch takes one lunch per care day: the booked days of a
// week by days, Monday to Friday of a whole week. The quote, the frozen
// contribution and the invoice name it as a position of its own; a week
// without lunch stays as it was.
func TestHolidayLunchIsBilledAsAPositionOfItsOwn(t *testing.T) {
	t.Parallel()
	l := newLunchFixture(t, "Mittagessen Position")
	require.Len(t, l.lunch.Weeks, 3)
	assert.Equal(t, int64(430), *l.lunch.Weeks[0].LunchPriceCents)
	assert.Equal(t, 5, l.lunch.Weeks[1].LunchDays, "a whole week has a lunch on each of its five care days")
	assert.Nil(t, l.lunch.Weeks[2].LunchPriceCents, "a week without lunch offers none")

	bookings := []map[string]any{l.booking(0, true, dayMonday, dayWednesday), l.booking(1, true), l.booking(2, false)}
	quoted, rec := l.quote(t, false, bookings)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int64(3000+4500+4500+860+2150), quoted.TotalCents, "two and five lunches at 4,30 € beside the care")

	_, rec = l.quote(t, false, []map[string]any{l.booking(2, true)})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a week without lunch quotes no lunch: %s", rec.Body.String())
	refused := l.registration("ohne.essen@example.org", "Ole", l.booking(2, true))
	rec = publicJSON(t, l.public, http.MethodPost, registrationPath(l.period.PublicToken), refused)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a week without lunch books no lunch: %s", rec.Body.String())

	receipt := l.submit(t, l.registration("mit.essen@example.org", "Tom", bookings...))
	frozen := l.confirm(t, receipt.ID)
	require.Len(t, frozen.Children, 1)
	lines := frozen.Children[0].Lines
	assert.Equal(t, []string{"base", "base", "base", "meal", "meal"}, lunchKinds(lines))
	assert.Equal(t, []int64{860, 2150}, []int64{lines[3].AmountCents, lines[4].AmountCents})
	assert.Equal(t, []int{2, 5}, []int{*lines[3].BookedDays, *lines[4].BookedDays})
	assert.Equal(t, dayMonday, *lines[3].WeekStartsOn)
	assert.Equal(t, int64(15010), frozen.TotalCents)

	billing := traegerColleague(t, l.review, identityaccess.TraegerRoleBilling)
	setNumberRange(t, l.review, billing, "FE-{NNNN}")
	run := billingRun(t, l.review, billing, startBillingRun(t, l.review, billing, l.period.ID).ID)
	assert.Equal(t, int64(15010), run.PreviewCents)
	rec = traegerJSON(t, l.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var issued []struct {
		Kind        string `bun:"kind"`
		AmountCents int64  `bun:"amount_cents"`
		BookedDays  *int   `bun:"booked_days"`
	}
	require.NoError(t, l.db.NewRaw(`SELECT position.kind, position.amount_cents, position.booked_days
		FROM parent_billing.document_positions AS position
		JOIN parent_billing.documents AS document ON document.id = position.document_id
		WHERE document.holiday_registration_id = ? ORDER BY position.position`, receipt.ID).Scan(context.Background(), &issued))
	require.Len(t, issued, 5)
	assert.Equal(t, "meal", issued[4].Kind)
	assert.Equal(t, int64(2150), issued[4].AmountCents)
	assert.Equal(t, 5, *issued[4].BookedDays, "the invoice names the number of lunches")
	assert.Nil(t, issued[0].BookedDays)
}

// No rule of the chain reaches the lunch: the sibling discount and the
// exemption lower the care, the family pays the lunch in full.
func TestHolidayLunchStaysOutsideSiblingDiscountAndExemption(t *testing.T) {
	t.Parallel()
	l := newLunchFixture(t, "Mittagessen Regeln")
	billing := traegerColleague(t, l.review, identityaccess.TraegerRoleBilling)
	createContributionRule(t, l.review, billing, siblingHalf())
	createContributionRule(t, l.review, billing, benefitsExemption())

	week := []map[string]any{l.booking(1, true)}
	siblings, rec := l.quote(t, false, week, week)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int64(4500+2150), siblings.Children[0].TotalCents)
	assert.Equal(t, int64(2250+2150), siblings.Children[1].TotalCents, "half the care, the whole lunch")
	assert.Equal(t, []string{"base", "sibling", "meal"}, lunchKinds(siblings.Children[1].Lines))

	exempt, rec := l.quote(t, true, week)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int64(2150), exempt.TotalCents, "the exemption frees the care, not the lunch")
	assert.Equal(t, []string{"base", "benefits_exemption", "meal"}, lunchKinds(exempt.Children[0].Lines))
}

// A booked lunch keeps its offer: the Träger-Büro may change the price, not
// take the offer away. A confirmed contribution keeps the price it froze.
func TestHolidayLunchOfferStaysWhileBookedAndAFrozenPriceHolds(t *testing.T) {
	t.Parallel()
	l := newLunchFixture(t, "Mittagessen Angebot")
	receipt := l.submit(t, l.registration("angebot.essen@example.org", "Tom", l.booking(1, true)))
	assert.Equal(t, int64(4500+2150), l.confirm(t, receipt.ID).TotalCents)

	update := func(lunchPrice any) *httptest.ResponseRecorder {
		weeks := lunchWeeks(l.office, lunchPrice)
		for i := range weeks {
			weeks[i]["id"] = l.lunch.Weeks[i].ID
			weeks[i]["time_models"].([]map[string]any)[0]["id"] = l.lunch.Weeks[i].TimeModels[0].ID
		}
		return traegerJSON(t, l.traeger, l.office.token, http.MethodPut, holidayPeriodPath(l.period.ID), map[string]any{
			"name": "Winterferien 2027", "registration_opens_on": "2020-01-01", "registration_closes_on": "2099-12-31",
			"binding": true, "notes": "", "weeks": weeks,
		})
	}
	rec := update(nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "enrollment.holiday_lunch_booked")

	rec = update(500)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	quoted, rec := l.quote(t, false, []map[string]any{l.booking(1, true)})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int64(4500+2500), quoted.TotalCents, "a new registration pays the new price")
	rec = traegerJSON(t, l.traeger, l.office.token, http.MethodGet, holidayContributionPath(l.period.ID, receipt.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int64(4500+2150), decodeSchoolPortalData[lunchContributionBody](t, rec.Body.Bytes()).TotalCents,
		"the confirmed contribution keeps the price of its confirmation")
}

// The hosting school sees which confirmed child eats lunch and how many
// lunches each care day needs, never a price. The Träger-Büro can take the
// lunch off a booking again.
func TestHolidayLunchShowsOnTheParticipantListOfTheHostingSchool(t *testing.T) {
	t.Parallel()
	l := newLunchFixture(t, "Mittagessen Teilnehmerliste")
	tom := l.submit(t, l.registration("tom.essen@example.org", "Tom", l.booking(0, true, dayMonday, dayWednesday)))
	ida := l.submit(t, l.registration("ida.essen@example.org", "Ida", l.booking(0, false, dayMonday)))
	max := l.submit(t, l.registration("max.essen@example.org", "Max", l.booking(0, true, dayMonday, dayFriday)))

	// Before the confirmation the Büro takes Max's lunch off again.
	rec := traegerJSON(t, l.traeger, l.office.token, http.MethodPut, holidayRebookPath(l.period.ID, max.ID, l.childOf(t, max.ID)),
		map[string]any{"bookings": []map[string]any{l.booking(0, false, dayMonday, dayFriday)}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, id := range []int64{tom.ID, ida.ID, max.ID} {
		l.confirm(t, id)
	}

	today := &atomic.Value{}
	today.Store(calendar.NewDate(2027, 1, 1))
	host := holidayHostFixture{holidayRegistrationFixture: l.holidayRegistrationFixture}
	router := newHolidayHostRouter(t, l.holidayRegistrationFixture, today)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/holiday-weeks/%d/participants", l.lunch.Weeks[0].ID), nil)
	req.Header.Set("Authorization", "Bearer "+host.staffOf(t, l.office.schools[0], permissions.ConfigRead))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "price", "the hosting school sees no price")
	list := decodeSchoolPortalData[struct {
		Week struct {
			LunchOffered bool `json:"lunch_offered"`
			Lunches      []struct {
				Date         string `json:"date"`
				Participants int    `json:"participants"`
			} `json:"lunches"`
		} `json:"week"`
		Participants []struct {
			FirstName string `json:"first_name"`
			Lunch     bool   `json:"lunch"`
		} `json:"participants"`
	}](t, rec.Body.Bytes())
	assert.True(t, list.Week.LunchOffered)
	eats := map[string]bool{}
	for _, participant := range list.Participants {
		eats[participant.FirstName] = participant.Lunch
	}
	assert.Equal(t, map[string]bool{"Tom": true, "Ida": false, "Max": false}, eats)
	lunches := map[string]int{}
	for _, day := range list.Week.Lunches {
		lunches[day.Date] = day.Participants
	}
	assert.Equal(t, map[string]int{dayMonday: 1, dayTuesday: 0, dayWednesday: 1, dayThursday: 0, dayFriday: 0}, lunches,
		"only Tom eats, on his two days; every care day of the week is named")
}
