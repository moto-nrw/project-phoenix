package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// amendmentRead is a school with two modules: care until 14:00 and care
// until 16:30 on every weekday.
func amendmentRead() schoolRead {
	days := func(end string) map[string]string {
		return map[string]string{"mon": end, "tue": end, "wed": end, "thu": end, "fri": end}
	}
	short, long := 5000, 7000
	return schoolRead{offerings: map[int64]carecontract.Offering{
		1: {ID: 1, Name: "Kurzbetreuung", PriceCents: &short, EndTimes: days("14:00")},
		2: {ID: 2, Name: "Ganztag", PriceCents: &long, EndTimes: days("16:30")},
	}}
}

func date(year int, month time.Month, day int) *calendar.Date {
	value := calendar.NewDate(year, month, day)
	return &value
}

// amendable is an active contract whose only version books the short module
// on Monday from 1 August 2030.
func amendable() AmendableContract {
	return AmendableContract{
		ID: 9, Key: ContractKey{SchoolID: 4, RequestChildID: 5}, FirstFrom: *date(2030, 8, 1), NextNumber: 2,
		Latest: carecontract.Version{
			ID: 31, Number: 1, Status: carecontract.VersionEffective, EffectiveFrom: *date(2030, 8, 1),
			ChildFirstName: "Lina", ChildLastName: "Muster", SchoolName: "OGS Am Lindenhof",
			Services: []carecontract.Service{{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"}},
			Terms:    carecontract.TemplateInput{TermKind: carecontract.TermOpenEnded, NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd},
			Source:   carecontract.SourceConclusion,
		},
	}
}

// The parents asked to switch Monday to the long module and add Tuesday
// from 1 March; the school approved it, which ended the old booking and
// started the new one. The contract gets version 2 from exactly that day.
func TestAnApprovedChangeBecomesTheNextVersionFromItsEffectiveDay(t *testing.T) {
	t.Parallel()
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2031, 3, 1)},
		{CareOfferingID: 2, Days: []string{"mon", "tue"}, ValidFrom: date(2031, 3, 1)},
	}}
	version, reason := nextAmendment(amendmentRead(), application, amendable(), *date(2031, 1, 10))
	require.Empty(t, reason)
	require.NotNil(t, version)
	assert.Equal(t, *date(2031, 3, 1), version.EffectiveFrom)
	assert.Equal(t, 2, version.Number)
	assert.Equal(t, carecontract.VersionEffective, version.Status)
	assert.Equal(t, carecontract.SourceBookingChange, version.Source)
	assert.Equal(t, []carecontract.Service{
		{Weekday: "mon", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"},
		{Weekday: "tue", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"},
	}, version.Services)
	assert.Equal(t, "Lina", version.ChildFirstName, "the version keeps the child and the terms")
	assert.Equal(t, amendable().Latest.Terms, version.Terms)
}

// Invariant: a run that finds the bookings matching the version changes
// nothing, so repeated runs make no second version.
func TestMatchingBookingsMakeNoVersion(t *testing.T) {
	t.Parallel()
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1)},
	}}
	version, reason := nextAmendment(amendmentRead(), application, amendable(), *date(2031, 1, 10))
	assert.Nil(t, version)
	assert.Empty(t, reason)
}

// Invariant: no version reaches into the past. A change whose day passed
// before the run saw it takes effect today, never earlier.
func TestAChangeSeenLateTakesEffectTodayNotInThePast(t *testing.T) {
	t.Parallel()
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2031, 1, 6)},
		{CareOfferingID: 1, Days: []string{"mon", "wed"}, ValidFrom: date(2031, 1, 6)},
	}}
	today := *date(2031, 1, 10)
	version, reason := nextAmendment(amendmentRead(), application, amendable(), today)
	require.Empty(t, reason)
	require.NotNil(t, version)
	assert.Equal(t, today, version.EffectiveFrom)
}

// Invariant: a shorter module is never promised longer than it runs. The
// new module is booked until the end of June; the open contract carries
// that last day, and the services of a later Monday leave it out.
func TestAShorterModuleIsNeverPromisedBeyondItsBooking(t *testing.T) {
	t.Parallel()
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1)},
		{CareOfferingID: 2, Days: []string{"tue"}, ValidFrom: date(2031, 3, 1), ValidUntil: date(2031, 7, 1)},
	}}
	version, reason := nextAmendment(amendmentRead(), application, amendable(), *date(2031, 1, 10))
	require.Empty(t, reason)
	require.NotNil(t, version)
	assert.Nil(t, version.EndsOn, "the contract runs on with the Monday module")
	require.Len(t, version.Services, 2)
	require.NotNil(t, version.Services[1].LastDay)
	assert.Equal(t, *date(2031, 6, 30), *version.Services[1].LastDay)
	assert.Len(t, version.ServicesOn(*date(2031, 6, 24)), 1, "a Tuesday in June books the module")
	assert.Empty(t, version.ServicesOn(*date(2031, 7, 1)), "a Tuesday in July does not")

	// When the module ends, the bookings and the version agree again: the
	// day after its last day makes no further version.
	amended := amendable()
	amended.Latest = *version
	amended.Latest.ID, amended.NextNumber = 32, 3
	next, reason := nextAmendment(amendmentRead(), application, amended, *date(2031, 1, 10))
	assert.Nil(t, next)
	assert.Empty(t, reason)
}

// A child whose care stops is a termination (#4682), no empty version.
func TestCareThatStopsIsNoVersion(t *testing.T) {
	t.Parallel()
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2031, 3, 1)},
	}}
	version, reason := nextAmendment(amendmentRead(), application, amendable(), *date(2031, 1, 10))
	assert.Nil(t, version)
	assert.Equal(t, carecontract.SkipCareEnds, reason)
}

// Invariant: the timeline only grows at its end. A change that takes effect
// before a version that is still ahead is reported, not written.
func TestAChangeBeforeAPlannedVersionIsHeldBack(t *testing.T) {
	t.Parallel()
	planned := amendable()
	planned.Latest.EffectiveFrom = *date(2031, 3, 1)
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 2, Days: []string{"mon"}, ValidFrom: date(2031, 2, 1)},
	}}
	version, reason := nextAmendment(amendmentRead(), application, planned, *date(2031, 1, 10))
	assert.Nil(t, version)
	assert.Equal(t, carecontract.SkipBeforeLaterVersion, reason)
}

// The billing span of a version is its validity within the asked span.
func TestClipPeriodKeepsTheVersionInsideTheSpan(t *testing.T) {
	t.Parallel()
	until := *date(2031, 2, 28)
	from, to := clipPeriod(carecontract.Version{EffectiveFrom: *date(2030, 8, 1), ValidUntil: &until}, *date(2031, 2, 1), *date(2031, 3, 31))
	assert.Equal(t, *date(2031, 2, 1), from)
	assert.Equal(t, until, to)
	from, to = clipPeriod(carecontract.Version{EffectiveFrom: *date(2031, 3, 1)}, *date(2031, 2, 1), *date(2031, 3, 31))
	assert.Equal(t, *date(2031, 3, 1), from)
	assert.Equal(t, *date(2031, 3, 31), to)
}
