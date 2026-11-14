package enrollment

import (
	"errors"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Lunch in the holiday care (#5631). The Träger-Büro offers lunch per week
// with the price of one lunch; parents book it per child and week. A booking
// with lunch takes one lunch on each of its care days: the booked days of a
// week by days, Monday to Friday of a week booked as a whole. A whole week
// with a day without care takes no new booking (#4702), and a week by days
// does not offer such a day (#6447), so the count holds no public holiday
// and no closing day known at the booking. The Elternabrechnung bills the
// lunch as a position of its own (ADR 0115 addendum).

// ErrHolidayLunchBooked reports a week whose lunch offer the Träger-Büro
// removes while a booking of the week has lunch.
var ErrHolidayLunchBooked = errors.New("a booking of the week has lunch")

// HolidayLunch is the lunch of one booking: the number of lunches and their
// price at the week's price now. A booking without lunch, or in a week
// without the offer, has neither.
func HolidayLunch(week HolidayWeek, days []calendar.Date, lunch bool) (count int, cents int64) {
	if !lunch || week.LunchPriceCents == nil {
		return 0, 0
	}
	count = len(HolidayBookedDays(week, days))
	return count, *week.LunchPriceCents * int64(count)
}

// HolidayLunchCounts counts the lunches of the participants of a hosted
// week per care day, every care day of the week in calendar order: a child
// with lunch eats on its booked days, or on every care day of a week it
// booked as a whole.
func HolidayLunchCounts(startsOn, endsOn calendar.Date, participants []HolidayParticipant) []HolidayDayCount {
	careDays := HolidayWeekCareDays(startsOn, endsOn)
	counts := make([]HolidayDayCount, len(careDays))
	index := make(map[calendar.Date]int, len(careDays))
	for i, day := range careDays {
		counts[i].Date, index[day] = day, i
	}
	for _, participant := range participants {
		if !participant.Lunch {
			continue
		}
		days := participant.Days
		if len(days) == 0 {
			days = careDays
		}
		for _, day := range days {
			if i, ok := index[day]; ok {
				counts[i].Participants++
			}
		}
	}
	return counts
}
