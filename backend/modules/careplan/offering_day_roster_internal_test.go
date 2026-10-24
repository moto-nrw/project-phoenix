package careplan

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// rosterMonday is a Monday; the roster cases ask about it.
const rosterMonday calendar.Date = "2026-08-24"

func rosterOffering() *CareOffering {
	return &CareOffering{
		ID: 7, Name: "bis 16 Uhr", IsActive: true, CountsAsCare: true,
		DaysOfWeekMode: "parent_choice", AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"},
		PickupTimes: map[string]string{"mon": "16:00", "tue": "16:00"},
	}
}

func rosterBooking(studentID, offeringID int64, days []string, from, until *calendar.Date) *ApprovedBooking {
	return &ApprovedBooking{StudentID: studentID, Link: &BookingSelection{
		CareOfferingID: offeringID, SelectedDays: days, ValidFrom: from, ValidUntil: until,
	}}
}

func rosterClock(hhmm string) *time.Time {
	parsed, err := time.Parse("15:04", hhmm)
	if err != nil {
		panic(err)
	}
	return &parsed
}

func TestChildrenOnOfferingDayKeepsBookedChildrenOfTheWeekday(t *testing.T) {
	t.Parallel()
	start := calendar.Date("2026-08-01")
	today := rosterMonday
	tomorrow := rosterMonday.AddDays(1)
	facts := offeringDayFacts{bookings: []*ApprovedBooking{
		rosterBooking(1, 7, []string{"mon", "tue"}, &start, nil),
		rosterBooking(2, 7, []string{"tue"}, &start, nil),       // Monday not booked
		rosterBooking(3, 7, []string{"mon"}, &start, &today),    // valid_until is exclusive
		rosterBooking(4, 7, []string{"mon"}, &tomorrow, nil),    // starts tomorrow
		rosterBooking(5, 8, []string{"mon"}, &start, nil),       // another offering
		rosterBooking(6, 7, []string{"mon"}, &start, &tomorrow), // last valid day
		rosterBooking(9, 7, []string{"mon"}, nil, nil),          // not asked about
	}}

	got := childrenOnOfferingDay(rosterOffering(), []int64{6, 5, 4, 3, 2, 1}, rosterMonday, facts)

	assert.Equal(t, []int64{6, 1}, got)
}

func TestChildrenOnOfferingDayLeavesOutChildrenSignedOffForTheDay(t *testing.T) {
	t.Parallel()
	ids := []int64{1, 2, 3, 4, 5, 6, 7}
	facts := offeringDayFacts{
		pickupExceptions: []PickupException{
			{StudentID: 1, ExceptionDate: Date(rosterMonday), PickupTime: rosterClock("14:30")}, // earlier
			{StudentID: 2, ExceptionDate: Date(rosterMonday), PickupTime: rosterClock("17:00")}, // later
			{StudentID: 3, ExceptionDate: Date(rosterMonday)},                                   // Kommt heute nicht
			{StudentID: 7, ExceptionDate: Date(rosterMonday), PickupTime: rosterClock("16:00")}, // same time
		},
		arrivalExceptions: []ArrivalException{
			{StudentID: 4, ExceptionDate: Date(rosterMonday)},                                        // Kommt heute nicht
			{StudentID: 6, ExceptionDate: Date(rosterMonday), ExpectedArrival: rosterClock("13:00")}, // later arrival only
		},
		statusDays: []StudentStatusDay{{StudentID: 5, Date: Date(rosterMonday), Status: "sick"}},
	}
	for _, id := range ids {
		facts.bookings = append(facts.bookings, rosterBooking(id, 7, []string{"mon"}, nil, nil))
	}

	got := childrenOnOfferingDay(rosterOffering(), ids, rosterMonday, facts)

	assert.Equal(t, []int64{2, 6, 7}, got)
}

func TestChildrenOnOfferingDayWithoutOfferingTimeSignsOffOnlyTimelessExceptions(t *testing.T) {
	t.Parallel()
	offering := rosterOffering()
	offering.PickupTimes = nil
	facts := offeringDayFacts{
		bookings: []*ApprovedBooking{
			rosterBooking(1, 7, []string{"mon"}, nil, nil),
			rosterBooking(2, 7, []string{"mon"}, nil, nil),
		},
		pickupExceptions: []PickupException{
			{StudentID: 1, PickupTime: rosterClock("12:00")},
			{StudentID: 2},
		},
	}

	assert.Equal(t, []int64{1}, childrenOnOfferingDay(offering, []int64{1, 2}, rosterMonday, facts))
}

func TestChildrenOnOfferingDayFixedOfferingUsesItsAvailableDays(t *testing.T) {
	t.Parallel()
	offering := rosterOffering()
	offering.DaysOfWeekMode = "fixed"
	offering.AvailableDays = []string{"mon"}
	facts := offeringDayFacts{bookings: []*ApprovedBooking{rosterBooking(1, 7, nil, nil, nil)}}

	assert.Equal(t, []int64{1}, childrenOnOfferingDay(offering, []int64{1}, rosterMonday, facts))
	assert.Empty(t, childrenOnOfferingDay(offering, []int64{1}, rosterMonday.AddDays(1), facts))
}

func TestChildrenOnOfferingDayInactiveOfferingHoldsNoChild(t *testing.T) {
	t.Parallel()
	facts := offeringDayFacts{bookings: []*ApprovedBooking{rosterBooking(1, 7, []string{"mon"}, nil, nil)}}
	inactive := rosterOffering()
	inactive.IsActive = false
	notCare := rosterOffering()
	notCare.CountsAsCare = false

	assert.Empty(t, childrenOnOfferingDay(nil, []int64{1}, rosterMonday, facts))
	assert.Empty(t, childrenOnOfferingDay(inactive, []int64{1}, rosterMonday, facts))
	assert.Empty(t, childrenOnOfferingDay(notCare, []int64{1}, rosterMonday, facts))
}
