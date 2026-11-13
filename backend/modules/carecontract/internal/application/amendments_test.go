package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
	contract := AmendableContract{
		ID: 9, Key: ContractKey{SchoolID: 4, RequestChildID: 5}, FirstFrom: *date(2030, 8, 1), NextNumber: 2,
		Latest: carecontract.Version{
			ID: 31, Number: 1, Status: carecontract.VersionEffective, EffectiveFrom: *date(2030, 8, 1),
			ChildFirstName: "Lina", ChildLastName: "Muster", SchoolName: "OGS Am Lindenhof",
			Services: []carecontract.Service{{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"}},
			Terms:    carecontract.TemplateInput{TermKind: carecontract.TermOpenEnded, NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd},
			Source:   carecontract.SourceConclusion,
		},
	}
	contract.Versions = []carecontract.Version{contract.Latest}
	return contract
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
	version, reason := nextVersion(amendmentRead(), application, amendable(), *date(2031, 1, 10))
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
	version, reason := nextVersion(amendmentRead(), application, amendable(), *date(2031, 1, 10))
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
	version, reason := nextVersion(amendmentRead(), application, amendable(), today)
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
	version, reason := nextVersion(amendmentRead(), application, amendable(), *date(2031, 1, 10))
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
	first := amendable().Latest
	first.Status = carecontract.VersionSuperseded
	amended.Versions = []carecontract.Version{first, amended.Latest}
	next, reason := nextVersion(amendmentRead(), application, amended, *date(2031, 1, 10))
	assert.Nil(t, next)
	assert.Empty(t, reason)
}

// A child whose care stops is a termination (#4682), no empty version.
func TestCareThatStopsIsNoVersion(t *testing.T) {
	t.Parallel()
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2031, 3, 1)},
	}}
	version, reason := nextVersion(amendmentRead(), application, amendable(), *date(2031, 1, 10))
	assert.Nil(t, version)
	assert.Equal(t, carecontract.SkipCareEnds, reason)
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

// The first version of a contract ends the day before a module switch,
// because the booking of its module ends there (ADR 0153 point 3). The
// switch is no end of the contract: the next version carries the new module
// from the day after.
func TestAModuleSwitchAfterTheFirstVersionsEndContinuesTheContract(t *testing.T) {
	t.Parallel()
	contract := amendable()
	contract.Latest.EndsOn = date(2031, 2, 28)
	contract.Versions = []carecontract.Version{contract.Latest}
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2031, 3, 1)},
		{CareOfferingID: 2, Days: []string{"mon"}, ValidFrom: date(2031, 3, 1)},
	}}
	version, reason := nextVersion(amendmentRead(), application, contract, *date(2031, 1, 10))
	require.Empty(t, reason)
	require.NotNil(t, version)
	assert.Equal(t, *date(2031, 3, 1), version.EffectiveFrom)
	assert.Nil(t, version.EndsOn)

	// A fixed term ends the contract itself: a switch after it is no
	// version but the termination's business (#4682).
	six := 6
	contract.Latest.Terms = carecontract.TemplateInput{TermKind: carecontract.TermFixedMonths, TermMonths: &six,
		NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd}
	contract.Latest.EndsOn = date(2031, 1, 31)
	contract.Versions = []carecontract.Version{contract.Latest}
	version, reason = nextVersion(amendmentRead(), application, contract, *date(2031, 1, 10))
	assert.Nil(t, version)
	assert.Empty(t, reason)
}

// A module that ended before the run saw it leaves no boundary ahead and,
// without dates on the remaining booking, no other day to look at. Today is
// always looked at, so the contract still drops the module.
func TestAModuleThatEndedUnseenIsDroppedFromToday(t *testing.T) {
	t.Parallel()
	contract := amendable()
	contract.Latest.Services = append(contract.Latest.Services,
		carecontract.Service{Weekday: "tue", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"})
	contract.Versions = []carecontract.Version{contract.Latest}
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}},
		{CareOfferingID: 2, Days: []string{"tue"}, ValidUntil: date(2031, 1, 5)},
	}}
	today := *date(2031, 1, 10)
	version, reason := nextVersion(amendmentRead(), application, contract, today)
	require.Empty(t, reason)
	require.NotNil(t, version)
	assert.Equal(t, today, version.EffectiveFrom)
	assert.Equal(t, []carecontract.Service{{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"}},
		version.Services)
}

// Once a later version holds, a send again delivers the PDF the parents
// have, with its own version, not the newer version without a PDF.
func TestASendAgainAfterAnAmendmentDeliversTheSentPDF(t *testing.T) {
	t.Parallel()
	first := carecontract.Version{ID: 11, Number: 1, ChildFirstName: "Lina"}
	second := carecontract.Version{ID: 12, Number: 2, ChildFirstName: "Lina"}
	record := ContractRecord{Summary: carecontract.ContractSummary{ID: 7, Status: carecontract.StatusActive}, Version: second,
		SentDocument: &SentDocument{ID: 31, VersionID: 11}}
	sent := withSentVersion(record, []carecontract.Version{first, second})
	assert.Equal(t, 1, sent.Version.Number)
	require.NotNil(t, sent.DocumentID)
	assert.Equal(t, int64(31), *sent.DocumentID)
}

// plannedTimeline is the contract of amendable() with version 2 already
// planned from 1 March 2031: Monday stays short, Wednesday is added.
func plannedTimeline() AmendableContract {
	contract := amendable()
	first := contract.Latest
	first.Status = carecontract.VersionSuperseded
	march := carecontract.Version{ID: 32, Number: 2, Status: carecontract.VersionEffective, EffectiveFrom: *date(2031, 3, 1),
		ChildFirstName: "Lina", ChildLastName: "Muster", SchoolName: "OGS Am Lindenhof", Terms: first.Terms,
		Source: carecontract.SourceBookingChange, Services: []carecontract.Service{
			{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
			{Weekday: "wed", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
		}}
	contract.Latest, contract.NextNumber = march, 3
	contract.Versions = []carecontract.Version{first, march}
	return contract
}

// Invariant 1 (#4681, reproduced case): version 2 is planned from 1 March.
// The school approves a change from 1 February (Monday until 16:30). The
// change becomes version 3 from 1 February, superseded in the timeline by
// the planned version, which stays as it is. Before the fix the change was
// dropped, and February kept the old Monday.
func TestAChangeBeforeAPlannedVersionGoesInFrontOfIt(t *testing.T) {
	t.Parallel()
	// The first approval wrote Monday and Wednesday in the short module from
	// 1 March (planned version 2); the second moves Monday to the long
	// module from 1 February.
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2031, 2, 1)},
		{CareOfferingID: 1, Days: []string{"mon", "wed"}, ValidFrom: date(2031, 3, 1)},
		{CareOfferingID: 2, Days: []string{"mon"}, ValidFrom: date(2031, 2, 1)},
	}}
	contract := plannedTimeline()
	version, reason := nextVersion(amendmentRead(), application, contract, *date(2031, 1, 10))
	require.Empty(t, reason)
	require.NotNil(t, version)
	assert.Equal(t, *date(2031, 2, 1), version.EffectiveFrom)
	assert.Equal(t, 3, version.Number)
	assert.Equal(t, carecontract.VersionSuperseded, version.Status, "a planned version follows it")
	assert.Equal(t, []carecontract.Service{{Weekday: "mon", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"}},
		version.ServicesOn(*date(2031, 2, 3)), "Monday 3 February books the long module")

	// With version 3 in the timeline the run is done: the planned version
	// does not take the change over automatically.
	inserted := *version
	inserted.ID = 33
	contract.Versions = []carecontract.Version{contract.Versions[0], inserted, contract.Versions[1]}
	contract.NextNumber = 4
	next, reason := nextVersion(amendmentRead(), application, contract, *date(2031, 1, 10))
	assert.Nil(t, next, "the planned version stays as it is")
	assert.Empty(t, reason)
}

// nextVersion is the version of the next amendment plan, nil without one.
func nextVersion(read schoolRead, application carecontract.Application, contract AmendableContract,
	today calendar.Date) (*carecontract.Version, string) {
	plan, reason := nextAmendment(read, application, contract, today)
	if plan == nil {
		return nil, reason
	}
	return &plan.version, reason
}

// Rule 3 (#4681): an approval whose day is the first day of the planned
// version (D = E) replaces that version by one with its services plus the
// change, under a new number; nothing is dropped.
func TestAChangeOnThePlannedVersionsFirstDayReplacesIt(t *testing.T) {
	t.Parallel()
	contract := plannedTimeline()
	// The first approval added Wednesday from 1 March (planned version 2);
	// the second adds Friday in the long module from the same day.
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2031, 3, 1)},
		{CareOfferingID: 1, Days: []string{"mon", "wed"}, ValidFrom: date(2031, 3, 1)},
		{CareOfferingID: 2, Days: []string{"fri"}, ValidFrom: date(2031, 3, 1)},
	}}
	plan, reason := nextAmendment(amendmentRead(), application, contract, *date(2031, 1, 10))
	require.Empty(t, reason)
	require.NotNil(t, plan)
	require.NotNil(t, plan.replace, "the planned version is replaced, not kept beside it")
	assert.Equal(t, contract.Latest.ID, plan.replace.ID)
	assert.Equal(t, *date(2031, 3, 1), plan.version.EffectiveFrom)
	assert.Equal(t, 3, plan.version.Number)
	assert.Equal(t, carecontract.VersionEffective, plan.version.Status, "the newest stays wirksam")
	assert.Equal(t, []carecontract.Service{
		{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
		{Weekday: "wed", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
		{Weekday: "fri", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"},
	}, plan.version.Services, "the planned Monday and Wednesday plus the new Friday")
}

// Rule 3 (#4681): a second approval with the same D replaces the version of
// the first one, cumulatively.
func TestASecondChangeOnTheSameDayReplacesTheFirstCumulatively(t *testing.T) {
	t.Parallel()
	contract := amendable()
	first := contract.Latest
	first.Status = carecontract.VersionSuperseded
	february := carecontract.Version{ID: 33, Number: 2, Status: carecontract.VersionEffective, EffectiveFrom: *date(2031, 2, 1),
		ChildFirstName: "Lina", SchoolName: "OGS Am Lindenhof", Terms: first.Terms, Source: carecontract.SourceBookingChange,
		Services: []carecontract.Service{{Weekday: "mon", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"}}}
	contract.Latest, contract.NextNumber = february, 3
	contract.Versions = []carecontract.Version{first, february}
	// The first approval moved Monday to the long module from 1 February,
	// the second adds Thursday to it from the same day.
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2031, 2, 1)},
		{CareOfferingID: 2, Days: []string{"mon", "thu"}, ValidFrom: date(2031, 2, 1)},
	}}
	plan, reason := nextAmendment(amendmentRead(), application, contract, *date(2031, 1, 10))
	require.Empty(t, reason)
	require.NotNil(t, plan)
	require.NotNil(t, plan.replace)
	assert.Equal(t, int64(33), plan.replace.ID)
	assert.Equal(t, []carecontract.Service{
		{Weekday: "mon", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"},
		{Weekday: "thu", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"},
	}, plan.version.Services)
}

// Overdue changes hold together (#4681 design, quorum critical): two modules
// changed on different past days without a version since. Both reach today's
// version; before, the first made today's version and the second was skipped
// for good, because a version had started after it.
func TestOverdueChangesHoldTogetherFromToday(t *testing.T) {
	t.Parallel()
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2030, 12, 1)},
		{CareOfferingID: 1, Days: []string{"mon", "thu"}, ValidFrom: date(2030, 12, 1)},
		{CareOfferingID: 2, Days: []string{"tue"}, ValidFrom: date(2030, 11, 1)},
	}}
	today := *date(2031, 1, 10)
	version, reason := nextVersion(amendmentRead(), application, amendable(), today)
	require.Empty(t, reason)
	require.NotNil(t, version)
	assert.Equal(t, today, version.EffectiveFrom)
	assert.Equal(t, []carecontract.Service{
		{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
		{Weekday: "tue", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"},
		{Weekday: "thu", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
	}, version.Services, "both overdue changes")
}

// An overdue switch from one module to another a fortnight later is one
// change from today, not the end of care.
func TestAnOverdueSwitchIsNoEndOfCare(t *testing.T) {
	t.Parallel()
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2030, 12, 1)},
		{CareOfferingID: 2, Days: []string{"mon"}, ValidFrom: date(2030, 12, 15)},
	}}
	version, reason := nextVersion(amendmentRead(), application, amendable(), *date(2031, 1, 10))
	require.Empty(t, reason)
	require.NotNil(t, version)
	assert.Equal(t, []carecontract.Service{{Weekday: "mon", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"}},
		version.Services)
}

// A version never promises unbooked care (#5928), quorum critical: the
// planned version books module 1 from 1 March; the school replaced it by
// module 2 on the same day and deleted its booking. The replacement drops
// module 1.
func TestReplacingAPlannedModuleDropsTheDeletedBooking(t *testing.T) {
	t.Parallel()
	// Version 2 plans the long module (B) on Monday from 1 March. The school
	// replaces it by the short module (C) on Monday and Wednesday from the
	// same day and deletes B's booking.
	contract := plannedTimeline()
	contract.Latest.Services = []carecontract.Service{{Weekday: "mon", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"}}
	contract.Versions[1] = contract.Latest
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2031, 3, 1)},
		{CareOfferingID: 1, Days: []string{"mon", "wed"}, ValidFrom: date(2031, 3, 1)},
	}}
	plan, reason := nextAmendment(amendmentRead(), application, contract, *date(2031, 1, 10))
	require.Empty(t, reason)
	require.NotNil(t, plan)
	require.NotNil(t, plan.replace)
	assert.Equal(t, []carecontract.Service{
		{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
		{Weekday: "wed", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
	}, plan.version.Services, "module B is booked no more")
}

// A module booked longer than the version promised (quorum suggestion):
// version 1 promises Monday until 30 June, the school extends the booking to
// 30 September. From 1 July a version carries the module on.
func TestAnExtendedModuleIsCarriedOnAfterItsPromisedEnd(t *testing.T) {
	t.Parallel()
	contract := amendable()
	june := *date(2031, 6, 30)
	contract.Latest.Services = []carecontract.Service{
		{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00", LastDay: &june},
		{Weekday: "tue", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"},
	}
	contract.Versions = []carecontract.Version{contract.Latest}
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2031, 10, 1)},
		{CareOfferingID: 2, Days: []string{"tue"}, ValidFrom: date(2030, 8, 1)},
	}}
	version, reason := nextVersion(amendmentRead(), application, contract, *date(2031, 1, 10))
	require.Empty(t, reason)
	require.NotNil(t, version)
	assert.Equal(t, *date(2031, 7, 1), version.EffectiveFrom)
	assert.Len(t, version.ServicesOn(*date(2031, 7, 7)), 1, "a Monday in July books the module")
	assert.Len(t, version.ServicesOn(*date(2031, 9, 29)), 1, "a Monday in September too")
}

// A version never promises a cancelled weekday (quorum critical): version 2
// plans Monday and Wednesday in the short module from 1 March; an approved
// change from 1 February reduces the module to Monday. From 1 March the
// planned version is replaced without Wednesday.
func TestACancelledWeekdayIsNoLongerPromised(t *testing.T) {
	t.Parallel()
	contract := plannedTimeline()
	inserted := contract.Versions[0]
	inserted.ID, inserted.Number, inserted.EffectiveFrom = 34, 3, *date(2031, 2, 1)
	inserted.Status = carecontract.VersionSuperseded
	contract.Versions = []carecontract.Version{contract.Versions[0], inserted, contract.Versions[1]}
	contract.NextNumber = 4
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1)},
	}}
	plan, reason := nextAmendment(amendmentRead(), application, contract, *date(2031, 1, 10))
	require.Empty(t, reason)
	require.NotNil(t, plan)
	require.NotNil(t, plan.replace)
	assert.Equal(t, *date(2031, 3, 1), plan.version.EffectiveFrom)
	assert.Equal(t, []carecontract.Service{{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"}},
		plan.version.Services, "Wednesday is booked no more")
}

// Care that goes on in another module is no end of care (quorum critical):
// version 2 plans the short module from 1 March; an approved change from
// 1 February switched to the long module. From 1 March the planned version
// takes the continuing booking instead of promising the cancelled module.
func TestContinuingCareInAnotherModuleReplacesAnUnbookedPlan(t *testing.T) {
	t.Parallel()
	contract := plannedTimeline()
	inserted := contract.Versions[0]
	inserted.ID, inserted.Number, inserted.EffectiveFrom = 34, 3, *date(2031, 2, 1)
	inserted.Status = carecontract.VersionSuperseded
	inserted.Services = []carecontract.Service{{Weekday: "mon", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"}}
	contract.Versions = []carecontract.Version{contract.Versions[0], inserted, contract.Versions[1]}
	contract.NextNumber = 4
	application := carecontract.Application{RequestChildID: 5, Bookings: []carecontract.Booking{
		{CareOfferingID: 1, Days: []string{"mon"}, ValidFrom: date(2030, 8, 1), ValidUntil: date(2031, 2, 1)},
		{CareOfferingID: 2, Days: []string{"mon"}, ValidFrom: date(2031, 2, 1)},
	}}
	plan, reason := nextAmendment(amendmentRead(), application, contract, *date(2031, 1, 10))
	require.Empty(t, reason, "care goes on, it does not end")
	require.NotNil(t, plan)
	require.NotNil(t, plan.replace)
	assert.Equal(t, []carecontract.Service{{Weekday: "mon", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"}},
		plan.version.Services)
}
