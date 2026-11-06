package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The contract prints every weekday from Monday to Friday: the modules of
// the day with their end times, or "keine Betreuung" (#4678).
func TestServiceFactsNameEveryWeekdayWithItsModules(t *testing.T) {
	t.Parallel()
	facts := ServiceFacts([]carecontract.Service{
		{Weekday: "mon", CareOfferingID: 3, ModuleName: "Frühbetreuung", EndsAt: "08:00"},
		{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
		{Weekday: "tue", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"},
		{Weekday: "thu", CareOfferingID: 4, ModuleName: "Hausaufgaben"},
	})
	assert.Equal(t, []LetterFact{
		{Label: "Montag", Value: "Frühbetreuung bis 08:00 Uhr und Kurzbetreuung bis 14:00 Uhr"},
		{Label: "Dienstag", Value: "Ganztag bis 16:30 Uhr"},
		{Label: "Mittwoch", Value: "keine Betreuung"},
		{Label: "Donnerstag", Value: "Hausaufgaben"},
		{Label: "Freitag", Value: "keine Betreuung"},
	}, facts)
}

// One contract carries one set of conditions: every module needs its
// conditions, and they must agree.
func TestContractTermsNeedEqualConditionsOfEveryModule(t *testing.T) {
	t.Parallel()
	twelve := 12
	fixed := carecontract.TemplateInput{TermKind: carecontract.TermFixedMonths, TermMonths: &twelve, NoticeMonths: 1,
		NoticeTo: carecontract.NoticeMonthEnd}
	sameMonths := 12
	copied := fixed
	copied.TermMonths = &sameMonths
	index := templateIndex{{7, 1}: fixed, {7, 2}: copied, {7, 3}: {TermKind: carecontract.TermOpenEnded, NoticeTo: carecontract.NoticeMonthEnd}}

	terms, issue := index.terms(7, []int64{1, 2})
	assert.Empty(t, issue, "equal conditions behind two pointers are equal")
	assert.Equal(t, fixed.TermKind, terms.TermKind)
	_, issue = index.terms(7, []int64{1, 3})
	assert.Equal(t, carecontract.SkipTemplatesDiffer, issue)
	_, issue = index.terms(7, []int64{1, 4})
	assert.Equal(t, carecontract.SkipTemplateMissing, issue)
	_, issue = index.terms(8, []int64{1})
	assert.Equal(t, carecontract.SkipTemplateMissing, issue, "conditions belong to the school's offering")
}

// A contract over one module keeps that module's monthly price. Over
// several modules it has none: their whole-month prices would add up to too
// much for a family that books each module on a few weekdays (#4684 prices
// them per weekday and module).
func TestBookedCarePricesOnlyAContractOfOneModule(t *testing.T) {
	t.Parallel()
	short, long := 9000, 16500
	read := schoolRead{offerings: map[int64]carecontract.Offering{
		1: {ID: 1, Name: "Kurzbetreuung", PriceCents: &short, EndTimes: map[string]string{"mon": "14:00", "fri": "14:00"}},
		2: {ID: 2, Name: "OGS Ganztag", PriceCents: &long, EndTimes: map[string]string{"tue": "16:00"}},
	}}
	today := calendar.NewDate(2026, 11, 6)
	one := bookedCare(read, carecontract.Application{Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon", "fri"}},
	}}, today)
	if assert.NotNil(t, one.price) {
		assert.Equal(t, int64(9000), *one.price)
	}
	two := bookedCare(read, carecontract.Application{Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon", "fri"}},
		{CareOfferingID: 2, Days: []string{"tue"}},
	}}, today)
	assert.Nil(t, two.price)
	assert.Equal(t, []carecontract.Service{
		{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
		{Weekday: "tue", CareOfferingID: 2, ModuleName: "OGS Ganztag", EndsAt: "16:00"},
		{Weekday: "fri", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
	}, two.services)
}

func TestFixedMonthTermsEndWithoutOverflowingShortMonths(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		from   string
		months int
		want   string
	}{
		{"2027-01-31", 1, "2027-02-28"},
		{"2028-01-31", 1, "2028-02-29"},
		{"2028-02-29", 12, "2029-02-28"},
		{"2027-03-31", 1, "2027-04-30"},
		{"2027-01-30", 1, "2027-02-28"},
		{"2027-01-28", 1, "2027-02-27"},
		{"2027-08-01", 1, "2027-08-31"},
		{"2027-12-15", 2, "2028-02-14"},
	} {
		t.Run(tc.from, func(t *testing.T) {
			got := endsOn(carecontract.TemplateInput{TermKind: carecontract.TermFixedMonths, TermMonths: &tc.months}, calendar.Date(tc.from), nil)
			if assert.NotNil(t, got) {
				assert.Equal(t, calendar.Date(tc.want), *got)
			}
		})
	}
}

func TestBookedCareResolvesFixedDaysButNotAnEmptyChoice(t *testing.T) {
	t.Parallel()
	read := schoolRead{offerings: map[int64]carecontract.Offering{
		1: {ID: 1, Name: "Feste Betreuung", DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "fri"}, EndTimes: map[string]string{"mon": "14:00", "fri": "13:00"}},
		2: {ID: 2, Name: "Wählbare Betreuung", DaysOfWeekMode: "selectable", AvailableDays: []string{"tue"}},
	}}
	got := bookedCare(read, carecontract.Application{Bookings: []carecontract.Booking{{CareOfferingID: 1}, {CareOfferingID: 2}}}, calendar.NewDate(2026, 11, 6))
	assert.Equal(t, []carecontract.Service{
		{Weekday: "mon", CareOfferingID: 1, ModuleName: "Feste Betreuung", EndsAt: "14:00"},
		{Weekday: "fri", CareOfferingID: 1, ModuleName: "Feste Betreuung", EndsAt: "13:00"},
	}, got.services)
	assert.Equal(t, []int64{1}, got.offerings)
}

func TestBookedCarePreservesExplicitFixedDays(t *testing.T) {
	t.Parallel()
	offering := carecontract.Offering{ID: 1, Name: "Betreuung", DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "tue"}}
	got := bookedCare(schoolRead{offerings: map[int64]carecontract.Offering{1: offering}}, carecontract.Application{Bookings: []carecontract.Booking{{CareOfferingID: 1, Days: []string{"mon"}}}}, calendar.NewDate(2026, 11, 6))
	assert.Equal(t, []carecontract.Service{{Weekday: "mon", CareOfferingID: 1, ModuleName: "Betreuung"}}, got.services)
}

func TestBookedCareBoundsBookingsToTheirOfferingPhase(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 11, 6)
	start, end := calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 7, 31)
	offering := carecontract.Offering{ID: 1, Name: "Betreuung", DaysOfWeekMode: "fixed", AvailableDays: []string{"mon"}, ServiceStart: &start, ServiceEnd: &end}
	read := schoolRead{offerings: map[int64]carecontract.Offering{1: offering}}
	application := carecontract.Application{Bookings: []carecontract.Booking{{CareOfferingID: 1}}}
	got := bookedCare(read, application, today)
	assert.Equal(t, start, got.effectiveFrom(application, today))
	assert.Equal(t, &end, endsOn(carecontract.TemplateInput{TermKind: carecontract.TermSchoolYear}, start, got.until))
	// An open row cannot outlive its phase or re-enter the candidates afterwards.
	assert.Empty(t, bookedCare(read, application, end.AddDays(1)).services)
	// Explicit booking bounds still narrow the phase.
	from, until := start.AddDays(20), end.AddDays(-20)
	application.Bookings[0].ValidFrom, application.Bookings[0].ValidUntil = &from, &until
	got = bookedCare(read, application, today)
	assert.Equal(t, from, got.effectiveFrom(application, today))
	assert.Equal(t, &until, got.until)
	// A booking that starts after the phase has no valid service window.
	after := end.AddDays(1)
	application.Bookings[0].ValidFrom, application.Bookings[0].ValidUntil = &after, nil
	assert.Empty(t, bookedCare(read, application, today).services)
}
