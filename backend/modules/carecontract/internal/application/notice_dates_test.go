package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The notice dates follow the terms frozen in the version (#4682): the
// first allowed end that leaves the notice period, never earlier, and a
// contract that ends earlier on its own ends then.
func TestNoticeDatesFollowTheFrozenTerms(t *testing.T) {
	t.Parallel()
	ownEnd := calendar.NewDate(2027, 1, 15)
	cases := []struct {
		name     string
		terms    carecontract.TemplateInput
		received calendar.Date
		end      *calendar.Date
		ordinary calendar.Date
		special  *calendar.Date
	}{
		{"one month to the month end", carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2026, 12, 31), nil},
		{"received on the last day of a month", carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd},
			calendar.NewDate(2026, 11, 30), nil, calendar.NewDate(2026, 12, 31), nil},
		{"31 January plus one month stays in February", carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd},
			calendar.NewDate(2027, 1, 31), nil, calendar.NewDate(2027, 2, 28), nil},
		{"without notice to this month's end", carecontract.TemplateInput{NoticeMonths: 0, NoticeTo: carecontract.NoticeMonthEnd},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2026, 11, 30), nil},
		{"three months to the half year", carecontract.TemplateInput{NoticeMonths: 3, NoticeTo: carecontract.NoticeHalfYear},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2027, 7, 31), nil},
		{"to the end of the school year", carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeSchoolYearEnd},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2027, 7, 31), nil},
		{"the special termination ends earlier at the half year",
			carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeSchoolYearEnd, SpecialTermination: true},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2027, 7, 31), new(calendar.NewDate(2027, 1, 31))},
		{"a special termination no earlier is not offered",
			carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd, SpecialTermination: true},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2026, 12, 31), nil},
		{"a contract that ends earlier ends then", carecontract.TemplateInput{NoticeMonths: 3, NoticeTo: carecontract.NoticeHalfYear},
			calendar.NewDate(2026, 11, 7), &ownEnd, ownEnd, nil},
	}
	for _, tc := range cases {
		ordinary, special := noticeDates(tc.terms, tc.received, tc.end)
		assert.Equal(t, tc.ordinary, ordinary, tc.name)
		assert.Equal(t, tc.special, special, tc.name)
	}
}

// The withdrawal period runs 14 days from the conclusion when the PDF
// carried the notice, else 12 months and 14 days; a paper contract from
// before has none (§ 355, § 356 Abs. 3 BGB).
func TestWithdrawalPeriodFollowsTheNotice(t *testing.T) {
	t.Parallel()
	concludedAt := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	informed := concludedAt
	electronic := &carecontract.Concluded{Method: carecontract.MethodWithApplication, At: concludedAt}
	paper := &carecontract.Concluded{Method: carecontract.MethodPaper, At: concludedAt}

	assert.Equal(t, new(calendar.NewDate(2026, 11, 20)), withdrawalUntil(electronic, &informed, nil))
	assert.Equal(t, new(calendar.NewDate(2026, 11, 20)), withdrawalUntil(paper, &informed, nil), "the PDF granted it")
	assert.Equal(t, new(calendar.NewDate(2027, 11, 22)), withdrawalUntil(electronic, nil, nil),
		"without the notice: 20 November 2027 is a Saturday, the period ends on Monday (§ 193 BGB)")
	assert.Nil(t, withdrawalUntil(paper, nil, nil), "signed at the office before the notice")
	assert.Nil(t, withdrawalUntil(nil, nil, nil), "no conclusion, no period")

	saturday := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	assert.Equal(t, new(calendar.NewDate(2026, 11, 23)), withdrawalUntil(
		&carecontract.Concluded{Method: carecontract.MethodWithApplication, At: saturday}, &saturday, nil),
		"21 November 2026 is a Saturday: the period ends on Monday 23 November (§ 193 BGB)")
	christmas := time.Date(2026, 12, 11, 9, 0, 0, 0, time.UTC)
	assert.Equal(t, new(calendar.NewDate(2026, 12, 28)), withdrawalUntil(
		&carecontract.Concluded{Method: carecontract.MethodWithApplication, At: christmas}, &christmas,
		map[calendar.Date]bool{calendar.NewDate(2026, 12, 25): true, calendar.NewDate(2026, 12, 26): true}),
		"25 December (holiday), 26 December (holiday) and Sunday 27 December move it to Monday 28 December")
	late := time.Date(2027, 11, 20, 9, 0, 0, 0, time.UTC)
	assert.Equal(t, new(calendar.NewDate(2027, 11, 22)), withdrawalUntil(electronic, &late, nil),
		"a notice provided late never reaches beyond 12 months and 14 days after the conclusion")
}

// The confirmation fills only the Träger's placeholders and refuses an
// empty template instead of falling back to a built-in text.
func TestConfirmationTextFillsTheTemplate(t *testing.T) {
	t.Parallel()
	endsOn := calendar.NewDate(2026, 12, 31)
	text, err := confirmationText("Kündigung für {Kind} ({Module}, {Schule}) am {Eingang}, Ende {Vertragsende}. {Träger}",
		confirmationValues{Child: "Lina Muster", Modules: "Ganztag", School: "OGS Am Lindenhof", EndsOn: &endsOn,
			Received: "07.11.2026", Traeger: "Ganztag im Quartier e.V."})
	assert.NoError(t, err)
	assert.Equal(t, "Kündigung für Lina Muster (Ganztag, OGS Am Lindenhof) am 07.11.2026, Ende 31.12.2026. Ganztag im Quartier e.V.", text)

	_, err = confirmationText("  ", confirmationValues{})
	assert.ErrorIs(t, err, carecontract.ErrNoticeTemplateMissing)
}

// § 312k Abs. 4 BGB: the confirmation of a declaration with the button names
// day and time of its receipt; a letter the office records has its day.
// A template that leaves out {Eingang}, or {Grund} for an extraordinary
// termination, still names them (#4682).
func TestConfirmationNamesTheReceiptWithItsTime(t *testing.T) {
	t.Parallel()
	declared := time.Date(2026, 11, 7, 8, 30, 0, 0, time.UTC) // 09:30 in Berlin
	parents := ports.NewNotice{Channel: carecontract.NoticeChannelParents, DeclaredAt: declared, ReceivedOn: calendar.NewDate(2026, 11, 7)}
	assert.Equal(t, "07.11.2026 um 09:30 Uhr", receipt(parents))
	office := ports.NewNotice{Channel: carecontract.NoticeChannelOffice, DeclaredAt: declared, ReceivedOn: calendar.NewDate(2026, 11, 5)}
	assert.Equal(t, "05.11.2026", receipt(office), "a letter has no time of receipt")

	text, err := confirmationText("Ihre Kündigung für {Kind}.", confirmationValues{Child: "Lina", Received: receipt(parents),
		Reason: "Wir ziehen um.", NeedsReason: true})
	assert.NoError(t, err)
	assert.Equal(t, "Ihre Kündigung für Lina.\n\nEingang Ihrer Erklärung: 07.11.2026 um 09:30 Uhr\n\nIhr Grund: Wir ziehen um.", text)
	text, err = confirmationText("Eingang {Eingang}. Grund: {Grund}", confirmationValues{Received: receipt(parents),
		Reason: "Umzug", NeedsReason: true})
	assert.NoError(t, err)
	assert.Equal(t, "Eingang 07.11.2026 um 09:30 Uhr. Grund: Umzug", text, "placed placeholders are not repeated")
	assert.Equal(t, carecontract.TerminationExtraordinary,
		templateKind(ports.NewNotice{Kind: carecontract.NoticeTermination, TerminationKind: carecontract.TerminationExtraordinary}))
	assert.Equal(t, carecontract.NoticeTermination,
		templateKind(ports.NewNotice{Kind: carecontract.NoticeTermination, TerminationKind: carecontract.TerminationOrdinary}))
}

// Statutory holidays of two federal states in 2026 as School Calendar
// computes them: Fronleichnam (4 June) is one in Nordrhein-Westfalen, not
// in Berlin; Karfreitag, Ostermontag and Christi Himmelfahrt are in both.
var (
	nrw2026 = map[calendar.Date]bool{calendar.NewDate(2026, 4, 3): true, calendar.NewDate(2026, 4, 6): true,
		calendar.NewDate(2026, 5, 14): true, calendar.NewDate(2026, 6, 4): true}
	berlin2026 = map[calendar.Date]bool{calendar.NewDate(2026, 4, 3): true, calendar.NewDate(2026, 4, 6): true,
		calendar.NewDate(2026, 5, 14): true}
)

const (
	schoolNRW    int64 = 11
	schoolBerlin int64 = 12
)

// § 193 BGB moves the last day of the withdrawal period past the statutory
// holidays at the contract's school, read through School Calendar (#6350).
// Each case starts the 14 days on a fixed day.
func TestWithdrawalPeriodMovesPastTheHolidaysAtTheSchool(t *testing.T) {
	t.Parallel()
	periods := withdrawalPeriods{holidays: schoolHolidays{days: map[int64]map[calendar.Date]bool{schoolNRW: nrw2026, schoolBerlin: berlin2026}}}
	cases := []struct {
		name   string
		school int64
		start  calendar.Date
		want   calendar.Date
	}{
		{"14 days end on Fronleichnam, a holiday in Nordrhein-Westfalen: Friday", schoolNRW,
			calendar.NewDate(2026, 5, 21), calendar.NewDate(2026, 6, 5)},
		{"the same days at a school in Berlin, where Fronleichnam is a working day", schoolBerlin,
			calendar.NewDate(2026, 5, 21), calendar.NewDate(2026, 6, 4)},
		{"Karfreitag, Saturday, Sunday and Ostermontag in a row: Tuesday", schoolNRW,
			calendar.NewDate(2026, 3, 20), calendar.NewDate(2026, 4, 7)},
		{"Christi Himmelfahrt, a nationwide holiday: Friday", schoolBerlin,
			calendar.NewDate(2026, 4, 30), calendar.NewDate(2026, 5, 15)},
		{"a Saturday without a holiday: Monday", schoolNRW,
			calendar.NewDate(2026, 11, 7), calendar.NewDate(2026, 11, 23)},
	}
	for _, tc := range cases {
		at := tc.start.BerlinMidnight().Add(9 * time.Hour)
		concluded := &carecontract.Concluded{Method: carecontract.MethodElectronic, At: at}
		until, err := periods.until(t.Context(), tc.school, concluded, &at)
		require.NoError(t, err, tc.name)
		assert.Equal(t, new(tc.want), until, tc.name)
		end, err := periods.withdrawalEnd(t.Context(), tc.school, at)
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.want, end, "the signature names the same day: "+tc.name)
	}
}

// One read dates the periods of contracts at several schools, each over its
// own school's holidays, and spans every day a period may move to.
func TestWithdrawalPeriodsReadTheHolidaysOfEachSchoolOnce(t *testing.T) {
	t.Parallel()
	var reads []holidayRead
	periods := withdrawalPeriods{holidays: schoolHolidays{
		days: map[int64]map[calendar.Date]bool{schoolNRW: nrw2026, schoolBerlin: berlin2026}, reads: &reads}}
	at := calendar.NewDate(2026, 5, 21).BerlinMidnight().Add(9 * time.Hour)
	concluded := &carecontract.Concluded{Method: carecontract.MethodElectronic, At: at}
	paper := &carecontract.Concluded{Method: carecontract.MethodPaper, At: at}
	until, err := periods.untilEach(t.Context(), []withdrawalSubject{
		{schoolID: schoolNRW, concluded: concluded, informedAt: &at},
		{schoolID: schoolBerlin, concluded: concluded, informedAt: &at},
		{schoolID: 99, concluded: paper},
		{},
	})
	require.NoError(t, err)
	assert.Equal(t, []*calendar.Date{new(calendar.NewDate(2026, 6, 5)), new(calendar.NewDate(2026, 6, 4)), nil, nil}, until)
	assert.Equal(t, []holidayRead{{schoolIDs: []int64{schoolNRW, schoolBerlin}, from: calendar.NewDate(2026, 5, 21),
		to: calendar.NewDate(2027, 6, 11)}}, reads,
		"one read of the schools with a period, from the conclusion to a week after 12 months and 14 days")

	reads = nil
	until, err = periods.untilEach(t.Context(), []withdrawalSubject{{schoolID: 99, concluded: paper}})
	require.NoError(t, err)
	assert.Equal(t, []*calendar.Date{nil}, until)
	assert.Empty(t, reads, "no period, no read")
}

// A period the holidays cannot date is an error, never a period that may
// end a day too early (#6350).
func TestWithdrawalPeriodFailsWithoutTheSchoolHolidays(t *testing.T) {
	t.Parallel()
	at := calendar.NewDate(2026, 5, 21).BerlinMidnight()
	concluded := &carecontract.Concluded{Method: carecontract.MethodElectronic, At: at}
	unavailable := errors.New("federal state is not available")

	_, err := withdrawalPeriods{holidays: schoolHolidays{failure: unavailable}}.until(t.Context(), schoolNRW, concluded, &at)
	assert.ErrorIs(t, err, unavailable)
	_, err = withdrawalPeriods{holidays: schoolHolidays{only: true, days: map[int64]map[calendar.Date]bool{schoolBerlin: berlin2026}}}.
		until(t.Context(), schoolNRW, concluded, &at)
	assert.ErrorContains(t, err, "no holidays for school 11", "a school the read leaves out")
	_, err = withdrawalPeriods{}.until(t.Context(), schoolNRW, concluded, &at)
	assert.ErrorContains(t, err, "the school holidays are not configured")
}

// § 193 BGB moves the last day of a period to declare something, not the
// end of a contract: an ordinary termination still ends the contract on a
// Saturday or a holiday the terms name (#6350).
func TestContractEndsDoNotMoveToAWorkingDay(t *testing.T) {
	t.Parallel()
	monthEnd := carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd}
	ordinary, _ := noticeDates(monthEnd, calendar.NewDate(2026, 9, 7), nil)
	assert.Equal(t, calendar.NewDate(2026, 10, 31), ordinary, "31 October 2026 is a Saturday and Reformationstag in several states")
	ownEnd := calendar.NewDate(2026, 12, 26)
	ordinary, _ = noticeDates(monthEnd, calendar.NewDate(2026, 11, 2), &ownEnd)
	assert.Equal(t, ownEnd, ordinary, "a fixed term ending on the second Christmas holiday ends then")
}

// schoolHolidays is the School Calendar port of a test: the holidays of
// the schools in days, none for every other school unless only is set, an
// injected failure, and the reads it answered when reads is set.
type schoolHolidays struct {
	days    map[int64]map[calendar.Date]bool
	only    bool
	failure error
	reads   *[]holidayRead
}

type holidayRead struct {
	schoolIDs []int64
	from, to  calendar.Date
}

func (h schoolHolidays) SchoolHolidayDates(_ context.Context, schoolIDs []int64, from, to calendar.Date) (map[int64]map[calendar.Date]bool, error) {
	if h.failure != nil {
		return nil, h.failure
	}
	if h.reads != nil {
		*h.reads = append(*h.reads, holidayRead{schoolIDs: schoolIDs, from: from, to: to})
	}
	result := make(map[int64]map[calendar.Date]bool, len(schoolIDs))
	for _, schoolID := range schoolIDs {
		if days, ok := h.days[schoolID]; ok {
			result[schoolID] = days
		} else if !h.only {
			result[schoolID] = map[calendar.Date]bool{}
		}
	}
	return result, nil
}

// The worker's mail carries the confirmation as stored, paragraph by
// paragraph, and a subject without the child's name.
func TestRenderNoticeMailKeepsTheConfirmation(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"kind":"widerruf","traeger_name":"Ganztag e.V.","guardian_name":"Erika Muster",` +
		`"confirmation_text":"wir bestätigen Ihren Widerruf.\n\nMit freundlichen Grüßen\nGanztag e.V."}`)
	mail, err := RenderNoticeMail(t.Context(), payload)
	assert.NoError(t, err)
	assert.Equal(t, "Bestätigung Ihres Widerrufs", mail.Subject)
	assert.True(t, mail.Withdrawal)
	assert.Equal(t, []string{"wir bestätigen Ihren Widerruf.", "Mit freundlichen Grüßen\nGanztag e.V."}, mail.Paragraphs)

	_, err = RenderNoticeMail(t.Context(), []byte(`{"kind":"kuendigung","confirmation_text":" "}`))
	assert.Error(t, err, "a confirmation without text is never sent")
}
