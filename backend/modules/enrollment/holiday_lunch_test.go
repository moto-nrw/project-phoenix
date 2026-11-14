package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// A booking with lunch takes one lunch on each of its care days (#5631):
// Monday to Friday of a whole week, the booked days of a week by days, and
// never a day the host school gives no care.
func TestHolidayLunchCountsOneLunchPerCareDay(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2027, 7, 12)
	price := int64(430)
	week := enrollment.HolidayWeek{StartsOn: monday, EndsOn: monday.AddDays(4), LunchPriceCents: &price}

	count, cents := enrollment.HolidayLunch(week, nil, true)
	assert.Equal(t, 5, count)
	assert.Equal(t, int64(2150), cents, "5 lunches at 4,30 € are 21,50 €")

	count, cents = enrollment.HolidayLunch(week, []calendar.Date{monday, monday.AddDays(2)}, true)
	assert.Equal(t, 2, count, "a booking of days eats on its days")
	assert.Equal(t, int64(860), cents)

	count, cents = enrollment.HolidayLunch(week, nil, false)
	assert.Zero(t, count, "a booking without lunch has none")
	assert.Zero(t, cents)

	withoutOffer := enrollment.HolidayWeek{StartsOn: monday, EndsOn: monday.AddDays(4)}
	count, cents = enrollment.HolidayLunch(withoutOffer, nil, true)
	assert.Zero(t, count, "a week without the offer has no lunch, whatever the booking says")
	assert.Zero(t, cents)

	// The whole week from Saturday to Sunday still has five lunches.
	long := enrollment.HolidayWeek{StartsOn: monday.AddDays(-2), EndsOn: monday.AddDays(6), LunchPriceCents: &price}
	count, _ = enrollment.HolidayLunch(long, nil, true)
	assert.Equal(t, 5, count, "a lunch on weekdays only")
}

func TestHolidayLunchSkipsDaysWithoutCare(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2027, 7, 12)
	price := int64(430)
	week := enrollment.HolidayWeek{
		StartsOn: monday, EndsOn: monday.AddDays(4), LunchPriceCents: &price,
		NoCareDays: []enrollment.HolidayNoCareDay{
			{Date: monday.AddDays(2), Kind: enrollment.HolidayNoCareClosingDay, Name: "Betriebsausflug"},
			{Date: monday.AddDays(4), Kind: enrollment.HolidayNoCareHoliday, Name: "Feiertag"},
		},
	}

	count, cents := enrollment.HolidayLunch(week, nil, true)
	assert.Equal(t, 3, count, "a closing day planned after the booking and a public holiday take their lunch away")
	assert.Equal(t, int64(1290), cents)
	assert.Equal(t, []calendar.Date{monday, monday.AddDays(1), monday.AddDays(3)}, enrollment.HolidayLunchDays(week, nil))

	count, _ = enrollment.HolidayLunch(week, []calendar.Date{monday, monday.AddDays(2)}, true)
	assert.Equal(t, 1, count, "a booked day without care has no lunch")
}

// The hosting school orders lunch per day: every care day of the week is
// named, a child with lunch counts on its booked days or on all of them.
func TestHolidayLunchCountsNameEveryCareDayOfTheWeek(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2027, 7, 12)
	participants := []enrollment.HolidayParticipant{
		{FirstName: "Tom", Lunch: true},
		{FirstName: "Ida", Lunch: true, Days: []calendar.Date{monday, monday.AddDays(2)}},
		{FirstName: "Max", Days: []calendar.Date{monday}},
	}

	counts := enrollment.HolidayLunchCounts(monday, monday.AddDays(4), participants)
	assert.Equal(t, []enrollment.HolidayDayCount{
		{Date: monday, Participants: 2},
		{Date: monday.AddDays(1), Participants: 1},
		{Date: monday.AddDays(2), Participants: 2},
		{Date: monday.AddDays(3), Participants: 1},
		{Date: monday.AddDays(4), Participants: 1},
	}, counts)
	assert.Empty(t, enrollment.HolidayLunchCounts(calendar.Date(""), calendar.Date(""), participants), "a week without days names none")
}
